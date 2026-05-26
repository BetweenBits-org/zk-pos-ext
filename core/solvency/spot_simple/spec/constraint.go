package spec

import (
	corecircuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/circuit"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/consensys/gnark/frontend"
)

// ConstraintModule is the spot_simple-typed extension hook for adding
// stricter or exchange-specific zk-constraints on top of the base
// spot_simple circuit. Mirrors tier_3bucket/spec.ConstraintModule in
// shape; the model-specific difference is the ConstraintContext.
//
// Composing the base circuit with a module forks the trusted setup —
// the resulting .pk/.vk are unique to the (spot_simple, module) pair.
// Modules MUST publish source so verifiers can audit .vk semantics.
//
// Modules CANNOT remove or weaken base-circuit constraints — only add.
type ConstraintModule interface {
	// ID returns the stable filesystem-safe identifier for this
	// module, embedded into key file names alongside the model ID.
	ID() corespec.ConstraintModuleID

	// Define adds the module's constraints to the circuit. ctx exposes
	// a read-only view of the standard witness data at the point where
	// the module hook runs (after all base constraints have been
	// emitted).
	Define(api frontend.API, ctx ConstraintContext) error
}

// ConstraintContext is the read-only view of spot_simple standard
// witness data that ConstraintModule sees.
//
// Compared to tier_3bucket's ConstraintContext: no collateral / tier-
// ratio fields, no TotalUserDebt / TotalUserCollateralReal — spot
// users have no liabilities. Only TotalUserEquity is exposed.
type ConstraintContext struct {
	// BeforeCexAssets are the per-asset global totals as input to the
	// batch (length == deployment capacity).
	BeforeCexAssets []CircuitCexAsset

	// AfterCexAssets are the per-asset global totals after applying
	// all user operations in this batch.
	AfterCexAssets []CircuitCexAsset

	// UserOps is the per-user data the base circuit operated on.
	UserOps []CircuitUserOp

	// R is the shared range-checker. Modules SHOULD reuse it rather
	// than instantiate their own to keep total constraint count down.
	R frontend.Rangechecker
}

// CircuitCexAsset is the gnark Variable view of CexAssetInfo as
// exposed to constraint modules. Single-equity field plus BasePrice;
// no tier ratios.
type CircuitCexAsset struct {
	TotalEquity corecircuit.Variable
	BasePrice   corecircuit.Variable
}

// CircuitUserOp is the gnark Variable view of a per-user batch entry
// as exposed to constraint modules. Only the totals modules need at
// v0 — add fields as concrete use-cases emerge.
type CircuitUserOp struct {
	AccountIndex    corecircuit.Variable
	AccountIDHash   corecircuit.Variable
	TotalUserEquity corecircuit.Variable
}
