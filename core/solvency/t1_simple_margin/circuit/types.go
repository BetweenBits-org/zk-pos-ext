package circuit

import (
	corecircuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/circuit"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// Type aliases so this package doesn't need to import gnark/frontend
// just to satisfy in-circuit signatures.
type (
	API      = corecircuit.API
	Variable = corecircuit.Variable
)

// CexAssetInfo is the per-asset CEX global state in gnark-Variable
// form for the t1_simple_margin model. TotalEquity + TotalDebt +
// BasePrice; no collateral, no tier ratios.
type CexAssetInfo struct {
	TotalEquity Variable
	TotalDebt   Variable
	BasePrice   Variable
}

// UserAssetInfo is the per-user, per-asset record exposed inside the
// circuit. T1 carries (Equity, Debt); no collateral tier index/flag
// needed (vs t4_tiered_haircut_margin_3pool's UserAssetInfo).
type UserAssetInfo struct {
	AssetIndex Variable
	Equity     Variable
	Debt       Variable
}

// UserAssetMeta is the per-user, per-asset slot in the
// AssetsForUpdateCex accumulation vector. One entry per CEX asset
// slot — empty assets are zero entries used for sum-equality
// linear-combination across the batch.
type UserAssetMeta struct {
	Equity Variable
	Debt   Variable
}

// CreateUserOperation is one batch entry: the per-user delta applied
// to the account Merkle tree. Assets is the non-empty subset (padded
// to the BatchShape's AssetCountTier), AssetsForUpdateCex is the full
// capacity accumulation vector, AccountProof is the sibling path
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
