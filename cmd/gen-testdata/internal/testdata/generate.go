package testdata

import (
	"fmt"

	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/BetweenBits-org/zk-pos-ext/profile/declarative"
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

	// AssetCount overrides the per-user non-empty asset count when > 0.
	// Decoupled from CapacityOverride to enable tier-isolated R11-D
	// measurements: profile.asset_capacity stays at production dim
	// (e.g., 500 = .pk circuit slots), but AssetCount=50 forces every
	// user to land in Tier 1 (50_700) and AssetCount=500 forces Tier 2
	// (500_92). Synthetic "asset_N" symbols extend a short profile
	// catalog when AssetCount exceeds its length.
	AssetCount int

	// Distribution selects the asset balance distribution sampler.
	// R11-A supports "uniform" only; "weighted" / "power" are planned
	// for follow-up commits.
	Distribution string

	// Seed is the RNG seed for reproducibility. 0 = time-based
	// (R11-A's uniform sampler is deterministic at fixed seed).
	Seed int64
}

// GenerateScale dispatches to the model-typed synthesis under
// t1.go / t2.go / t3.go / t4.go based on profile.Profile.Model.
//
// R11-A scope: uniform distribution only. Per-user invariants
// (Σ collateral ≤ equity for T2/T3/T4, TotalEquity ≥ TotalDebt for
// T1) hold trivially because debt = 0 and collateral ≤ equity by
// construction.
func GenerateScale(prof *declarative.Profile, opts Options) error {
	if opts.OutDir == "" {
		return fmt.Errorf("Options.OutDir required")
	}
	if opts.Users <= 0 {
		return fmt.Errorf("Options.Users must be > 0 (got %d)", opts.Users)
	}
	if opts.Distribution == "" {
		opts.Distribution = "uniform"
	}
	if opts.Distribution != "uniform" {
		return fmt.Errorf("distribution %q not implemented yet (R11-A supports uniform only)", opts.Distribution)
	}

	capacity := prof.Profile.AssetCapacity
	if opts.CapacityOverride > 0 {
		capacity = opts.CapacityOverride
	}
	if capacity <= 0 {
		return fmt.Errorf("asset capacity must be > 0 (got %d)", capacity)
	}

	target := opts.AssetCount
	if target <= 0 {
		target = capacity
	}
	if target > capacity {
		return fmt.Errorf("Options.AssetCount=%d exceeds asset_capacity=%d (circuit cannot fit more non-empty slots than its capacity)", target, capacity)
	}
	symbols := assetCatalog(prof.Catalog.Symbols, target)
	if len(symbols) == 0 {
		return fmt.Errorf("no asset symbols (profile.Catalog.Symbols empty + default fallback failed)")
	}

	switch corespec.SolvencyModelID(prof.Profile.Model) {
	case "t1_simple_margin":
		return generateT1(opts, symbols)
	case "t2_static_haircut_margin":
		return generateT2(opts, symbols)
	case "t3_tiered_haircut_margin_1pool":
		return generateT3(opts, symbols)
	case "t4_tiered_haircut_margin_3pool":
		return generateT4(opts, symbols)
	default:
		return fmt.Errorf("unsupported solvency model %q", prof.Profile.Model)
	}
}
