package host

import (
	t1spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t1_simple_margin/spec"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/consensys/gnark/frontend"
)

// noopConstraintModule is the engine-default ConstraintModule for the
// T1 model. Same shape as the T4 default — only the ConstraintContext
// type differs (no collateral / tier ratios on T1).
//
// Promoted from profile/t1_reference/constraint_noop.go in R8-B/3.
type noopConstraintModule struct{}

// NewNoopConstraint returns the engine-default no-op ConstraintModule
// for T1.
func NewNoopConstraint() t1spec.ConstraintModule { return noopConstraintModule{} }

func (noopConstraintModule) ID() corespec.ConstraintModuleID {
	return corespec.ConstraintModuleID(corespec.NoExtensionID)
}

func (noopConstraintModule) Define(api frontend.API, ctx t1spec.ConstraintContext) error {
	_ = api
	_ = ctx
	return nil
}
