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
// R12-A library extraction: the previous 198-line main.go body moved
// to zkpor/pkg/userproof. config/ holds only the runtime JSON file
// now; the Go schema moved alongside the library.
package main

import (
	"flag"

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

	userproof.Run(userproof.Options{
		ProfilePath:      *profilePath,
		ConfigPath:       "config/config.json",
		UserDataDir:      *userDataDir,
		SnapshotID:       *snapshotID,
		CapacityOverride: *capacityOverride,
		DumpUserIndex:    *dumpUserIndex,
		DumpUserPath:     *dumpUserPath,
	})
}
