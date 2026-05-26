// Package spec declares the data shapes and interfaces specific to
// the t2_static_haircut_margin solvency model: per-asset *static*
// (non-tiered) haircut over a single collateral pool. T3's
// piecewise-linear tier curve is collapsed to a single Haircut basis
// points value per asset.
//
// Solvency claim:
//
//	∀ user :  Σ_i (collateral_i × haircut_i / 10000) ≥ user.TotalDebt
//
// where haircut_i is the per-asset constant supplied by
// RiskPolicy.Haircut (expressed in basis points: 10000 = 100% =
// no haircut, 9000 = 90% = 10% haircut).
//
// AccountLeaf: Poseidon(AccountID, TotalEquity, TotalDebt,
// TotalCollateral, AssetsCommitment) — universal 5-input signature
// shared by every model.
//
// Differences vs t3_tiered_haircut_margin_1pool/spec (intentional
// simplifications):
//   - No tier table — single Haircut value per asset
//   - No TierRatio type
//   - RiskPolicy.Haircut returns uint16 (basis points) per asset
//   - CexAssetInfo carries Haircut (uint16) instead of CollateralRatios
//
// Reference industry use case: Aave V3 per-asset LTV / Liquidation
// Threshold constants (see docs/04-solvency-models.md §5).
package spec

import "math/big"

// AccountAsset is the per-user, per-asset 4-tuple of the
// t2_static_haircut_margin model — identical to T3.
type AccountAsset struct {
	Index      uint16
	Equity     uint64
	Debt       uint64
	Collateral uint64
}

// AccountInfo is the per-user record in the t2 model. Same shape as
// every other v1 catalog model at the leaf level.
type AccountInfo struct {
	AccountIndex    uint32
	AccountID       []byte
	TotalEquity     *big.Int
	TotalDebt       *big.Int
	TotalCollateral *big.Int
	Assets          []AccountAsset
}

// CexAssetInfo is the per-asset global state in the t2 model.
// Haircut is expressed in basis points (10000 = 100%, 9000 = 90%).
//
// The constant-haircut formulation makes T2's circuit cheaper than
// T3's by avoiding tier-curve lookup tables — a single multiply per
// asset suffices.
type CexAssetInfo struct {
	TotalEquity uint64
	TotalDebt   uint64
	BasePrice   uint64
	Symbol      string
	Index       uint32
	Collateral  uint64
	Haircut     uint16
}
