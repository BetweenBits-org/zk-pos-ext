package host

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	t1spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	bsmt "github.com/bnb-chain/zkbnb-smt"
)

// dbBatchSize is the legacy chunk size for UserProof inserts (~100 rows
// per round-trip); preserved to match operational throughput.
const dbBatchSize = 100

// UserProofRunnerConfig is the T1 model-typed dependency bundle for one
// userproof run. T1 has no TotalCollateral — UserConfig and UserProof
// row fields handle that via a nil/empty TotalCollateral.
type UserProofRunnerConfig struct {
	Ctx             context.Context
	Snapshot        t1spec.SnapshotSource
	AccountTree     bsmt.SparseMerkleTree
	UserProofStore  corehost.UserProofStore
	ShapeProvider   corespec.BatchShapeProvider
	AssetCountTiers []int
}

// RunUserProof — see t4_tiered_haircut_margin_3pool/host.RunUserProof
// docstring; the T1 variant differs only in the per-account leaf shape
// (slot-3 zero) and the UserConfig payload (no TotalCollateral).
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

func sortedKeys(m map[int][]t1spec.AccountInfo) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

func populateTree(
	accountTree bsmt.SparseMerkleTree,
	accountsByTier map[int][]t1spec.AccountInfo,
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

func writeUserProofs(
	accountTree bsmt.SparseMerkleTree,
	accountsByTier map[int][]t1spec.AccountInfo,
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

// BuildUserProofRow serialises one T1 account into a UserProof DB row.
// T1 has no TotalCollateral — both the UserConfig payload and the
// store row's TotalCollateral string column are kept empty so the
// schema can stay model-blind without forcing T1-specific columns.
func BuildUserProofRow(
	account *t1spec.AccountInfo,
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
		AccountIndex:  account.AccountIndex,
		AccountIdHash: hex.EncodeToString(account.AccountID),
		TotalEquity:   account.TotalEquity,
		TotalDebt:     account.TotalDebt,
		Assets:        account.Assets,
		Root:          rootHex,
		Proof:         proof,
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
		TotalCollateral: "",
		Assets:          string(assetsJSON),
		Proof:           string(proofJSON),
		Config:          string(configJSON),
	}, nil
}
