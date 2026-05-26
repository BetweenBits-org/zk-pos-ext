// Package circuit implements the gnark in-circuit definition of the
// t2_static_haircut_margin solvency model: per-asset *static* haircut
// over a SINGLE collateral pool, 4-tuple per-user per-asset record
// (Index, Equity, Debt, Collateral).
//
// T2 is a structural simplification of T3 — same per-asset and
// per-user layout, but the piecewise-linear tier curve is collapsed
// to a single Haircut constant (basis points: 10000 = 100% =
// no haircut, 9000 = 90% = 10% haircut). One multiply per asset,
// no tier-table lookup. Cheaper circuit than T3.
//
// Account leaf signature is universal across the catalog (see
// core/host.AccountLeafHash):
//
//	Poseidon(AccountID, TotalEquity, TotalDebt, TotalCollateral, AssetsCommitment)
//
// Industry reference: Aave V3 per-asset LTV / Liquidation Threshold
// (see docs/04-solvency-models.md §5).
package circuit

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// EmptyAccountLeafNodeHash is the Poseidon hash of the empty (all-zero)
// account leaf encoding used by every model in the v1 catalog.
var EmptyAccountLeafNodeHash, _ = new(big.Int).SetString("0f870d7404597dad9eca7c50a6f0af812ab7cd6a11d5c464d4031a3272377b95", 16)

// Field-encoded constants used by t2-specific packing arithmetic.
var (
	// haircutDenominator == 10000. Divisor for basis-points math
	// (collateral * haircut / 10000).
	haircutDenominator   = new(big.Int).SetUint64(10000)
	HaircutDenominatorFr = new(fr.Element).SetBigInt(haircutDenominator)

	uint16MaxValueBigInt, _ = new(big.Int).SetString("65536", 10)
	Uint16MaxValueFr        = new(fr.Element).SetBigInt(uint16MaxValueBigInt)

	// PowersOfSixteenBits[k] == (2^16)^k as a field element, for k in
	// [0, 15). Used to pack up to 15 asset indexes (each <2^16) into
	// one field element when hashing the per-user assetId vector.
	// Duplicate of t1 / t3 / t4 — R6 promotion candidate.
	PowersOfSixteenBits [15]fr.Element
)

func init() {
	cur := new(big.Int).SetUint64(1)
	step := big.NewInt(65536)
	for i := range 15 {
		PowersOfSixteenBits[i].SetBigInt(cur)
		cur.Mul(cur, step)
	}
}
