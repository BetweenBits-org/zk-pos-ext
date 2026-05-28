// Command keygen is the CLI shim around pkg/keygen. The engine logic
// lives in zkpor/pkg/keygen; this main only parses flags and dispatches
// into Run.
//
// Run for a customer profile + production shapes:
//
//	go run ./cmd/keygen -profile ./profile/t4_reference/t4_reference.toml \
//	    -out .artifacts/t4_reference
//
// Run for the smoke harness (override capacity + shapes):
//
//	ZKPOR_BATCH_SHAPE_OVERRIDE=5_10 \
//	  go run ./cmd/keygen \
//	      -profile ./profile/t4_reference/t4_reference.toml \
//	      -asset-capacity 5 \
//	      -out .artifacts/smoke
//
// R12-A library extraction: the previous 206-line main.go body moved
// to zkpor/pkg/keygen.
package main

import (
	"flag"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/keygen"
)

func main() {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	out := flag.String("out", ".", "output directory for .pk/.vk/.r1cs files")
	capacityOverride := flag.Int("asset-capacity", 0,
		"override profile.asset_capacity (smoke harness only; 0 = use profile.toml value)")
	flag.Parse()

	keygen.Run(keygen.Options{
		ProfilePath:      *profilePath,
		OutDir:           *out,
		CapacityOverride: *capacityOverride,
	})
}
