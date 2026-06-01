package host

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	t4spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	bsmt "github.com/bnb-chain/zkbnb-smt"
)

// dbBatchSize is the legacy chunk size for UserProof inserts (~100 rows
// per round-trip); preserved to match operational throughput.
const dbBatchSize = 100

// UserProofRunnerConfig is the T4 model-typed dependency bundle for one
// userproof run. Same shape as the witness runner — only the typed
// SnapshotSource and the per-tier padding semantics differ.
type UserProofRunnerConfig struct {
	Ctx             context.Context
	Snapshot        t4spec.SnapshotSource
	AccountTree     bsmt.SparseMerkleTree
	UserProofStore  corehost.UserProofStore
	ShapeProvider   corespec.BatchShapeProvider
	AssetCountTiers []int
}

// RunUserProof performs the full self-contained userproof run for the
// T4 model:
//
//  1. Stream + bucket the snapshot's accounts by tier.
//  2. Apply PaddingAccounts to each tier so the tree is populated to
//     the same shape the witness service published.
//  3. Set every leaf into the tree (real + padding).
//  4. For every real account, GetProof + build a UserConfig payload,
//     persist as a UserProof row in dbBatchSize chunks.
//
// Returns the populated account tree's root (hex-encoded) and the
// number of UserProof rows written. cmd/userproof handles any post-run
// reporting (dump-user-index, etc).
func RunUserProof(cfg UserProofRunnerConfig) (string, int, error) {
	accountsByTier, err := streamAndBucket(cfg.Ctx, cfg.Snapshot, cfg.AssetCountTiers)
	if err != nil {
		return "", 0, err
	}
	totalReal := 0
	for _, accs := range accountsByTier {
		totalReal += len(accs)
	}
	fmt.Printf("loaded %d real accounts across %d tiers\n", totalReal, len(accountsByTier))

	// Per-tier real-vs-padding boundary. After PaddingAccounts,
	// accounts[0..realCount[tier]) are real; the rest are padding.
	realCount := make(map[int]int, len(accountsByTier))
	for k, v := range accountsByTier {
		realCount[k] = len(v)
	}

	tiers := sortedKeys(accountsByTier)
	paddingStart := totalReal
	for _, k := range tiers {
		shape, err := cfg.ShapeProvider.SelectFor(k)
		if err != nil {
			return "", 0, err
		}
		paddingStart, accountsByTier[k] = PaddingAccounts(accountsByTier[k], k, paddingStart, shape.UsersPerBatch)
	}

	fmt.Printf("account tree initialised, root = %x\n", cfg.AccountTree.Root())
	populateTree(cfg.AccountTree, accountsByTier, tiers, cfg.AssetCountTiers)
	if _, err := cfg.AccountTree.Commit(nil); err != nil {
		return "", 0, err
	}
	rootHex := hex.EncodeToString(cfg.AccountTree.Root())
	fmt.Printf("account tree populated, root = %s\n", rootHex)

	written, err := writeUserProofs(cfg.AccountTree, accountsByTier, tiers, realCount, cfg.AssetCountTiers, rootHex, cfg.UserProofStore)
	if err != nil {
		return "", 0, err
	}
	fmt.Printf("userproof run finished, %d rows written\n", written)
	return rootHex, written, nil
}

// sortedKeys returns the map's keys in ascending order — the
// tier-iteration order witness and userproof must agree on so the
// resulting tree state matches.
func sortedKeys(m map[int][]t4spec.AccountInfo) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// populateTree walks every account (real and padding, in the same
// tier-then-position order the witness uses) and Sets its leaf hash
// into the tree. Padding accounts contribute their zero-balance leaf
// hash so the resulting root matches the witness's published root.
func populateTree(
	accountTree bsmt.SparseMerkleTree,
	accountsByTier map[int][]t4spec.AccountInfo,
	tiers []int,
	assetCountTiers []int,
) {
	for _, k := range tiers {
		for i := range accountsByTier[k] {
			account := &accountsByTier[k][i]
			leaf := AccountLeafHash(account, assetCountTiers)
			if err := accountTree.Set(uint64(account.AccountIndex), leaf); err != nil {
				panic(err.Error())
			}
		}
	}
}

// writeUserProofs iterates real accounts (skipping padding) in
// tier-then-position order, fetches each proof from the populated
// tree, builds the UserProof + embedded UserConfig JSON, and flushes
// to the DB in dbBatchSize chunks. Returns the total number of rows
// written.
func writeUserProofs(
	accountTree bsmt.SparseMerkleTree,
	accountsByTier map[int][]t4spec.AccountInfo,
	tiers []int,
	realCount map[int]int,
	assetCountTiers []int,
	rootHex string,
	userProofStore corehost.UserProofStore,
) (int, error) {
	batch := make([]corehost.UserProofDTO, 0, dbBatchSize)
	written := 0

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := userProofStore.Create(batch); err != nil {
			return err
		}
		written += len(batch)
		batch = batch[:0]
		return nil
	}

	for _, k := range tiers {
		realN := realCount[k]
		for i := range realN {
			account := &accountsByTier[k][i]
			proof, err := accountTree.GetProof(uint64(account.AccountIndex))
			if err != nil {
				return written, err
			}
			leaf := AccountLeafHash(account, assetCountTiers)

			row, err := BuildUserProofRow(account, leaf, proof, rootHex)
			if err != nil {
				return written, err
			}
			batch = append(batch, row)
			if len(batch) >= dbBatchSize {
				if err := flush(); err != nil {
					return written, err
				}
			}
		}
	}
	if err := flush(); err != nil {
		return written, err
	}
	return written, nil
}

// BuildUserProofRow serialises one T4 account into a UserProof DB row,
// including the JSON-marshalled UserConfig payload the verifier -user
// mode reads to recompute the leaf.
func BuildUserProofRow(
	account *t4spec.AccountInfo,
	leaf []byte,
	proof [][]byte,
	rootHex string,
) (corehost.UserProofDTO, error) {
	proofJSON, err := json.Marshal(proof)
	if err != nil {
		return corehost.UserProofDTO{}, fmt.Errorf("marshal proof: %w", err)
	}
	assetsJSON, err := json.Marshal(account.Assets)
	if err != nil {
		return corehost.UserProofDTO{}, fmt.Errorf("marshal assets: %w", err)
	}
	userConfig := UserConfig{
		AccountIndex:    account.AccountIndex,
		AccountIdHash:   hex.EncodeToString(account.AccountID),
		TotalEquity:     account.TotalEquity,
		TotalDebt:       account.TotalDebt,
		TotalCollateral: account.TotalCollateral,
		Assets:          account.Assets,
		Root:            rootHex,
		Proof:           proof,
	}
	configJSON, err := json.Marshal(userConfig)
	if err != nil {
		return corehost.UserProofDTO{}, fmt.Errorf("marshal user config: %w", err)
	}

	return corehost.UserProofDTO{
		AccountIndex:    account.AccountIndex,
		AccountId:       hex.EncodeToString(account.AccountID),
		AccountLeafHash: hex.EncodeToString(leaf),
		TotalEquity:     account.TotalEquity.String(),
		TotalDebt:       account.TotalDebt.String(),
		TotalCollateral: account.TotalCollateral.String(),
		Assets:          string(assetsJSON),
		Proof:           string(proofJSON),
		Config:          string(configJSON),
	}, nil
}
