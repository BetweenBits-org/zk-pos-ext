package circuit

import (
	"math/big"

	corecircuit "github.com/BetweenBits-org/zk-pos-ext/core/circuit"
	t3spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t3_tiered_haircut_margin_1pool/spec"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
)

// getVariableCountOfCexAsset returns the number of field elements that
// fillCexAssetCommitment emits per CexAssetInfo for the t3 model:
//   - 1: {TotalEquity, TotalDebt, BasePrice} packed under 2^128/2^64/1
//   - 1: {Collateral} alone in a field element (one bucket only)
//   - len(CollateralRatios)/2 (tier table, two TierRatios per field)
//
// CollateralRatios MUST have even length (enforced by callers via
// corespec.TierCount being even).
func getVariableCountOfCexAsset(cexAsset CexAssetInfo) int {
	return 2 + len(cexAsset.CollateralRatios)/2
}

// convertTierRatiosToVariables packs pairs of TierRatio entries into
// field elements. Identical layout to t4's helper.
func convertTierRatiosToVariables(api API, ratios []TierRatio, res []Variable) {
	for i := 0; i < len(ratios); i += 2 {
		v := api.Add(ratios[i].Ratio, api.Mul(ratios[i].BoundaryValue, Uint8MaxValueFr))
		v1 := api.Add(
			api.Mul(ratios[i+1].Ratio, Uint126MaxValueFr),
			api.Mul(ratios[i+1].BoundaryValue, Uint134MaxValueFr),
		)
		res[i/2] = api.Add(v, v1)
	}
}

// fillCexAssetCommitment writes the field-element representation of
// one CexAssetInfo into commitments[currentIndex*counts:...]. For T3:
// 1 field for (Equity, Debt, BasePrice), 1 field for Collateral alone,
// then ceil(len(CollateralRatios)/2) fields for the tier table.
func fillCexAssetCommitment(api API, asset CexAssetInfo, currentIndex int, commitments []Variable) {
	counts := getVariableCountOfCexAsset(asset)
	commitments[currentIndex*counts] = api.Add(
		api.Mul(asset.TotalEquity, corecircuit.TwoToTheOneTwentyEight),
		api.Mul(asset.TotalDebt, corecircuit.TwoToTheSixtyFour),
		asset.BasePrice,
	)
	commitments[currentIndex*counts+1] = asset.Collateral
	convertTierRatiosToVariables(api, asset.CollateralRatios, commitments[currentIndex*counts+2:])
}

// generateRapidArithmeticForCollateral asserts the tier table is
// well-formed (monotonic boundaries, ratios <= 100, boundaries within
// MaxTierBoundaryValueFr) and fills PrecomputedValue with cumulative
// sum of (boundary_diff * ratio / 100) up through that tier. Identical
// to t4's helper (universal piecewise-linear cumulative-sum recipe).
func generateRapidArithmeticForCollateral(api API, r frontend.Rangechecker, tierRatios []TierRatio) {
	tierRatios[0].PrecomputedValue = checkAndGetIntegerDivisionRes(
		api, r,
		api.Mul(tierRatios[0].BoundaryValue, tierRatios[0].Ratio),
	)
	api.AssertIsLessOrEqualNOp(tierRatios[0].Ratio, PercentageMultiplierFr, 8, true)
	api.AssertIsLessOrEqualNOp(tierRatios[0].BoundaryValue, MaxTierBoundaryValueFr, 128, true)
	for i := 1; i < len(tierRatios); i++ {
		api.AssertIsLessOrEqualNOp(tierRatios[i-1].BoundaryValue, tierRatios[i].BoundaryValue, 128, true)
		api.AssertIsLessOrEqualNOp(tierRatios[i].Ratio, PercentageMultiplierFr, 8, true)
		api.AssertIsLessOrEqualNOp(tierRatios[i].BoundaryValue, MaxTierBoundaryValueFr, 128, true)
		diffBoundary := api.Sub(tierRatios[i].BoundaryValue, tierRatios[i-1].BoundaryValue)
		current := checkAndGetIntegerDivisionRes(api, r, api.Mul(diffBoundary, tierRatios[i].Ratio))
		tierRatios[i].PrecomputedValue = api.Add(tierRatios[i-1].PrecomputedValue, current)
	}

	for i := 0; i < len(tierRatios); i++ {
		r.Check(tierRatios[i].PrecomputedValue, 128)
		r.Check(tierRatios[i].Ratio, 8)
		r.Check(tierRatios[i].BoundaryValue, 128)
	}
}

// checkAndGetIntegerDivisionRes is the t3 specialisation of
// corecircuit.CheckedDivByConstant — same args as t4.
func checkAndGetIntegerDivisionRes(api API, r frontend.Rangechecker, dividend Variable) Variable {
	return corecircuit.CheckedDivByConstant(api, r, dividend, PercentageMultiplierFr, 128, 8)
}

// getAndCheckTierRatiosQueryResults looks up the user's collateral
// tier in the SINGLE per-asset lookup table and returns the value of
// the collateral after applying the piecewise-linear haircut.
//
// Identical algorithm to t4's helper — only "one bucket per call" vs
// t4 calling this thrice (loan/margin/pm).
func getAndCheckTierRatiosQueryResults(
	api API, r frontend.Rangechecker, tierRatiosTable *logderivlookup.Table,
	assetIndex, userCollateral, collateralIndex, collateralFlag, assetPrice, collateralTierRatiosLen Variable,
) Variable {
	numOfTierRatioFields := 3
	queries := make([]Variable, 6)
	gap := api.Mul(assetIndex, collateralTierRatiosLen)
	for i := 0; i < 2; i++ {
		startPosition := api.Mul(collateralIndex, 3)
		queries[i*numOfTierRatioFields+0] = api.Add(startPosition, gap)
		queries[i*numOfTierRatioFields+1] = api.Add(startPosition, api.Add(gap, 1))
		queries[i*numOfTierRatioFields+2] = api.Add(startPosition, api.Add(gap, 2))
		collateralIndex = api.Add(collateralIndex, 1)
	}
	results := tierRatiosTable.Lookup(queries...)
	collateralValue := api.Mul(userCollateral, assetPrice)
	cr := api.CmpNOp(collateralValue, results[0], 128, true)
	api.AssertIsEqual(cr, api.Select(api.IsZero(collateralValue), 0, 1))
	upperBoundaryValue := api.Select(api.IsZero(collateralFlag), results[3], MaxTierBoundaryValueFr)
	api.AssertIsLessOrEqualNOp(collateralValue, upperBoundaryValue, 128, true)
	diffValue := api.Mul(api.Sub(collateralValue, results[0]), results[4])
	quotient := checkAndGetIntegerDivisionRes(api, r, diffValue)
	return api.Select(api.IsZero(collateralFlag), api.Add(results[2], quotient), results[5])
}

// constructCollateralTierRatiosLookupTable builds the SINGLE per-asset
// logderiv lookup table consumed by getAndCheckTierRatiosQueryResults.
// One dummy (0,0,0) tuple prepended per asset, then (BoundaryValue,
// Ratio, PrecomputedValue) for each tier.
func constructCollateralTierRatiosLookupTable(api API, cexAssetInfo []CexAssetInfo) *logderivlookup.Table {
	t := logderivlookup.New(api)
	for i := 0; i < len(cexAssetInfo); i++ {
		for j := 0; j < 3; j++ {
			t.Insert(0)
		}
		for j := 0; j < len(cexAssetInfo[i].CollateralRatios); j++ {
			t.Insert(cexAssetInfo[i].CollateralRatios[j].BoundaryValue)
			t.Insert(cexAssetInfo[i].CollateralRatios[j].Ratio)
			t.Insert(cexAssetInfo[i].CollateralRatios[j].PrecomputedValue)
		}
	}
	return t
}

// calcAndSetCollateralInfo computes the witness-side per-asset tier
// index for one user-asset. Single-bucket version of t4's helper.
func calcAndSetCollateralInfo(assetIndex int, ua *UserAssetInfo, um *t3spec.AccountAsset, cexInfo []t3spec.CexAssetInfo) {
	p := cexInfo[assetIndex]
	assetPrice := new(big.Int).SetUint64(p.BasePrice)
	userCollateralValue := new(big.Int).Mul(new(big.Int).SetUint64(um.Collateral), assetPrice)

	idx, flag := pickTier(userCollateralValue, p.CollateralRatios)
	ua.CollateralIndex = idx
	ua.CollateralFlag = flag
}

// pickTier returns the (index, flag) pair the circuit expects for one
// (collateralValue, tierTable) pair. Identical to t4's helper.
func pickTier(collateralValue *big.Int, ratios []t3spec.TierRatio) (int, int) {
	for i := range ratios {
		if collateralValue.Cmp(ratios[i].BoundaryValue) <= 0 {
			return i, 0
		}
	}
	return len(ratios) - 1, 1
}
