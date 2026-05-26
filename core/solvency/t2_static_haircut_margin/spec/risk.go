package spec

// RiskPolicy supplies per-asset *static* haircut values for the
// t2_static_haircut_margin solvency model. Single uint16 per asset
// (basis points: 10000 = 100% = no haircut, 0 = collateral not
// accepted in this asset).
//
// Stricter risk rules (concentration, KYC tiers, ...) belong in a
// ConstraintModule, not here.
//
// Reference: Aave V3 LTV / Liquidation Threshold per-asset constants
// (see docs/04-solvency-models.md §5).
type RiskPolicy interface {
	// Haircut returns the per-asset haircut multiplier in basis
	// points. Caller MAY supply a per-customer mapping that varies
	// asset-by-asset (BTC: 9000, ETH: 8500, alts: 5000, etc.).
	Haircut(symbol string) uint16
}

// HaircutDenominator is the basis-points denominator (10000) used by
// the circuit: realCollateral = collateral * price * haircut / 10000.
const HaircutDenominator uint64 = 10000
