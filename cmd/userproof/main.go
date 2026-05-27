// Command userproof is the zkpor-native per-user inclusion-proof
// builder. Reads the customer snapshot, rebuilds the depth-28
// account SMT (matching the witness service's tree state via the
// same padding semantics), and writes one UserProof row per real
// account — embedded UserConfig payload lets the verifier -user
// mode recompute and check inclusion locally.
//
// Phase 3e (R10+1) swap: every model-typed loop (streamAndBucket,
// populateTree, writeUserProofs, buildUserProofRow) has been pulled
// into model-specific runner packages at
// core/solvency/<model>/host/userproof_runner.go. This main is now a
// thin wiring layer — load profile.toml, build shared dependencies,
// switch on profile.Model, and delegate to the matching runner.
//
// R8-D wiring foundation: snapshot/asset-capacity/batch-shape come
// from profile.toml + the host registries. config.json keeps DB +
// TreeDB only.
//
// This is the R3 step 4 core-path service: sequential per-account
// hashing and proof generation, fresh-start only (no DB resume, no
// parallel workers, no -memory_tree utility flag).
//
// Self-contained tree build: the userproof does NOT depend on the
// witness service's persisted tree state. Same snapshot + same
// padding rules = same tree leaves = same root, so per-user proofs
// verify against the same root the witness/prover published.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	uconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/cmd/userproof/config"
	t1host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/host"
	t2host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t2_static_haircut_margin/host"
	t3host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t3_tiered_haircut_margin_1pool/host"
	t4host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/tree"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"

	// Register all four model-specific standard CSV snapshot connectors.
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
	dumpUserIndex := flag.Int("dump-user-index", -1, "if >=0, after writing all userproofs, dump that account's UserConfig JSON to -dump-user-path (smoke harness convenience)")
	dumpUserPath := flag.String("dump-user-path", "", "destination path for -dump-user-index dump")
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

	// G6 closure: BuildPricing carries the ValueScale invariant assert.
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
	userProofStore := store.NewUserProofStore(db, cfg.DbSuffix)
	if err := userProofStore.CreateTable(); err != nil {
		panic(err.Error())
	}

	ctx := context.Background()

	switch model {
	case "t1_simple_margin":
		snapshot := t1host.NewSnapshot(prof.Snapshot.SourceType, dataDir, snapID, capacity, pricing)
		_, _, err = t1host.RunUserProof(t1host.UserProofRunnerConfig{
			Ctx:             ctx,
			Snapshot:        snapshot,
			AccountTree:     accountTree,
			UserProofStore:  userProofStore,
			ShapeProvider:   shapeProvider,
			AssetCountTiers: assetCountTiers,
		})
	case "t2_static_haircut_margin":
		snapshot := t2host.NewSnapshot(prof.Snapshot.SourceType, dataDir, snapID, capacity, pricing)
		_, _, err = t2host.RunUserProof(t2host.UserProofRunnerConfig{
			Ctx:             ctx,
			Snapshot:        snapshot,
			AccountTree:     accountTree,
			UserProofStore:  userProofStore,
			ShapeProvider:   shapeProvider,
			AssetCountTiers: assetCountTiers,
		})
	case "t3_tiered_haircut_margin_1pool":
		snapshot := t3host.NewSnapshot(prof.Snapshot.SourceType, dataDir, snapID, capacity, pricing)
		_, _, err = t3host.RunUserProof(t3host.UserProofRunnerConfig{
			Ctx:             ctx,
			Snapshot:        snapshot,
			AccountTree:     accountTree,
			UserProofStore:  userProofStore,
			ShapeProvider:   shapeProvider,
			AssetCountTiers: assetCountTiers,
		})
	case "t4_tiered_haircut_margin_3pool":
		snapshot := t4host.NewSnapshot(prof.Snapshot.SourceType, dataDir, snapID, capacity, pricing)
		_, _, err = t4host.RunUserProof(t4host.UserProofRunnerConfig{
			Ctx:             ctx,
			Snapshot:        snapshot,
			AccountTree:     accountTree,
			UserProofStore:  userProofStore,
			ShapeProvider:   shapeProvider,
			AssetCountTiers: assetCountTiers,
		})
	default:
		panic(fmt.Sprintf("userproof: unsupported solvency model %q", model))
	}
	if err != nil {
		panic(err.Error())
	}

	if *dumpUserIndex >= 0 {
		if *dumpUserPath == "" {
			panic("-dump-user-index requires -dump-user-path")
		}
		row, err := userProofStore.GetByIndex(uint32(*dumpUserIndex))
		if err != nil {
			panic(fmt.Sprintf("read userproof index %d: %v", *dumpUserIndex, err))
		}
		if err := os.WriteFile(*dumpUserPath, []byte(row.Config), 0o644); err != nil {
			panic(fmt.Sprintf("write %q: %v", *dumpUserPath, err))
		}
		fmt.Printf("user_config[%d] written to %s\n", *dumpUserIndex, *dumpUserPath)
	}
}

func loadConfig(path string) *uconfig.Config {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err.Error())
	}
	cfg := &uconfig.Config{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		panic(err.Error())
	}
	return cfg
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
