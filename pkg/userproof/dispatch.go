// Model dispatch. The userproof engine is model-blind in Run; per-model
// snapshot construction + RunUserProof invocation lives here. The
// runner's first two return values (account count + tree root) are
// discarded — the caller only cares about runner success/failure.

package userproof

import (
	"context"
	"fmt"

	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
	"github.com/BetweenBits-org/zk-pos-ext/core/io/vfs"
	t1host "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t1_simple_margin/host"
	t2host "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t2_static_haircut_margin/host"
	t3host "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t3_tiered_haircut_margin_1pool/host"
	t4host "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t4_tiered_haircut_margin_3pool/host"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	bsmt "github.com/bnb-chain/zkbnb-smt"
)

// dispatchInput bundles every shared dependency the model-typed
// snapshot + RunUserProof invocation needs.
type dispatchInput struct {
	model           corespec.SolvencyModelID
	ctx             context.Context
	sourceType      string
	snapshot        vfs.Opener
	snapID          string
	capacity        int
	pricing         corespec.PriceScaleProvider
	accountTree     bsmt.SparseMerkleTree
	userProofStore  corehost.UserProofStore
	shapeProvider   corespec.BatchShapeProvider
	assetCountTiers []int
}

// dispatchRunUserProof builds the model-typed snapshot and invokes
// the matching <model>/host.RunUserProof. The host runners hold the
// model-typed bucketing + per-account proof emission; this dispatch
// just plugs shared deps in.
func dispatchRunUserProof(d dispatchInput) error {
	switch d.model {
	case "t1_simple_margin":
		snapshot := t1host.NewSnapshot(d.sourceType, d.snapshot, d.snapID, d.capacity, d.pricing)
		_, _, err := t1host.RunUserProof(t1host.UserProofRunnerConfig{
			Ctx:             d.ctx,
			Snapshot:        snapshot,
			AccountTree:     d.accountTree,
			UserProofStore:  d.userProofStore,
			ShapeProvider:   d.shapeProvider,
			AssetCountTiers: d.assetCountTiers,
		})
		return err
	case "t2_static_haircut_margin":
		snapshot := t2host.NewSnapshot(d.sourceType, d.snapshot, d.snapID, d.capacity, d.pricing)
		_, _, err := t2host.RunUserProof(t2host.UserProofRunnerConfig{
			Ctx:             d.ctx,
			Snapshot:        snapshot,
			AccountTree:     d.accountTree,
			UserProofStore:  d.userProofStore,
			ShapeProvider:   d.shapeProvider,
			AssetCountTiers: d.assetCountTiers,
		})
		return err
	case "t3_tiered_haircut_margin_1pool":
		snapshot := t3host.NewSnapshot(d.sourceType, d.snapshot, d.snapID, d.capacity, d.pricing)
		_, _, err := t3host.RunUserProof(t3host.UserProofRunnerConfig{
			Ctx:             d.ctx,
			Snapshot:        snapshot,
			AccountTree:     d.accountTree,
			UserProofStore:  d.userProofStore,
			ShapeProvider:   d.shapeProvider,
			AssetCountTiers: d.assetCountTiers,
		})
		return err
	case "t4_tiered_haircut_margin_3pool":
		snapshot := t4host.NewSnapshot(d.sourceType, d.snapshot, d.snapID, d.capacity, d.pricing)
		_, _, err := t4host.RunUserProof(t4host.UserProofRunnerConfig{
			Ctx:             d.ctx,
			Snapshot:        snapshot,
			AccountTree:     d.accountTree,
			UserProofStore:  d.userProofStore,
			ShapeProvider:   d.shapeProvider,
			AssetCountTiers: d.assetCountTiers,
		})
		return err
	default:
		return fmt.Errorf("userproof: unsupported solvency model %q", d.model)
	}
}
