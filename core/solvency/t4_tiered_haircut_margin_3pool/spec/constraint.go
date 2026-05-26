package spec

import (
	corecircuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/circuit"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/consensys/gnark/frontend"
)

// ConstraintModule is the t4_tiered_haircut_margin_3pool-typed extension hook for
// adding stricter or exchange-specific zk-constraints on top of the
// base t4_tiered_haircut_margin_3pool circuit.
//
// Modules see the witness data exposed by the base circuit and add
// constraints that enforce additional rules (concentration limits,
// stablecoin segregation, KYC-tier caps, regulator requirements, ...).
//
// Composing the base circuit with a module forks the trusted setup —
// the resulting .pk/.vk are unique to the (t4_tiered_haircut_margin_3pool, module)
// pair. Modules MUST publish source so verifiers can audit .vk
// semantics.
//
// Modules CANNOT remove or weaken base-circuit constraints — only add.
type ConstraintModule interface {
	// ID returns the stable filesystem-safe identifier for this
	// module, embedded into key file names alongside the model ID.
	ID() corespec.ConstraintModuleID

	// Define adds the module's constraints to the circuit. ctx
	// exposes a read-only view of the standard witness data at the
	// point where the module hook runs (after all base constraints
	// have been emitted).
	Define(api frontend.API, ctx ConstraintContext) error
}

// ConstraintContext is the read-only view of t4_tiered_haircut_margin_3pool standard
// witness data that ConstraintModule sees.
//
// Field names mirror the variables inside the base circuit's Define()
// at the module hook point. AfterCexAssets reflects values *after*
// per-user accumulation completes; BeforeCexAssets is the unchanged
// snapshot input.
type ConstraintContext struct {
	// BeforeCexAssets are the per-asset global totals as input to
	// the batch (one entry per AssetCatalog index, length == AssetCounts).
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
// exposed to constraint modules. Mirrors the in-circuit type used
// by the base circuit.
type CircuitCexAsset struct {
	TotalEquity               corecircuit.Variable
	TotalDebt                 corecircuit.Variable
	BasePrice                 corecircuit.Variable
	LoanCollateral            corecircuit.Variable
	MarginCollateral          corecircuit.Variable
	PortfolioMarginCollateral corecircuit.Variable

	LoanRatios            []CircuitTierRatio
	MarginRatios          []CircuitTierRatio
	PortfolioMarginRatios []CircuitTierRatio
}

// CircuitTierRatio mirrors a single tier-ratio entry inside the
// base circuit.
type CircuitTierRatio struct {
	BoundaryValue    corecircuit.Variable
	Ratio            corecircuit.Variable
	PrecomputedValue corecircuit.Variable
}

// CircuitUserOp is the gnark Variable view of a per-user batch entry
// as exposed to constraint modules. Per-asset per-user values are
// intentionally not exposed at v0 to keep the module surface small;
// add fields here as concrete module use-cases emerge.
type CircuitUserOp struct {
	AccountIndex            corecircuit.Variable
	AccountIDHash           corecircuit.Variable
	TotalUserEquity         corecircuit.Variable
	TotalUserDebt           corecircuit.Variable
	TotalUserCollateralReal corecircuit.Variable
}
