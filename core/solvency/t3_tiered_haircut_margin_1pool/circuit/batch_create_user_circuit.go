package circuit

import (
	corecircuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/circuit"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	t3spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t3_tiered_haircut_margin_1pool/spec"
	"github.com/consensys/gnark/std/hash/poseidon"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"github.com/consensys/gnark/std/rangecheck"
)

// BatchCreateUserCircuit is the gnark Circuit type for the
// t3_tiered_haircut_margin_1pool model. Structural simplification of
// t4_tiered_haircut_margin_3pool — 3 collateral buckets collapsed into
// one. Per-asset 4-tuple (Index, Equity, Debt, Collateral) vs T4's
// 5-tuple; one tier curve per asset vs T4's three.
//
// module is the unexported alpha-layer ConstraintModule hook (same
// pattern as t1 / t4 — invisible to gnark reflection, zero in-circuit
// cost when nil).
type BatchCreateUserCircuit struct {
	BatchCommitment           Variable `gnark:",public"`
	BeforeAccountTreeRoot     Variable
	AfterAccountTreeRoot      Variable
	BeforeCEXAssetsCommitment Variable
	AfterCEXAssetsCommitment  Variable
	BeforeCexAssets           []CexAssetInfo
	CreateUserOps             []CreateUserOperation

	module t3spec.ConstraintModule
}

// SetConstraintModule wires the alpha-layer ConstraintModule hook.
// Composing a non-nil module forks the trusted setup: .pk/.vk pair
// is unique to the (t3, module) pair.
func (b *BatchCreateUserCircuit) SetConstraintModule(m t3spec.ConstraintModule) {
	b.module = m
}

// NewVerifyBatchCreateUserCircuit returns a circuit instance with
// only the public BatchCommitment populated.
func NewVerifyBatchCreateUserCircuit(commitment []byte) *BatchCreateUserCircuit {
	return &BatchCreateUserCircuit{BatchCommitment: commitment}
}

// NewBatchCreateUserCircuit allocates a fully zero-valued circuit
// instance sized for the given shape.
func NewBatchCreateUserCircuit(userAssetCounts, allAssetCounts, batchCounts uint32) *BatchCreateUserCircuit {
	c := &BatchCreateUserCircuit{
		BeforeCexAssets: make([]CexAssetInfo, allAssetCounts),
		CreateUserOps:   make([]CreateUserOperation, batchCounts),
	}
	for i := range c.BeforeCexAssets {
		c.BeforeCexAssets[i] = CexAssetInfo{
			CollateralRatios: make([]TierRatio, corespec.TierCount),
		}
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

// Define emits the t3 constraint set:
//
//  1. BatchCommitment == Poseidon(BeforeRoot, AfterRoot, BeforeCEX, AfterCEX)
//  2. BeforeCEXAssetsCommitment == Poseidon of the packed BeforeCexAssets
//  3. Tier table monotonic & ratio-capped (rapid arithmetic precompute)
//  4. Per-user Merkle proof verifies (before) and updates (after)
//  5. Per-user, per-asset: Collateral ≤ Equity (sanity)
//  6. Per-user: TotalDebt ≤ haircut(TotalCollateral)
//  7. Linear-combination check covers Equity / Debt / Collateral
//  8. AfterCEXAssetsCommitment accumulates per-user deltas
//  9. CreateUserOps roots chain (op[i].After == op[i+1].Before)
// 10. ConstraintModule hook fires last (if non-nil)
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
	for i := 0; i < len(b.BeforeCexAssets); i++ {
		r.Check(b.BeforeCexAssets[i].TotalEquity, 64)
		r.Check(b.BeforeCexAssets[i].TotalDebt, 64)
		r.Check(b.BeforeCexAssets[i].BasePrice, 64)
		r.Check(b.BeforeCexAssets[i].Collateral, 64)

		fillCexAssetCommitment(api, b.BeforeCexAssets[i], i, cexAssets)
		generateRapidArithmeticForCollateral(api, r, b.BeforeCexAssets[i].CollateralRatios)
		afterCexAssets[i] = b.BeforeCexAssets[i]

		assetPriceTable.Insert(b.BeforeCexAssets[i].BasePrice)
	}
	actualCexAssetsCommitment := poseidon.Poseidon(api, cexAssets...)
	api.AssertIsEqual(b.BeforeCEXAssetsCommitment, actualCexAssetsCommitment)
	api.AssertIsEqual(b.BeforeAccountTreeRoot, b.CreateUserOps[0].BeforeAccountTreeRoot)
	api.AssertIsEqual(b.AfterAccountTreeRoot, b.CreateUserOps[len(b.CreateUserOps)-1].AfterAccountTreeRoot)

	collateralTierRatiosTable := constructCollateralTierRatiosLookupTable(api, b.BeforeCexAssets)
	userAssetIdHashes := make([]Variable, len(b.CreateUserOps)+1)

	userAssetsResults := make([][]Variable, len(b.CreateUserOps))
	userAssetsQueries := make([][]Variable, len(b.CreateUserOps))
	moduleUserOps := make([]t3spec.CircuitUserOp, len(b.CreateUserOps))

	for i := 0; i < len(b.CreateUserOps); i++ {
		accountIndexHelper := corecircuit.AccountIndexToMerkleHelper(api, b.CreateUserOps[i].AccountIndex)
		corecircuit.VerifyMerkleProof(api, b.CreateUserOps[i].BeforeAccountTreeRoot, EmptyAccountLeafNodeHash, b.CreateUserOps[i].AccountProof[:], accountIndexHelper)
		var totalUserEquity Variable = 0
		var totalUserDebt Variable = 0
		userAssets := b.CreateUserOps[i].Assets
		var totalUserCollateralRealValue Variable = 0

		// One lookup table per batch entry, holding (Equity, Debt,
		// Collateral) triples for every CEX asset slot.
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

		// 3 queries per user-asset (Equity, Debt, Collateral) for the
		// linear-combination cross-check below.
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

			// Per-asset sanity: pledged collateral can't exceed equity.
			api.AssertIsLessOrEqualNOp(userCollateral, userEquity, 64, true)

			collateralRealValue := getAndCheckTierRatiosQueryResults(api, r, collateralTierRatiosTable, userAssets[j].AssetIndex,
				userCollateral,
				userAssets[j].CollateralIndex,
				userAssets[j].CollateralFlag,
				assetPriceResponses[j],
				3*(len(b.BeforeCexAssets[j].CollateralRatios)+1))

			totalUserCollateralRealValue = api.Add(totalUserCollateralRealValue, collateralRealValue)

			totalUserEquity = api.Add(totalUserEquity, api.Mul(userEquity, assetPriceResponses[j]))
			totalUserDebt = api.Add(totalUserDebt, api.Mul(userDebt, assetPriceResponses[j]))
		}

		// Accumulate per-slot equity / debt / collateral into the
		// running AfterCex view.
		for j := 0; j < len(b.CreateUserOps[i].AssetsForUpdateCex); j++ {
			afterCexAssets[j].TotalEquity = api.Add(afterCexAssets[j].TotalEquity, b.CreateUserOps[i].AssetsForUpdateCex[j].Equity)
			afterCexAssets[j].TotalDebt = api.Add(afterCexAssets[j].TotalDebt, b.CreateUserOps[i].AssetsForUpdateCex[j].Debt)
			afterCexAssets[j].Collateral = api.Add(afterCexAssets[j].Collateral, b.CreateUserOps[i].AssetsForUpdateCex[j].Collateral)
		}

		r.Check(totalUserDebt, 128)
		r.Check(totalUserCollateralRealValue, 128)
		// T3 defining constraint: debt ≤ haircut(collateral) at account level.
		api.AssertIsLessOrEqualNOp(totalUserDebt, totalUserCollateralRealValue, 128, true)

		// Account leaf: universal 5-input Poseidon
		// (AccountID, TotalEquity, TotalDebt, TotalCollateral,
		// AssetsCommitment) — same signature as every other model.
		userAssetsCommitment := corecircuit.ComputeFlatUint64Commitment(api, flattenAssetFieldsForHash)
		accountHash := poseidon.Poseidon(api, b.CreateUserOps[i].AccountIdHash, totalUserEquity, totalUserDebt, totalUserCollateralRealValue, userAssetsCommitment)
		actualAccountTreeRoot := corecircuit.UpdateMerkleProof(api, accountHash, b.CreateUserOps[i].AccountProof[:], accountIndexHelper)
		api.AssertIsEqual(actualAccountTreeRoot, b.CreateUserOps[i].AfterAccountTreeRoot)

		moduleUserOps[i] = t3spec.CircuitUserOp{
			AccountIndex:            b.CreateUserOps[i].AccountIndex,
			AccountIDHash:           b.CreateUserOps[i].AccountIdHash,
			TotalUserEquity:         totalUserEquity,
			TotalUserDebt:           totalUserDebt,
			TotalUserCollateralReal: totalUserCollateralRealValue,
		}
	}

	// Random-linear-combination check: user.Assets covers every
	// non-zero AssetsForUpdateCex entry. 3 powers per CEX slot
	// (Equity / Debt / Collateral).
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
		ctx := t3spec.ConstraintContext{
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
// into the t3spec.CircuitCexAsset view shape exposed to
// ConstraintModule hooks.
func toCircuitCexAssetView(src []CexAssetInfo) []t3spec.CircuitCexAsset {
	out := make([]t3spec.CircuitCexAsset, len(src))
	for i := range src {
		out[i] = t3spec.CircuitCexAsset{
			TotalEquity:      src[i].TotalEquity,
			TotalDebt:        src[i].TotalDebt,
			BasePrice:        src[i].BasePrice,
			Collateral:       src[i].Collateral,
			CollateralRatios: toCircuitTierRatioView(src[i].CollateralRatios),
		}
	}
	return out
}

func toCircuitTierRatioView(src []TierRatio) []t3spec.CircuitTierRatio {
	out := make([]t3spec.CircuitTierRatio, len(src))
	for i := range src {
		out[i] = t3spec.CircuitTierRatio{
			BoundaryValue:    src[i].BoundaryValue,
			Ratio:            src[i].Ratio,
			PrecomputedValue: src[i].PrecomputedValue,
		}
	}
	return out
}

// copyTierRatios copies BoundaryValue / Ratio / PrecomputedValue from
// a spec.TierRatio slice into a circuit-typed TierRatio slice.
func copyTierRatios(dst []TierRatio, src []t3spec.TierRatio) {
	for i := range dst {
		dst[i].BoundaryValue = src[i].BoundaryValue
		dst[i].Ratio = src[i].Ratio
		dst[i].PrecomputedValue = src[i].PrecomputedValue
	}
}

// SetBatchCreateUserCircuitWitness converts a snapshot-shaped witness
// into the in-circuit BatchCreateUserCircuit. Single-bucket version
// of t4's helper.
func SetBatchCreateUserCircuitWitness(
	batchWitness *t3spec.BatchCreateUserWitness,
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
		w.BeforeCexAssets[i].CollateralRatios = make([]TierRatio, len(src.CollateralRatios))
		copyTierRatios(w.BeforeCexAssets[i].CollateralRatios, src.CollateralRatios)
	}

	cexAssetsCount := len(w.BeforeCexAssets)
	targetCounts := t3spec.PickAssetCountTier(
		t3spec.CountNonEmptyAssets(batchWitness.CreateUserOps[0].Assets),
		assetCountTiers,
	)
	for i := range w.CreateUserOps {
		w.CreateUserOps[i].BeforeAccountTreeRoot = batchWitness.CreateUserOps[i].BeforeAccountTreeRoot
		w.CreateUserOps[i].AfterAccountTreeRoot = batchWitness.CreateUserOps[i].AfterAccountTreeRoot
		// AssetsForUpdateCex must be dense zero-init even for sparse
		// user input — same fix class as the T1 regression.
		w.CreateUserOps[i].AssetsForUpdateCex = make([]UserAssetMeta, cexAssetsCount)
		for j := range w.CreateUserOps[i].AssetsForUpdateCex {
			w.CreateUserOps[i].AssetsForUpdateCex[j] = UserAssetMeta{
				Equity:     uint64(0),
				Debt:       uint64(0),
				Collateral: uint64(0),
			}
		}

		// Place per-asset contribution at the slot named by Index.
		existingKeys := make([]int, 0)
		for j := range batchWitness.CreateUserOps[i].Assets {
			u := batchWitness.CreateUserOps[i].Assets[j]
			w.CreateUserOps[i].AssetsForUpdateCex[u.Index] = UserAssetMeta{
				Equity:     u.Equity,
				Debt:       u.Debt,
				Collateral: u.Collateral,
			}
			if !t3spec.IsAccountAssetEmpty(&u) {
				existingKeys = append(existingKeys, int(u.Index))
			}
		}

		paddingCounts := targetCounts - len(existingKeys)
		w.CreateUserOps[i].Assets = make([]UserAssetInfo, targetCounts)
		currentPaddingCounts := 0
		currentAssetIndex := 0
		index := 0
		// paddingAsset: fully zero-initialised UserAssetInfo at the
		// given asset slot. All 3 fields explicitly zero (gnark rejects
		// nil Variables — same bug class as d7c23f3 fix in T4).
		paddingAsset := func(slot uint32) UserAssetInfo {
			return UserAssetInfo{
				AssetIndex:      slot,
				CollateralIndex: uint32(0),
				CollateralFlag:  uint32(0),
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
			var u UserAssetInfo
			u.AssetIndex = uint32(v)
			calcAndSetCollateralInfo(v, &u, &batchWitness.CreateUserOps[i].Assets[v], batchWitness.BeforeCexAssets)
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
