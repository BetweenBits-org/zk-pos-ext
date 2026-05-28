// Command gen-testdata generates real-scale synthetic standard-CSV
// snapshots for measurement (R11-A). Reads a profile.toml + sizing
// flags, dispatches to the model-typed synthesis under
// internal/testdata, and writes accounts.csv + cex_assets.csv
// [+ tier_ratios.csv] to the chosen output directory.
//
// Output is intended for `.artifacts/testdata/<profile>_<N>/` —
// gitignored, large (tens of MB to GB at 1M users), regenerable.
//
// R11-A scope:
//   - account_id BN254 fr.Element reduced (canonicalAccountID parity)
//   - per-asset sum equality automatic (Σ user.equity = cex.total_equity)
//   - per-user invariants per model (Σ collateral ≤ equity etc.)
//   - uniform distribution default; weighted / power-law as -dist flag
//
// Usage (planned):
//
//	gen-testdata \
//	    -profile profile/t1_reference/t1_reference.toml \
//	    -users 100000 \
//	    -asset-capacity 50 \
//	    -dist uniform \
//	    -out .artifacts/testdata/t1_reference_100k/
//
// The smoke harness (R11-B) then feeds the generated directory to
// cmd/witness / cmd/userproof via the existing -user-data-dir flag.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	users := flag.Int("users", 0, "target real account count (required; padding handled by witness builder)")
	capacityOverride := flag.Int("asset-capacity", 0, "override profile.asset_capacity (0 = use toml value)")
	distribution := flag.String("dist", "uniform", "asset distribution: uniform | weighted | power")
	out := flag.String("out", "", "output directory for generated CSVs (required)")
	seed := flag.Int64("seed", 0, "RNG seed for reproducibility (0 = time-based)")
	flag.Parse()

	if *profilePath == "" || *users <= 0 || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: gen-testdata -profile <toml> -users <N> -out <dir>")
		flag.PrintDefaults()
		os.Exit(2)
	}

	// TODO(R11-A): wire into internal/testdata.GenerateScale(...)
	// once the synthesis package is implemented. Skeleton only
	// confirms the CLI shape and entry point compiles.
	_ = capacityOverride
	_ = distribution
	_ = seed
	fmt.Fprintln(os.Stderr, "gen-testdata: not implemented yet (R11-A skeleton)")
	os.Exit(1)
}
