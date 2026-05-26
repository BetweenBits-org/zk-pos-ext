package spec

import corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"

// RiskPolicy supplies per-asset tier-ratio *values* for the
// t3_tiered_haircut_margin_1pool solvency model. The piecewise-linear
// haircut *model* (tier table, boundary-and-ratio shape) is fixed by
// the model's circuit and is not negotiable through this interface.
//
// Single collateral pool — one CollateralRatios slice per asset (vs
// T4's three: Loan / Margin / PortfolioMargin).
//
// A slice MAY be empty (no collateral allowed in this asset). Length
// MUST be <= corespec.TierCount.
//
// Boundary values are interpreted under the standard value scale
// (PriceMultiplier * BalanceMultiplier == ValueScale).
//
// Stricter risk rules (concentration, KYC tiers, ...) belong in a
// ConstraintModule, not here.
type RiskPolicy interface {
	CollateralRatios(symbol string) []TierRatio
}

// MaxTiers is re-exported for adapter implementations.
const MaxTiers = corespec.TierCount
