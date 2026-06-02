package circuit

import (
	corecircuit "github.com/BetweenBits-org/zk-pos-ext/core/circuit"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	t4spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	"github.com/consensys/gnark/std/hash/poseidon"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"github.com/consensys/gnark/std/rangecheck"
)

// BatchCreateUserCircuit is the gnark Circuit type for the
// t4_tiered_haircut_margin_3pool model. Its constraint shape determines the trusted
// setup; changing field layout or Define()'s constraint sequence
// invalidates published .pk/.vk pairs.
//
// module is the unexported alpha-layer ConstraintModule hook. gnark's
// frontend reflects only on exported, Variable-bearing fields, so this
// field is invisible to Compile and adds no in-circuit cost when left
// nil or set to a noop module. Wire customer/regulator-specific
// constraints via SetConstraintModule before Compile.
type BatchCreateUserCircuit struct {
	BatchCommitment           Variable `gnark:",public"`
	BeforeAccountTreeRoot     Variable
	AfterAccountTreeRoot      Variable
	BeforeCEXAssetsCommitment Variable
	AfterCEXAssetsCommitment  Variable
	BeforeCexAssets           []CexAssetInfo
	CreateUserOps             []CreateUserOperation

	module t4spec.ConstraintModule
}

// SetConstraintModule wires the alpha-layer ConstraintModule hook
// invoked at the end of Define after every base constraint has been
// emitted. Setting nil (the default) reverts to the no-hook shape used
// during R3 step 0 — Compile yields the legacy NbConstraints baseline.
//
// Composing a non-nil module forks the trusted setup: the resulting
// .pk/.vk pair is unique to the (t4_tiered_haircut_margin_3pool, module) pair.
func (b *BatchCreateUserCircuit) SetConstraintModule(m t4spec.ConstraintModule) {
	b.module = m
}

// NewVerifyBatchCreateUserCircuit returns a circuit instance with only
// the public BatchCommitment populated — used on the verifier side to
// check a serialized proof.
func NewVerifyBatchCreateUserCircuit(commitment []byte) *BatchCreateUserCircuit {
	return &BatchCreateUserCircuit{BatchCommitment: commitment}
}

// NewBatchCreateUserCircuit allocates a fully zero-valued circuit
// instance sized for the given shape. Used during trusted setup and as
// the witness template at proving time.
func NewBatchCreateUserCircuit(userAssetCounts, allAssetCounts, batchCounts uint32) *BatchCreateUserCircuit {
	c := &BatchCreateUserCircuit{
		BeforeCexAssets: make([]CexAssetInfo, allAssetCounts),
		CreateUserOps:   make([]CreateUserOperation, batchCounts),
	}
	for i := range c.BeforeCexAssets {
		c.BeforeCexAssets[i] = CexAssetInfo{
			LoanRatios:            make([]TierRatio, corespec.TierCount),
			MarginRatios:          make([]TierRatio, corespec.TierCount),
			PortfolioMarginRatios: make([]TierRatio, corespec.TierCount),
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

// Define emits the t4_tiered_haircut_margin_3pool constraint set:
//
//  1. BatchCommitment == Poseidon(BeforeRoot, AfterRoot, BeforeCEX, AfterCEX)
//  2. BeforeCEXAssetsCommitment is the Poseidon of the packed BeforeCexAssets
//  3. Each tier table is monotonic and ratio-capped (rapid-arithmetic precompute)
//  4. Each user's account proof verifies (before) and updates (after)
//  5. Per-user: sum(loan+margin+pm) ≤ equity per asset; totalDebt ≤ haircut(totalCollateral)
//  6. Linear-combination check: the user's Assets slice covers every
//     non-zero AssetsForUpdateCex entry (random challenge from Poseidon)
//  7. AfterCEXAssetsCommitment is the Poseidon of the packed AfterCexAssets
//     (accumulated from BeforeCexAssets + per-user deltas)
//  8. CreateUserOps roots chain (op[i].After == op[i+1].Before)
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
		r.Check(b.BeforeCexAssets[i].LoanCollateral, 64)
		r.Check(b.BeforeCexAssets[i].MarginCollateral, 64)
		r.Check(b.BeforeCexAssets[i].PortfolioMarginCollateral, 64)

		fillCexAssetCommitment(api, b.BeforeCexAssets[i], i, cexAssets)
		generateRapidArithmeticForCollateral(api, r, b.BeforeCexAssets[i].LoanRatios)
		generateRapidArithmeticForCollateral(api, r, b.BeforeCexAssets[i].MarginRatios)
		generateRapidArithmeticForCollateral(api, r, b.BeforeCexAssets[i].PortfolioMarginRatios)
		afterCexAssets[i] = b.BeforeCexAssets[i]

		assetPriceTable.Insert(b.BeforeCexAssets[i].BasePrice)
	}
	actualCexAssetsCommitment := poseidon.Poseidon(api, cexAssets...)
	api.AssertIsEqual(b.BeforeCEXAssetsCommitment, actualCexAssetsCommitment)
	api.AssertIsEqual(b.BeforeAccountTreeRoot, b.CreateUserOps[0].BeforeAccountTreeRoot)
	api.AssertIsEqual(b.AfterAccountTreeRoot, b.CreateUserOps[len(b.CreateUserOps)-1].AfterAccountTreeRoot)

	loanTierRatiosTable := constructLoanTierRatiosLookupTable(api, b.BeforeCexAssets)
	marginTierRatiosTable := constructMarginTierRatiosLookupTable(api, b.BeforeCexAssets)
	portfolioMarginTierRatiosTable := constructPortfolioTierRatiosLookupTable(api, b.BeforeCexAssets)
	userAssetIdHashes := make([]Variable, len(b.CreateUserOps)+1)

	userAssetsResults := make([][]Variable, len(b.CreateUserOps))
	userAssetsQueries := make([][]Variable, len(b.CreateUserOps))
	moduleUserOps := make([]t4spec.CircuitUserOp, len(b.CreateUserOps))

	for i := 0; i < len(b.CreateUserOps); i++ {
		accountIndexHelper := corecircuit.AccountIndexToMerkleHelper(api, b.CreateUserOps[i].AccountIndex)
		corecircuit.VerifyMerkleProof(api, b.CreateUserOps[i].BeforeAccountTreeRoot, EmptyAccountLeafNodeHash, b.CreateUserOps[i].AccountProof[:], accountIndexHelper)
		var totalUserEquity Variable = 0
		var totalUserDebt Variable = 0
		userAssets := b.CreateUserOps[i].Assets
		var totalUserCollateralRealValue Variable = 0

		userAssetsLookupTable := logderivlookup.New(api)
		for j := 0; j < len(b.CreateUserOps[i].AssetsForUpdateCex); j++ {
			userAssetsLookupTable.Insert(b.CreateUserOps[i].AssetsForUpdateCex[j].Equity)
			userAssetsLookupTable.Insert(b.CreateUserOps[i].AssetsForUpdateCex[j].Debt)
			userAssetsLookupTable.Insert(b.CreateUserOps[i].AssetsForUpdateCex[j].LoanCollateral)
			userAssetsLookupTable.Insert(b.CreateUserOps[i].AssetsForUpdateCex[j].MarginCollateral)
			userAssetsLookupTable.Insert(b.CreateUserOps[i].AssetsForUpdateCex[j].PortfolioMarginCollateral)
		}

		// Strictly-increasing AssetIndex enforces uniqueness across the user's assets.
		for j := 0; j < len(userAssets)-1; j++ {
			r.Check(userAssets[j].AssetIndex, 16)
			cr := api.CmpNOp(userAssets[j+1].AssetIndex, userAssets[j].AssetIndex, 16, true)
			api.AssertIsEqual(cr, 1)
		}

		// Pack 15 asset indexes (each <2^16) per field element, then hash.
		assetIdsToVariables := make([]Variable, (len(userAssets)+14)/15)
		for j := 0; j < len(assetIdsToVariables); j++ {
			var v Variable = 0
			for p := j * 15; p < (j+1)*15 && p < len(userAssets); p++ {
				v = api.Add(v, api.Mul(userAssets[p].AssetIndex, PowersOfSixteenBits[p%15]))
			}
			assetIdsToVariables[j] = v
		}
		userAssetIdHashes[i] = poseidon.Poseidon(api, assetIdsToVariables...)

		userAssetsQueries[i] = make([]Variable, len(userAssets)*5)
		assetPriceQueries := make([]Variable, len(userAssets))
		numOfAssetsFields := 6
		for j := 0; j < len(userAssets); j++ {
			p := api.Mul(userAssets[j].AssetIndex, 5)
			for k := 0; k < 5; k++ {
				userAssetsQueries[i][j*5+k] = api.Add(p, k)
			}
			assetPriceQueries[j] = userAssets[j].AssetIndex
		}
		userAssetsResults[i] = userAssetsLookupTable.Lookup(userAssetsQueries[i]...)
		assetPriceResponses := assetPriceTable.Lookup(assetPriceQueries...)

		flattenAssetFieldsForHash := make([]Variable, len(userAssets)*numOfAssetsFields)
		for j := 0; j < len(userAssets); j++ {
			userEquity := userAssetsResults[i][j*5]
			r.Check(userEquity, 64)
			userDebt := userAssetsResults[i][j*5+1]
			r.Check(userDebt, 64)
			userLoanCollateral := userAssetsResults[i][j*5+2]
			r.Check(userLoanCollateral, 64)
			userMarginCollateral := userAssetsResults[i][j*5+3]
			r.Check(userMarginCollateral, 64)
			userPortfolioMarginCollateral := userAssetsResults[i][j*5+4]
			r.Check(userPortfolioMarginCollateral, 64)

			flattenAssetFieldsForHash[j*numOfAssetsFields] = userAssets[j].AssetIndex
			flattenAssetFieldsForHash[j*numOfAssetsFields+1] = userEquity
			flattenAssetFieldsForHash[j*numOfAssetsFields+2] = userDebt
			flattenAssetFieldsForHash[j*numOfAssetsFields+3] = userLoanCollateral
			flattenAssetFieldsForHash[j*numOfAssetsFields+4] = userMarginCollateral
			flattenAssetFieldsForHash[j*numOfAssetsFields+5] = userPortfolioMarginCollateral

			assetTotalCollateral := api.Add(userLoanCollateral, userMarginCollateral, userPortfolioMarginCollateral)
			r.Check(assetTotalCollateral, 64)
			api.AssertIsLessOrEqualNOp(assetTotalCollateral, userEquity, 64, true)

			loanRealValue := getAndCheckTierRatiosQueryResults(api, r, loanTierRatiosTable, userAssets[j].AssetIndex,
				userLoanCollateral,
				userAssets[j].LoanCollateralIndex,
				userAssets[j].LoanCollateralFlag,
				assetPriceResponses[j],
				3*(len(b.BeforeCexAssets[j].LoanRatios)+1))

			marginRealValue := getAndCheckTierRatiosQueryResults(api, r, marginTierRatiosTable, userAssets[j].AssetIndex,
				userMarginCollateral,
				userAssets[j].MarginCollateralIndex,
				userAssets[j].MarginCollateralFlag,
				assetPriceResponses[j],
				3*(len(b.BeforeCexAssets[j].MarginRatios)+1))

			portfolioMarginRealValue := getAndCheckTierRatiosQueryResults(api, r, portfolioMarginTierRatiosTable, userAssets[j].AssetIndex,
				userPortfolioMarginCollateral,
				userAssets[j].PortfolioMarginCollateralIndex,
				userAssets[j].PortfolioMarginCollateralFlag,
				assetPriceResponses[j],
				3*(len(b.BeforeCexAssets[j].PortfolioMarginRatios)+1))

			totalUserCollateralRealValue = api.Add(totalUserCollateralRealValue, loanRealValue, marginRealValue, portfolioMarginRealValue)

			totalUserEquity = api.Add(totalUserEquity, api.Mul(userEquity, assetPriceResponses[j]))
			totalUserDebt = api.Add(totalUserDebt, api.Mul(userDebt, assetPriceResponses[j]))
		}

		for j := 0; j < len(b.CreateUserOps[i].AssetsForUpdateCex); j++ {
			afterCexAssets[j].TotalEquity = api.Add(afterCexAssets[j].TotalEquity, b.CreateUserOps[i].AssetsForUpdateCex[j].Equity)
			afterCexAssets[j].TotalDebt = api.Add(afterCexAssets[j].TotalDebt, b.CreateUserOps[i].AssetsForUpdateCex[j].Debt)
			afterCexAssets[j].LoanCollateral = api.Add(afterCexAssets[j].LoanCollateral, b.CreateUserOps[i].AssetsForUpdateCex[j].LoanCollateral)
			afterCexAssets[j].MarginCollateral = api.Add(afterCexAssets[j].MarginCollateral, b.CreateUserOps[i].AssetsForUpdateCex[j].MarginCollateral)
			afterCexAssets[j].PortfolioMarginCollateral = api.Add(afterCexAssets[j].PortfolioMarginCollateral, b.CreateUserOps[i].AssetsForUpdateCex[j].PortfolioMarginCollateral)
		}

		r.Check(totalUserDebt, 128)
		r.Check(totalUserCollateralRealValue, 128)
		api.AssertIsLessOrEqualNOp(totalUserDebt, totalUserCollateralRealValue, 128, true)
		userAssetsCommitment := corecircuit.ComputeFlatUint64Commitment(api, flattenAssetFieldsForHash)
		accountHash := poseidon.Poseidon(api, b.CreateUserOps[i].AccountIdHash, totalUserEquity, totalUserDebt, totalUserCollateralRealValue, userAssetsCommitment)
		actualAccountTreeRoot := corecircuit.UpdateMerkleProof(api, accountHash, b.CreateUserOps[i].AccountProof[:], accountIndexHelper)
		api.AssertIsEqual(actualAccountTreeRoot, b.CreateUserOps[i].AfterAccountTreeRoot)

		moduleUserOps[i] = t4spec.CircuitUserOp{
			AccountIndex:            b.CreateUserOps[i].AccountIndex,
			AccountIDHash:           b.CreateUserOps[i].AccountIdHash,
			TotalUserEquity:         totalUserEquity,
			TotalUserDebt:           totalUserDebt,
			TotalUserCollateralReal: totalUserCollateralRealValue,
		}
	}

	// Random-linear-combination check that user.Assets covers every non-zero
	// AssetsForUpdateCex entry. Challenge = Poseidon(userAssetIdHashes... ++ batchCommitment).
	userAssetIdHashes[len(b.CreateUserOps)] = b.BatchCommitment
	randomChallenge := poseidon.Poseidon(api, userAssetIdHashes...)
	powersOfRandomChallenge := make([]Variable, 5*len(b.BeforeCexAssets))
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
			sumB = api.Add(sumB, api.Mul(b.CreateUserOps[i].AssetsForUpdateCex[j].Equity, powersOfRandomChallenge[5*j]))
			sumB = api.Add(sumB, api.Mul(b.CreateUserOps[i].AssetsForUpdateCex[j].Debt, powersOfRandomChallenge[5*j+1]))
			sumB = api.Add(sumB, api.Mul(b.CreateUserOps[i].AssetsForUpdateCex[j].LoanCollateral, powersOfRandomChallenge[5*j+2]))
			sumB = api.Add(sumB, api.Mul(b.CreateUserOps[i].AssetsForUpdateCex[j].MarginCollateral, powersOfRandomChallenge[5*j+3]))
			sumB = api.Add(sumB, api.Mul(b.CreateUserOps[i].AssetsForUpdateCex[j].PortfolioMarginCollateral, powersOfRandomChallenge[5*j+4]))
		}
		api.AssertIsEqual(sumA, sumB)
	}

	tempAfterCexAssets := make([]Variable, len(b.BeforeCexAssets)*countOfCexAsset)
	for j := 0; j < len(b.BeforeCexAssets); j++ {
		r.Check(afterCexAssets[j].TotalEquity, 64)
		r.Check(afterCexAssets[j].TotalDebt, 64)
		r.Check(afterCexAssets[j].LoanCollateral, 64)
		r.Check(afterCexAssets[j].MarginCollateral, 64)
		r.Check(afterCexAssets[j].PortfolioMarginCollateral, 64)

		fillCexAssetCommitment(api, afterCexAssets[j], j, tempAfterCexAssets)
	}

	actualAfterCEXAssetsCommitment := poseidon.Poseidon(api, tempAfterCexAssets...)
	api.AssertIsEqual(actualAfterCEXAssetsCommitment, b.AfterCEXAssetsCommitment)
	api.Println("actualAfterCEXAssetsCommitment: ", actualAfterCEXAssetsCommitment)
	api.Println("AfterCEXAssetsCommitment: ", b.AfterCEXAssetsCommitment)
	for i := 0; i < len(b.CreateUserOps)-1; i++ {
		api.AssertIsEqual(b.CreateUserOps[i].AfterAccountTreeRoot, b.CreateUserOps[i+1].BeforeAccountTreeRoot)
	}

	if b.module != nil {
		ctx := t4spec.ConstraintContext{
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
// into the t4spec.CircuitCexAsset view shape exposed to
// ConstraintModule hooks. Field types match underneath
// (corecircuit.Variable everywhere), so this is a flat copy — no
// in-circuit constraints are emitted.
func toCircuitCexAssetView(src []CexAssetInfo) []t4spec.CircuitCexAsset {
	out := make([]t4spec.CircuitCexAsset, len(src))
	for i := range src {
		out[i] = t4spec.CircuitCexAsset{
			TotalEquity:               src[i].TotalEquity,
			TotalDebt:                 src[i].TotalDebt,
			BasePrice:                 src[i].BasePrice,
			LoanCollateral:            src[i].LoanCollateral,
			MarginCollateral:          src[i].MarginCollateral,
			PortfolioMarginCollateral: src[i].PortfolioMarginCollateral,
			LoanRatios:                toCircuitTierRatioView(src[i].LoanRatios),
			MarginRatios:              toCircuitTierRatioView(src[i].MarginRatios),
			PortfolioMarginRatios:     toCircuitTierRatioView(src[i].PortfolioMarginRatios),
		}
	}
	return out
}

// toCircuitTierRatioView translates an in-circuit TierRatio slice into
// the t4spec.CircuitTierRatio view shape. Same flat-copy semantics
// as toCircuitCexAssetView — no in-circuit constraints emitted.
func toCircuitTierRatioView(src []TierRatio) []t4spec.CircuitTierRatio {
	out := make([]t4spec.CircuitTierRatio, len(src))
	for i := range src {
		out[i] = t4spec.CircuitTierRatio{
			BoundaryValue:    src[i].BoundaryValue,
			Ratio:            src[i].Ratio,
			PrecomputedValue: src[i].PrecomputedValue,
		}
	}
	return out
}

// copyTierRatios copies BoundaryValue / Ratio / PrecomputedValue from a
// spec.TierRatio slice into a circuit-typed TierRatio slice of the
// same length.
func copyTierRatios(dst []TierRatio, src []t4spec.TierRatio) {
	for i := range dst {
		dst[i].BoundaryValue = src[i].BoundaryValue
		dst[i].Ratio = src[i].Ratio
		dst[i].PrecomputedValue = src[i].PrecomputedValue
	}
}

// SetBatchCreateUserCircuitWitness converts a snapshot-shaped witness
// into the in-circuit BatchCreateUserCircuit. assetCountTiers MUST
// match the BatchShape this circuit was sized for (sorted ascending);
// each user's non-empty asset count is rounded up to the smallest tier
// and the slice is padded with zero entries.
func SetBatchCreateUserCircuitWitness(
	batchWitness *t4spec.BatchCreateUserWitness,
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
		w.BeforeCexAssets[i].LoanCollateral = src.LoanCollateral
		w.BeforeCexAssets[i].MarginCollateral = src.MarginCollateral
		w.BeforeCexAssets[i].PortfolioMarginCollateral = src.PortfolioMarginCollateral
		w.BeforeCexAssets[i].LoanRatios = make([]TierRatio, len(src.LoanRatios))
		copyTierRatios(w.BeforeCexAssets[i].LoanRatios, src.LoanRatios)
		w.BeforeCexAssets[i].MarginRatios = make([]TierRatio, len(src.MarginRatios))
		copyTierRatios(w.BeforeCexAssets[i].MarginRatios, src.MarginRatios)
		w.BeforeCexAssets[i].PortfolioMarginRatios = make([]TierRatio, len(src.PortfolioMarginRatios))
		copyTierRatios(w.BeforeCexAssets[i].PortfolioMarginRatios, src.PortfolioMarginRatios)
	}

	cexAssetsCount := len(w.BeforeCexAssets)
	// Per-batch asset count is decided by the first user; subsequent users may be padding.
	targetCounts := t4spec.PickAssetCountTier(
		t4spec.CountNonEmptyAssets(batchWitness.CreateUserOps[0].Assets),
		assetCountTiers,
	)
	for i := range w.CreateUserOps {
		w.CreateUserOps[i].BeforeAccountTreeRoot = batchWitness.CreateUserOps[i].BeforeAccountTreeRoot
		w.CreateUserOps[i].AfterAccountTreeRoot = batchWitness.CreateUserOps[i].AfterAccountTreeRoot
		// AssetsForUpdateCex must be dense zero-init even for sparse
		// user input — same fix class as the T1 regression. The legacy
		// raw CSV path happened to emit one row per (user, asset_slot)
		// and so populated every j == Index by coincidence; the R9
		// standard CSV path emits only non-zero rows, exposing the
		// indexing bug.
		w.CreateUserOps[i].AssetsForUpdateCex = make([]UserAssetMeta, cexAssetsCount)
		for j := range w.CreateUserOps[i].AssetsForUpdateCex {
			w.CreateUserOps[i].AssetsForUpdateCex[j] = UserAssetMeta{
				Equity:                    uint64(0),
				Debt:                      uint64(0),
				LoanCollateral:            uint64(0),
				MarginCollateral:          uint64(0),
				PortfolioMarginCollateral: uint64(0),
			}
		}

		// Place per-asset contribution at the slot named by Index.
		existingKeys := make([]int, 0)
		for j := range batchWitness.CreateUserOps[i].Assets {
			u := batchWitness.CreateUserOps[i].Assets[j]
			w.CreateUserOps[i].AssetsForUpdateCex[u.Index] = UserAssetMeta{
				Equity:                    u.Equity,
				Debt:                      u.Debt,
				LoanCollateral:            u.Loan,
				MarginCollateral:          u.Margin,
				PortfolioMarginCollateral: u.PortfolioMargin,
			}
			if !t4spec.IsAccountAssetEmpty(&u) {
				existingKeys = append(existingKeys, int(u.Index))
			}
		}

		paddingCounts := targetCounts - len(existingKeys)
		w.CreateUserOps[i].Assets = make([]UserAssetInfo, targetCounts)
		currentPaddingCounts := 0
		currentAssetIndex := 0
		index := 0
		// paddingAsset returns a fully zero-initialised UserAssetInfo at
		// the given asset slot. All 6 collateral-related fields MUST be
		// explicit zero values rather than left nil — gnark's frontend
		// rejects nil Variables with "can't set fr.Element with <nil>".
		paddingAsset := func(slot uint32) UserAssetInfo {
			return UserAssetInfo{
				AssetIndex:                     slot,
				LoanCollateralIndex:            uint32(0),
				LoanCollateralFlag:             uint32(0),
				MarginCollateralIndex:          uint32(0),
				MarginCollateralFlag:           uint32(0),
				PortfolioMarginCollateralIndex: uint32(0),
				PortfolioMarginCollateralFlag:  uint32(0),
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
