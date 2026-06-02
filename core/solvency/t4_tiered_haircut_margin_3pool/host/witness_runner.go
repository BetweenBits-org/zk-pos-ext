package host

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
	t4spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	bsmt "github.com/bnb-chain/zkbnb-smt"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// WitnessRunnerConfig is the model-typed dependency bundle for one
// witness run. cmd/witness's main constructs this per the loaded
// profile.toml + flags and hands it to RunWitness.
//
// Snapshot is model-typed (t4spec.SnapshotSource) so the runner can
// use the model-specific AccountInfo + CexAssetInfo shapes without
// type assertions. The rest of the fields are universal.
type WitnessRunnerConfig struct {
	Ctx             context.Context
	Snapshot        t4spec.SnapshotSource
	AccountTree     bsmt.SparseMerkleTree
	WitnessStore    corehost.WitnessQueue
	ShapeProvider   corespec.BatchShapeProvider
	AssetCountTiers []int
	// DumpFinalCex (optional) — when non-empty, after the last batch
	// closes the runner writes the running CexAssetInfo slice as JSON
	// to this path. Smoke harness convenience.
	DumpFinalCex string
}

// RunWitness drains the model-typed snapshot, buckets accounts by
// BatchShape tier, walks the batches in ascending tier order, and
// persists one BatchCreateUserWitness row per batch. The tree is
// committed at the end of every batch with pruning disabled
// (Commit(nil)) — same convention as the legacy witness, safe for
// memory + Redis backends.
//
// G6 invariant (PriceMultiplier × BalanceMultiplier == ValueScale) is
// asserted inside declarative.BuildPricing before the snapshot is
// constructed, so this runner can assume scaled uint64 inputs.
func RunWitness(cfg WitnessRunnerConfig) error {
	cexAssets, err := cfg.Snapshot.CexAssets(cfg.Ctx)
	if err != nil {
		return fmt.Errorf("CexAssets: %w", err)
	}

	// Zero the published sum fields. Static fields (BasePrice,
	// LoanRatios, MarginRatios, PortfolioMarginRatios) preserved.
	// Same fix class as T1.
	for i := range cexAssets {
		cexAssets[i].TotalEquity = 0
		cexAssets[i].TotalDebt = 0
		cexAssets[i].LoanCollateral = 0
		cexAssets[i].MarginCollateral = 0
		cexAssets[i].PortfolioMarginCollateral = 0
	}

	accountsByTier, err := streamAndBucket(cfg.Ctx, cfg.Snapshot, cfg.AssetCountTiers)
	if err != nil {
		return err
	}
	totalAccounts := 0
	for _, accs := range accountsByTier {
		totalAccounts += len(accs)
	}
	fmt.Printf("loaded %d accounts across %d tiers\n", totalAccounts, len(accountsByTier))

	fmt.Printf("account tree initialised, root = %x\n", cfg.AccountTree.Root())

	if err := runBatches(
		accountsByTier, cexAssets, cfg.AccountTree, cfg.WitnessStore,
		cfg.AssetCountTiers, cfg.ShapeProvider, totalAccounts,
	); err != nil {
		return err
	}

	fmt.Printf("witness run finished, account tree root = %x\n", cfg.AccountTree.Root())

	if cfg.DumpFinalCex != "" {
		raw, err := json.MarshalIndent(cexAssets, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal final cex assets: %w", err)
		}
		if err := os.WriteFile(cfg.DumpFinalCex, raw, 0o644); err != nil {
			return fmt.Errorf("write %q: %w", cfg.DumpFinalCex, err)
		}
		fmt.Printf("final cex assets written to %s\n", cfg.DumpFinalCex)
	}
	return nil
}

func streamAndBucket(ctx context.Context, snapshot t4spec.SnapshotSource, tiers []int) (map[int][]t4spec.AccountInfo, error) {
	ch, err := snapshot.AccountStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("AccountStream: %w", err)
	}
	out := make(map[int][]t4spec.AccountInfo)
	for account := range ch {
		tier := t4spec.PickAssetCountTier(t4spec.CountNonEmptyAssets(account.Assets), tiers)
		if tier == 0 {
			return nil, fmt.Errorf("account %d has %d non-empty assets — no tier in %v fits",
				account.AccountIndex, t4spec.CountNonEmptyAssets(account.Assets), tiers)
		}
		out[tier] = append(out[tier], account)
	}
	return out, nil
}

func runBatches(
	accountsByTier map[int][]t4spec.AccountInfo,
	cexAssets []t4spec.CexAssetInfo,
	accountTree bsmt.SparseMerkleTree,
	witnessStore corehost.WitnessQueue,
	assetCountTiers []int,
	shapeProvider corespec.BatchShapeProvider,
	totalAccounts int,
) error {
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
			return err
		}
		usersPerBatch := shape.UsersPerBatch

		paddingStart, accountsByTier[assetKey] = PaddingAccounts(
			accountsByTier[assetKey], assetKey, paddingStart, usersPerBatch,
		)
		accounts := accountsByTier[assetKey]
		batches := len(accounts) / usersPerBatch
		fmt.Printf("tier %d: %d accounts → %d batches (%d/batch)\n", assetKey, len(accounts), batches, usersPerBatch)

		for b := range batches {
			batch := accounts[b*usersPerBatch : (b+1)*usersPerBatch]
			wit, err := buildBatch(batch, cexAssets, accountTree, assetCountTiers)
			if err != nil {
				return fmt.Errorf("batch %d: %w", height, err)
			}

			encoded, err := EncodeBatchWitness(wit)
			if err != nil {
				return err
			}
			row := corehost.BatchWitnessDTO{
				Height:      height,
				WitnessData: base64.StdEncoding.EncodeToString(encoded),
				Status:      corehost.StatusPublished,
			}
			if err := witnessStore.Create([]corehost.BatchWitnessDTO{row}); err != nil {
				return err
			}

			// bsmt.Commit takes a *pruning* version (not the new commit
			// version). Passing nil disables pruning — fine for memory +
			// Redis backends until a multi-batch resume slice needs to
			// bound disk growth.
			if _, err := accountTree.Commit(nil); err != nil {
				return err
			}
			height++
		}
	}
	return nil
}

func buildBatch(
	batch []t4spec.AccountInfo,
	cexAssets []t4spec.CexAssetInfo,
	accountTree bsmt.SparseMerkleTree,
	assetCountTiers []int,
) (*t4spec.BatchCreateUserWitness, error) {
	wit := &t4spec.BatchCreateUserWitness{
		BeforeAccountTreeRoot: accountTree.Root(),
		BeforeCexAssets:       make([]t4spec.CexAssetInfo, len(cexAssets)),
		CreateUserOps:         make([]t4spec.CreateUserOperation, len(batch)),
	}
	copy(wit.BeforeCexAssets, cexAssets)
	wit.BeforeCEXAssetsCommitment = ComputeCexAssetsCommitment(cexAssets, len(cexAssets))

	for i, account := range batch {
		op := &wit.CreateUserOps[i]
		op.BeforeAccountTreeRoot = accountTree.Root()

		proof, err := accountTree.GetProof(uint64(account.AccountIndex))
		if err != nil {
			return nil, err
		}
		copy(op.AccountProof[:], proof)

		// Update running per-asset totals from this user's contribution.
		for _, a := range account.Assets {
			next, err := safeAdd(cexAssets[a.Index].TotalEquity, a.Equity)
			if err != nil {
				return nil, fmt.Errorf("cex TotalEquity overflow: %w", err)
			}
			cexAssets[a.Index].TotalEquity = next
			if next, err = safeAdd(cexAssets[a.Index].TotalDebt, a.Debt); err != nil {
				return nil, fmt.Errorf("cex TotalDebt overflow: %w", err)
			}
			cexAssets[a.Index].TotalDebt = next
			if next, err = safeAdd(cexAssets[a.Index].LoanCollateral, a.Loan); err != nil {
				return nil, fmt.Errorf("cex LoanCollateral overflow: %w", err)
			}
			cexAssets[a.Index].LoanCollateral = next
			if next, err = safeAdd(cexAssets[a.Index].MarginCollateral, a.Margin); err != nil {
				return nil, fmt.Errorf("cex MarginCollateral overflow: %w", err)
			}
			cexAssets[a.Index].MarginCollateral = next
			if next, err = safeAdd(cexAssets[a.Index].PortfolioMarginCollateral, a.PortfolioMargin); err != nil {
				return nil, fmt.Errorf("cex PortfolioMarginCollateral overflow: %w", err)
			}
			cexAssets[a.Index].PortfolioMarginCollateral = next
		}

		leaf := AccountLeafHash(&account, assetCountTiers)
		if err := accountTree.Set(uint64(account.AccountIndex), leaf); err != nil {
			return nil, err
		}

		op.AfterAccountTreeRoot = accountTree.Root()
		op.AccountIndex = account.AccountIndex
		op.AccountIDHash = account.AccountID
		op.Assets = account.Assets
	}

	wit.AfterCEXAssetsCommitment = ComputeCexAssetsCommitment(cexAssets, len(cexAssets))
	wit.AfterAccountTreeRoot = accountTree.Root()
	wit.BatchCommitment = poseidon.PoseidonBytes(
		wit.BeforeAccountTreeRoot,
		wit.AfterAccountTreeRoot,
		wit.BeforeCEXAssetsCommitment,
		wit.AfterCEXAssetsCommitment,
	)
	return wit, nil
}

// safeAdd returns a+b when the sum fits in uint64, error otherwise.
// The snapshot adapter already rejects per-row balance overflow, so
// reaching here means a genuine batch-accumulation overflow.
func safeAdd(a, b uint64) (uint64, error) {
	s := a + b
	if s < a {
		return 0, fmt.Errorf("uint64 overflow adding %d + %d", a, b)
	}
	return s, nil
}
