// Package userproof is the zkpor-native per-user inclusion-proof
// builder engine. It reads the customer snapshot, rebuilds the
// depth-28 account SMT (matching the witness service's tree state via
// the same padding semantics), and writes one UserProof row per real
// account — embedded UserConfig payload lets the verifier user-mode
// recompute and check inclusion locally.
//
// Phase 3e (R10+1) swap: every model-typed loop (streamAndBucket,
// populateTree, writeUserProofs, buildUserProofRow) is pulled into
// model-specific runner packages at
// core/solvency/<model>/host/userproof_runner.go. This package is a
// thin wiring layer — load profile.toml, build shared dependencies,
// switch on profile.Model, and delegate to the matching runner.
//
// Self-contained tree build: the userproof engine does NOT depend on
// the witness service's persisted tree state. Same snapshot + same
// padding rules = same tree leaves = same root, so per-user proofs
// verify against the same root the witness/prover published.
//
// R12-E contract: Run no longer reads files, parses the profile/config,
// or opens the store itself — those inputs arrive pre-built in Options.
// cmd/userproof is the sole os/path + store wiring point. The one
// remaining os call is the optional DumpUserPath WriteFile, a dual-use
// output sink that legitimately belongs to the engine's smoke path.
//
// R12-G contract: the depth-28 SMT backing (TreeDB) is injected — the
// engine receives an already-built sparse Merkle tree (rebuilt fresh per
// run) and holds no tree DSN; the cmd shim constructs it from the
// deployment config's TreeDB block, keeping the engine backend-agnostic.
//
// R12-B contract: Run returns error; in-process callers can drive
// userproof without recover() and propagate the error up. The
// cmd/userproof shim is the only layer that converts errors into exit
// codes.
//
// R12-C contract: Run takes a context.Context, threaded into the
// snapshot stream (AccountStream already honours ctx by closing its
// producer on cancellation). Userproof is a one-shot batch job — a
// cancelled run leaves the userproof table partially populated, so
// cmd/userproof treats any error (including context.Canceled) as a
// failure (exit 1), unlike the prover daemon.
//
// The four standard snapshot connectors are blank-imported below so
// in-process callers of Run automatically have every model's
// source_type registered; G17/G18 panic semantics on unknown
// source_type are preserved.
package userproof

import (
	"context"
	"fmt"
	"os"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/io/vfs"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
	bsmt "github.com/bnb-chain/zkbnb-smt"

	// Register all four model-specific standard CSV snapshot connectors.
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t1_simple_margin"
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t2_static_haircut_margin"
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t3_tiered_haircut_margin_1pool"
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t4_tiered_haircut_margin_3pool"
)

// Options bundles the inputs Run needs. The cmd/userproof shim builds
// every injected value (parses the profile + config, resolves the
// snapshot directory into a vfs.Opener, and wires the user-proof store
// adapter); the engine never touches os/path or store itself.
type Options struct {
	// Profile is the parsed declarative profile.toml. Required.
	Profile *declarative.Profile

	// Snapshot opens the customer snapshot inputs by name. The cmd
	// shim resolves the data directory (UserDataDir override else
	// profile.snapshot.user_data_dir) into this opener. Required.
	Snapshot vfs.Opener

	// UserProofs is the injected persistence port for the per-user
	// inclusion-proof table. Required; the cmd shim provides the MySQL
	// adapter and has already called EnsureSchema.
	UserProofs corehost.UserProofStore

	// AccountTree is the injected depth-28 sparse Merkle tree the engine
	// rebuilds into to recompute per-user inclusion proofs. The cmd shim
	// constructs it from the deployment config's TreeDB backing
	// (memory/redis) so the engine holds no tree DSN. Required.
	AccountTree bsmt.SparseMerkleTree

	// UserDataDir overrides profile.snapshot.user_data_dir when
	// non-empty. The engine no longer uses this for IO (the cmd shim
	// already baked the resolved directory into Snapshot); it is kept
	// only as the snapshot connector's logical identifier passthrough.
	UserDataDir string

	// SnapshotID overrides profile.snapshot.snapshot_id when non-empty.
	SnapshotID string

	// CapacityOverride supersedes profile.asset_capacity when > 0.
	// Smoke-harness use only.
	CapacityOverride int

	// DumpUserIndex, when >= 0, makes Run dump that account's stored
	// UserConfig JSON to DumpUserPath after writing all userproofs.
	// Smoke harness convenience; production callers should leave at -1.
	DumpUserIndex int

	// DumpUserPath is the destination path for DumpUserIndex. Required
	// when DumpUserIndex >= 0.
	DumpUserPath string
}

// Run reads the snapshot, rebuilds the SMT, writes one UserProof row
// per real account, then returns. Returns an error describing the
// first wiring or runner failure encountered; nil on success. A
// cancelled ctx aborts the snapshot stream and surfaces as a (wrapped)
// context error.
func Run(ctx context.Context, opts Options) error {
	if opts.Profile == nil {
		return fmt.Errorf("userproof: Profile is required")
	}
	if opts.Snapshot == nil {
		return fmt.Errorf("userproof: Snapshot is required")
	}
	if opts.UserProofs == nil {
		return fmt.Errorf("userproof: UserProofs is required")
	}
	if opts.AccountTree == nil {
		return fmt.Errorf("userproof: AccountTree is required")
	}
	prof := opts.Profile

	// G6 closure: BuildPricing carries the ValueScale invariant assert.
	pricing, err := declarative.BuildPricing(prof.Pricing)
	if err != nil {
		return fmt.Errorf("userproof: BuildPricing: %w", err)
	}

	model := corespec.SolvencyModelID(prof.Profile.Model)
	shapeProvider, err := declarative.BuildBatchShapeProvider(model, prof.BatchShapes)
	if err != nil {
		return fmt.Errorf("userproof: BuildBatchShapeProvider: %w", err)
	}
	assetCountTiers := tiersFromShapes(shapeProvider.Shapes())

	capacity := prof.Profile.AssetCapacity
	if opts.CapacityOverride > 0 {
		capacity = opts.CapacityOverride
	}
	snapID := prof.Snapshot.SnapshotID
	if opts.SnapshotID != "" {
		snapID = opts.SnapshotID
	}

	accountTree := opts.AccountTree

	userProofStore := opts.UserProofs

	deps := dispatchInput{
		model:           model,
		ctx:             ctx,
		sourceType:      prof.Snapshot.SourceType,
		snapshot:        opts.Snapshot,
		snapID:          snapID,
		capacity:        capacity,
		pricing:         pricing,
		accountTree:     accountTree,
		userProofStore:  userProofStore,
		shapeProvider:   shapeProvider,
		assetCountTiers: assetCountTiers,
	}
	if err := dispatchRunUserProof(deps); err != nil {
		return fmt.Errorf("userproof: run: %w", err)
	}

	if opts.DumpUserIndex >= 0 {
		if opts.DumpUserPath == "" {
			return fmt.Errorf("userproof: DumpUserIndex requires DumpUserPath")
		}
		row, err := userProofStore.GetByIndex(uint32(opts.DumpUserIndex))
		if err != nil {
			return fmt.Errorf("userproof: read userproof index %d: %w", opts.DumpUserIndex, err)
		}
		if err := os.WriteFile(opts.DumpUserPath, []byte(row.Config), 0o644); err != nil {
			return fmt.Errorf("userproof: write %q: %w", opts.DumpUserPath, err)
		}
		fmt.Printf("user_config[%d] written to %s\n", opts.DumpUserIndex, opts.DumpUserPath)
	}
	return nil
}

// tiersFromShapes flattens the deployment's BatchShape set into the
// sorted-ascending []int the host commitment helpers consume.
func tiersFromShapes(shapes []corespec.BatchShape) []int {
	out := make([]int, len(shapes))
	for i, s := range shapes {
		out[i] = s.AssetCountTier
	}
	return out
}
