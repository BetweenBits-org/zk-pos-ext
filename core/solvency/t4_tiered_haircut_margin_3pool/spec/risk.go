package spec

import corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"

// RiskPolicy supplies per-asset tier-ratio *values* for the
// t4_tiered_haircut_margin_3pool solvency model. The piecewise-linear haircut *model*
// (tier table, boundary-and-ratio shape) is fixed by this model's
// circuit and is not negotiable through this interface.
//
// Each returned slice represents one collateral bucket. A slice MAY
// be empty (no collateral allowed in that bucket for the asset).
// Length MUST be <= corespec.TierCount.
//
// Boundary values are interpreted under the standard value scale
// (PriceMultiplier * BalanceMultiplier == ValueScale).
//
// Stricter risk rules (concentration, segregation, KYC tiers, ...)
// belong in a ConstraintModule, not here.
type RiskPolicy interface {
	LoanRatios(symbol string) []TierRatio
	MarginRatios(symbol string) []TierRatio
	PortfolioMarginRatios(symbol string) []TierRatio
}

// MaxTiers is re-exported for adapter implementations.
const MaxTiers = corespec.TierCount
