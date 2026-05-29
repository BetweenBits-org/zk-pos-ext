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
// R12-B/2: pkg/keygen returns errors. This shim is the only layer that
// converts them into exit codes — stderr + os.Exit(1) on failure.
//
// R12-C: SIGINT/SIGTERM are wired into Run's context via
// signal.NotifyContext so a multi-shape setup can be stopped between
// shapes. Keygen is a one-shot job — any error (including
// context.Canceled) exits 1.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/keygen"
)

func main() {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	out := flag.String("out", ".", "output directory for .pk/.vk/.r1cs files")
	capacityOverride := flag.Int("asset-capacity", 0,
		"override profile.asset_capacity (smoke harness only; 0 = use profile.toml value)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := keygen.Run(ctx, keygen.Options{
		ProfilePath:      *profilePath,
		OutDir:           *out,
		CapacityOverride: *capacityOverride,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
