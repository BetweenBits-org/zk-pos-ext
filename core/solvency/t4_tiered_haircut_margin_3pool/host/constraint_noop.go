package host

import (
	t4spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/consensys/gnark/frontend"
)

// noopConstraintModule is the engine-default ConstraintModule for the
// T4 model — adds no in-circuit constraints beyond the base
// t4_tiered_haircut_margin_3pool solvency logic. Key files generated with this
// module use the un-suffixed naming (see corespec.NoExtensionID).
//
// Promoted from profile/t4_reference/constraint_noop.go in R8-B/3 so
// service startup can resolve "no extension" without depending on a
// customer profile package.
type noopConstraintModule struct{}

// NewNoopConstraint returns the engine-default no-op ConstraintModule
// for T4. Service startup uses this when profile.toml's
// constraint.module is empty (the v1 catalog default).
func NewNoopConstraint() t4spec.ConstraintModule { return noopConstraintModule{} }

func (noopConstraintModule) ID() corespec.ConstraintModuleID {
	return corespec.ConstraintModuleID(corespec.NoExtensionID)
}

func (noopConstraintModule) Define(api frontend.API, ctx t4spec.ConstraintContext) error {
	_ = api
	_ = ctx
	return nil
}
