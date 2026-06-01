// Model dispatch + tier-mismatch retry. The prover is model-blind in
// its outer Run loop; per-model decode + circuit-witness + Prove path
// is reached through runDecodeAndProve. dispatchDecodeAndProve adds
// the lazy snark-params cache + tier retry on top: if the first try
// fails because the cached tier doesn't match the witness's tier,
// reload each declared tier in turn until one succeeds.

package prover

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
)

// dispatchDecodeAndProve fronts the model-typed prover_runner with
// the lazy snark-params cache. The runner needs (r1cs, pk, vk) for
// its model's circuit; we pre-decode the first batch of each tier
// to discover the assetsCount, load params, and finally call the
// runner with the loaded params.
//
// To preserve the legacy single-decode behaviour we drive a two-pass
// strategy: if params.tier is already correct (hit), call the runner
// directly. On miss, load the params for *any* tier first (cmd loops
// in ascending tier order, so the first claimed batch is always the
// first tier), then call the runner.
//
// The lazy cache is correct as long as ascending tier order is
// preserved by the witness builder + DB ordering. R8/R10 witness
// runners both honor ascending tier order.
func dispatchDecodeAndProve(ctx context.Context, plan *resolved, params *snarkParams, keys vfs.KeyOpener, encoded []byte, batchNumber int64) (*corehost.BatchProofResult, error) {
	// On first call params.r1cs is nil — load the smallest tier
	// (assumed first by ascending order). Subsequent calls compare
	// against the cached tier; mismatch triggers a reload.
	if params.r1cs == nil {
		if err := loadSnarkParams(ctx, params, plan, keys, plan.assetCountTiers[0]); err != nil {
			return nil, err
		}
	}
	result, err := runDecodeAndProve(plan.model, encoded, params, plan.assetCountTiers, batchNumber)
	if err == nil {
		return result, nil
	}
	// Tier mismatch is signaled by the runner via an error message
	// containing the targetAssetsCount; the lazy-cache retry below
	// reloads and re-runs. The check is by substring rather than
	// a typed error to keep the runner interface model-blind.
	_ = err
	// Simpler retry strategy: try every declared tier in order until
	// one succeeds. Avoids parsing error messages.
	for _, tier := range plan.assetCountTiers {
		if tier == params.tier {
			continue
		}
		if err := loadSnarkParams(ctx, params, plan, keys, tier); err != nil {
			return nil, err
		}
		result, retryErr := runDecodeAndProve(plan.model, encoded, params, plan.assetCountTiers, batchNumber)
		if retryErr == nil {
			return result, nil
		}
	}
	return nil, fmt.Errorf("no declared tier produced a valid prove: %w", err)
}

// runDecodeAndProve dispatches to the model-typed runner.
func runDecodeAndProve(
	model corespec.SolvencyModelID,
	encoded []byte,
	params *snarkParams,
	assetCountTiers []int,
	batchNumber int64,
) (*corehost.BatchProofResult, error) {
	switch model {
	case "t1_simple_margin":
		return t1host.DecodeAndProve(encoded, params.r1cs, params.provingKey, params.verifyingKey, assetCountTiers, batchNumber)
	case "t2_static_haircut_margin":
		return t2host.DecodeAndProve(encoded, params.r1cs, params.provingKey, params.verifyingKey, assetCountTiers, batchNumber)
	case "t3_tiered_haircut_margin_1pool":
		return t3host.DecodeAndProve(encoded, params.r1cs, params.provingKey, params.verifyingKey, assetCountTiers, batchNumber)
	case "t4_tiered_haircut_margin_3pool":
		return t4host.DecodeAndProve(encoded, params.r1cs, params.provingKey, params.verifyingKey, assetCountTiers, batchNumber)
	default:
		return nil, fmt.Errorf("prover: unsupported model %q", model)
	}
}
