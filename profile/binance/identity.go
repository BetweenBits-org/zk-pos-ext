package binance

import (
	"encoding/hex"
	"fmt"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// identityScheme is the v1 frozen identifier (G2 closure, R3 step 4).
// Encodes the two-stage AccountID derivation the Binance deployment
// performs: hex-decode of a pre-hashed 32-byte ID, then BN254 fr.Element
// reduction so the result is always a canonical field element.
//
// Reduction matters because the snapshot adapter (parseAccountRow)
// performs the same SetBytes → Marshal round-trip on the raw bytes
// (G13 impl, R3 step 2). If DeriveAccountID returned the un-reduced
// form, a user reconstructing their AccountID from their internal ID
// would get bytes that don't match the value embedded in the leaf
// hash for any input whose raw 32-byte value exceeds fr.Modulus —
// roughly half of all SHA-256 outputs. The scheme name reflects this
// to keep the audit story honest.
const identityScheme = "passthrough_hex_bn254_reduced.v0"

type identity struct{}

// NewIdentity returns Binance's AccountIDProvider.
func NewIdentity() spec.AccountIDProvider { return identity{} }

// DeriveAccountID maps an internal user ID (a 64-hex-char string
// already produced by the customer's pre-hash pipeline) to the
// 32-byte canonical AccountID embedded in the account-tree leaf
// hash. Two steps:
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
		panic(fmt.Sprintf("binance identity: expected 64-hex-char internal user id, got len=%d", len(internalUserID)))
	}
	raw, err := hex.DecodeString(internalUserID)
	if err != nil {
		panic("binance identity: hex decode failed: " + err.Error())
	}
	reduced := new(fr.Element).SetBytes(raw).Marshal()
	var out [32]byte
	copy(out[:], reduced)
	return out
}

func (identity) Scheme() string { return identityScheme }
