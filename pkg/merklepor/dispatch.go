// Model dispatch. The merklepor engine is model-blind in its build /
// reconcile / verify core; the only model-typed step is streaming the
// snapshot into sum-leaf records, reached through this single switch.
// Adding a model means adding a case plus the matching <model>/host
// collector — nothing else changes. The side line is T1-only by product
// scope (gate G19, D3), so the switch currently has one real case.

package merklepor

import (
	"context"
	"fmt"

	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
	"github.com/BetweenBits-org/zk-pos-ext/core/io/vfs"
	t1host "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t1_simple_margin/host"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
)

// collectDeps bundles the shared inputs the model-typed snapshot build +
// CollectSumLeaves invocation needs.
type collectDeps struct {
	ctx        context.Context
	sourceType string
	snapshot   vfs.Opener
	snapID     string
	capacity   int
	pricing    corespec.PriceScaleProvider
	tiers      []int
}

// dispatchCollectSumLeaves builds the model-typed snapshot and collects
// its dense sum-leaf records.
func dispatchCollectSumLeaves(model corespec.SolvencyModelID, d collectDeps) ([]corehost.SumLeafRecord, error) {
	switch model {
	case "t1_simple_margin":
		snap := t1host.NewSnapshot(d.sourceType, d.snapshot, d.snapID, d.capacity, d.pricing)
		return t1host.CollectSumLeaves(d.ctx, snap, d.tiers)
	default:
		return nil, fmt.Errorf("merklepor: unsupported model %q (Merkle-sum side line is T1-only — gate G19, D3)", model)
	}
}
