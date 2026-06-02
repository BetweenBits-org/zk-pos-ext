// Package verifier is the zkpor proof-of-solvency verifier engine,
// exposed as a Go library so other in-process clients can drive
// verification without re-implementing the CLI wiring.
//
// Three modes are supported:
//
//	RunBatch  verify every proof in the proof table chains correctly and
//	          the final CEX commitment matches the published totals.
//	RunUser   single-user inclusion verification against a user-config
//	          JSON artifact.
//	RunHash   helper that prints Poseidon(A, B) for two base64 inputs.
//
// The library is dispatched on profile.Model (T1..T4); the per-model
// runners live in core/solvency/<model>/host/verifier_runner.go.
//
// R12-B contract: every exported entry point returns error. In-process
// callers can drive the verifier without recover() and propagate the
// error up. The cmd/verifier shim is the only layer that converts
// errors into exit codes.
//
// R12-C contract: RunBatch and RunUser take a context.Context. RunBatch
// threads it into the proof worker pool, so a cancellation aborts the
// in-flight verification (the first worker to observe ctx records its
// error and the pool unwinds). RunUser checks ctx at entry — user-mode
// inclusion is a single fast recompute, not worth threading deeper.
// RunHash is a pure, instant Poseidon helper and keeps its ctx-free
// signature. Verification is a one-shot job, so cmd/verifier treats any
// error (including context.Canceled) as exit 1.
package verifier

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
	"github.com/BetweenBits-org/zk-pos-ext/core/io/vfs"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	vconfig "github.com/BetweenBits-org/zk-pos-ext/pkg/verifier/config"
	"github.com/BetweenBits-org/zk-pos-ext/profile/declarative"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// Options bundles the inputs every Run* entry point needs. R12-EF
// inverted the IO: cmd/verifier builds every value/opener/port and
// injects them here, so the engine never touches os/path or store. The
// fields not relevant to a given mode are ignored:
//
//   - RunBatch consumes Profile, Keys, CapacityOverride, Config, and either
//     Proofs (DB path) or ProofCSV (legacy CSV path).
//   - RunUser  consumes Profile, Keys, CapacityOverride, UserConfig.
//   - RunHash  consumes nothing (its two base64 args are passed directly).
//
// CapacityOverride > 0 supersedes profile.asset_capacity; this is the
// smoke-harness escape hatch and must stay zero in production callers.
type Options struct {
	// Profile is the parsed declarative profile. Required for RunBatch
	// and RunUser; cmd/verifier loads it via declarative.Load.
	Profile *declarative.Profile

	// Keys opens the .vk verifying-key artifacts the keygen service
	// wrote. Required for RunBatch (the .vk files drive groth16.Verify).
	// cmd/verifier wraps the keys directory via osvfs.KeyDir.
	Keys vfs.KeyOpener

	// CapacityOverride supersedes profile.asset_capacity when > 0.
	// Smoke-harness use only.
	CapacityOverride int

	// Config is the parsed verifier deployment config (DB DSN,
	// CexAssetsInfo, proof-table CSV path). Required for RunBatch;
	// cmd/verifier parses it via vconfig.Parse.
	Config *vconfig.Config

	// Proofs is the injected proof-store port. Used by RunBatch when
	// Config.MysqlDataSource is set; cmd/verifier wires the MySQL adapter.
	Proofs corehost.ProofStore

	// ProofCSV reads the legacy / smoke proof CSV. Used by RunBatch when
	// Config.MysqlDataSource is empty; cmd/verifier wraps cfg.ProofTable
	// via osvfs.File so the engine reads no path itself.
	ProofCSV vfs.ByteSource

	// UserConfig reads the per-user inclusion-proof artifact the
	// userproof service emitted. Required for RunUser; cmd/verifier wraps
	// the path via osvfs.File.
	UserConfig vfs.ByteSource
}

// resolved bundles the derived plan plus the chosen model for dispatch.
type resolved struct {
	model corespec.SolvencyModelID
	plan  *corehost.VerifierPlan
}

// emptyAccountTreeRootHex is the root of a fully empty depth-28 sparse
// Merkle account tree (every leaf the empty-leaf hash). The first
// batch's before-account-root must equal this. Pinned by the engine
// standard (corespec.AccountTreeDepth); mirrors the legacy verifier
// constant.
const emptyAccountTreeRootHex = "08696bfcb563a2ee4dde9e1dbd34f68d3f4643df6e3709cdb1855c9f886240c7"

// RunBatch executes the batch-verification mode: every proof in the
// proof table must groth16-verify, consecutive batches must chain
// (batch i's after-state == batch i+1's before-state), the first batch
// must start from the empty account-tree root, and the final CEX
// commitment must equal the commitment of the published totals.
//
// Returns an error describing the first verification failure
// encountered; nil on success. A cancelled ctx aborts the proof worker
// pool and surfaces as a (wrapped) context error.
func RunBatch(ctx context.Context, opts Options) error {
	r, err := resolveFromProfile(opts)
	if err != nil {
		return err
	}
	if opts.Keys == nil {
		return fmt.Errorf("verifier: Keys is required (vfs.KeyOpener over the keygen .artifacts/)")
	}
	if opts.Config == nil {
		return fmt.Errorf("verifier: Config is required")
	}
	verifierConfig := opts.Config

	proofs, err := loadProofs(ctx, verifierConfig, opts.Proofs, opts.ProofCSV)
	if err != nil {
		return fmt.Errorf("verifier: load proofs: %w", err)
	}

	emptyAccountTreeRoot, err := hex.DecodeString(emptyAccountTreeRootHex)
	if err != nil {
		return fmt.Errorf("verifier: decode empty account tree root: %w", err)
	}

	if r.plan.AssetCapacity <= 0 {
		return fmt.Errorf("verifier: profile.asset_capacity must be > 0")
	}
	emptyCexAssetListCommitment, expectFinalCexAssetsInfoComm, err := dispatchBuildCexCommitments(
		r.model, verifierConfig.CexAssetsInfo, r.plan.AssetCapacity,
	)
	if err != nil {
		return fmt.Errorf("verifier: build cex commitments: %w", err)
	}

	prevCexAssetListCommitments := make([][]byte, 2)
	prevAccountTreeRoots := make([][]byte, 2)
	prevAccountTreeRoots[1] = emptyAccountTreeRoot
	prevCexAssetListCommitments[1] = emptyCexAssetListCommitment

	if err := verifyAllProofs(ctx, proofs, r, opts.Keys); err != nil {
		return fmt.Errorf("verifier: verify proofs: %w", err)
	}

	if err := chainCheck(proofs, prevAccountTreeRoots, prevCexAssetListCommitments, expectFinalCexAssetsInfoComm); err != nil {
		return fmt.Errorf("verifier: chain check: %w", err)
	}
	return nil
}

// RunUser dispatches to the model's user-inclusion runner. Returns an
// error on failure. ctx is checked at entry; user-mode inclusion is a
// single fast recompute, so it is not threaded deeper.
func RunUser(ctx context.Context, opts Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r, err := resolveFromProfile(opts)
	if err != nil {
		return err
	}
	if opts.UserConfig == nil {
		return fmt.Errorf("verifier: UserConfig is required (vfs.ByteSource over the user-config artifact)")
	}
	userConfigBytes, err := opts.UserConfig.ReadAll(ctx)
	if err != nil {
		return fmt.Errorf("verifier: read user config: %w", err)
	}
	if err := dispatchVerifyUserInclusion(r.model, r.plan, userConfigBytes); err != nil {
		return fmt.Errorf("verifier: verify user inclusion: %w", err)
	}
	return nil
}

// RunHash prints Poseidon(arg0, arg1) for two base64-encoded inputs.
// Model-blind. Returns an error if either argument is not valid base64.
func RunHash(arg0, arg1 string) error {
	hasher := poseidon.NewPoseidon()
	p0, err := base64.StdEncoding.DecodeString(arg0)
	if err != nil {
		return fmt.Errorf("verifier: hash arg0 is not base64-encoded: %w", err)
	}
	p1, err := base64.StdEncoding.DecodeString(arg1)
	if err != nil {
		return fmt.Errorf("verifier: hash arg1 is not base64-encoded: %w", err)
	}
	hasher.Write(p0)
	hasher.Write(p1)
	res := hasher.Sum(nil)
	fmt.Printf("hash result base64 encode: %s\n", base64.StdEncoding.EncodeToString(res))
	fmt.Printf("hash result hex encode: %x\n", res)
	return nil
}

// resolveFromProfile derives the (model, capacity, tiers, stems) plan
// from the injected profile. Used by both batch + user modes.
// CapacityOverride supersedes profile.asset_capacity when > 0.
//
// R12-EF: ZkKeyStems are LOGICAL identifiers (provider.KeyName output
// only); the directory join moved into the injected vfs.KeyOpener
// (osvfs.KeyDir.Open joins dir + stem + ext). The plan no longer
// path-prefixes the stems.
func resolveFromProfile(opts Options) (*resolved, error) {
	if opts.Profile == nil {
		return nil, fmt.Errorf("verifier: Profile is required")
	}
	prof := opts.Profile
	model := corespec.SolvencyModelID(prof.Profile.Model)
	provider, err := declarative.BuildBatchShapeProvider(model, prof.BatchShapes)
	if err != nil {
		return nil, err
	}
	shapes := provider.Shapes()
	plan := &corehost.VerifierPlan{
		AssetCapacity:   prof.Profile.AssetCapacity,
		AssetCountTiers: make([]int, len(shapes)),
		ZkKeyStems:      make([]string, len(shapes)),
	}
	if opts.CapacityOverride > 0 {
		plan.AssetCapacity = opts.CapacityOverride
	}
	for i, s := range shapes {
		plan.AssetCountTiers[i] = s.AssetCountTier
		plan.ZkKeyStems[i] = provider.KeyName(s, prof.Constraint.Module)
	}
	return &resolved{model: model, plan: plan}, nil
}
