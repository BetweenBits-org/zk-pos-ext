package host

import (
	"context"
	"math/big"

	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
	t1spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t1_simple_margin/spec"
)

// CollectSumLeaves streams the T1 snapshot and produces the dense,
// positional Merkle-sum leaf records for the non-zk proof-of-liabilities
// side line (PRODUCTION_ROADMAP Stage MS). Each real account becomes one
// corehost.SumLeafRecord:
//
//   - LeafHash = AccountLeafHash(account) — the frozen 5-input account
//     leaf, reused as the sum-tree leaf identity (gate G19, D5).
//   - Balance  = TotalEquity - TotalDebt — the non-negative net liability
//     the exchange owes the account. For T1 the circuit-side invariant is
//     TotalEquity >= TotalDebt, so a real account's net is >= 0; a dataset
//     that violates it is caught by corehost.Reconcile before any tree is
//     built.
//
// Positions are assigned in deterministic order (tier ascending, then
// stream order), independent of the sparse account-tree AccountIndex.
//
// Unlike the witness/userproof runners this does NOT pad: the dense sum
// tree pads itself to the next power of two with zero-sum leaves
// (core/sumtree.Build), and padding accounts carry no liability, so they
// are intentionally excluded from the proof-of-liabilities set.
func CollectSumLeaves(ctx context.Context, snapshot t1spec.SnapshotSource, assetCountTiers []int) ([]corehost.SumLeafRecord, error) {
	accountsByTier, err := streamAndBucket(ctx, snapshot, assetCountTiers)
	if err != nil {
		return nil, err
	}

	tiers := sortedKeys(accountsByTier)
	out := make([]corehost.SumLeafRecord, 0)
	pos := 0
	for _, k := range tiers {
		accs := accountsByTier[k]
		for i := range accs {
			acc := &accs[i]
			equity := acc.TotalEquity
			if equity == nil {
				equity = big.NewInt(0)
			}
			debt := acc.TotalDebt
			if debt == nil {
				debt = big.NewInt(0)
			}
			out = append(out, corehost.SumLeafRecord{
				Position:  pos,
				AccountID: acc.AccountID,
				LeafHash:  AccountLeafHash(acc, assetCountTiers),
				Balance:   new(big.Int).Sub(equity, debt),
			})
			pos++
		}
	}
	return out, nil
}
