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
// R12-A library extraction: the previous 184-line main.go body moved
// to zkpor/pkg/witness. config/ holds only the runtime JSON file now;
// the Go schema moved alongside the library.
package main

import (
	"flag"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/witness"
)

func main() {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	userDataDir := flag.String("user-data-dir", "", "override profile.snapshot.user_data_dir (smoke + per-snapshot ops)")
	snapshotID := flag.String("snapshot-id", "", "override profile.snapshot.snapshot_id (per-snapshot ops)")
	capacityOverride := flag.Int("asset-capacity", 0, "override profile.asset_capacity (smoke only; 0 = use toml value)")
	dumpFinalCex := flag.String("dump-final-cex", "", "if set, write the post-batch CexAssetsInfo slice as JSON to this path (smoke harness convenience)")
	flag.Parse()

	witness.Run(witness.Options{
		ProfilePath:      *profilePath,
		ConfigPath:       "config/config.json",
		UserDataDir:      *userDataDir,
		SnapshotID:       *snapshotID,
		CapacityOverride: *capacityOverride,
		DumpFinalCex:     *dumpFinalCex,
	})
}
