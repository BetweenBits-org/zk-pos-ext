// Package circuit implements the gnark in-circuit definition of the
// t4_tiered_haircut_margin_3pool solvency model: a 3-bucket (Loan / Margin /
// PortfolioMargin) piecewise-linear haircut over a 5-tuple per-user
// per-asset record. Use BatchCreateUserCircuit as the gnark Circuit
// type and SetBatchCreateUserCircuitWitness to convert a snapshot
// witness into in-circuit Variables.
//
// The constraint count and Define() shape are byte-locked — changing
// them invalidates the existing trusted setup.
package circuit

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// EmptyAccountLeafNodeHash is the Poseidon hash of the empty (all-zero)
// account leaf encoding used by this model. Inserted as the verified
// "before" node when an account index has never been written.
//
// Migrated verbatim from legacy circuit/constants.go.
var EmptyAccountLeafNodeHash, _ = new(big.Int).SetString("0f870d7404597dad9eca7c50a6f0af812ab7cd6a11d5c464d4031a3272377b95", 16)

// Field-encoded constants used by t4_tiered_haircut_margin_3pool-specific packing
// arithmetic. The universal uint64-packing weights (2^64, 2^128) live
// in corecircuit; this file only declares constants whose semantics
// are unique to the t4_tiered_haircut_margin_3pool model.
var (
	percentageMultiplier      = new(big.Int).SetUint64(100)
	maxTierBoundaryValue, _   = new(big.Int).SetString("332306998946228968225951765070086144", 10) // 2^118
	uint8MaxValueBigInt, _    = new(big.Int).SetString("256", 10)
	uint126MaxValueBigInt, _  = new(big.Int).SetString("85070591730234615865843651857942052864", 10)
	uint134MaxValueBigInt, _  = new(big.Int).SetString("21778071482940061661655974875633165533184", 10)
	uint16MaxValueBigInt, _   = new(big.Int).SetString("65536", 10)

	// PercentageMultiplierFr is the divisor used by the haircut math
	// (Ratio is expressed as a /100 fraction).
	PercentageMultiplierFr = new(fr.Element).SetBigInt(percentageMultiplier)

	// MaxTierBoundaryValueFr caps the BoundaryValue of any TierRatio
	// entry. Sized so that BoundaryValue * Ratio (8 bits) fits well
	// under the field modulus.
	MaxTierBoundaryValueFr = new(fr.Element).SetBigInt(maxTierBoundaryValue)

	// Uint8MaxValueFr / Uint126MaxValueFr / Uint134MaxValueFr are
	// positional weights used by convertTierRatiosToVariables to pack
	// two TierRatio entries (Ratio:8 + BoundaryValue:128 + Ratio':8 +
	// BoundaryValue':128 = 272 bits, fits in one field element) into
	// one Variable.
	Uint8MaxValueFr   = new(fr.Element).SetBigInt(uint8MaxValueBigInt)
	Uint126MaxValueFr = new(fr.Element).SetBigInt(uint126MaxValueBigInt)
	Uint134MaxValueFr = new(fr.Element).SetBigInt(uint134MaxValueBigInt)

	// Uint16MaxValueFr is kept for symmetry though not used by the
	// current circuit; PowersOfSixteenBits below is the primary 16-bit
	// packing constant.
	Uint16MaxValueFr = new(fr.Element).SetBigInt(uint16MaxValueBigInt)

	// PowersOfSixteenBits[k] == (2^16)^k as a field element, for k in
	// [0, 15). Used to pack up to 15 asset indexes (each <2^16) into
	// one field element when hashing the per-user assetId vector.
	//
	// Initialised in init() to mirror legacy semantics exactly.
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
