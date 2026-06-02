package circuit

import (
	corecircuit "github.com/BetweenBits-org/zk-pos-ext/core/circuit"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
)

// Type aliases so this package doesn't need to import gnark/frontend
// just to satisfy in-circuit signatures.
type (
	API      = corecircuit.API
	Variable = corecircuit.Variable
)

// TierRatio is one tier of the piecewise-linear haircut curve, in
// gnark-Variable form. PrecomputedValue is filled in-circuit by
// generateRapidArithmeticForCollateral (cumulative
// BoundaryValue*Ratio/100 up through this tier) so per-user haircut
// lookups stay O(1).
type TierRatio struct {
	BoundaryValue    Variable
	Ratio            Variable
	PrecomputedValue Variable
}

// CexAssetInfo is the per-asset CEX global state in gnark-Variable
// form. The three TierRatio slices MUST each have length
// corespec.TierCount; circuits pack two TierRatios into one field
// element so an odd length is invalid.
type CexAssetInfo struct {
	TotalEquity Variable
	TotalDebt   Variable
	BasePrice   Variable

	LoanCollateral            Variable
	MarginCollateral          Variable
	PortfolioMarginCollateral Variable

	LoanRatios            []TierRatio
	MarginRatios          []TierRatio
	PortfolioMarginRatios []TierRatio
}

// UserAssetInfo is the per-user, per-asset *index* witness — it tells
// the circuit which tier each of the user's three collateral buckets
// falls into. The collateral amounts themselves come from
// UserAssetMeta; this struct is the lookup-table cursor.
//
// For each bucket: CollateralIndex is the smallest tier whose
// BoundaryValue >= collateral*price. If no tier qualifies (collateral
// exceeds the largest BoundaryValue), CollateralFlag is set and the
// circuit uses the precomputed-cap value instead of interpolating.
type UserAssetInfo struct {
	AssetIndex                     Variable
	LoanCollateralIndex            Variable
	LoanCollateralFlag             Variable
	MarginCollateralIndex          Variable
	MarginCollateralFlag           Variable
	PortfolioMarginCollateralIndex Variable
	PortfolioMarginCollateralFlag  Variable
}

// UserAssetMeta is the per-user, per-asset 5-tuple of raw uint64 values
// (Variables in-circuit). One entry per CEX asset slot — empty assets
// are zero entries, used for the CEX-total accumulation step.
type UserAssetMeta struct {
	Equity                    Variable
	Debt                      Variable
	LoanCollateral            Variable
	MarginCollateral          Variable
	PortfolioMarginCollateral Variable
}

// CreateUserOperation is one batch entry: the per-user delta applied
// to the account Merkle tree. Assets is the non-empty subset (padded
// to the BatchShape's AssetCountTier), AssetsForUpdateCex is the full
// 500-asset accumulation vector, AccountProof is the sibling path
// (length corespec.AccountTreeDepth).
type CreateUserOperation struct {
	BeforeAccountTreeRoot Variable
	AfterAccountTreeRoot  Variable
	Assets                []UserAssetInfo
	AssetsForUpdateCex    []UserAssetMeta
	AccountIndex          Variable
	AccountIdHash         Variable
	AccountProof          [corespec.AccountTreeDepth]Variable
}
