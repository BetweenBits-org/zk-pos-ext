// Command witness is the zkpor-native witness builder. Reads the
// customer snapshot, builds the depth-28 account SMT, walks accounts
// in tier-grouped batches, and writes one BatchCreateUserWitness per
// batch into the witness MySQL table for the prover to pick up.
//
// Phase 3b (R10+1) swap: every model-typed loop (streamAndBucket,
// runBatches, buildBatch, safeAdd) has been pulled into model-specific
// runner packages at core/solvency/<model>/host/witness_runner.go.
// This main is now a thin wiring layer — load profile.toml, build
// shared dependencies, switch on profile.Model, and delegate to the
// matching runner.
//
// R8-C/2 wiring foundation: profile-specific constructors live in
// declarative.BuildPricing / BuildBatchShapeProvider / host.NewSnapshot.
// config.json keeps deployment-secret + ops fields (DB DSN, TreeDB).
//
// Sequential per-account hashing, fresh-start only (no DB resume, no
// tree rollback). G6 (ValueScale invariant) closure happens inside
// declarative.BuildPricing.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	wconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/cmd/witness/config"
	t1host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/host"
	t2host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t2_static_haircut_margin/host"
	t3host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t3_tiered_haircut_margin_1pool/host"
	t4host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/tree"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"

	// Register all four model-specific standard CSV snapshot connectors.
	// init() in each parser package adds the connector to its model's
	// host registry; an unknown source_type in profile.toml panics at
	// service startup (G17/G18).
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t1_simple_margin"
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t2_static_haircut_margin"
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t3_tiered_haircut_margin_1pool"
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t4_tiered_haircut_margin_3pool"
)

func main() {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	userDataDir := flag.String("user-data-dir", "", "override profile.snapshot.user_data_dir (smoke + per-snapshot ops)")
	snapshotID := flag.String("snapshot-id", "", "override profile.snapshot.snapshot_id (per-snapshot ops)")
	capacityOverride := flag.Int("asset-capacity", 0, "override profile.asset_capacity (smoke only; 0 = use toml value)")
	dumpFinalCex := flag.String("dump-final-cex", "", "if set, write the post-batch CexAssetsInfo slice as JSON to this path (smoke harness convenience)")
	flag.Parse()

	if *profilePath == "" {
		fmt.Fprintln(os.Stderr, "-profile is required (path to profile.toml)")
		os.Exit(2)
	}

	cfg := loadConfig("config/config.json")
	prof, err := declarative.Load(*profilePath)
	if err != nil {
		panic(err.Error())
	}

	// G6 closure: BuildPricing rejects invariant violations at build time.
	pricing, err := declarative.BuildPricing(prof.Pricing)
	if err != nil {
		panic(fmt.Sprintf("BuildPricing: %v", err))
	}

	model := corespec.SolvencyModelID(prof.Profile.Model)
	shapeProvider, err := declarative.BuildBatchShapeProvider(model, prof.BatchShapes)
	if err != nil {
		panic(fmt.Sprintf("BuildBatchShapeProvider: %v", err))
	}
	assetCountTiers := tiersFromShapes(shapeProvider.Shapes())

	capacity := prof.Profile.AssetCapacity
	if *capacityOverride > 0 {
		capacity = *capacityOverride
	}
	dataDir := prof.Snapshot.UserDataDir
	if *userDataDir != "" {
		dataDir = *userDataDir
	}
	snapID := prof.Snapshot.SnapshotID
	if *snapshotID != "" {
		snapID = *snapshotID
	}

	accountTree, err := tree.NewAccountTree(cfg.TreeDB.Driver, cfg.TreeDB.Option.Addr)
	if err != nil {
		panic(err.Error())
	}

	db, err := store.Open(cfg.MysqlDataSource)
	if err != nil {
		panic(err.Error())
	}
	witnessStore := store.NewWitnessStore(db, cfg.DbSuffix)
	if err := witnessStore.CreateTable(); err != nil {
		panic(err.Error())
	}

	ctx := context.Background()

	switch model {
	case "t1_simple_margin":
		snapshot := t1host.NewSnapshot(prof.Snapshot.SourceType, dataDir, snapID, capacity, pricing)
		err = t1host.RunWitness(t1host.WitnessRunnerConfig{
			Ctx:             ctx,
			Snapshot:        snapshot,
			AccountTree:     accountTree,
			WitnessStore:    witnessStore,
			ShapeProvider:   shapeProvider,
			AssetCountTiers: assetCountTiers,
			DumpFinalCex:    *dumpFinalCex,
		})
	case "t2_static_haircut_margin":
		snapshot := t2host.NewSnapshot(prof.Snapshot.SourceType, dataDir, snapID, capacity, pricing)
		err = t2host.RunWitness(t2host.WitnessRunnerConfig{
			Ctx:             ctx,
			Snapshot:        snapshot,
			AccountTree:     accountTree,
			WitnessStore:    witnessStore,
			ShapeProvider:   shapeProvider,
			AssetCountTiers: assetCountTiers,
			DumpFinalCex:    *dumpFinalCex,
		})
	case "t3_tiered_haircut_margin_1pool":
		snapshot := t3host.NewSnapshot(prof.Snapshot.SourceType, dataDir, snapID, capacity, pricing)
		err = t3host.RunWitness(t3host.WitnessRunnerConfig{
			Ctx:             ctx,
			Snapshot:        snapshot,
			AccountTree:     accountTree,
			WitnessStore:    witnessStore,
			ShapeProvider:   shapeProvider,
			AssetCountTiers: assetCountTiers,
			DumpFinalCex:    *dumpFinalCex,
		})
	case "t4_tiered_haircut_margin_3pool":
		snapshot := t4host.NewSnapshot(prof.Snapshot.SourceType, dataDir, snapID, capacity, pricing)
		err = t4host.RunWitness(t4host.WitnessRunnerConfig{
			Ctx:             ctx,
			Snapshot:        snapshot,
			AccountTree:     accountTree,
			WitnessStore:    witnessStore,
			ShapeProvider:   shapeProvider,
			AssetCountTiers: assetCountTiers,
			DumpFinalCex:    *dumpFinalCex,
		})
	default:
		panic(fmt.Sprintf("witness: unsupported solvency model %q", model))
	}
	if err != nil {
		panic(err.Error())
	}
}

// loadConfig reads and parses the on-disk JSON config.
func loadConfig(path string) *wconfig.Config {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err.Error())
	}
	cfg := &wconfig.Config{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		panic(err.Error())
	}
	return cfg
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
