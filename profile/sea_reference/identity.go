package sea_reference

import (
	"encoding/hex"
	"fmt"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// identityScheme — same v1 frozen identifier as profile/binance/identity.go.
// The scheme is engine-universal: hex-decode of a pre-hashed 32-byte ID,
// then bn254 fr.Element reduction so the result is a canonical field
// element matching what the snapshot adapter writes into the witness.
//
// Two customer profiles sharing identical scheme names is intentional —
// the scheme is a contract about derivation semantics, not a customer
// identifier. The customer-side reconstruction code is identical.
const identityScheme = "passthrough_hex_bn254_reduced.v0"

type identity struct{}

// NewIdentity returns sea_reference's AccountIDProvider.
func NewIdentity() spec.AccountIDProvider { return identity{} }

// DeriveAccountID maps an internal user ID (a 64-hex-char string
// produced by the customer's pre-hash pipeline) to the 32-byte
// canonical AccountID embedded in the account-tree leaf hash. Two
// steps:
//
//  1. Hex-decode the 64-char string to 32 raw bytes;
//  2. Reduce the 32 bytes as a BN254 fr.Element (SetBytes →
//     Marshal) so the result is < fr.Modulus and matches the form
//     the snapshot adapter writes into the witness.
//
// Inputs that are not exactly 64 hex chars panic — surfaces ETL
// bugs immediately rather than producing silent garbage.
func (identity) DeriveAccountID(internalUserID string) [32]byte {
	if len(internalUserID) != 64 {
		panic(fmt.Sprintf("sea_reference identity: expected 64-hex-char internal user id, got len=%d", len(internalUserID)))
	}
	raw, err := hex.DecodeString(internalUserID)
	if err != nil {
		panic("sea_reference identity: hex decode failed: " + err.Error())
	}
	reduced := new(fr.Element).SetBytes(raw).Marshal()
	var out [32]byte
	copy(out[:], reduced)
	return out
}

func (identity) Scheme() string { return identityScheme }
