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
//   - uniform distribution default; weighted / power-law as follow-up
//
// Usage:
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
	"time"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/internal/testdata"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
)

func main() {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	users := flag.Int("users", 0, "target real account count (required; padding handled by witness builder)")
	capacityOverride := flag.Int("asset-capacity", 0, "override profile.asset_capacity (0 = use toml value)")
	assetCount := flag.Int("asset-count", 0, "per-user non-empty asset count (0 = same as asset-capacity); R11-D tier isolation: 50 → Tier 1, 500 → Tier 2")
	distribution := flag.String("dist", "uniform", "asset distribution: uniform (R11-A only); weighted/power planned")
	out := flag.String("out", "", "output directory for generated CSVs (required)")
	seed := flag.Int64("seed", 0, "RNG seed for reproducibility (0 = time-based)")
	flag.Parse()

	if *profilePath == "" || *users <= 0 || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: gen-testdata -profile <toml> -users <N> -out <dir>")
		flag.PrintDefaults()
		os.Exit(2)
	}

	prof, err := declarative.Load(*profilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load profile: %v\n", err)
		os.Exit(1)
	}

	if *seed == 0 {
		*seed = time.Now().UnixNano()
	}

	opts := testdata.Options{
		OutDir:           *out,
		Users:            *users,
		CapacityOverride: *capacityOverride,
		AssetCount:       *assetCount,
		Distribution:     *distribution,
		Seed:             *seed,
	}

	fmt.Printf("gen-testdata: profile=%s model=%s users=%d cap=%d asset_count=%d out=%s seed=%d\n",
		*profilePath, prof.Profile.Model, *users, *capacityOverride, *assetCount, *out, *seed)

	if err := testdata.GenerateScale(prof, opts); err != nil {
		fmt.Fprintf(os.Stderr, "GenerateScale: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("gen-testdata: wrote standard-CSV testdata for %d users to %s\n", *users, *out)
}
