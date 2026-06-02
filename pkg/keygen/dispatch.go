// Model dispatch. The keygen engine is model-blind in Run; per-model
// circuit construction lives here. Adding a new solvency model means
// adding a case here plus the matching <model>/circuit package —
// nothing else in this package changes.

package keygen

import (
	"fmt"

	t1circuit "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t1_simple_margin/circuit"
	t2circuit "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t2_static_haircut_margin/circuit"
	t3circuit "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t3_tiered_haircut_margin_1pool/circuit"
	t4circuit "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t4_tiered_haircut_margin_3pool/circuit"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/consensys/gnark/frontend"
)

// newCircuit returns the gnark circuit for the model at the given
// shape. Each model has its own BatchCreateUserCircuit constructor;
// adding a new model means extending this switch.
func newCircuit(model corespec.SolvencyModelID, s corespec.BatchShape, assetCapacity int) (frontend.Circuit, error) {
	switch model {
	case "t1_simple_margin":
		return t1circuit.NewBatchCreateUserCircuit(
			uint32(s.AssetCountTier),
			uint32(assetCapacity),
			uint32(s.UsersPerBatch),
		), nil
	case "t2_static_haircut_margin":
		return t2circuit.NewBatchCreateUserCircuit(
			uint32(s.AssetCountTier),
			uint32(assetCapacity),
			uint32(s.UsersPerBatch),
		), nil
	case "t3_tiered_haircut_margin_1pool":
		return t3circuit.NewBatchCreateUserCircuit(
			uint32(s.AssetCountTier),
			uint32(assetCapacity),
			uint32(s.UsersPerBatch),
		), nil
	case "t4_tiered_haircut_margin_3pool":
		return t4circuit.NewBatchCreateUserCircuit(
			uint32(s.AssetCountTier),
			uint32(assetCapacity),
			uint32(s.UsersPerBatch),
		), nil
	default:
		return nil, fmt.Errorf("keygen: unsupported solvency model %q (add a case in newCircuit)", model)
	}
}
