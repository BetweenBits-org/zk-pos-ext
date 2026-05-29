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
package main

import (
	"flag"
	"fmt"
	"os"

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

	if err := userproof.Run(userproof.Options{
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
