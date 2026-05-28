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
// R12-A library extraction: previously this code lived in
// cmd/userproof/main.go as package main. The orchestration body moved
// here unchanged (Conservative slice). cmd/userproof is now a thin
// shim that parses flags and calls Run.
//
// The four standard snapshot connectors are blank-imported below so
// in-process callers of Run automatically have every model's
// source_type registered; G17/G18 panic semantics on unknown
// source_type are preserved.
package userproof

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/tree"
	uconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/userproof/config"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"

	// Register all four model-specific standard CSV snapshot connectors.
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t1_simple_margin"
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t2_static_haircut_margin"
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t3_tiered_haircut_margin_1pool"
	_ "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t4_tiered_haircut_margin_3pool"
)

// Options bundles the inputs Run needs.
type Options struct {
	// ProfilePath points at the declarative profile.toml. Required.
	ProfilePath string

	// ConfigPath points at the userproof deployment config JSON
	// (DB DSN + TreeDB driver/endpoint). Defaults to
	// "config/config.json" when empty.
	ConfigPath string

	// UserDataDir overrides profile.snapshot.user_data_dir when
	// non-empty. Smoke + per-snapshot ops.
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
// per real account, then returns. Panics on any wiring or runner
// failure (v0 reference behaviour).
func Run(opts Options) {
	if opts.ProfilePath == "" {
		fmt.Fprintln(os.Stderr, "ProfilePath is required (path to profile.toml)")
		os.Exit(2)
	}
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = "config/config.json"
	}

	cfg := loadConfig(configPath)
	prof, err := declarative.Load(opts.ProfilePath)
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
	if opts.CapacityOverride > 0 {
		capacity = opts.CapacityOverride
	}
	dataDir := prof.Snapshot.UserDataDir
	if opts.UserDataDir != "" {
		dataDir = opts.UserDataDir
	}
	snapID := prof.Snapshot.SnapshotID
	if opts.SnapshotID != "" {
		snapID = opts.SnapshotID
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

	deps := dispatchInput{
		model:           model,
		ctx:             ctx,
		sourceType:      prof.Snapshot.SourceType,
		dataDir:         dataDir,
		snapID:          snapID,
		capacity:        capacity,
		pricing:         pricing,
		accountTree:     accountTree,
		userProofStore:  userProofStore,
		shapeProvider:   shapeProvider,
		assetCountTiers: assetCountTiers,
	}
	if err := dispatchRunUserProof(deps); err != nil {
		panic(err.Error())
	}

	if opts.DumpUserIndex >= 0 {
		if opts.DumpUserPath == "" {
			panic("DumpUserIndex requires DumpUserPath")
		}
		row, err := userProofStore.GetByIndex(uint32(opts.DumpUserIndex))
		if err != nil {
			panic(fmt.Sprintf("read userproof index %d: %v", opts.DumpUserIndex, err))
		}
		if err := os.WriteFile(opts.DumpUserPath, []byte(row.Config), 0o644); err != nil {
			panic(fmt.Sprintf("write %q: %v", opts.DumpUserPath, err))
		}
		fmt.Printf("user_config[%d] written to %s\n", opts.DumpUserIndex, opts.DumpUserPath)
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
