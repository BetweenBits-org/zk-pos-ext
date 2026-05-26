package circuit

import (
	t1spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/spec"
)

// SetBatchCreateUserCircuitWitness converts a snapshot-shaped witness
// into the in-circuit BatchCreateUserCircuit. assetCountTiers MUST
// match the BatchShape this circuit was sized for (sorted ascending);
// each user's non-empty asset count is rounded up to the smallest tier
// and the slice is padded with zero entries at synthetic indexes so
// the per-user Assets slice length is constant within a batch.
//
// Compared to t4_tiered_haircut_margin_3pool's SetBatchCreateUserCircuitWitness: no
// per-asset tier-index computation, no collateral routing — spot users
// have only Equity, so each padding entry is fully zero-initialised.
func SetBatchCreateUserCircuitWitness(
	batchWitness *t1spec.BatchCreateUserWitness,
	assetCountTiers []int,
) (*BatchCreateUserCircuit, error) {
	w := &BatchCreateUserCircuit{
		BatchCommitment:           batchWitness.BatchCommitment,
		BeforeAccountTreeRoot:     batchWitness.BeforeAccountTreeRoot,
		AfterAccountTreeRoot:      batchWitness.AfterAccountTreeRoot,
		BeforeCEXAssetsCommitment: batchWitness.BeforeCEXAssetsCommitment,
		AfterCEXAssetsCommitment:  batchWitness.AfterCEXAssetsCommitment,
		BeforeCexAssets:           make([]CexAssetInfo, len(batchWitness.BeforeCexAssets)),
		CreateUserOps:             make([]CreateUserOperation, len(batchWitness.CreateUserOps)),
	}

	for i := range w.BeforeCexAssets {
		src := &batchWitness.BeforeCexAssets[i]
		w.BeforeCexAssets[i].TotalEquity = src.TotalEquity
		w.BeforeCexAssets[i].TotalDebt = src.TotalDebt
		w.BeforeCexAssets[i].BasePrice = src.BasePrice
	}

	cexAssetsCount := len(w.BeforeCexAssets)
	// Per-batch asset count is decided by the first user; subsequent users may be padding.
	targetCounts := t1spec.PickAssetCountTier(
		t1spec.CountNonEmptyAssets(batchWitness.CreateUserOps[0].Assets),
		assetCountTiers,
	)
	for i := range w.CreateUserOps {
		w.CreateUserOps[i].BeforeAccountTreeRoot = batchWitness.CreateUserOps[i].BeforeAccountTreeRoot
		w.CreateUserOps[i].AfterAccountTreeRoot = batchWitness.CreateUserOps[i].AfterAccountTreeRoot
		w.CreateUserOps[i].AssetsForUpdateCex = make([]UserAssetMeta, cexAssetsCount)

		existingKeys := make([]int, 0)
		for j := range batchWitness.CreateUserOps[i].Assets {
			u := batchWitness.CreateUserOps[i].Assets[j]
			w.CreateUserOps[i].AssetsForUpdateCex[j] = UserAssetMeta{
				Equity: u.Equity,
				Debt:   u.Debt,
			}
			if !t1spec.IsAccountAssetEmpty(&u) {
				existingKeys = append(existingKeys, int(u.Index))
			}
		}

		paddingCounts := targetCounts - len(existingKeys)
		w.CreateUserOps[i].Assets = make([]UserAssetInfo, targetCounts)
		currentPaddingCounts := 0
		currentAssetIndex := 0
		index := 0
		// paddingAsset returns a fully zero-initialised UserAssetInfo at
		// the given asset slot. AssetIndex set, Equity AND Debt
		// explicitly 0 (vs nil — gnark rejects nil Variables with
		// "can't set fr.Element with <nil>" — same bug class as the
		// t4_tiered_haircut_margin_3pool fix in commit d7c23f3).
		paddingAsset := func(slot uint32) UserAssetInfo {
			return UserAssetInfo{
				AssetIndex: slot,
				Equity:     uint64(0),
				Debt:       uint64(0),
			}
		}
		for _, v := range existingKeys {
			if currentPaddingCounts < paddingCounts {
				for k := currentAssetIndex; k < v; k++ {
					currentPaddingCounts++
					w.CreateUserOps[i].Assets[index] = paddingAsset(uint32(k))
					index++
					if currentPaddingCounts >= paddingCounts {
						break
					}
				}
			}
			u := UserAssetInfo{
				AssetIndex: uint32(v),
				Equity:     batchWitness.CreateUserOps[i].Assets[v].Equity,
				Debt:       batchWitness.CreateUserOps[i].Assets[v].Debt,
			}
			w.CreateUserOps[i].Assets[index] = u
			index++
			currentAssetIndex = v + 1
		}
		for k := index; k < targetCounts; k++ {
			w.CreateUserOps[i].Assets[k] = paddingAsset(uint32(currentAssetIndex))
			currentAssetIndex++
		}
		w.CreateUserOps[i].AccountIdHash = batchWitness.CreateUserOps[i].AccountIDHash
		w.CreateUserOps[i].AccountIndex = batchWitness.CreateUserOps[i].AccountIndex
		for j := 0; j < len(w.CreateUserOps[i].AccountProof); j++ {
			w.CreateUserOps[i].AccountProof[j] = batchWitness.CreateUserOps[i].AccountProof[j]
		}
	}
	return w, nil
}
