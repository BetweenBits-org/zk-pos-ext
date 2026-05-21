// Package spec declares the data shapes and interfaces specific to
// the tier_3bucket solvency model: tier-based collateral haircut,
// 3-bucket collateral (Loan / Margin / PortfolioMargin), 5-tuple
// per-asset user state.
//
// Types here MIRROR the existing src/utils types so the legacy
// implementation can be migrated incrementally. Once the new packages
// are wired into all services, the legacy types should be removed.
package spec

import "math/big"

// AccountAsset is the per-user, per-asset 5-tuple of the tier_3bucket
// model. Mirrors utils.AccountAsset.
type AccountAsset struct {
	Index           uint16
	Equity          uint64
	Debt            uint64
	Loan            uint64
	Margin          uint64
	PortfolioMargin uint64
}

// AccountInfo is the per-user record in the tier_3bucket model.
// Mirrors utils.AccountInfo.
type AccountInfo struct {
	AccountIndex    uint32
	AccountID       []byte
	TotalEquity     *big.Int
	TotalDebt       *big.Int
	TotalCollateral *big.Int
	Assets          []AccountAsset
}

// TierRatio is a single tier in the piecewise-linear haircut model.
// Mirrors utils.TierRatio.
type TierRatio struct {
	BoundaryValue    *big.Int
	Ratio            uint8
	PrecomputedValue *big.Int
}

// CexAssetInfo is the per-asset global state in the tier_3bucket model.
// Mirrors utils.CexAssetInfo.
type CexAssetInfo struct {
	TotalEquity               uint64
	TotalDebt                 uint64
	BasePrice                 uint64
	Symbol                    string
	Index                     uint32
	LoanCollateral            uint64
	MarginCollateral          uint64
	PortfolioMarginCollateral uint64
	LoanRatios                []TierRatio
	MarginRatios              []TierRatio
	PortfolioMarginRatios     []TierRatio
}
