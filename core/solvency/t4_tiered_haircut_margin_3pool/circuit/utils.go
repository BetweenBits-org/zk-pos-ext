package circuit

import (
	"math/big"

	corecircuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/circuit"
	t4spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
)

// getVariableCountOfCexAsset returns the number of field elements that
// fillCexAssetCommitment emits per CexAssetInfo:
//   - 1: {TotalEquity, TotalDebt, BasePrice} packed under 2^128/2^64/1
//   - 1: {LoanCollateral, MarginCollateral, PortfolioMarginCollateral}
//   - len(LoanRatios)/2 + len(MarginRatios)/2 + len(PortfolioMarginRatios)/2
//
// TierRatio slices MUST have even length (enforced by callers via
// corespec.TierCount being even).
func getVariableCountOfCexAsset(cexAsset CexAssetInfo) int {
	res := 2
	res += len(cexAsset.LoanRatios) / 2
	res += len(cexAsset.MarginRatios) / 2
	res += len(cexAsset.PortfolioMarginRatios) / 2
	return res
}

// convertTierRatiosToVariables packs pairs of TierRatio entries into
// field elements. Layout per pair (positional weights, low→high):
//
//	res[i/2] = ratios[i].Ratio                         // 8 bits
//	         + ratios[i].BoundaryValue   * 2^8         // 128 bits
//	         + ratios[i+1].Ratio         * 2^126       // 8 bits
//	         + ratios[i+1].BoundaryValue * 2^134       // 128 bits
//	         = 272 bits, fits in one field element.
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

// fillCexAssetCommitment writes the field-element representation of one
// CexAssetInfo into commitments[currentIndex*counts:...]. counts is
// getVariableCountOfCexAsset(asset). Caller hashes the full slice with
// poseidon to obtain the CEX-assets commitment.
func fillCexAssetCommitment(api API, asset CexAssetInfo, currentIndex int, commitments []Variable) {
	counts := getVariableCountOfCexAsset(asset)
	commitments[currentIndex*counts] = api.Add(
		api.Mul(asset.TotalEquity, corecircuit.TwoToTheOneTwentyEight),
		api.Mul(asset.TotalDebt, corecircuit.TwoToTheSixtyFour),
		asset.BasePrice,
	)
	commitments[currentIndex*counts+1] = api.Add(
		api.Mul(asset.LoanCollateral, corecircuit.TwoToTheOneTwentyEight),
		api.Mul(asset.MarginCollateral, corecircuit.TwoToTheSixtyFour),
		asset.PortfolioMarginCollateral,
	)
	convertTierRatiosToVariables(api, asset.LoanRatios, commitments[currentIndex*counts+2:])
	convertTierRatiosToVariables(api, asset.MarginRatios, commitments[currentIndex*counts+2+len(asset.LoanRatios)/2:])
	convertTierRatiosToVariables(api, asset.PortfolioMarginRatios, commitments[currentIndex*counts+2+len(asset.LoanRatios)/2+len(asset.MarginRatios)/2:])
}

// generateRapidArithmeticForCollateral asserts the tier table is
// well-formed (monotonic boundaries, ratios <= 100, boundaries within
// MaxTierBoundaryValueFr) and fills the PrecomputedValue field of each
// TierRatio with the cumulative sum of (boundary_diff * ratio / 100)
// up through that tier. Lets the per-user haircut lookup be O(1).
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

// checkAndGetIntegerDivisionRes is the t4_tiered_haircut_margin_3pool specialisation of
// corecircuit.CheckedDivByConstant: divisor is the haircut percentage
// (100), quotient is 128-bit and remainder is 8-bit. Wrapping keeps
// the call sites in this package terse and the byte-level constraint
// shape identical to the legacy circuit.
func checkAndGetIntegerDivisionRes(api API, r frontend.Rangechecker, dividend Variable) Variable {
	return corecircuit.CheckedDivByConstant(api, r, dividend, PercentageMultiplierFr, 128, 8)
}

// getAndCheckTierRatiosQueryResults looks up the user's collateral tier
// in the per-bucket lookup table and returns the value of the
// collateral after applying the piecewise-linear haircut.
//
//	collateralValue   = userCollateral * assetPrice
//	if collateralFlag == 0: linearly interpolate inside the tier
//	  (collateralValue must be in (lower, upper] of the indicated tier)
//	if collateralFlag != 0: return the precomputed cap of the last tier
//
// The lookup table layout is described in
// constructLoanTierRatiosLookupTable below: a single dummy 3-tuple of
// zeros prepended per asset, then (Boundary, Ratio, Precomputed)*tierCount.
// Indexes in UserAssetInfo are 1-based to point past the dummy tuple.
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
	// results[0] is the lower boundary; bounded <2^128 by generateRapidArithmeticForCollateral.
	cr := api.CmpNOp(collateralValue, results[0], 128, true)
	api.AssertIsEqual(cr, api.Select(api.IsZero(collateralValue), 0, 1))
	upperBoundaryValue := api.Select(api.IsZero(collateralFlag), results[3], MaxTierBoundaryValueFr)
	api.AssertIsLessOrEqualNOp(collateralValue, upperBoundaryValue, 128, true)
	// results[4] is the ratio at the indicated tier.
	diffValue := api.Mul(api.Sub(collateralValue, results[0]), results[4])
	quotient := checkAndGetIntegerDivisionRes(api, r, diffValue)
	// results[2] is the precomputed cumulative haircut at the lower boundary;
	// results[5] is the cap (precomputed value at the last tier).
	return api.Select(api.IsZero(collateralFlag), api.Add(results[2], quotient), results[5])
}

// constructLoanTierRatiosLookupTable, constructMarginTierRatiosLookupTable,
// and constructPortfolioTierRatiosLookupTable build the per-bucket
// logderiv lookup tables consumed by getAndCheckTierRatiosQueryResults.
//
// Layout per asset: one dummy (0,0,0) tuple prepended, then
// (BoundaryValue, Ratio, PrecomputedValue) for each tier. The dummy
// tuple lets index 0 mean "no tier hit yet" (a sentinel) without
// requiring a CmpNOp on the lookup index itself.
func constructLoanTierRatiosLookupTable(api API, cexAssetInfo []CexAssetInfo) *logderivlookup.Table {
	t := logderivlookup.New(api)
	for i := 0; i < len(cexAssetInfo); i++ {
		for j := 0; j < 3; j++ {
			t.Insert(0)
		}
		for j := 0; j < len(cexAssetInfo[i].LoanRatios); j++ {
			t.Insert(cexAssetInfo[i].LoanRatios[j].BoundaryValue)
			t.Insert(cexAssetInfo[i].LoanRatios[j].Ratio)
			t.Insert(cexAssetInfo[i].LoanRatios[j].PrecomputedValue)
		}
	}
	return t
}

func constructMarginTierRatiosLookupTable(api API, cexAssetInfo []CexAssetInfo) *logderivlookup.Table {
	t := logderivlookup.New(api)
	for i := 0; i < len(cexAssetInfo); i++ {
		for j := 0; j < 3; j++ {
			t.Insert(0)
		}
		for j := 0; j < len(cexAssetInfo[i].MarginRatios); j++ {
			t.Insert(cexAssetInfo[i].MarginRatios[j].BoundaryValue)
			t.Insert(cexAssetInfo[i].MarginRatios[j].Ratio)
			t.Insert(cexAssetInfo[i].MarginRatios[j].PrecomputedValue)
		}
	}
	return t
}

func constructPortfolioTierRatiosLookupTable(api API, cexAssetInfo []CexAssetInfo) *logderivlookup.Table {
	t := logderivlookup.New(api)
	for i := 0; i < len(cexAssetInfo); i++ {
		for j := 0; j < 3; j++ {
			t.Insert(0)
		}
		for j := 0; j < len(cexAssetInfo[i].PortfolioMarginRatios); j++ {
			t.Insert(cexAssetInfo[i].PortfolioMarginRatios[j].BoundaryValue)
			t.Insert(cexAssetInfo[i].PortfolioMarginRatios[j].Ratio)
			t.Insert(cexAssetInfo[i].PortfolioMarginRatios[j].PrecomputedValue)
		}
	}
	return t
}

// calcAndSetCollateralInfo computes the witness-side per-bucket tier
// index for one user-asset, given the asset's tier table. Sets the
// CollateralIndex/Flag fields on `ua`. Called by the witness builder
// (not the circuit) — the circuit verifies these indices, the witness
// supplies them.
func calcAndSetCollateralInfo(assetIndex int, ua *UserAssetInfo, um *t4spec.AccountAsset, cexInfo []t4spec.CexAssetInfo) {
	p := cexInfo[assetIndex]
	assetPrice := new(big.Int).SetUint64(p.BasePrice)
	userLoan := new(big.Int).Mul(new(big.Int).SetUint64(um.Loan), assetPrice)
	userMargin := new(big.Int).Mul(new(big.Int).SetUint64(um.Margin), assetPrice)
	userPM := new(big.Int).Mul(new(big.Int).SetUint64(um.PortfolioMargin), assetPrice)

	idx, flag := pickTier(userLoan, p.LoanRatios)
	ua.LoanCollateralIndex = idx
	ua.LoanCollateralFlag = flag

	idx, flag = pickTier(userMargin, p.MarginRatios)
	ua.MarginCollateralIndex = idx
	ua.MarginCollateralFlag = flag

	idx, flag = pickTier(userPM, p.PortfolioMarginRatios)
	ua.PortfolioMarginCollateralIndex = idx
	ua.PortfolioMarginCollateralFlag = flag
}

// pickTier returns the (index, flag) pair the circuit expects for one
// (collateralValue, tierTable) pair: smallest index whose BoundaryValue
// >= collateralValue, flag=0. If no tier qualifies, the last index and
// flag=1 (capped at the precomputed cap).
func pickTier(collateralValue *big.Int, ratios []t4spec.TierRatio) (int, int) {
	for i := range ratios {
		if collateralValue.Cmp(ratios[i].BoundaryValue) <= 0 {
			return i, 0
		}
	}
	return len(ratios) - 1, 1
}
