// Package circuit implements the gnark in-circuit definition of the
// t3_tiered_haircut_margin_1pool solvency model: tier-based
// piecewise-linear haircut over a SINGLE collateral pool, 4-tuple
// per-user per-asset record (Index, Equity, Debt, Collateral).
//
// T3 is a structural simplification of T4 — same tier-curve evaluation
// per asset, but the 3-bucket (Loan / Margin / PortfolioMargin) split
// is collapsed into one Collateral pool. Account leaf signature is
// universal across the catalog (see core/host.AccountLeafHash).
package circuit

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// EmptyAccountLeafNodeHash is the Poseidon hash of the empty (all-zero)
// account leaf encoding used by every model in the v1 catalog. Inserted
// as the verified "before" node when an account index has never been
// written.
var EmptyAccountLeafNodeHash, _ = new(big.Int).SetString("0f870d7404597dad9eca7c50a6f0af812ab7cd6a11d5c464d4031a3272377b95", 16)

// Field-encoded constants used by t3-specific packing arithmetic.
// Identical layout to t4 for the universal tier-table packing
// (Boundary:128 + Ratio:8) and the asset-id 16-bit positional weights.
var (
	percentageMultiplier     = new(big.Int).SetUint64(100)
	maxTierBoundaryValue, _  = new(big.Int).SetString("332306998946228968225951765070086144", 10) // 2^118
	uint8MaxValueBigInt, _   = new(big.Int).SetString("256", 10)
	uint126MaxValueBigInt, _ = new(big.Int).SetString("85070591730234615865843651857942052864", 10)
	uint134MaxValueBigInt, _ = new(big.Int).SetString("21778071482940061661655974875633165533184", 10)
	uint16MaxValueBigInt, _  = new(big.Int).SetString("65536", 10)

	PercentageMultiplierFr = new(fr.Element).SetBigInt(percentageMultiplier)
	MaxTierBoundaryValueFr = new(fr.Element).SetBigInt(maxTierBoundaryValue)
	Uint8MaxValueFr        = new(fr.Element).SetBigInt(uint8MaxValueBigInt)
	Uint126MaxValueFr      = new(fr.Element).SetBigInt(uint126MaxValueBigInt)
	Uint134MaxValueFr      = new(fr.Element).SetBigInt(uint134MaxValueBigInt)
	Uint16MaxValueFr       = new(fr.Element).SetBigInt(uint16MaxValueBigInt)

	// PowersOfSixteenBits[k] == (2^16)^k as a field element, for k in
	// [0, 15). Used to pack up to 15 asset indexes (each <2^16) into
	// one field element when hashing the per-user assetId vector.
	// Duplicate of t1 / t4 — R6 promotion candidate (see G11 carry list).
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
