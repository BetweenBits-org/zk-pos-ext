// Command witness is the zkpor-native witness builder for the Binance
// deployment. Reads the customer snapshot, builds the depth-28 account
// SMT, walks accounts in tier-grouped batches, and writes one
// BatchCreateUserWitness per batch into the witness MySQL table for
// the prover to pick up.
//
// This is the R3 step 4 core-path service: sequential per-account
// hashing, fresh-start only (no DB resume, no tree rollback). Resume
// and rollback are tracked as follow-up slices — see HANDOFF Resume
// Actions and the legacy src/witness for the full state machine they
// will eventually replace.
//
// G6 (ValueScale invariant) closes here: the witness is the first
// pipeline stage that takes a PriceScaleProvider dependency, so it
// asserts PriceMultiplier × BalanceMultiplier == ValueScale on
// startup. Two-digit-asset deployments rely on the per-symbol split
// (1e14 × 1e2 == 1e16) — services that touch raw symbols are expected
// to extend the assert with their own coverage.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	wconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/cmd/witness/config"
	tier3host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/host"
	tier3spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/tree"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/binance"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"
	bsmt "github.com/bnb-chain/zkbnb-smt"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

func main() {
	dumpFinalCex := flag.String("dump-final-cex", "", "if set, write the post-batch CexAssetsInfo slice as JSON to this path (smoke harness convenience)")
	flag.Parse()

	cfg := loadConfig("config/config.json")

	// G6 — PriceMultiplier × BalanceMultiplier == ValueScale invariant.
	// Asserted at the deployment-default symbol path; per-symbol splits
	// (e.g. binance's two-digit assets) are exercised by the profile's
	// own test suite.
	pricing := binance.NewPricing()
	if got := pricing.PriceMultiplier("") * pricing.BalanceMultiplier(""); got != pricing.ValueScale() {
		panic(fmt.Sprintf(
			"pricing invariant violated: PriceMultiplier × BalanceMultiplier = %d, ValueScale = %d",
			got, pricing.ValueScale(),
		))
	}

	snapshot := binance.NewSnapshotCSV(binance.SnapshotConfig{
		UserDataDir:   cfg.UserDataFile,
		AssetCapacity: cfg.AssetCapacity,
	})
	shapeProvider := binance.NewBatchShape()
	assetCountTiers := tiersFromShapes(shapeProvider.Shapes())

	ctx := context.Background()
	cexAssets, err := snapshot.CexAssets(ctx)
	if err != nil {
		panic(err.Error())
	}

	accountsByTier := streamAndBucket(ctx, snapshot, assetCountTiers)
	totalAccounts := 0
	for _, accs := range accountsByTier {
		totalAccounts += len(accs)
	}
	fmt.Printf("loaded %d accounts across %d tiers\n", totalAccounts, len(accountsByTier))

	accountTree, err := tree.NewAccountTree(cfg.TreeDB.Driver, cfg.TreeDB.Option.Addr)
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("account tree initialised, root = %x\n", accountTree.Root())

	db, err := store.Open(cfg.MysqlDataSource)
	if err != nil {
		panic(err.Error())
	}
	witnessStore := store.NewWitnessStore(db, cfg.DbSuffix)
	if err := witnessStore.CreateTable(); err != nil {
		panic(err.Error())
	}

	runBatches(accountsByTier, cexAssets, accountTree, witnessStore, assetCountTiers, shapeProvider, totalAccounts)

	fmt.Printf("witness run finished, account tree root = %x\n", accountTree.Root())

	if *dumpFinalCex != "" {
		raw, err := json.MarshalIndent(cexAssets, "", "  ")
		if err != nil {
			panic(fmt.Sprintf("marshal final cex assets: %v", err))
		}
		if err := os.WriteFile(*dumpFinalCex, raw, 0o644); err != nil {
			panic(fmt.Sprintf("write %q: %v", *dumpFinalCex, err))
		}
		fmt.Printf("final cex assets written to %s\n", *dumpFinalCex)
	}
}

// loadConfig reads and parses the on-disk JSON config.
func loadConfig(path string) *wconfig.Config {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err.Error())
	}
	cfg := &wconfig.Config{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		panic(err.Error())
	}
	return cfg
}

// tiersFromShapes flattens the deployment's BatchShape set to the
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
// non-empty asset count. Returns map[tier] -> []AccountInfo.
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

// runBatches walks the tiers in ascending order, pads each tier's
// account slice to a whole number of batches, and writes one
// BatchWitness row per batch. Sequential — multi-worker account
// hashing is a follow-up slice.
func runBatches(
	accountsByTier map[int][]tier3spec.AccountInfo,
	cexAssets []tier3spec.CexAssetInfo,
	accountTree bsmt.SparseMerkleTree,
	witnessStore *store.WitnessStore,
	assetCountTiers []int,
	shapeProvider corespec.BatchShapeProvider,
	totalAccounts int,
) {
	tiers := make([]int, 0, len(accountsByTier))
	for k := range accountsByTier {
		tiers = append(tiers, k)
	}
	sort.Ints(tiers)

	height := int64(0)
	paddingStart := totalAccounts
	for _, assetKey := range tiers {
		shape, err := shapeProvider.SelectFor(assetKey)
		if err != nil {
			panic(err.Error())
		}
		usersPerBatch := shape.UsersPerBatch

		paddingStart, accountsByTier[assetKey] = tier3host.PaddingAccounts(
			accountsByTier[assetKey], assetKey, paddingStart, usersPerBatch,
		)
		accounts := accountsByTier[assetKey]
		batches := len(accounts) / usersPerBatch
		fmt.Printf("tier %d: %d accounts → %d batches (%d/batch)\n", assetKey, len(accounts), batches, usersPerBatch)

		for b := range batches {
			batch := accounts[b*usersPerBatch : (b+1)*usersPerBatch]
			wit := buildBatch(batch, cexAssets, accountTree, assetCountTiers)

			encoded, err := tier3host.EncodeBatchWitness(wit)
			if err != nil {
				panic(err.Error())
			}
			row := store.BatchWitness{
				Height:      height,
				WitnessData: base64.StdEncoding.EncodeToString(encoded),
				Status:      store.StatusPublished,
			}
			if err := witnessStore.Create([]store.BatchWitness{row}); err != nil {
				panic(err.Error())
			}

			// bsmt.Commit takes a *pruning* version (not the new commit
			// version, which is tree.version+1 auto-increment). Passing
			// nil disables pruning — fine for the memory-backed tree the
			// smoke uses and the default redis path until a multi-batch
			// resume slice needs to bound disk growth.
			if _, err := accountTree.Commit(nil); err != nil {
				panic(err.Error())
			}
			height++
		}
	}
}

// buildBatch produces one BatchCreateUserWitness from a slice of
// usersPerBatch accounts: before/after roots, before/after CEX
// commitments, per-user Merkle proofs + leaf updates, batch
// commitment. Mutates cexAssets in place (running running-sum of
// per-asset totals) and writes new leaves to accountTree.
func buildBatch(
	batch []tier3spec.AccountInfo,
	cexAssets []tier3spec.CexAssetInfo,
	accountTree bsmt.SparseMerkleTree,
	assetCountTiers []int,
) *tier3spec.BatchCreateUserWitness {
	wit := &tier3spec.BatchCreateUserWitness{
		BeforeAccountTreeRoot: accountTree.Root(),
		BeforeCexAssets:       make([]tier3spec.CexAssetInfo, len(cexAssets)),
		CreateUserOps:         make([]tier3spec.CreateUserOperation, len(batch)),
	}
	copy(wit.BeforeCexAssets, cexAssets)
	wit.BeforeCEXAssetsCommitment = tier3host.ComputeCexAssetsCommitment(cexAssets, len(cexAssets))

	for i, account := range batch {
		op := &wit.CreateUserOps[i]
		op.BeforeAccountTreeRoot = accountTree.Root()

		proof, err := accountTree.GetProof(uint64(account.AccountIndex))
		if err != nil {
			panic(err.Error())
		}
		copy(op.AccountProof[:], proof)

		// Update running per-asset totals from this user's contribution.
		for _, a := range account.Assets {
			cexAssets[a.Index].TotalEquity = safeAdd(cexAssets[a.Index].TotalEquity, a.Equity)
			cexAssets[a.Index].TotalDebt = safeAdd(cexAssets[a.Index].TotalDebt, a.Debt)
			cexAssets[a.Index].LoanCollateral = safeAdd(cexAssets[a.Index].LoanCollateral, a.Loan)
			cexAssets[a.Index].MarginCollateral = safeAdd(cexAssets[a.Index].MarginCollateral, a.Margin)
			cexAssets[a.Index].PortfolioMarginCollateral = safeAdd(cexAssets[a.Index].PortfolioMarginCollateral, a.PortfolioMargin)
		}

		leaf := tier3host.AccountLeafHash(&account, assetCountTiers)
		if err := accountTree.Set(uint64(account.AccountIndex), leaf); err != nil {
			panic(err.Error())
		}

		op.AfterAccountTreeRoot = accountTree.Root()
		op.AccountIndex = account.AccountIndex
		op.AccountIDHash = account.AccountID
		op.Assets = account.Assets
	}

	wit.AfterCEXAssetsCommitment = tier3host.ComputeCexAssetsCommitment(cexAssets, len(cexAssets))
	wit.AfterAccountTreeRoot = accountTree.Root()
	wit.BatchCommitment = poseidon.PoseidonBytes(
		wit.BeforeAccountTreeRoot,
		wit.AfterAccountTreeRoot,
		wit.BeforeCEXAssetsCommitment,
		wit.AfterCEXAssetsCommitment,
	)
	return wit
}

// safeAdd panics on uint64 overflow. Mirrors legacy src/utils.SafeAdd:
// the snapshot adapter already rejects rows whose per-asset balances
// would individually exceed uint64, so the only way to land here is a
// genuine accumulation overflow — a deployment-scale data error worth
// crashing.
func safeAdd(a, b uint64) uint64 {
	s := a + b
	if s < a {
		panic(fmt.Sprintf("uint64 overflow adding %d + %d", a, b))
	}
	return s
}
