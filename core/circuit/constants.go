// Package circuit holds the substrate-level zk-circuit helpers shared
// by every solvency model. Anything in this package is invariant
// across models and customer deployments — Merkle proof verification,
// commitment scheme, generic verified arithmetic. Model-specific
// circuits build on top of these.
package circuit

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/frontend"
)

// Type aliases so callers don't have to import gnark/frontend just to
// satisfy signatures.
type (
	API      = frontend.API
	Variable = frontend.Variable
)

// Field-encoded powers of two used by the standard uint64-packing
// commitment scheme. ComputeFlatUint64Commitment packs three 64-bit
// values into one field element using these as positional weights.
var (
	uint64MaxBigInt, _       = new(big.Int).SetString("18446744073709551616", 10)                    // 2^64
	uint64MaxBigIntSquare, _ = new(big.Int).SetString("340282366920938463463374607431768211456", 10) // 2^128

	// TwoToTheSixtyFour and TwoToTheOneTwentyEight are exported for
	// use by model-specific circuits that pack uint64 fields under
	// the standard layout.
	TwoToTheSixtyFour      = new(fr.Element).SetBigInt(uint64MaxBigInt)
	TwoToTheOneTwentyEight = new(fr.Element).SetBigInt(uint64MaxBigIntSquare)
)
