// Package witness is the zkpor-native witness builder engine. It reads
// the customer snapshot, builds the depth-28 account SMT, walks
// accounts in tier-grouped batches, and writes one
// BatchCreateUserWitness per batch into the witness MySQL table for
// the prover to pick up.
//
// Phase 3b (R10+1) swap: every model-typed loop (streamAndBucket,
// runBatches, buildBatch, safeAdd) is pulled into model-specific
// runner packages at core/solvency/<model>/host/witness_runner.go.
// This package is a thin wiring layer — it receives the parsed profile,
// snapshot opener, witness-queue port, and account tree as injected
// Options (the cmd shim builds them), derives shared dependencies,
// switches on profile.Model, and delegates to the matching runner.
//
// R12-E contract: Run no longer reads files, parses the profile/config,
// or opens the store itself — those inputs arrive pre-built in Options.
// cmd/witness is the sole os/path + store wiring point.
//
// R12-G contract: the depth-28 SMT backing (TreeDB) is injected too —
// the engine receives an already-built sparse Merkle tree and holds no
// tree DSN; the cmd shim constructs it from the deployment config's
// TreeDB block, keeping the engine agnostic to the tree backend.
//
// Sequential per-account hashing, fresh-start only (no DB resume, no
// tree rollback). G6 (ValueScale invariant) closure happens inside
// declarative.BuildPricing.
//
// R12-B contract: Run returns error; in-process callers can drive
// witness without recover() and propagate the error up. The
// cmd/witness shim is the only layer that converts errors into exit
// codes.
//
// R12-C contract: Run takes a context.Context, threaded into the
// snapshot stream (AccountStream / CexAssets already honour ctx by
// closing their producer on cancellation). Witness is a one-shot batch
// job — a cancelled run leaves the witness table partially populated,
// so cmd/witness treats any error (including context.Canceled) as a
// failure (exit 1), unlike the prover daemon.
//
// The four standard snapshot connectors are blank-imported below so
// in-process callers of Run automatically have every model's
// source_type registered; G17/G18 panic semantics on unknown
// source_type are preserved.
package witness

import (
	"context"
	"fmt"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/io/vfs"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
	bsmt "github.com/bnb-chain/zkbnb-smt"

	// Register all four model-specific standard CSV snapshot connectors.
	// init() in each parser package adds the connector to its model's
	// host registry; an unknown source_type in profile.toml panics at
	// engine startup (G17/G18).
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t1_simple_margin"
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t2_static_haircut_margin"
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t3_tiered_haircut_margin_1pool"
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t4_tiered_haircut_margin_3pool"
)

// Options bundles the inputs Run needs. The cmd/witness shim builds
// every injected value (parses the profile + config, resolves the
// snapshot directory into a vfs.Opener, and wires the witness queue
// adapter); the engine never touches os/path or store itself.
type Options struct {
	// Profile is the parsed declarative profile.toml. Required.
	Profile *declarative.Profile

	// Snapshot opens the customer snapshot inputs by name. The cmd
	// shim resolves the data directory (UserDataDir override else
	// profile.snapshot.user_data_dir) into this opener. Required.
	Snapshot vfs.Opener

	// WitnessQueue is the injected persistence port for the
	// witness↔prover artifact channel. Required; the cmd shim provides
	// the MySQL adapter and has already called EnsureSchema.
	WitnessQueue corehost.WitnessQueue

	// AccountTree is the injected depth-28 sparse Merkle tree the witness
	// builds into. The cmd shim constructs it from the deployment config's
	// TreeDB backing (memory/redis) so the engine holds no tree DSN and
	// stays agnostic to the tree backend. Required.
	AccountTree bsmt.SparseMerkleTree

	// UserDataDir overrides profile.snapshot.user_data_dir when
	// non-empty. The engine no longer uses this for IO (the cmd shim
	// already baked the resolved directory into Snapshot); it is kept
	// only as the snapshot connector's logical identifier passthrough.
	UserDataDir string

	// SnapshotID overrides profile.snapshot.snapshot_id when non-empty.
	// Per-snapshot ops use this to write a fresh batch series for a new
	// snapshot generation.
	SnapshotID string

	// CapacityOverride supersedes profile.asset_capacity when > 0.
	// Smoke-harness use only.
	CapacityOverride int

	// DumpFinalCex, when non-empty, makes the runner write the
	// post-batch CexAssetsInfo slice as JSON to this path. Smoke
	// harness convenience; leave empty in production callers.
	DumpFinalCex string
}

// Run builds and writes one full snapshot's worth of batch witness
// rows, then returns. Returns an error describing the first wiring or
// runner failure encountered; nil on success. A cancelled ctx aborts
// the snapshot stream and surfaces as a (wrapped) context error.
func Run(ctx context.Context, opts Options) error {
	if opts.Profile == nil {
		return fmt.Errorf("witness: Profile is required")
	}
	if opts.Snapshot == nil {
		return fmt.Errorf("witness: Snapshot is required")
	}
	if opts.WitnessQueue == nil {
		return fmt.Errorf("witness: WitnessQueue is required")
	}
	if opts.AccountTree == nil {
		return fmt.Errorf("witness: AccountTree is required")
	}
	prof := opts.Profile

	// G6 closure: BuildPricing rejects invariant violations at build time.
	pricing, err := declarative.BuildPricing(prof.Pricing)
	if err != nil {
		return fmt.Errorf("witness: BuildPricing: %w", err)
	}

	model := corespec.SolvencyModelID(prof.Profile.Model)
	shapeProvider, err := declarative.BuildBatchShapeProvider(model, prof.BatchShapes)
	if err != nil {
		return fmt.Errorf("witness: BuildBatchShapeProvider: %w", err)
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

	deps := dispatchInput{
		model:           model,
		ctx:             ctx,
		sourceType:      prof.Snapshot.SourceType,
		snapshot:        opts.Snapshot,
		snapID:          snapID,
		capacity:        capacity,
		pricing:         pricing,
		accountTree:     accountTree,
		witnessStore:    opts.WitnessQueue,
		shapeProvider:   shapeProvider,
		assetCountTiers: assetCountTiers,
		dumpFinalCex:    opts.DumpFinalCex,
	}
	if err := dispatchRunWitness(deps); err != nil {
		return fmt.Errorf("witness: run: %w", err)
	}
	return nil
}

// tiersFromShapes flattens the deployment's BatchShape set to the
// sorted-ascending []int the host commitment helpers consume.
func tiersFromShapes(shapes []corespec.BatchShape) []int {
	out := make([]int, len(shapes))
	for i, s := range shapes {
		out[i] = s.AssetCountTier
	}
	return out
}
