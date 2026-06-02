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
// gnark-Variable form. Identical to t4's TierRatio.
type TierRatio struct {
	BoundaryValue    Variable
	Ratio            Variable
	PrecomputedValue Variable
}

// CexAssetInfo is the per-asset CEX global state in gnark-Variable
// form for the t3 model. SINGLE Collateral aggregate + SINGLE
// CollateralRatios slice (vs T4's three of each).
//
// CollateralRatios MUST have length corespec.TierCount; the circuit
// packs two TierRatios into one field element so an odd length is
// invalid.
type CexAssetInfo struct {
	TotalEquity      Variable
	TotalDebt        Variable
	BasePrice        Variable
	Collateral       Variable
	CollateralRatios []TierRatio
}

// UserAssetInfo is the per-user, per-asset *index* witness for one
// collateral bucket: which tier the user's collateral falls into.
// Single CollateralIndex/Flag pair (vs T4's three pairs).
type UserAssetInfo struct {
	AssetIndex      Variable
	CollateralIndex Variable
	CollateralFlag  Variable
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
