// Package spec declares the data shapes and interfaces specific to
// the t3_tiered_haircut_margin_1pool solvency model: tier-based
// piecewise-linear haircut over a SINGLE collateral pool. T4's
// 3-bucket split (Loan / Margin / PortfolioMargin) is collapsed —
// every user-asset has one Collateral field, one tier curve per asset.
//
// Solvency claim:
//
//	∀ user :  Σ_i haircut_curve_i(collateral_i) ≥ user.TotalDebt
//
// where haircut_curve_i is the per-asset piecewise-linear curve
// supplied by RiskPolicy.CollateralRatios (matches T4's curve shape).
//
// AccountLeaf: Poseidon(AccountID, TotalEquity, TotalDebt,
// TotalCollateral, AssetsCommitment) — universal 5-input signature
// shared by every model (see docs/04-solvency-models.md §3).
//
// Differences vs t4_tiered_haircut_margin_3pool/spec (intentional
// simplifications):
//   - AccountAsset has Index + Equity + Debt + Collateral (4-tuple
//     instead of T4's 5-tuple Loan/Margin/PortfolioMargin)
//   - CexAssetInfo carries a single Collateral aggregate + single
//     CollateralRatios slice (vs T4's three of each)
//   - RiskPolicy returns one ratios slice per asset (vs T4's three)
//
// Reference industry use case: DeFi perp DEX with size-tiered initial
// margin curves (dYdX IMF) over a single cross-margin collateral pool
// — see docs/04-solvency-models.md §6.
package spec

import "math/big"

// AccountAsset is the per-user, per-asset 4-tuple of the
// t3_tiered_haircut_margin_1pool model.
type AccountAsset struct {
	Index      uint16
	Equity     uint64
	Debt       uint64
	Collateral uint64
}

// AccountInfo is the per-user record in the t3 model. Same shape as
// T4's AccountInfo at the leaf level (universal 5-input Poseidon).
type AccountInfo struct {
	AccountIndex    uint32
	AccountID       []byte
	TotalEquity     *big.Int
	TotalDebt       *big.Int
	TotalCollateral *big.Int
	Assets          []AccountAsset
}

// TierRatio is a single tier in the piecewise-linear haircut model —
// identical shape to t4's TierRatio (universal tier-curve datum).
type TierRatio struct {
	BoundaryValue    *big.Int
	Ratio            uint8
	PrecomputedValue *big.Int
}

// CexAssetInfo is the per-asset global state in the t3 model. Single
// Collateral aggregate + single CollateralRatios slice (vs T4's three
// of each).
type CexAssetInfo struct {
	TotalEquity      uint64
	TotalDebt        uint64
	BasePrice        uint64
	Symbol           string
	Index            uint32
	Collateral       uint64
	CollateralRatios []TierRatio
}
