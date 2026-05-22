// Command verifier is the zkpor-native proof of solvency verifier for
// the Binance deployment. It is the zkpor/cmd replacement for legacy
// src/verifier — same three CLI modes, same on-disk inputs, but wired
// entirely through zkpor packages (no src/utils, no legacy circuit/).
//
// Modes:
//
//	verifier                 batch mode — verify every proof in the
//	                         proof table chains correctly and the final
//	                         CEX commitment matches the published totals
//	verifier -user           single-user inclusion verification against
//	                         config/user_config.json
//	verifier -hash A B       print Poseidon(A, B) for two base64 inputs
//
// The verifier never solves a witness, so it registers no solver hints;
// groth16.Verify consumes only the proving artifacts and public input.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"

	vconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/cmd/verifier/config"
	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	tier3circuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/circuit"
	tier3host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/host"
	tier3spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/binance"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/gocarina/gocsv"
)

// emptyAccountTreeRootHex is the root of a fully empty depth-28 sparse
// Merkle account tree (every leaf the empty-leaf hash). The first
// batch's before-account-root must equal this. Pinned by the engine
// standard (corespec.AccountTreeDepth); mirrors the legacy verifier
// constant.
const emptyAccountTreeRootHex = "08696bfcb563a2ee4dde9e1dbd34f68d3f4643df6e3709cdb1855c9f886240c7"

// proofRow is one record of the prover-produced proof table.
type proofRow struct {
	BatchNumber        int64    `csv:"batch_number"`
	ZkProof            string   `csv:"proof_info"`
	CexAssetCommitment []string `csv:"cex_asset_list_commitments"`
	AccountTreeRoots   []string `csv:"account_tree_roots"`
	BatchCommitment    string   `csv:"batch_commitment"`
	AssetsCount        int      `csv:"assets_count"`
}

func main() {
	userFlag := flag.Bool("user", false, "verify a single user's inclusion proof")
	hashFlag := flag.Bool("hash", false, "print Poseidon hash of two base64 arguments")
	flag.Parse()

	switch {
	case *userFlag:
		runUserVerification()
	case *hashFlag:
		runHash(flag.Args())
	default:
		runBatchVerification()
	}
}

// loadVerifyingKey reads a groth16 BN254 verifying key from disk.
func loadVerifyingKey(path string) (groth16.VerifyingKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	vk := groth16.NewVerifyingKey(ecc.BN254)
	if _, err := vk.ReadFrom(bytes.NewBuffer(raw)); err != nil {
		return nil, err
	}
	return vk, nil
}

// assetCountTiers returns the Binance deployment's per-batch asset
// count tiers (ascending), sourced from the profile's batch-shape
// provider. Used to pad a single user's asset list to the committed
// tier when recomputing their leaf hash.
func assetCountTiers() []int {
	shapes := binance.NewBatchShape().Shapes()
	tiers := make([]int, len(shapes))
	for i, s := range shapes {
		tiers[i] = s.AssetCountTier
	}
	return tiers
}

// runUserVerification recomputes a single account's leaf hash from
// config/user_config.json and checks it against the published account
// tree root via the Merkle path. This is the engine-side primitive a
// customer's self-inclusion UI would wrap (the UI itself is out of V1
// scope).
func runUserVerification() {
	content, err := os.ReadFile("config/user_config.json")
	if err != nil {
		panic(err.Error())
	}
	userConfig := &vconfig.UserConfig{}
	if err := json.Unmarshal(content, userConfig); err != nil {
		panic(err.Error())
	}

	root, err := hex.DecodeString(userConfig.Root)
	if err != nil || len(root) != 32 {
		panic("invalid account tree root")
	}

	proof := make([][]byte, 0, len(userConfig.Proof))
	for i := range userConfig.Proof {
		p, err := base64.StdEncoding.DecodeString(userConfig.Proof[i])
		if err != nil || len(p) != 32 {
			panic("invalid proof")
		}
		proof = append(proof, p)
	}

	assetCommitment := tier3host.ComputeUserAssetsCommitment(userConfig.Assets, assetCountTiers())

	accountIDHash, err := hex.DecodeString(userConfig.AccountIdHash)
	if err != nil || len(accountIDHash) != 32 {
		panic("the AccountIdHash is invalid")
	}
	accountHash := poseidon.PoseidonBytes(
		accountIDHash,
		userConfig.TotalEquity.Bytes(),
		userConfig.TotalDebt.Bytes(),
		userConfig.TotalCollateral.Bytes(),
		assetCommitment,
	)
	fmt.Println("user merkle leave hash base64 encode: ", base64.StdEncoding.EncodeToString(accountHash))
	fmt.Printf("user merkle leave hash hex encode: %x\n", accountHash)

	if corehost.VerifyMerkleProof(root, userConfig.AccountIndex, proof, accountHash) {
		fmt.Println("verify pass!!!")
	} else {
		fmt.Println("verify failed...")
	}
}

// runHash prints Poseidon(arg0, arg1) for two base64-encoded inputs.
func runHash(args []string) {
	if len(args) != 2 {
		panic("invalid hash command, it needs two arguments")
	}
	hasher := poseidon.NewPoseidon()
	p0, err := base64.StdEncoding.DecodeString(args[0])
	if err != nil {
		panic("invalid hash command, the first argument is not base64 encoded")
	}
	p1, err := base64.StdEncoding.DecodeString(args[1])
	if err != nil {
		panic("invalid hash command, the second argument is not base64 encoded")
	}
	hasher.Write(p0)
	hasher.Write(p1)
	res := hasher.Sum(nil)
	fmt.Printf("hash result base64 encode: %s\n", base64.StdEncoding.EncodeToString(res))
	fmt.Printf("hash result hex encode: %x\n", res)
}

// runBatchVerification verifies every proof in the proof table: each
// proof's groth16 check passes, consecutive batches chain (batch i's
// after-state is batch i+1's before-state), the first batch starts
// from the empty tree, and the final CEX commitment equals the
// commitment of the published totals.
func runBatchVerification() {
	content, err := os.ReadFile("config/config.json")
	if err != nil {
		panic(err.Error())
	}
	verifierConfig := &vconfig.Config{}
	if err := json.Unmarshal(content, verifierConfig); err != nil {
		panic(err.Error())
	}

	f, err := os.Open(verifierConfig.ProofTable)
	if err != nil {
		panic(err.Error())
	}
	defer f.Close()

	tmpProofs := []*proofRow{}
	if err := gocsv.UnmarshalFile(f, &tmpProofs); err != nil {
		panic(err.Error())
	}
	proofs := make([]proofRow, len(tmpProofs))
	for i := range tmpProofs {
		proofs[tmpProofs[i].BatchNumber] = *tmpProofs[i]
	}

	emptyAccountTreeRoot, err := hex.DecodeString(emptyAccountTreeRootHex)
	if err != nil {
		panic("wrong empty account tree root")
	}

	// Index the published CEX totals by their declared asset index and
	// enforce the per-asset equity >= debt floor before computing the
	// expected final commitment.
	cexAssetsInfo := make([]tier3spec.CexAssetInfo, len(verifierConfig.CexAssetsInfo))
	for i := range verifierConfig.CexAssetsInfo {
		entry := verifierConfig.CexAssetsInfo[i]
		cexAssetsInfo[entry.Index] = entry
		if entry.TotalEquity < entry.TotalDebt {
			fmt.Printf("%s asset equity %d less then debt %d\n", entry.Symbol, entry.TotalEquity, entry.TotalDebt)
			panic("invalid cex asset info")
		}
	}
	emptyCexAssetsInfo := make([]tier3spec.CexAssetInfo, len(cexAssetsInfo))
	copy(emptyCexAssetsInfo, cexAssetsInfo)
	for i := range emptyCexAssetsInfo {
		emptyCexAssetsInfo[i].TotalDebt = 0
		emptyCexAssetsInfo[i].TotalEquity = 0
		emptyCexAssetsInfo[i].LoanCollateral = 0
		emptyCexAssetsInfo[i].MarginCollateral = 0
		emptyCexAssetsInfo[i].PortfolioMarginCollateral = 0
	}
	emptyCexAssetListCommitment := tier3host.ComputeCexAssetsCommitment(emptyCexAssetsInfo)
	expectFinalCexAssetsInfoComm := tier3host.ComputeCexAssetsCommitment(cexAssetsInfo)

	prevCexAssetListCommitments := make([][]byte, 2)
	prevAccountTreeRoots := make([][]byte, 2)
	prevAccountTreeRoots[1] = emptyAccountTreeRoot
	prevCexAssetListCommitments[1] = emptyCexAssetListCommitment

	if !verifyAllProofs(proofs, verifierConfig) {
		os.Exit(1)
	}

	// Chain check: walk batches in order, each before-state must equal
	// the prior after-state.
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

// decodeBatchMetadata base64-decodes the account-tree-roots and
// cex-asset-commitment pairs of one proof row.
func decodeBatchMetadata(p proofRow) (roots [][]byte, commits [][]byte) {
	roots = make([][]byte, 2)
	commits = make([][]byte, 2)
	for i := 0; i < len(p.AccountTreeRoots) && i < 2; i++ {
		v, err := base64.StdEncoding.DecodeString(p.AccountTreeRoots[i])
		if err != nil {
			panic("decode account tree root failed")
		}
		roots[i] = v
	}
	for i := 0; i < len(p.CexAssetCommitment) && i < 2; i++ {
		v, err := base64.StdEncoding.DecodeString(p.CexAssetCommitment[i])
		if err != nil {
			panic("decode cex asset commitment failed")
		}
		commits[i] = v
	}
	return roots, commits
}

// verifyAllProofs runs the groth16 check for every proof row across a
// worker pool. Each row's public input is recomputed from its
// account-tree-roots and cex-commitments and checked against the
// embedded batch commitment before the proof is verified. Returns
// false if any proof fails.
func verifyAllProofs(proofs []proofRow, cfg *vconfig.Config) bool {
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
			if !verifyProofRange(proofs[startIndex:endIndex], cfg) {
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
// matching the prover's tier-grouped ordering.
func verifyProofRange(rows []proofRow, cfg *vconfig.Config) bool {
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

		verifyWitness := tier3circuit.NewVerifyBatchCreateUserCircuit(actualHash)
		vWitness, err := frontend.NewWitness(verifyWitness, ecc.BN254.ScalarField(), frontend.PublicOnly())
		if err != nil {
			panic(err.Error())
		}

		if row.AssetsCount != currentAssetCountsTier {
			keyIndex := -1
			for p := range cfg.AssetsCountTiers {
				if cfg.AssetsCountTiers[p] == row.AssetsCount {
					keyIndex = p
					break
				}
			}
			if keyIndex == -1 {
				panic("invalid asset counts tier")
			}
			vk, err = loadVerifyingKey(cfg.ZkKeyName[keyIndex] + ".vk")
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
