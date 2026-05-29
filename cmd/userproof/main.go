// Command userproof is the CLI shim around pkg/userproof. The engine
// logic lives in zkpor/pkg/userproof; this main only parses flags and
// dispatches into Run.
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
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/userproof"
)

func main() {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	userDataDir := flag.String("user-data-dir", "", "override profile.snapshot.user_data_dir (smoke + per-snapshot ops)")
	snapshotID := flag.String("snapshot-id", "", "override profile.snapshot.snapshot_id (per-snapshot ops)")
	capacityOverride := flag.Int("asset-capacity", 0, "override profile.asset_capacity (smoke only; 0 = use toml value)")
	dumpUserIndex := flag.Int("dump-user-index", -1, "if >=0, after writing all userproofs, dump that account's UserConfig JSON to -dump-user-path (smoke harness convenience)")
	dumpUserPath := flag.String("dump-user-path", "", "destination path for -dump-user-index dump")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := userproof.Run(ctx, userproof.Options{
		ProfilePath:      *profilePath,
		ConfigPath:       "config/config.json",
		UserDataDir:      *userDataDir,
		SnapshotID:       *snapshotID,
		CapacityOverride: *capacityOverride,
		DumpUserIndex:    *dumpUserIndex,
		DumpUserPath:     *dumpUserPath,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
