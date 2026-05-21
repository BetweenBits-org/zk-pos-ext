package binance

import (
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	modelspec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/spec"
	"github.com/consensys/gnark/frontend"
)

// noopModule is the reference Binance constraint module — adds no
// additional in-circuit constraints beyond the base tier_3bucket
// solvency logic. Key files generated with this module use the
// un-suffixed naming (see spec.NoExtensionID).
type noopModule struct{}

// NewNoopConstraint returns a ConstraintModule that adds no constraints.
func NewNoopConstraint() modelspec.ConstraintModule { return noopModule{} }

func (noopModule) ID() corespec.ConstraintModuleID {
	return corespec.ConstraintModuleID(corespec.NoExtensionID)
}

func (noopModule) Define(api frontend.API, ctx modelspec.ConstraintContext) error {
	return nil
}
