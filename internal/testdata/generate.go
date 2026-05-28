package testdata

import (
	"fmt"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
)

// Options carries the synthesis parameters cmd/gen-testdata passes
// to GenerateScale.
type Options struct {
	// OutDir is the target directory where accounts.csv,
	// cex_assets.csv, and tier_ratios.csv (T3/T4 only) are written.
	OutDir string

	// Users is the target real account count. The witness builder
	// pads to the BatchShape's users_per_batch separately — this is
	// the real (non-padding) account count to synthesise.
	Users int

	// CapacityOverride supersedes profile.asset_capacity when > 0.
	// Useful for smoke runs at smaller capacity.
	CapacityOverride int

	// Distribution selects the asset balance distribution sampler:
	// "uniform" (R11-A default), "weighted" (80/20), "power" (Pareto).
	Distribution string

	// Seed is the RNG seed for reproducibility. 0 = time-based.
	Seed int64
}

// GenerateScale dispatches to the model-typed synthesis under
// t1.go / t2.go / t3.go / t4.go based on profile.Profile.Model.
//
// TODO(R11-A): not implemented. Skeleton only.
func GenerateScale(prof *declarative.Profile, opts Options) error {
	_ = prof
	_ = opts
	return fmt.Errorf("internal/testdata: GenerateScale not implemented yet (R11-A skeleton)")
}
