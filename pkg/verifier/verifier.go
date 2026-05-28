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
// R12-A library extraction: this code previously lived in
// cmd/verifier/main.go as package main. The orchestration body moved
// here unchanged (Conservative slice). cmd/verifier is now a thin shim
// that parses flags and calls into this package. Failure semantics
// remain panic-on-error for this slice; a future slice migrates to
// returned errors.
package verifier

import (
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
// Panics on any verification failure; v0 reference behaviour.
func RunBatch(opts Options) {
	r, err := resolveFromProfile(opts)
	if err != nil {
		panic(err.Error())
	}
	if opts.KeysDir == "" {
		panic("KeysDir is required (path to keygen .artifacts/)")
	}
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = "config/config.json"
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		panic(err.Error())
	}
	verifierConfig := &vconfig.Config{}
	if err := json.Unmarshal(content, verifierConfig); err != nil {
		panic(err.Error())
	}

	proofs, err := loadProofs(verifierConfig)
	if err != nil {
		panic(err.Error())
	}

	emptyAccountTreeRoot, err := hex.DecodeString(emptyAccountTreeRootHex)
	if err != nil {
		panic("wrong empty account tree root")
	}

	if r.plan.AssetCapacity <= 0 {
		panic("verifier: profile.asset_capacity must be > 0")
	}
	emptyCexAssetListCommitment, expectFinalCexAssetsInfoComm, err := dispatchBuildCexCommitments(
		r.model, verifierConfig.CexAssetsInfo, r.plan.AssetCapacity,
	)
	if err != nil {
		panic(err.Error())
	}

	prevCexAssetListCommitments := make([][]byte, 2)
	prevAccountTreeRoots := make([][]byte, 2)
	prevAccountTreeRoots[1] = emptyAccountTreeRoot
	prevCexAssetListCommitments[1] = emptyCexAssetListCommitment

	if !verifyAllProofs(proofs, r) {
		os.Exit(1)
	}

	chainCheck(proofs, prevAccountTreeRoots, prevCexAssetListCommitments, expectFinalCexAssetsInfoComm)
}

// RunUser dispatches to the model's user-inclusion runner. Panics on
// failure; v0 reference behaviour.
func RunUser(opts Options) {
	r, err := resolveFromProfile(opts)
	if err != nil {
		panic(err.Error())
	}
	userConfigPath := opts.UserConfigPath
	if userConfigPath == "" {
		userConfigPath = "config/user_config.json"
	}
	if err := dispatchVerifyUserInclusion(r.model, r.plan, userConfigPath); err != nil {
		panic(err.Error())
	}
}

// RunHash prints Poseidon(arg0, arg1) for two base64-encoded inputs.
// Model-blind. Panics on bad input.
func RunHash(arg0, arg1 string) {
	hasher := poseidon.NewPoseidon()
	p0, err := base64.StdEncoding.DecodeString(arg0)
	if err != nil {
		panic("invalid hash command, the first argument is not base64 encoded")
	}
	p1, err := base64.StdEncoding.DecodeString(arg1)
	if err != nil {
		panic("invalid hash command, the second argument is not base64 encoded")
	}
	hasher.Write(p0)
	hasher.Write(p1)
	res := hasher.Sum(nil)
	fmt.Printf("hash result base64 encode: %s\n", base64.StdEncoding.EncodeToString(res))
	fmt.Printf("hash result hex encode: %x\n", res)
}

// resolveFromProfile loads profile.toml + derives the (model, capacity,
// tiers, stems) plan. Used by both batch + user modes. CapacityOverride
// supersedes profile.asset_capacity when > 0.
func resolveFromProfile(opts Options) (*resolved, error) {
	if opts.ProfilePath == "" {
		return nil, fmt.Errorf("ProfilePath is required (path to profile.toml)")
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
