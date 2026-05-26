// Command userproof is the zkpor-native per-user inclusion-proof
// builder for the Binance deployment. Reads the customer snapshot,
// rebuilds the depth-28 account SMT (matching the witness service's
// tree state via the same padding semantics), and writes one
// UserProof row per real account — embedded UserConfig payload lets
// the verifier -user mode recompute and check inclusion locally.
//
// This is the R3 step 4 core-path service: sequential per-account
// hashing and proof generation, fresh-start only (no DB resume, no
// parallel workers, no -memory_tree utility flag). Recovery and
// throughput follow-ups are tracked separately.
//
// Self-contained tree build: the userproof does NOT depend on the
// witness service's persisted tree state. Same snapshot + same
// padding rules = same tree leaves = same root, so per-user proofs
// verify against the same root the witness/prover published.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	uconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/cmd/userproof/config"
	tier3host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/host"
	tier3spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/tree"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/binance"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"
	bsmt "github.com/bnb-chain/zkbnb-smt"
)

// dbBatchSize is the legacy chunk size for UserProof inserts (~100
// rows per round-trip); preserved to match operational throughput.
const dbBatchSize = 100

func main() {
	flag.Parse()

	cfg := loadConfig("config/config.json")

	snapshot := binance.NewSnapshotCSV(binance.SnapshotConfig{UserDataDir: cfg.UserDataFile})
	shapeProvider := binance.NewBatchShape()
	assetCountTiers := tiersFromShapes(shapeProvider.Shapes())

	ctx := context.Background()
	accountsByTier := streamAndBucket(ctx, snapshot, assetCountTiers)
	totalReal := 0
	for _, accs := range accountsByTier {
		totalReal += len(accs)
	}
	fmt.Printf("loaded %d real accounts across %d tiers\n", totalReal, len(accountsByTier))

	// Per-tier real-vs-padding boundary. After tier3host.PaddingAccounts,
	// accounts[0..realCount[tier]) are real; the rest are padding.
	realCount := make(map[int]int, len(accountsByTier))
	for k, v := range accountsByTier {
		realCount[k] = len(v)
	}

	tiers := sortedKeys(accountsByTier)
	paddingStart := totalReal
	for _, k := range tiers {
		shape, err := shapeProvider.SelectFor(k)
		if err != nil {
			panic(err.Error())
		}
		paddingStart, accountsByTier[k] = tier3host.PaddingAccounts(accountsByTier[k], k, paddingStart, shape.UsersPerBatch)
	}

	accountTree, err := tree.NewAccountTree(cfg.TreeDB.Driver, cfg.TreeDB.Option.Addr)
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("account tree initialised, root = %x\n", accountTree.Root())

	populateTree(accountTree, accountsByTier, tiers, assetCountTiers)
	if _, err := accountTree.Commit(nil); err != nil {
		panic(err.Error())
	}
	rootHex := hex.EncodeToString(accountTree.Root())
	fmt.Printf("account tree populated, root = %s\n", rootHex)

	db, err := store.Open(cfg.MysqlDataSource)
	if err != nil {
		panic(err.Error())
	}
	userProofStore := store.NewUserProofStore(db, cfg.DbSuffix)
	if err := userProofStore.CreateTable(); err != nil {
		panic(err.Error())
	}

	written := writeUserProofs(accountTree, accountsByTier, tiers, realCount, assetCountTiers, rootHex, userProofStore)
	fmt.Printf("userproof run finished, %d rows written\n", written)
}

func loadConfig(path string) *uconfig.Config {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err.Error())
	}
	cfg := &uconfig.Config{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		panic(err.Error())
	}
	return cfg
}

// tiersFromShapes flattens the deployment's BatchShape set into the
// sorted-ascending []int the host commitment helpers consume.
func tiersFromShapes(shapes []corespec.BatchShape) []int {
	out := make([]int, len(shapes))
	for i, s := range shapes {
		out[i] = s.AssetCountTier
	}
	return out
}

// streamAndBucket drains the snapshot's account stream and groups
// accounts by the smallest BatchShape AssetCountTier that fits their
// non-empty asset count.
func streamAndBucket(ctx context.Context, snapshot tier3spec.SnapshotSource, tiers []int) map[int][]tier3spec.AccountInfo {
	ch, err := snapshot.AccountStream(ctx)
	if err != nil {
		panic(err.Error())
	}
	out := make(map[int][]tier3spec.AccountInfo)
	for account := range ch {
		tier := tier3spec.PickAssetCountTier(tier3spec.CountNonEmptyAssets(account.Assets), tiers)
		if tier == 0 {
			panic(fmt.Sprintf("account %d has %d non-empty assets — no tier in %v fits",
				account.AccountIndex, tier3spec.CountNonEmptyAssets(account.Assets), tiers))
		}
		out[tier] = append(out[tier], account)
	}
	return out
}

// sortedKeys returns the map's keys in ascending order — the
// tier-iteration order witness and userproof must agree on so the
// resulting tree state matches.
func sortedKeys(m map[int][]tier3spec.AccountInfo) []int {
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
	accountsByTier map[int][]tier3spec.AccountInfo,
	tiers []int,
	assetCountTiers []int,
) {
	for _, k := range tiers {
		for i := range accountsByTier[k] {
			account := &accountsByTier[k][i]
			leaf := tier3host.AccountLeafHash(account, assetCountTiers)
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
	accountsByTier map[int][]tier3spec.AccountInfo,
	tiers []int,
	realCount map[int]int,
	assetCountTiers []int,
	rootHex string,
	userProofStore *store.UserProofStore,
) int {
	batch := make([]store.UserProof, 0, dbBatchSize)
	written := 0

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := userProofStore.Create(batch); err != nil {
			panic(err.Error())
		}
		written += len(batch)
		batch = batch[:0]
	}

	for _, k := range tiers {
		realN := realCount[k]
		for i := range realN {
			account := &accountsByTier[k][i]
			proof, err := accountTree.GetProof(uint64(account.AccountIndex))
			if err != nil {
				panic(err.Error())
			}
			leaf := tier3host.AccountLeafHash(account, assetCountTiers)

			row, err := buildUserProofRow(account, leaf, proof, rootHex)
			if err != nil {
				panic(err.Error())
			}
			batch = append(batch, row)
			if len(batch) >= dbBatchSize {
				flush()
			}
		}
	}
	flush()
	return written
}

// buildUserProofRow serialises one account into a UserProof DB row,
// including the JSON-marshalled UserConfig payload the verifier
// -user mode reads to recompute the leaf.
func buildUserProofRow(
	account *tier3spec.AccountInfo,
	leaf []byte,
	proof [][]byte,
	rootHex string,
) (store.UserProof, error) {
	proofJSON, err := json.Marshal(proof)
	if err != nil {
		return store.UserProof{}, fmt.Errorf("marshal proof: %w", err)
	}
	assetsJSON, err := json.Marshal(account.Assets)
	if err != nil {
		return store.UserProof{}, fmt.Errorf("marshal assets: %w", err)
	}
	userConfig := tier3host.UserConfig{
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
		return store.UserProof{}, fmt.Errorf("marshal user config: %w", err)
	}

	return store.UserProof{
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

