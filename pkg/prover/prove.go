// Per-batch claim → prove → persist body. proveOne is the single unit
// of work the Run loop drives one row at a time; persistProof writes
// the proof row and flips the witness status to Finished.
//
// Idempotency is enforced before any decode work: a proof row already
// at this height means a previous run produced it, so we just flip
// the witness status instead of re-proving.

package prover

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"
)

// proveOne handles one claimed batch: decode → prove → verify →
// persist proof → mark witness Finished. The idempotency probe
// (GetByBatchNumber before Create) makes a crashed retry safe:
// re-claim, see the proof already exists, just flip the witness
// status to Finished without re-proving.
func proveOne(
	row *store.BatchWitness,
	params *snarkParams,
	plan *resolved,
	witnessStore *store.WitnessStore,
	proofStore *store.ProofStore,
) error {
	encoded, err := base64.StdEncoding.DecodeString(row.WitnessData)
	if err != nil {
		return fmt.Errorf("base64 decode: %w", err)
	}

	// Idempotency probe: if a proof row already exists at this height,
	// skip Prove and just flip the witness status. Load snark params
	// for the row's assetsCount tier lazily — but we don't know the
	// tier until decode. Resolve by peeking the witness header? For
	// now we run decode + assertion regardless; lazy load handles
	// reuse across batches.
	if existing, err := proofStore.GetByBatchNumber(row.Height); err == nil {
		fmt.Printf("proof of height %d already exists (tier %d), marking witness finished\n", row.Height, existing.AssetsCount)
		return witnessStore.MarkStatus(row.Height, store.StatusFinished)
	} else if !errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("idempotency probe: %w", err)
	}

	// Decode + Prove via the model-typed runner. The runner picks
	// the per-user padded tier (targetAssetsCount) from the decoded
	// circuit witness, but we need it to drive lazy snark-params
	// loading. The cheapest path is: decode once via the runner,
	// match the tier into our plan, ensure params, then re-call the
	// runner with the cached params. To avoid double-decode we
	// pre-load params optimistically (first tier) and let the runner
	// fail-fast on tier mismatch. In practice the lazy cache flips
	// only at tier boundaries; the cache hit covers same-tier runs.
	result, err := dispatchDecodeAndProve(plan, params, encoded, row.Height)
	if err != nil {
		return fmt.Errorf("prove/verify: %w", err)
	}

	if err := persistProof(row, result, witnessStore, proofStore); err != nil {
		return fmt.Errorf("persist: %w", err)
	}
	return nil
}

// persistProof writes the proof row + flips the witness status.
// Idempotency check is done by proveOne before this is called.
func persistProof(
	row *store.BatchWitness,
	result *corehost.BatchProofResult,
	witnessStore *store.WitnessStore,
	proofStore *store.ProofStore,
) error {
	cexCommitments, err := json.Marshal([][]byte{
		result.BeforeCEXAssetsCommitment,
		result.AfterCEXAssetsCommitment,
	})
	if err != nil {
		return fmt.Errorf("marshal cex commitments: %w", err)
	}
	accountRoots, err := json.Marshal([][]byte{
		result.BeforeAccountTreeRoot,
		result.AfterAccountTreeRoot,
	})
	if err != nil {
		return fmt.Errorf("marshal account roots: %w", err)
	}

	if err := proofStore.Create(&store.Proof{
		ProofInfo:               base64.StdEncoding.EncodeToString(result.ProofRaw),
		BatchNumber:             row.Height,
		CexAssetListCommitments: string(cexCommitments),
		AccountTreeRoots:        string(accountRoots),
		BatchCommitment:         base64.StdEncoding.EncodeToString(result.BatchCommitment),
		AssetsCount:             result.AssetsCount,
	}); err != nil {
		return fmt.Errorf("create proof row: %w", err)
	}
	return witnessStore.MarkStatus(row.Height, store.StatusFinished)
}
