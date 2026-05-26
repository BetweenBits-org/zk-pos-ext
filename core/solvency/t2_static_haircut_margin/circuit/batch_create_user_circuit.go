package circuit

import (
	corecircuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/circuit"
	t2spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t2_static_haircut_margin/spec"
	"github.com/consensys/gnark/std/hash/poseidon"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"github.com/consensys/gnark/std/rangecheck"
)

// BatchCreateUserCircuit is the gnark Circuit type for the
// t2_static_haircut_margin model. Structural simplification of T3 —
// same per-asset and per-user layout, but the piecewise-linear tier
// curve is collapsed to a single Haircut constant per asset (basis
// points). One multiply + one division per asset, no tier-table lookup.
//
// module is the unexported alpha-layer ConstraintModule hook (same
// pattern as t1 / t3 / t4).
type BatchCreateUserCircuit struct {
	BatchCommitment           Variable `gnark:",public"`
	BeforeAccountTreeRoot     Variable
	AfterAccountTreeRoot      Variable
	BeforeCEXAssetsCommitment Variable
	AfterCEXAssetsCommitment  Variable
	BeforeCexAssets           []CexAssetInfo
	CreateUserOps             []CreateUserOperation

	module t2spec.ConstraintModule
}

func (b *BatchCreateUserCircuit) SetConstraintModule(m t2spec.ConstraintModule) {
	b.module = m
}

func NewVerifyBatchCreateUserCircuit(commitment []byte) *BatchCreateUserCircuit {
	return &BatchCreateUserCircuit{BatchCommitment: commitment}
}

func NewBatchCreateUserCircuit(userAssetCounts, allAssetCounts, batchCounts uint32) *BatchCreateUserCircuit {
	c := &BatchCreateUserCircuit{
		BeforeCexAssets: make([]CexAssetInfo, allAssetCounts),
		CreateUserOps:   make([]CreateUserOperation, batchCounts),
	}
	for i := range c.CreateUserOps {
		c.CreateUserOps[i] = CreateUserOperation{
			Assets:             make([]UserAssetInfo, userAssetCounts),
			AssetsForUpdateCex: make([]UserAssetMeta, allAssetCounts),
		}
		for j := uint32(0); j < userAssetCounts; j++ {
			c.CreateUserOps[i].Assets[j].AssetIndex = j
		}
	}
	return c
}

// Define emits the t2 constraint set:
//
//  1. BatchCommitment == Poseidon(BeforeRoot, AfterRoot, BeforeCEX, AfterCEX)
//  2. BeforeCEXAssetsCommitment of the packed BeforeCexAssets
//  3. Per-user Merkle proof verifies (before) + updates (after)
//  4. Per-asset, per-user: Collateral ≤ Equity (sanity)
//  5. Per-user: TotalDebt ≤ Σ_i (Collateral_i × Price_i × Haircut_i / 10000)
//  6. Linear-combination check covers Equity / Debt / Collateral
//  7. AfterCEXAssetsCommitment accumulates per-user deltas
//  8. CreateUserOps roots chain
func (b BatchCreateUserCircuit) Define(api API) error {
	actualBatchCommitment := corecircuit.BatchCommitment(
		api, b.BeforeAccountTreeRoot, b.AfterAccountTreeRoot,
		b.BeforeCEXAssetsCommitment, b.AfterCEXAssetsCommitment,
	)
	api.AssertIsEqual(b.BatchCommitment, actualBatchCommitment)

	countOfCexAsset := getVariableCountOfCexAsset(b.BeforeCexAssets[0])
	cexAssets := make([]Variable, len(b.BeforeCexAssets)*countOfCexAsset)
	afterCexAssets := make([]CexAssetInfo, len(b.BeforeCexAssets))

	r := rangecheck.New(api)
	assetPriceTable := logderivlookup.New(api)
	assetHaircutTable := logderivlookup.New(api)
	for i := 0; i < len(b.BeforeCexAssets); i++ {
		r.Check(b.BeforeCexAssets[i].TotalEquity, 64)
		r.Check(b.BeforeCexAssets[i].TotalDebt, 64)
		r.Check(b.BeforeCexAssets[i].BasePrice, 64)
		r.Check(b.BeforeCexAssets[i].Collateral, 64)
		r.Check(b.BeforeCexAssets[i].Haircut, 16)

		fillCexAssetCommitment(api, b.BeforeCexAssets[i], i, cexAssets)
		afterCexAssets[i] = b.BeforeCexAssets[i]

		assetPriceTable.Insert(b.BeforeCexAssets[i].BasePrice)
		assetHaircutTable.Insert(b.BeforeCexAssets[i].Haircut)
	}
	actualCexAssetsCommitment := poseidon.Poseidon(api, cexAssets...)
	api.AssertIsEqual(b.BeforeCEXAssetsCommitment, actualCexAssetsCommitment)
	api.AssertIsEqual(b.BeforeAccountTreeRoot, b.CreateUserOps[0].BeforeAccountTreeRoot)
	api.AssertIsEqual(b.AfterAccountTreeRoot, b.CreateUserOps[len(b.CreateUserOps)-1].AfterAccountTreeRoot)

	userAssetIdHashes := make([]Variable, len(b.CreateUserOps)+1)
	userAssetsResults := make([][]Variable, len(b.CreateUserOps))
	userAssetsQueries := make([][]Variable, len(b.CreateUserOps))
	moduleUserOps := make([]t2spec.CircuitUserOp, len(b.CreateUserOps))

	for i := 0; i < len(b.CreateUserOps); i++ {
		accountIndexHelper := corecircuit.AccountIndexToMerkleHelper(api, b.CreateUserOps[i].AccountIndex)
		corecircuit.VerifyMerkleProof(api, b.CreateUserOps[i].BeforeAccountTreeRoot, EmptyAccountLeafNodeHash, b.CreateUserOps[i].AccountProof[:], accountIndexHelper)
		var totalUserEquity Variable = 0
		var totalUserDebt Variable = 0
		userAssets := b.CreateUserOps[i].Assets
		var totalUserCollateralRealValue Variable = 0

		// Per-batch entry: (Equity, Debt, Collateral) lookup table.
		userAssetsLookupTable := logderivlookup.New(api)
		for j := 0; j < len(b.CreateUserOps[i].AssetsForUpdateCex); j++ {
			userAssetsLookupTable.Insert(b.CreateUserOps[i].AssetsForUpdateCex[j].Equity)
			userAssetsLookupTable.Insert(b.CreateUserOps[i].AssetsForUpdateCex[j].Debt)
			userAssetsLookupTable.Insert(b.CreateUserOps[i].AssetsForUpdateCex[j].Collateral)
		}

		// Strictly-increasing AssetIndex enforces uniqueness.
		for j := 0; j < len(userAssets)-1; j++ {
			r.Check(userAssets[j].AssetIndex, 16)
			cr := api.CmpNOp(userAssets[j+1].AssetIndex, userAssets[j].AssetIndex, 16, true)
			api.AssertIsEqual(cr, 1)
		}

		// Pack 15 asset indexes per field element, then hash.
		assetIdsToVariables := make([]Variable, (len(userAssets)+14)/15)
		for j := 0; j < len(assetIdsToVariables); j++ {
			var v Variable = 0
			for p := j * 15; p < (j+1)*15 && p < len(userAssets); p++ {
				v = api.Add(v, api.Mul(userAssets[p].AssetIndex, PowersOfSixteenBits[p%15]))
			}
			assetIdsToVariables[j] = v
		}
		userAssetIdHashes[i] = poseidon.Poseidon(api, assetIdsToVariables...)

		// 3 queries per user-asset (Equity, Debt, Collateral) for the RLC.
		userAssetsQueries[i] = make([]Variable, len(userAssets)*3)
		assetPriceQueries := make([]Variable, len(userAssets))
		numOfAssetsFields := 4 // (Index, Equity, Debt, Collateral)
		for j := 0; j < len(userAssets); j++ {
			p := api.Mul(userAssets[j].AssetIndex, 3)
			for k := 0; k < 3; k++ {
				userAssetsQueries[i][j*3+k] = api.Add(p, k)
			}
			assetPriceQueries[j] = userAssets[j].AssetIndex
		}
		userAssetsResults[i] = userAssetsLookupTable.Lookup(userAssetsQueries[i]...)
		assetPriceResponses := assetPriceTable.Lookup(assetPriceQueries...)
		assetHaircutResponses := assetHaircutTable.Lookup(assetPriceQueries...)

		flattenAssetFieldsForHash := make([]Variable, len(userAssets)*numOfAssetsFields)
		for j := 0; j < len(userAssets); j++ {
			userEquity := userAssetsResults[i][j*3]
			r.Check(userEquity, 64)
			userDebt := userAssetsResults[i][j*3+1]
			r.Check(userDebt, 64)
			userCollateral := userAssetsResults[i][j*3+2]
			r.Check(userCollateral, 64)

			flattenAssetFieldsForHash[j*numOfAssetsFields] = userAssets[j].AssetIndex
			flattenAssetFieldsForHash[j*numOfAssetsFields+1] = userEquity
			flattenAssetFieldsForHash[j*numOfAssetsFields+2] = userDebt
			flattenAssetFieldsForHash[j*numOfAssetsFields+3] = userCollateral

			// Sanity: pledged collateral can't exceed equity per asset.
			api.AssertIsLessOrEqualNOp(userCollateral, userEquity, 64, true)

			// T2 per-asset haircut multiply:
			//   collateralBpValue = userCollateral × assetPrice × haircut  (~144 bits)
			//   realValue         = collateralBpValue / 10000
			collateralBpValue := api.Mul(userCollateral, assetPriceResponses[j], assetHaircutResponses[j])
			collateralRealValue := checkAndGetIntegerDivisionRes(api, r, collateralBpValue)

			totalUserCollateralRealValue = api.Add(totalUserCollateralRealValue, collateralRealValue)

			totalUserEquity = api.Add(totalUserEquity, api.Mul(userEquity, assetPriceResponses[j]))
			totalUserDebt = api.Add(totalUserDebt, api.Mul(userDebt, assetPriceResponses[j]))
		}

		// Accumulate per-slot deltas.
		for j := 0; j < len(b.CreateUserOps[i].AssetsForUpdateCex); j++ {
			afterCexAssets[j].TotalEquity = api.Add(afterCexAssets[j].TotalEquity, b.CreateUserOps[i].AssetsForUpdateCex[j].Equity)
			afterCexAssets[j].TotalDebt = api.Add(afterCexAssets[j].TotalDebt, b.CreateUserOps[i].AssetsForUpdateCex[j].Debt)
			afterCexAssets[j].Collateral = api.Add(afterCexAssets[j].Collateral, b.CreateUserOps[i].AssetsForUpdateCex[j].Collateral)
		}

		r.Check(totalUserDebt, 128)
		r.Check(totalUserCollateralRealValue, 128)
		// T2 defining constraint: TotalDebt ≤ haircut-weighted TotalCollateral.
		api.AssertIsLessOrEqualNOp(totalUserDebt, totalUserCollateralRealValue, 128, true)

		// Account leaf — universal 5-input Poseidon.
		userAssetsCommitment := corecircuit.ComputeFlatUint64Commitment(api, flattenAssetFieldsForHash)
		accountHash := poseidon.Poseidon(api, b.CreateUserOps[i].AccountIdHash, totalUserEquity, totalUserDebt, totalUserCollateralRealValue, userAssetsCommitment)
		actualAccountTreeRoot := corecircuit.UpdateMerkleProof(api, accountHash, b.CreateUserOps[i].AccountProof[:], accountIndexHelper)
		api.AssertIsEqual(actualAccountTreeRoot, b.CreateUserOps[i].AfterAccountTreeRoot)

		moduleUserOps[i] = t2spec.CircuitUserOp{
			AccountIndex:            b.CreateUserOps[i].AccountIndex,
			AccountIDHash:           b.CreateUserOps[i].AccountIdHash,
			TotalUserEquity:         totalUserEquity,
			TotalUserDebt:           totalUserDebt,
			TotalUserCollateralReal: totalUserCollateralRealValue,
		}
	}

	// RLC check: 3 powers per CEX slot (Equity / Debt / Collateral).
	userAssetIdHashes[len(b.CreateUserOps)] = b.BatchCommitment
	randomChallenge := poseidon.Poseidon(api, userAssetIdHashes...)
	powersOfRandomChallenge := make([]Variable, 3*len(b.BeforeCexAssets))
	powersOfRandomChallenge[0] = randomChallenge
	powersOfRandomChallengeLookupTable := logderivlookup.New(api)
	powersOfRandomChallengeLookupTable.Insert(randomChallenge)
	for i := 1; i < len(powersOfRandomChallenge); i++ {
		powersOfRandomChallenge[i] = api.Mul(powersOfRandomChallenge[i-1], randomChallenge)
		powersOfRandomChallengeLookupTable.Insert(powersOfRandomChallenge[i])
	}

	for i := 0; i < len(b.CreateUserOps); i++ {
		powersOfRCResults := powersOfRandomChallengeLookupTable.Lookup(userAssetsQueries[i]...)
		var sumA Variable = 0
		for j := 0; j < len(powersOfRCResults); j++ {
			sumA = api.Add(sumA, api.Mul(powersOfRCResults[j], userAssetsResults[i][j]))
		}

		var sumB Variable = 0
		for j := 0; j < len(b.CreateUserOps[i].AssetsForUpdateCex); j++ {
			sumB = api.Add(sumB, api.Mul(b.CreateUserOps[i].AssetsForUpdateCex[j].Equity, powersOfRandomChallenge[3*j]))
			sumB = api.Add(sumB, api.Mul(b.CreateUserOps[i].AssetsForUpdateCex[j].Debt, powersOfRandomChallenge[3*j+1]))
			sumB = api.Add(sumB, api.Mul(b.CreateUserOps[i].AssetsForUpdateCex[j].Collateral, powersOfRandomChallenge[3*j+2]))
		}
		api.AssertIsEqual(sumA, sumB)
	}

	tempAfterCexAssets := make([]Variable, len(b.BeforeCexAssets)*countOfCexAsset)
	for j := 0; j < len(b.BeforeCexAssets); j++ {
		r.Check(afterCexAssets[j].TotalEquity, 64)
		r.Check(afterCexAssets[j].TotalDebt, 64)
		r.Check(afterCexAssets[j].Collateral, 64)
		fillCexAssetCommitment(api, afterCexAssets[j], j, tempAfterCexAssets)
	}
	actualAfterCEXAssetsCommitment := poseidon.Poseidon(api, tempAfterCexAssets...)
	api.AssertIsEqual(actualAfterCEXAssetsCommitment, b.AfterCEXAssetsCommitment)
	for i := 0; i < len(b.CreateUserOps)-1; i++ {
		api.AssertIsEqual(b.CreateUserOps[i].AfterAccountTreeRoot, b.CreateUserOps[i+1].BeforeAccountTreeRoot)
	}

	if b.module != nil {
		ctx := t2spec.ConstraintContext{
			BeforeCexAssets: toCircuitCexAssetView(b.BeforeCexAssets),
			AfterCexAssets:  toCircuitCexAssetView(afterCexAssets),
			UserOps:         moduleUserOps,
			R:               r,
		}
		if err := b.module.Define(api, ctx); err != nil {
			return err
		}
	}
	return nil
}

// toCircuitCexAssetView translates the in-circuit CexAssetInfo slice
// into the t2spec.CircuitCexAsset view shape exposed to
// ConstraintModule hooks.
func toCircuitCexAssetView(src []CexAssetInfo) []t2spec.CircuitCexAsset {
	out := make([]t2spec.CircuitCexAsset, len(src))
	for i := range src {
		out[i] = t2spec.CircuitCexAsset{
			TotalEquity: src[i].TotalEquity,
			TotalDebt:   src[i].TotalDebt,
			BasePrice:   src[i].BasePrice,
			Collateral:  src[i].Collateral,
			Haircut:     src[i].Haircut,
		}
	}
	return out
}

// SetBatchCreateUserCircuitWitness converts a snapshot-shaped witness
// into the in-circuit BatchCreateUserCircuit. T2's witness shape is
// simpler than T3 — no per-asset tier-index calculation.
func SetBatchCreateUserCircuitWitness(
	batchWitness *t2spec.BatchCreateUserWitness,
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
		w.BeforeCexAssets[i].Collateral = src.Collateral
		w.BeforeCexAssets[i].Haircut = uint64(src.Haircut)
	}

	cexAssetsCount := len(w.BeforeCexAssets)
	targetCounts := t2spec.PickAssetCountTier(
		t2spec.CountNonEmptyAssets(batchWitness.CreateUserOps[0].Assets),
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
				Equity:     u.Equity,
				Debt:       u.Debt,
				Collateral: u.Collateral,
			}
			if !t2spec.IsAccountAssetEmpty(&u) {
				existingKeys = append(existingKeys, int(u.Index))
			}
		}

		paddingCounts := targetCounts - len(existingKeys)
		w.CreateUserOps[i].Assets = make([]UserAssetInfo, targetCounts)
		currentPaddingCounts := 0
		currentAssetIndex := 0
		index := 0
		paddingAsset := func(slot uint32) UserAssetInfo {
			return UserAssetInfo{AssetIndex: slot}
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
			w.CreateUserOps[i].Assets[index] = UserAssetInfo{AssetIndex: uint32(v)}
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
