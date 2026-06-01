package host

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	t3spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t3_tiered_haircut_margin_1pool/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	bsmt "github.com/bnb-chain/zkbnb-smt"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// WitnessRunnerConfig is the T3 model-typed dependency bundle. T3
// shares T2's per-asset collateral pool but replaces the static
// haircut_bp with a tiered curve (CollateralRatios) — the curve is
// static per snapshot, so the batch accumulation is identical to T2.
type WitnessRunnerConfig struct {
	Ctx             context.Context
	Snapshot        t3spec.SnapshotSource
	AccountTree     bsmt.SparseMerkleTree
	WitnessStore    corehost.WitnessQueue
	ShapeProvider   corespec.BatchShapeProvider
	AssetCountTiers []int
	DumpFinalCex    string
}

// RunWitness — see t4_tiered_haircut_margin_3pool/host.RunWitness
// docstring.
func RunWitness(cfg WitnessRunnerConfig) error {
	cexAssets, err := cfg.Snapshot.CexAssets(cfg.Ctx)
	if err != nil {
		return fmt.Errorf("CexAssets: %w", err)
	}

	// Zero the published sum fields. Static fields (BasePrice,
	// CollateralRatios) preserved. Same fix class as T1.
	for i := range cexAssets {
		cexAssets[i].TotalEquity = 0
		cexAssets[i].TotalDebt = 0
		cexAssets[i].Collateral = 0
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

func streamAndBucket(ctx context.Context, snapshot t3spec.SnapshotSource, tiers []int) (map[int][]t3spec.AccountInfo, error) {
	ch, err := snapshot.AccountStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("AccountStream: %w", err)
	}
	out := make(map[int][]t3spec.AccountInfo)
	for account := range ch {
		tier := t3spec.PickAssetCountTier(t3spec.CountNonEmptyAssets(account.Assets), tiers)
		if tier == 0 {
			return nil, fmt.Errorf("account %d has %d non-empty assets — no tier in %v fits",
				account.AccountIndex, t3spec.CountNonEmptyAssets(account.Assets), tiers)
		}
		out[tier] = append(out[tier], account)
	}
	return out, nil
}

func runBatches(
	accountsByTier map[int][]t3spec.AccountInfo,
	cexAssets []t3spec.CexAssetInfo,
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
			if _, err := accountTree.Commit(nil); err != nil {
				return err
			}
			height++
		}
	}
	return nil
}

func buildBatch(
	batch []t3spec.AccountInfo,
	cexAssets []t3spec.CexAssetInfo,
	accountTree bsmt.SparseMerkleTree,
	assetCountTiers []int,
) (*t3spec.BatchCreateUserWitness, error) {
	wit := &t3spec.BatchCreateUserWitness{
		BeforeAccountTreeRoot: accountTree.Root(),
		BeforeCexAssets:       make([]t3spec.CexAssetInfo, len(cexAssets)),
		CreateUserOps:         make([]t3spec.CreateUserOperation, len(batch)),
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

		// T3: equity/debt + single collateral pool. CollateralRatios
		// (tier curve) is static metadata, untouched per batch.
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
			if next, err = safeAdd(cexAssets[a.Index].Collateral, a.Collateral); err != nil {
				return nil, fmt.Errorf("cex Collateral overflow: %w", err)
			}
			cexAssets[a.Index].Collateral = next
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

func safeAdd(a, b uint64) (uint64, error) {
	s := a + b
	if s < a {
		return 0, fmt.Errorf("uint64 overflow adding %d + %d", a, b)
	}
	return s, nil
}
