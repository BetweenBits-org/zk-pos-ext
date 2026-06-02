// Command userproof is the CLI shim around pkg/userproof. The engine
// logic lives in zkpor/pkg/userproof; this main parses flags, builds
// every input the engine needs (profile, config, snapshot opener,
// user-proof store port), and dispatches into Run.
//
// Usage:
//
//	userproof -profile path/to/profile.toml \
//	    [-user-data-dir DIR] [-snapshot-id ID] \
//	    [-asset-capacity N] \
//	    [-dump-user-index N -dump-user-path PATH]
//
// R12-B/3: pkg/userproof returns errors. This shim is the only layer
// that converts them into exit codes — stderr + os.Exit(1) on failure.
//
// R12-C: SIGINT/SIGTERM are wired into Run's context via
// signal.NotifyContext so a long snapshot build can be aborted.
// Userproof is a one-shot job, so an interrupted run is an
// incomplete-output failure — any error (including context.Canceled)
// exits 1.
//
// R12-E: input construction (profile/config parse, snapshot-directory
// resolution into a vfs.Opener, user-proof store adapter wiring) moved
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

	"github.com/BetweenBits-org/zk-pos-ext/core/io/vfs/osvfs"
	"github.com/BetweenBits-org/zk-pos-ext/core/tree"
	"github.com/BetweenBits-org/zk-pos-ext/pkg/userproof"
	uconfig "github.com/BetweenBits-org/zk-pos-ext/pkg/userproof/config"
	"github.com/BetweenBits-org/zk-pos-ext/profile/declarative"
	"github.com/BetweenBits-org/zk-pos-ext/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	configPath := flag.String("config", "config/config.json", "path to the userproof deployment config JSON")
	userDataDir := flag.String("user-data-dir", "", "override profile.snapshot.user_data_dir (smoke + per-snapshot ops)")
	snapshotID := flag.String("snapshot-id", "", "override profile.snapshot.snapshot_id (per-snapshot ops)")
	capacityOverride := flag.Int("asset-capacity", 0, "override profile.asset_capacity (smoke only; 0 = use toml value)")
	dumpUserIndex := flag.Int("dump-user-index", -1, "if >=0, after writing all userproofs, dump that account's UserConfig JSON to -dump-user-path (smoke harness convenience)")
	dumpUserPath := flag.String("dump-user-path", "", "destination path for -dump-user-index dump")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *profilePath == "" {
		return fmt.Errorf("userproof: -profile is required (path to profile.toml)")
	}

	prof, err := declarative.Load(*profilePath)
	if err != nil {
		return fmt.Errorf("userproof: load profile %q: %w", *profilePath, err)
	}

	raw, err := os.ReadFile(*configPath)
	if err != nil {
		return fmt.Errorf("userproof: read config %q: %w", *configPath, err)
	}
	cfg, err := uconfig.Parse(raw)
	if err != nil {
		return fmt.Errorf("userproof: parse config %q: %w", *configPath, err)
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
		return fmt.Errorf("userproof: open mysql: %w", err)
	}
	ups := store.NewUserProofStoreAdapter(store.NewUserProofStore(db, cfg.DbSuffix))
	if err := ups.EnsureSchema(); err != nil {
		return fmt.Errorf("userproof: create userproof table: %w", err)
	}

	// Build the SMT backing from the deployment config's TreeDB block and
	// inject the tree handle — the engine never holds a tree DSN (R12-G).
	accountTree, err := tree.NewAccountTree(cfg.TreeDB.Driver, cfg.TreeDB.Option.Addr)
	if err != nil {
		return fmt.Errorf("userproof: open account tree: %w", err)
	}

	return userproof.Run(ctx, userproof.Options{
		Profile:          prof,
		Snapshot:         snap,
		UserProofs:       ups,
		AccountTree:      accountTree,
		UserDataDir:      *userDataDir,
		SnapshotID:       *snapshotID,
		CapacityOverride: *capacityOverride,
		DumpUserIndex:    *dumpUserIndex,
		DumpUserPath:     *dumpUserPath,
	})
}
