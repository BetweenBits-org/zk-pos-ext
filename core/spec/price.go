package spec

// PriceScaleProvider returns the multipliers that convert quote-
// currency float prices and float balances into the uint64 values
// embedded in the witness (BasePrice and per-asset user balance fields).
//
// Two multipliers exist because the circuit stores both as bounded
// uint64 (64 bits) and their *product* drives the precision of
// TotalEquity / TotalDebt / TotalCollateral. Splitting the budget
// asymmetrically — e.g. 1e14 price × 1e2 balance for SHIB vs 1e8 ×
// 1e8 for BTC — lets each asset use its 64-bit head-room where it
// actually needs it.
//
// The standard invariant:
//
//	PriceMultiplier(s) * BalanceMultiplier(s) == ValueScale()
//
// for every supported symbol s. Without this invariant, sums across
// different assets compare values in different units and the solvency
// constraint becomes meaningless. Services MUST assert the invariant
// at start-up.
//
// The provider's mapping is part of published proof artifacts;
// verifiers MUST receive the same mapping to interpret BasePrice and
// balance values.
//
// Implementations MUST be deterministic and case-insensitive on
// symbol. Symbols not specifically configured SHOULD return
// DefaultPriceScale / DefaultBalanceScale.
type PriceScaleProvider interface {
	PriceMultiplier(symbol string) int64
	BalanceMultiplier(symbol string) int64

	// ValueScale is the product PriceMultiplier(s) * BalanceMultiplier(s),
	// constant across symbols. It is the unit of TotalEquity, TotalDebt,
	// and TotalCollateral in the witness — verifier UX divides totals
	// by ValueScale to recover quote-currency real value.
	ValueScale() int64
}
