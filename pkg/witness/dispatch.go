// Model dispatch. The witness engine is model-blind in Run; per-model
// snapshot construction + RunWitness invocation lives here. Adding a
// new solvency model means adding a case here plus the matching
// <model>/host runner — nothing else in this package changes.

package witness

import (
	"context"
	"fmt"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/io/vfs"
	t1host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/host"
	t2host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t2_static_haircut_margin/host"
	t3host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t3_tiered_haircut_margin_1pool/host"
	t4host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	bsmt "github.com/bnb-chain/zkbnb-smt"
)

// dispatchInput bundles every shared dependency the model-typed
// snapshot + RunWitness invocation needs.
type dispatchInput struct {
	model           corespec.SolvencyModelID
	ctx             context.Context
	sourceType      string
	snapshot        vfs.Opener
	snapID          string
	capacity        int
	pricing         corespec.PriceScaleProvider
	accountTree     bsmt.SparseMerkleTree
	witnessStore    corehost.WitnessQueue
	shapeProvider   corespec.BatchShapeProvider
	assetCountTiers []int
	dumpFinalCex    string
}

// dispatchRunWitness builds the model-typed snapshot and invokes the
// matching <model>/host.RunWitness. The host runners hold the
// model-typed bucketing + commitment math; this dispatch just plugs
// shared deps in.
func dispatchRunWitness(d dispatchInput) error {
	switch d.model {
	case "t1_simple_margin":
		snapshot := t1host.NewSnapshot(d.sourceType, d.snapshot, d.snapID, d.capacity, d.pricing)
		return t1host.RunWitness(t1host.WitnessRunnerConfig{
			Ctx:             d.ctx,
			Snapshot:        snapshot,
			AccountTree:     d.accountTree,
			WitnessStore:    d.witnessStore,
			ShapeProvider:   d.shapeProvider,
			AssetCountTiers: d.assetCountTiers,
			DumpFinalCex:    d.dumpFinalCex,
		})
	case "t2_static_haircut_margin":
		snapshot := t2host.NewSnapshot(d.sourceType, d.snapshot, d.snapID, d.capacity, d.pricing)
		return t2host.RunWitness(t2host.WitnessRunnerConfig{
			Ctx:             d.ctx,
			Snapshot:        snapshot,
			AccountTree:     d.accountTree,
			WitnessStore:    d.witnessStore,
			ShapeProvider:   d.shapeProvider,
			AssetCountTiers: d.assetCountTiers,
			DumpFinalCex:    d.dumpFinalCex,
		})
	case "t3_tiered_haircut_margin_1pool":
		snapshot := t3host.NewSnapshot(d.sourceType, d.snapshot, d.snapID, d.capacity, d.pricing)
		return t3host.RunWitness(t3host.WitnessRunnerConfig{
			Ctx:             d.ctx,
			Snapshot:        snapshot,
			AccountTree:     d.accountTree,
			WitnessStore:    d.witnessStore,
			ShapeProvider:   d.shapeProvider,
			AssetCountTiers: d.assetCountTiers,
			DumpFinalCex:    d.dumpFinalCex,
		})
	case "t4_tiered_haircut_margin_3pool":
		snapshot := t4host.NewSnapshot(d.sourceType, d.snapshot, d.snapID, d.capacity, d.pricing)
		return t4host.RunWitness(t4host.WitnessRunnerConfig{
			Ctx:             d.ctx,
			Snapshot:        snapshot,
			AccountTree:     d.accountTree,
			WitnessStore:    d.witnessStore,
			ShapeProvider:   d.shapeProvider,
			AssetCountTiers: d.assetCountTiers,
			DumpFinalCex:    d.dumpFinalCex,
		})
	default:
		return fmt.Errorf("witness: unsupported solvency model %q", d.model)
	}
}
