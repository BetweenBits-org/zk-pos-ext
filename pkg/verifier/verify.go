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
	"encoding/base64"
	"fmt"
	"runtime"
	"strconv"
	"sync"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
	"github.com/consensys/gnark/backend/groth16"
)

// verifyAllProofs runs the groth16 check for every proof row across a
// worker pool. Each row's public input is recomputed from its
// account-tree-roots and cex-commitments and checked against the
// embedded batch commitment before the proof is verified. Returns
// false if any proof fails.
func verifyAllProofs(proofs []corehost.ProofRow, r *resolved) bool {
	workersNum := max(16, runtime.NumCPU())
	averageProofCount := (len(proofs) + workersNum - 1) / workersNum

	var (
		wg sync.WaitGroup
		ok = true
		mu sync.Mutex
	)
	for w := range workersNum {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			startIndex := index * averageProofCount
			if startIndex >= len(proofs) {
				return
			}
			endIndex := min((index+1)*averageProofCount, len(proofs))
			if !verifyProofRange(proofs[startIndex:endIndex], r) {
				mu.Lock()
				ok = false
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
	return ok
}

// verifyProofRange verifies a contiguous slice of proof rows. The
// verifying key is (re)loaded only when the asset-count tier changes,
// matching the prover's tier-grouped ordering. The public witness is
// constructed via the model-specific runner so the verify circuit
// shape matches the .vk.
func verifyProofRange(rows []corehost.ProofRow, r *resolved) bool {
	var vk groth16.VerifyingKey
	currentAssetCountsTier := -1

	for j := range rows {
		row := rows[j]
		batchNumber := int(row.BatchNumber)

		proof := groth16.NewProof(ecc.BN254)
		proofRaw, err := base64.StdEncoding.DecodeString(row.ZkProof)
		if err != nil {
			fmt.Println("decode proof failed:", batchNumber)
			panic("verify proof " + strconv.Itoa(batchNumber) + " failed")
		}
		if _, err := proof.ReadFrom(bytes.NewBuffer(proofRaw)); err != nil {
			panic("verify proof " + strconv.Itoa(batchNumber) + " failed")
		}

		roots, commits := decodeBatchMetadata(row)

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
			fmt.Println("decode batch commitment failed", batchNumber)
			panic("verify proof " + strconv.Itoa(batchNumber) + " failed")
		}
		if !bytes.Equal(expectHash, actualHash) {
			fmt.Println("public input verify failed ", batchNumber)
			fmt.Printf("%x:%x\n", expectHash, actualHash)
			panic("verify proof " + strconv.Itoa(batchNumber) + " failed")
		}

		vWitness, err := dispatchNewVerifyPublicWitness(r.model, actualHash)
		if err != nil {
			panic(err.Error())
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
				panic("invalid asset counts tier")
			}
			vk, err = loadVerifyingKey(r.plan.ZkKeyStems[keyIndex] + ".vk")
			if err != nil {
				panic(err.Error())
			}
			currentAssetCountsTier = row.AssetsCount
		}

		if err := groth16.Verify(proof, vk, vWitness); err != nil {
			fmt.Println("proof verify failed:", batchNumber, err.Error())
			return false
		}
		fmt.Println("proof verify success", batchNumber)
	}
	return true
}

// chainCheck walks batches in order: each before-state must equal the
// prior after-state; the final after-CEX commitment must match the
// commitment of the published totals. Panics on any mismatch. Prints
// the final account tree root + success line on pass.
func chainCheck(
	proofs []corehost.ProofRow,
	prevAccountTreeRoots, prevCexAssetListCommitments [][]byte,
	expectFinalCexAssetsInfoComm []byte,
) {
	var accountTreeRoot, finalCexAssetsInfoComm []byte
	for batchNumber := range proofs {
		roots, commits := decodeBatchMetadata(proofs[batchNumber])
		if !bytes.Equal(roots[0], prevAccountTreeRoots[1]) {
			panic("account tree root not match: " + strconv.Itoa(batchNumber))
		}
		if !bytes.Equal(commits[0], prevCexAssetListCommitments[1]) {
			panic("cex asset list commitment not match: " + strconv.Itoa(batchNumber))
		}
		prevAccountTreeRoots = roots
		prevCexAssetListCommitments = commits
		accountTreeRoot = roots[1]
		finalCexAssetsInfoComm = commits[1]
	}

	if !bytes.Equal(finalCexAssetsInfoComm, expectFinalCexAssetsInfoComm) {
		panic("Final Cex Assets Info Not Match")
	}
	fmt.Printf("account merkle tree root is %x\n", accountTreeRoot)
	fmt.Println("All proofs verify passed!!!")
}
