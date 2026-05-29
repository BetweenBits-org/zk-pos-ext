// Command witness is the CLI shim around pkg/witness. The engine
// logic lives in zkpor/pkg/witness; this main only parses flags and
// dispatches into Run.
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
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/witness"
)

func main() {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	userDataDir := flag.String("user-data-dir", "", "override profile.snapshot.user_data_dir (smoke + per-snapshot ops)")
	snapshotID := flag.String("snapshot-id", "", "override profile.snapshot.snapshot_id (per-snapshot ops)")
	capacityOverride := flag.Int("asset-capacity", 0, "override profile.asset_capacity (smoke only; 0 = use toml value)")
	dumpFinalCex := flag.String("dump-final-cex", "", "if set, write the post-batch CexAssetsInfo slice as JSON to this path (smoke harness convenience)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := witness.Run(ctx, witness.Options{
		ProfilePath:      *profilePath,
		ConfigPath:       "config/config.json",
		UserDataDir:      *userDataDir,
		SnapshotID:       *snapshotID,
		CapacityOverride: *capacityOverride,
		DumpFinalCex:     *dumpFinalCex,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
