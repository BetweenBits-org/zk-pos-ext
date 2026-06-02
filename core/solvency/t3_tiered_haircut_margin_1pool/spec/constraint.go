package spec

import (
	corecircuit "github.com/BetweenBits-org/zk-pos-ext/core/circuit"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/consensys/gnark/frontend"
)

// ConstraintModule is the t3-typed extension hook. Same alpha-layer
// interface as t1 / t4; the model-specific difference is the
// ConstraintContext field set.
//
// Modules CANNOT remove or weaken base-circuit constraints — only add.
type ConstraintModule interface {
	ID() corespec.ConstraintModuleID
	Define(api frontend.API, ctx ConstraintContext) error
}

// ConstraintContext is the read-only view of t3 standard witness
// data that ConstraintModule sees. Single Collateral aggregate (vs
// T4's three buckets).
type ConstraintContext struct {
	BeforeCexAssets []CircuitCexAsset
	AfterCexAssets  []CircuitCexAsset
	UserOps         []CircuitUserOp
	R               frontend.Rangechecker
}

// CircuitCexAsset is the gnark Variable view of CexAssetInfo as
// exposed to constraint modules.
type CircuitCexAsset struct {
	TotalEquity      corecircuit.Variable
	TotalDebt        corecircuit.Variable
	BasePrice        corecircuit.Variable
	Collateral       corecircuit.Variable
	CollateralRatios []CircuitTierRatio
}

// CircuitTierRatio mirrors a single tier-ratio entry.
type CircuitTierRatio struct {
	BoundaryValue    corecircuit.Variable
	Ratio            corecircuit.Variable
	PrecomputedValue corecircuit.Variable
}

// CircuitUserOp is the gnark Variable view of a per-user batch entry.
type CircuitUserOp struct {
	AccountIndex            corecircuit.Variable
	AccountIDHash           corecircuit.Variable
	TotalUserEquity         corecircuit.Variable
	TotalUserDebt           corecircuit.Variable
	TotalUserCollateralReal corecircuit.Variable
}
