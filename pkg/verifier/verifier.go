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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	vconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/verifier/config"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// Options bundles the inputs every Run* entry point needs. Fields not
// relevant to a given mode are ignored:
//
//   - RunBatch consumes ProfilePath, KeysDir, CapacityOverride, ConfigPath.
//   - RunUser  consumes ProfilePath, KeysDir, CapacityOverride, UserConfigPath.
//   - RunHash  consumes nothing (its two base64 args are passed directly).
//
// CapacityOverride > 0 supersedes profile.asset_capacity; this is the
// smoke-harness escape hatch and must stay zero in production callers.
type Options struct {
	// ProfilePath points at the declarative profile.toml. Required for
	// RunBatch and RunUser.
	ProfilePath string

	// KeysDir is the directory holding the .vk verifying-key artifacts
	// the keygen service wrote. Required for RunBatch (the .vk files
	// drive groth16.Verify) and for RunUser when the runner needs to
	// resolve a per-tier stem.
	KeysDir string

	// CapacityOverride supersedes profile.asset_capacity when > 0.
	// Smoke-harness use only.
	CapacityOverride int

	// ConfigPath points at the verifier's deployment config JSON
	// (DB DSN, CexAssetsInfo, proof-table CSV path). Defaults to
	// "config/config.json" when empty. RunBatch only.
	ConfigPath string

	// UserConfigPath points at the per-user inclusion-proof artifact
	// the userproof service emitted. Defaults to
	// "config/user_config.json" when empty. RunUser only.
	UserConfigPath string
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
	if opts.KeysDir == "" {
		return fmt.Errorf("verifier: KeysDir is required (path to keygen .artifacts/)")
	}
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = "config/config.json"
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("verifier: read config %q: %w", configPath, err)
	}
	verifierConfig := &vconfig.Config{}
	if err := json.Unmarshal(content, verifierConfig); err != nil {
		return fmt.Errorf("verifier: parse config %q: %w", configPath, err)
	}

	proofs, err := loadProofs(verifierConfig)
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

	if err := verifyAllProofs(ctx, proofs, r); err != nil {
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
	userConfigPath := opts.UserConfigPath
	if userConfigPath == "" {
		userConfigPath = "config/user_config.json"
	}
	if err := dispatchVerifyUserInclusion(r.model, r.plan, userConfigPath); err != nil {
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

// resolveFromProfile loads profile.toml + derives the (model, capacity,
// tiers, stems) plan. Used by both batch + user modes. CapacityOverride
// supersedes profile.asset_capacity when > 0.
func resolveFromProfile(opts Options) (*resolved, error) {
	if opts.ProfilePath == "" {
		return nil, fmt.Errorf("verifier: ProfilePath is required (path to profile.toml)")
	}
	prof, err := declarative.Load(opts.ProfilePath)
	if err != nil {
		return nil, err
	}
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
		plan.ZkKeyStems[i] = filepath.Join(opts.KeysDir, provider.KeyName(s, prof.Constraint.Module))
	}
	return &resolved{model: model, plan: plan}, nil
}
