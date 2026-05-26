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
// form for the t2 model. Single Haircut Variable (basis points; vs
// T3's CollateralRatios slice).
type CexAssetInfo struct {
	TotalEquity Variable
	TotalDebt   Variable
	BasePrice   Variable
	Collateral  Variable
	Haircut     Variable
}

// UserAssetInfo is the per-user, per-asset record exposed inside the
// circuit. T2 carries no tier index — the haircut is a global
// constant per asset.
type UserAssetInfo struct {
	AssetIndex Variable
}

// UserAssetMeta is the per-user, per-asset 3-tuple of raw uint64
// values for the CEX-total accumulation step.
type UserAssetMeta struct {
	Equity     Variable
	Debt       Variable
	Collateral Variable
}

// CreateUserOperation is one batch entry: the per-user delta applied
// to the account Merkle tree.
type CreateUserOperation struct {
	BeforeAccountTreeRoot Variable
	AfterAccountTreeRoot  Variable
	Assets                []UserAssetInfo
	AssetsForUpdateCex    []UserAssetMeta
	AccountIndex          Variable
	AccountIdHash         Variable
	AccountProof          [corespec.AccountTreeDepth]Variable
}
