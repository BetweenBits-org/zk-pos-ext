// groth16 verification loop + chain walk. verifyAllProofs fans the
// per-proof checks across a worker pool; verifyProofRange owns one
// contiguous slice and (re)loads the verifying key only at tier
// boundaries to match the prover's tier-grouped ordering. chainCheck
// walks the proof series in order and confirms each batch's
// before-state equals the prior after-state plus the final CEX
// commitment equals the published-totals commitment.

package verifier

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"runtime"
	"sync"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
	"github.com/consensys/gnark/backend/groth16"
)

// verifyAllProofs runs the groth16 check for every proof row across a
// worker pool. Each row's public input is recomputed from its
// account-tree-roots and cex-commitments and checked against the
// embedded batch commitment before the proof is verified. Returns the
// first error encountered across workers (deterministic ordering not
// guaranteed; the first arriving error wins). A cancelled ctx is a
// termination cause: each worker observes it between proofs and records
// ctx.Err(), so the pool unwinds without verifying the remaining rows.
func verifyAllProofs(ctx context.Context, proofs []corehost.ProofRow, r *resolved) error {
	workersNum := max(16, runtime.NumCPU())
	averageProofCount := (len(proofs) + workersNum - 1) / workersNum

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		failed error
	)
	recordErr := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		if failed == nil {
			failed = err
		}
	}
	for w := range workersNum {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			startIndex := index * averageProofCount
			if startIndex >= len(proofs) {
				return
			}
			endIndex := min((index+1)*averageProofCount, len(proofs))
			if err := verifyProofRange(ctx, proofs[startIndex:endIndex], r); err != nil {
				recordErr(err)
			}
		}(w)
	}
	wg.Wait()
	return failed
}

// verifyProofRange verifies a contiguous slice of proof rows. The
// verifying key is (re)loaded only when the asset-count tier changes,
// matching the prover's tier-grouped ordering. The public witness is
// constructed via the model-specific runner so the verify circuit
// shape matches the .vk. A cancelled ctx aborts the range between
// proofs, returning ctx.Err().
func verifyProofRange(ctx context.Context, rows []corehost.ProofRow, r *resolved) error {
	var vk groth16.VerifyingKey
	currentAssetCountsTier := -1

	for j := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		row := rows[j]
		batchNumber := int(row.BatchNumber)

		proof := groth16.NewProof(ecc.BN254)
		proofRaw, err := base64.StdEncoding.DecodeString(row.ZkProof)
		if err != nil {
			return fmt.Errorf("batch %d: decode proof: %w", batchNumber, err)
		}
		if _, err := proof.ReadFrom(bytes.NewBuffer(proofRaw)); err != nil {
			return fmt.Errorf("batch %d: read proof: %w", batchNumber, err)
		}

		roots, commits, err := decodeBatchMetadata(row)
		if err != nil {
			return fmt.Errorf("batch %d: decode metadata: %w", batchNumber, err)
		}

		// The public input is Poseidon(beforeRoot, afterRoot,
		// beforeCexCommit, afterCexCommit); it must equal the embedded
		// batch commitment.
		hasher := poseidon.NewPoseidon()
		hasher.Write(roots[0])
		hasher.Write(roots[1])
		hasher.Write(commits[0])
		hasher.Write(commits[1])
		expectHash := hasher.Sum(nil)
		actualHash, err := base64.StdEncoding.DecodeString(row.BatchCommitment)
		if err != nil {
			return fmt.Errorf("batch %d: decode batch commitment: %w", batchNumber, err)
		}
		if !bytes.Equal(expectHash, actualHash) {
			return fmt.Errorf("batch %d: public input mismatch: expect %x got %x", batchNumber, expectHash, actualHash)
		}

		vWitness, err := dispatchNewVerifyPublicWitness(r.model, actualHash)
		if err != nil {
			return fmt.Errorf("batch %d: build public witness: %w", batchNumber, err)
		}

		if row.AssetsCount != currentAssetCountsTier {
			keyIndex := -1
			for p := range r.plan.AssetCountTiers {
				if r.plan.AssetCountTiers[p] == row.AssetsCount {
					keyIndex = p
					break
				}
			}
			if keyIndex == -1 {
				return fmt.Errorf("batch %d: invalid asset counts tier %d", batchNumber, row.AssetsCount)
			}
			vk, err = loadVerifyingKey(r.plan.ZkKeyStems[keyIndex] + ".vk")
			if err != nil {
				return fmt.Errorf("batch %d: load verifying key: %w", batchNumber, err)
			}
			currentAssetCountsTier = row.AssetsCount
		}

		if err := groth16.Verify(proof, vk, vWitness); err != nil {
			return fmt.Errorf("batch %d: groth16 verify: %w", batchNumber, err)
		}
		fmt.Println("proof verify success", batchNumber)
	}
	return nil
}

// chainCheck walks batches in order: each before-state must equal the
// prior after-state; the final after-CEX commitment must match the
// commitment of the published totals. Returns an error on any
// mismatch. Prints the final account tree root + success line on pass.
func chainCheck(
	proofs []corehost.ProofRow,
	prevAccountTreeRoots, prevCexAssetListCommitments [][]byte,
	expectFinalCexAssetsInfoComm []byte,
) error {
	var accountTreeRoot, finalCexAssetsInfoComm []byte
	for batchNumber := range proofs {
		roots, commits, err := decodeBatchMetadata(proofs[batchNumber])
		if err != nil {
			return fmt.Errorf("batch %d: decode metadata: %w", batchNumber, err)
		}
		if !bytes.Equal(roots[0], prevAccountTreeRoots[1]) {
			return fmt.Errorf("batch %d: account tree root not match", batchNumber)
		}
		if !bytes.Equal(commits[0], prevCexAssetListCommitments[1]) {
			return fmt.Errorf("batch %d: cex asset list commitment not match", batchNumber)
		}
		prevAccountTreeRoots = roots
		prevCexAssetListCommitments = commits
		accountTreeRoot = roots[1]
		finalCexAssetsInfoComm = commits[1]
	}

	if !bytes.Equal(finalCexAssetsInfoComm, expectFinalCexAssetsInfoComm) {
		return fmt.Errorf("final cex assets info not match")
	}
	fmt.Printf("account merkle tree root is %x\n", accountTreeRoot)
	fmt.Println("All proofs verify passed!!!")
	return nil
}
