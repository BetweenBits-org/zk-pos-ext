// Command verifier is the zkpor-native proof of solvency verifier.
// Same three CLI modes as legacy src/verifier, but wired entirely
// through zkpor packages and (post R8-D) the declarative profile.
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
// R8-D swap: AssetCapacity / AssetsCountTiers / ZkKeyName come from
// profile.toml + -keys-dir (batch + -user modes). config.json keeps
// DB + CexAssetsInfo (per-snapshot published totals).
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
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	vconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/cmd/verifier/config"
	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	t4circuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/circuit"
	t4host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host"
	t4spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/gocarina/gocsv"
)

// expectedModel — V1-PROD T4 only.
const expectedModel corespec.SolvencyModelID = "t4_tiered_haircut_margin_3pool"

// resolved holds the derived plan the verifier walks for both batch
// mode (.vk per tier) and -user mode (assetCountTiers for leaf padding).
type resolved struct {
	assetCapacity   int
	assetCountTiers []int
	zkKeyStems      []string // same index as assetCountTiers
}

// pflags carries the parsed profile-driven flags. main passes a
// pointer through to whichever subcommand runs.
type pflags struct {
	profilePath string
	keysDir     string
	capacity    int
}

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
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required for batch + -user modes)")
	keysDir := flag.String("keys-dir", "", "directory containing the verifying-key .vk files (required for batch mode)")
	capacityOverride := flag.Int("asset-capacity", 0, "override profile.asset_capacity (smoke only; 0 = use toml value)")
	flag.Parse()

	flags := &pflags{
		profilePath: *profilePath,
		keysDir:     *keysDir,
		capacity:    *capacityOverride,
	}

	switch {
	case *hashFlag:
		runHash(flag.Args())
	case *userFlag:
		runUserVerification(flags)
	default:
		runBatchVerification(flags)
	}
}

// resolveFromProfile loads profile.toml + derives the (capacity, tiers,
// stems) plan. Used by both batch + -user modes.
func resolveFromProfile(flags *pflags) (*resolved, error) {
	if flags.profilePath == "" {
		return nil, fmt.Errorf("-profile is required (path to profile.toml)")
	}
	prof, err := declarative.Load(flags.profilePath)
	if err != nil {
		return nil, err
	}
	if model := corespec.SolvencyModelID(prof.Profile.Model); model != expectedModel {
		return nil, fmt.Errorf("verifier binary supports %q only; profile.toml model = %q", expectedModel, model)
	}
	provider, err := declarative.BuildBatchShapeProvider(
		corespec.SolvencyModelID(prof.Profile.Model), prof.BatchShapes)
	if err != nil {
		return nil, err
	}
	shapes := provider.Shapes()
	out := &resolved{
		assetCapacity:   prof.Profile.AssetCapacity,
		assetCountTiers: make([]int, len(shapes)),
		zkKeyStems:      make([]string, len(shapes)),
	}
	if flags.capacity > 0 {
		out.assetCapacity = flags.capacity
	}
	for i, s := range shapes {
		out.assetCountTiers[i] = s.AssetCountTier
		out.zkKeyStems[i] = filepath.Join(flags.keysDir, provider.KeyName(s, prof.Constraint.Module))
	}
	return out, nil
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

// runUserVerification recomputes a single account's leaf hash from
// config/user_config.json and checks it against the published account
// tree root via the Merkle path. This is the engine-side primitive a
// customer's self-inclusion UI would wrap (the UI itself is out of V1
// scope).
func runUserVerification(flags *pflags) {
	plan, err := resolveFromProfile(flags)
	if err != nil {
		panic(err.Error())
	}
	content, err := os.ReadFile("config/user_config.json")
	if err != nil {
		panic(err.Error())
	}
	userConfig := &t4host.UserConfig{}
	if err := json.Unmarshal(content, userConfig); err != nil {
		panic(err.Error())
	}

	root, err := hex.DecodeString(userConfig.Root)
	if err != nil || len(root) != 32 {
		panic("invalid account tree root")
	}

	// UserConfig.Proof is [][]byte — JSON decode already base64'd the
	// wire form back to raw 32-byte sibling hashes.
	for i, p := range userConfig.Proof {
		if len(p) != 32 {
			panic(fmt.Sprintf("invalid proof[%d] len=%d, want 32", i, len(p)))
		}
	}
	proof := userConfig.Proof

	assetCommitment := t4host.ComputeUserAssetsCommitment(userConfig.Assets, plan.assetCountTiers)

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
func runBatchVerification(flags *pflags) {
	plan, err := resolveFromProfile(flags)
	if err != nil {
		panic(err.Error())
	}
	if flags.keysDir == "" {
		panic("-keys-dir is required (path to keygen .artifacts/)")
	}
	content, err := os.ReadFile("config/config.json")
	if err != nil {
		panic(err.Error())
	}
	verifierConfig := &vconfig.Config{}
	if err := json.Unmarshal(content, verifierConfig); err != nil {
		panic(err.Error())
	}

	proofs, err := loadProofs(verifierConfig)
	if err != nil {
		panic(err.Error())
	}

	emptyAccountTreeRoot, err := hex.DecodeString(emptyAccountTreeRootHex)
	if err != nil {
		panic("wrong empty account tree root")
	}

	// Index the published CEX totals by their declared asset index and
	// enforce the per-asset equity >= debt floor before computing the
	// expected final commitment.
	cexAssetsInfo := make([]t4spec.CexAssetInfo, len(verifierConfig.CexAssetsInfo))
	for i := range verifierConfig.CexAssetsInfo {
		entry := verifierConfig.CexAssetsInfo[i]
		cexAssetsInfo[entry.Index] = entry
		// Per-asset equity < debt is allowed under t4_tiered_haircut_margin_3pool: model
		// invariants are per-account (sum collateral ≥ sum debt across
		// the user's portfolio), not per-asset. Surface as a warning so
		// operators notice unusual distributions, but do not panic.
		if entry.TotalEquity < entry.TotalDebt {
			fmt.Printf("warning: %s asset equity %d less than debt %d (allowed by model; check distribution)\n",
				entry.Symbol, entry.TotalEquity, entry.TotalDebt)
		}
	}
	emptyCexAssetsInfo := make([]t4spec.CexAssetInfo, len(cexAssetsInfo))
	copy(emptyCexAssetsInfo, cexAssetsInfo)
	for i := range emptyCexAssetsInfo {
		emptyCexAssetsInfo[i].TotalDebt = 0
		emptyCexAssetsInfo[i].TotalEquity = 0
		emptyCexAssetsInfo[i].LoanCollateral = 0
		emptyCexAssetsInfo[i].MarginCollateral = 0
		emptyCexAssetsInfo[i].PortfolioMarginCollateral = 0
	}
	if plan.assetCapacity <= 0 {
		panic("verifier: profile.asset_capacity must be > 0")
	}
	emptyCexAssetListCommitment := t4host.ComputeCexAssetsCommitment(emptyCexAssetsInfo, plan.assetCapacity)
	expectFinalCexAssetsInfoComm := t4host.ComputeCexAssetsCommitment(cexAssetsInfo, plan.assetCapacity)

	prevCexAssetListCommitments := make([][]byte, 2)
	prevAccountTreeRoots := make([][]byte, 2)
	prevAccountTreeRoots[1] = emptyAccountTreeRoot
	prevCexAssetListCommitments[1] = emptyCexAssetListCommitment

	if !verifyAllProofs(proofs, plan) {
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

// loadProofs reads proof rows either from the prover's MySQL proof
// table (when cfg.MysqlDataSource is set) or from the legacy CSV at
// cfg.ProofTable. In both cases the returned slice is indexed by
// BatchNumber — i.e. result[i] is the proof for batch i, assuming
// batch numbers are a dense 0..N-1 sequence as the prover produces.
func loadProofs(cfg *vconfig.Config) ([]proofRow, error) {
	if cfg.MysqlDataSource != "" {
		return loadProofsFromDB(cfg)
	}
	return loadProofsFromCSV(cfg.ProofTable)
}

// loadProofsFromCSV unmarshals the CSV at path and re-indexes the
// resulting slice so result[i] is the proof for batch i.
func loadProofsFromCSV(path string) ([]proofRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open proof table %q: %w", path, err)
	}
	defer f.Close()

	tmp := []*proofRow{}
	if err := gocsv.UnmarshalFile(f, &tmp); err != nil {
		return nil, fmt.Errorf("parse proof table %q: %w", path, err)
	}
	out := make([]proofRow, len(tmp))
	for i := range tmp {
		out[tmp[i].BatchNumber] = *tmp[i]
	}
	return out, nil
}

// loadProofsFromDB reads every proof row from the prover's proof table
// in BatchNumber order and converts each store.Proof into the proofRow
// shape the verifier downstream consumes. The conversion mirrors the
// CSV column layout: ProofInfo / BatchCommitment / AssetsCount /
// BatchNumber are direct copies; CexAssetListCommitments and
// AccountTreeRoots are unmarshalled from JSON (the prover writes them
// as JSON-encoded [][]byte → []base64-string arrays).
func loadProofsFromDB(cfg *vconfig.Config) ([]proofRow, error) {
	db, err := store.Open(cfg.MysqlDataSource)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	proofStore := store.NewProofStore(db, cfg.DbSuffix)
	rows, err := proofStore.ListAllInOrder()
	if err != nil {
		return nil, fmt.Errorf("list proofs: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("proof table is empty (suffix %q)", cfg.DbSuffix)
	}
	out := make([]proofRow, len(rows))
	for _, row := range rows {
		converted, err := convertStoredProof(row)
		if err != nil {
			return nil, fmt.Errorf("batch %d: %w", row.BatchNumber, err)
		}
		if row.BatchNumber < 0 || int(row.BatchNumber) >= len(rows) {
			return nil, fmt.Errorf("batch number %d out of dense range [0,%d)", row.BatchNumber, len(rows))
		}
		out[row.BatchNumber] = converted
	}
	return out, nil
}

// convertStoredProof maps one store.Proof into the proofRow shape the
// verifier uses. The two JSON-encoded slices are decoded directly into
// []string — json.Marshal of [][]byte writes base64-encoded strings,
// which is the same on-wire shape the CSV path produces.
func convertStoredProof(row store.Proof) (proofRow, error) {
	var cexCommits []string
	if err := json.Unmarshal([]byte(row.CexAssetListCommitments), &cexCommits); err != nil {
		return proofRow{}, fmt.Errorf("unmarshal cex commitments: %w", err)
	}
	var treeRoots []string
	if err := json.Unmarshal([]byte(row.AccountTreeRoots), &treeRoots); err != nil {
		return proofRow{}, fmt.Errorf("unmarshal account tree roots: %w", err)
	}
	return proofRow{
		BatchNumber:        row.BatchNumber,
		ZkProof:            row.ProofInfo,
		CexAssetCommitment: cexCommits,
		AccountTreeRoots:   treeRoots,
		BatchCommitment:    row.BatchCommitment,
		AssetsCount:        row.AssetsCount,
	}, nil
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
func verifyAllProofs(proofs []proofRow, plan *resolved) bool {
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
			if !verifyProofRange(proofs[startIndex:endIndex], plan) {
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
func verifyProofRange(rows []proofRow, plan *resolved) bool {
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

		verifyWitness := t4circuit.NewVerifyBatchCreateUserCircuit(actualHash)
		vWitness, err := frontend.NewWitness(verifyWitness, ecc.BN254.ScalarField(), frontend.PublicOnly())
		if err != nil {
			panic(err.Error())
		}

		if row.AssetsCount != currentAssetCountsTier {
			keyIndex := -1
			for p := range plan.assetCountTiers {
				if plan.assetCountTiers[p] == row.AssetsCount {
					keyIndex = p
					break
				}
			}
			if keyIndex == -1 {
				panic("invalid asset counts tier")
			}
			vk, err = loadVerifyingKey(plan.zkKeyStems[keyIndex] + ".vk")
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
