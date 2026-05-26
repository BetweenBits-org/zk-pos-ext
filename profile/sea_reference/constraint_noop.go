package sea_reference

import (
	t1spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/consensys/gnark/frontend"
)

// noopModule is the reference sea_reference constraint module — adds
// no additional in-circuit constraints beyond the base t1_simple_margin
// solvency logic. Key files generated with this module use the
// un-suffixed naming (see corespec.NoExtensionID).
//
// Same pattern as profile/binance/constraint_noop.go; spot-typed
// ConstraintContext is the model-specific difference. Whether to
// promote noop to a universal core/constraint_modules/noop is
// reassessed at R6 — current consensus is profile-local since
// ConstraintContext field sets differ across models.
type noopModule struct{}

// NewNoopConstraint returns a ConstraintModule that adds no constraints.
func NewNoopConstraint() t1spec.ConstraintModule { return noopModule{} }

func (noopModule) ID() corespec.ConstraintModuleID {
	return corespec.ConstraintModuleID(corespec.NoExtensionID)
}

func (noopModule) Define(api frontend.API, ctx t1spec.ConstraintContext) error {
	_ = api
	_ = ctx
	return nil
}
