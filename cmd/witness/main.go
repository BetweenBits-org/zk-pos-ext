// Command witness is the CLI shim around pkg/witness. The engine
// logic lives in zkpor/pkg/witness; this main parses flags, builds
// every input the engine needs (profile, config, snapshot opener,
// witness-queue port), and dispatches into Run.
//
// Usage:
//
//	witness -profile path/to/profile.toml \
//	    [-user-data-dir DIR] [-snapshot-id ID] \
//	    [-asset-capacity N] [-dump-final-cex PATH]
//
// R12-B/3: pkg/witness returns errors. This shim is the only layer
// that converts them into exit codes — stderr + os.Exit(1) on failure.
//
// R12-C: SIGINT/SIGTERM are wired into Run's context via
// signal.NotifyContext so a long snapshot build can be aborted.
// Witness is a one-shot job, so an interrupted run is an
// incomplete-output failure — any error (including context.Canceled)
// exits 1.
//
// R12-E: input construction (profile/config parse, snapshot-directory
// resolution into a vfs.Opener, witness-queue adapter wiring) moved
// out of the engine and into this shim. The engine receives injected
// values and never touches os/path or store itself.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/io/vfs/osvfs"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/tree"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/witness"
	wconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/witness/config"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	configPath := flag.String("config", "config/config.json", "path to the witness deployment config JSON")
	userDataDir := flag.String("user-data-dir", "", "override profile.snapshot.user_data_dir (smoke + per-snapshot ops)")
	snapshotID := flag.String("snapshot-id", "", "override profile.snapshot.snapshot_id (per-snapshot ops)")
	capacityOverride := flag.Int("asset-capacity", 0, "override profile.asset_capacity (smoke only; 0 = use toml value)")
	dumpFinalCex := flag.String("dump-final-cex", "", "if set, write the post-batch CexAssetsInfo slice as JSON to this path (smoke harness convenience)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *profilePath == "" {
		return fmt.Errorf("witness: -profile is required (path to profile.toml)")
	}

	prof, err := declarative.Load(*profilePath)
	if err != nil {
		return fmt.Errorf("witness: load profile %q: %w", *profilePath, err)
	}

	raw, err := os.ReadFile(*configPath)
	if err != nil {
		return fmt.Errorf("witness: read config %q: %w", *configPath, err)
	}
	cfg, err := wconfig.Parse(raw)
	if err != nil {
		return fmt.Errorf("witness: parse config %q: %w", *configPath, err)
	}

	// Resolve the snapshot data directory: the -user-data-dir flag wins,
	// else the profile value. The engine no longer does this — it just
	// reads through the opener built here.
	dataDir := prof.Snapshot.UserDataDir
	if *userDataDir != "" {
		dataDir = *userDataDir
	}
	snap := osvfs.Dir(dataDir)

	db, err := store.Open(cfg.MysqlDataSource)
	if err != nil {
		return fmt.Errorf("witness: open mysql: %w", err)
	}
	wq := store.NewWitnessQueueAdapter(store.NewWitnessStore(db, cfg.DbSuffix))
	if err := wq.EnsureSchema(); err != nil {
		return fmt.Errorf("witness: create witness table: %w", err)
	}

	// Build the SMT backing from the deployment config's TreeDB block and
	// inject the tree handle — the engine never holds a tree DSN (R12-G).
	accountTree, err := tree.NewAccountTree(cfg.TreeDB.Driver, cfg.TreeDB.Option.Addr)
	if err != nil {
		return fmt.Errorf("witness: open account tree: %w", err)
	}

	return witness.Run(ctx, witness.Options{
		Profile:          prof,
		Snapshot:         snap,
		WitnessQueue:     wq,
		AccountTree:      accountTree,
		UserDataDir:      *userDataDir,
		SnapshotID:       *snapshotID,
		CapacityOverride: *capacityOverride,
		DumpFinalCex:     *dumpFinalCex,
	})
}
