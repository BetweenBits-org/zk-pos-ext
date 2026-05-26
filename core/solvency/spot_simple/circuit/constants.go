// Package circuit implements the gnark in-circuit definition of the
// spot_simple solvency model: single-balance-per-asset per-user state
// (equity only), no tier table, no collateral haircut. The model's
// invariant reduces to per-asset sum equality, enforced via the
// per-user account-tree updates and the After=Before+Δ accumulation
// on the CEX side.
//
// Use BatchCreateUserCircuit as the gnark Circuit type and
// SetBatchCreateUserCircuitWitness to convert a snapshot witness into
// in-circuit Variables.
//
// The account-leaf hash shape is intentionally 5-input
// (accountID, totalEquity, 0, 0, userAssetsCommitment) so that the
// universal core/tree.EmptyAccountLeafHash constant
// (Poseidon(0,0,0,0,0)) applies unchanged. Positions 3 and 4 are
// fixed zeros in this model (no debt, no collateral) and contribute
// zero in-circuit cost.
package circuit

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// EmptyAccountLeafNodeHash is the Poseidon hash of the empty
// (all-zero) account leaf encoding. Identical value to tier_3bucket's
// constant — both models use a 5-input zero leaf, so the
// trusted-setup-shared empty-leaf hash is universal.
//
// Inserted as the verified "before" node when an account index has
// never been written.
var EmptyAccountLeafNodeHash, _ = new(big.Int).SetString("0f870d7404597dad9eca7c50a6f0af812ab7cd6a11d5c464d4031a3272377b95", 16)

// PowersOfSixteenBits[k] == (2^16)^k as a field element, for k in
// [0, 15). Used to pack up to 15 asset indexes (each <2^16) into one
// field element when hashing the per-user assetId vector. Duplicated
// from tier_3bucket/circuit/constants.go intentionally — this is the
// second model using the constant, R6 promotion candidate per rule-of-
// three (G11). Once a third model lands, promote to core/circuit.
var PowersOfSixteenBits [15]fr.Element

func init() {
	cur := new(big.Int).SetUint64(1)
	step := big.NewInt(65536)
	for i := range 15 {
		PowersOfSixteenBits[i].SetBigInt(cur)
		cur.Mul(cur, step)
	}
}
