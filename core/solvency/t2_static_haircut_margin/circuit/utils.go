package circuit

import (
	corecircuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/circuit"
	"github.com/consensys/gnark/frontend"
)

// getVariableCountOfCexAsset returns the number of field elements
// fillCexAssetCommitment emits per CexAssetInfo for the t2 model:
//   - 1: {TotalEquity, TotalDebt, BasePrice} packed 2^128/2^64/1
//   - 1: {Collateral * 2^16 + Haircut}
func getVariableCountOfCexAsset(_ CexAssetInfo) int {
	return 2
}

// fillCexAssetCommitment writes the field-element representation of
// one CexAssetInfo into commitments[currentIndex*counts:...]. For T2:
// 1 field for (Equity, Debt, BasePrice) (192 bits), 1 field for
// (Collateral, Haircut) (64+16=80 bits). Both well under bn254 modulus.
func fillCexAssetCommitment(api API, asset CexAssetInfo, currentIndex int, commitments []Variable) {
	counts := getVariableCountOfCexAsset(asset)
	commitments[currentIndex*counts] = api.Add(
		api.Mul(asset.TotalEquity, corecircuit.TwoToTheOneTwentyEight),
		api.Mul(asset.TotalDebt, corecircuit.TwoToTheSixtyFour),
		asset.BasePrice,
	)
	// Pack Collateral (64-bit) high + Haircut (16-bit) low.
	commitments[currentIndex*counts+1] = api.Add(
		api.Mul(asset.Collateral, Uint16MaxValueFr),
		asset.Haircut,
	)
}

// checkAndGetIntegerDivisionRes divides a wide dividend by the
// HaircutDenominator (10000) constant, range-checking the quotient
// (128-bit) and remainder (14-bit, since denominator is <2^14).
//
// Used by the per-asset haircut multiply: realCollateral =
// (collateral * price * haircut) / 10000.
func checkAndGetIntegerDivisionRes(api API, r frontend.Rangechecker, dividend Variable) Variable {
	return corecircuit.CheckedDivByConstant(api, r, dividend, HaircutDenominatorFr, 128, 14)
}
