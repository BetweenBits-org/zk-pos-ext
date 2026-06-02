package host

import (
	"encoding/hex"
	"fmt"

	"github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// IdentitySchemePassthroughHexBN254ReducedV0 is the v1 frozen scheme
// ID for the canonical engine identity provider (G2 closure, R3 step 4).
//
// Two-stage derivation: hex-decode the customer-supplied 64-hex
// internal user ID, then reduce the 32 raw bytes as a BN254 fr.Element
// (SetBytes → Marshal). The result is always a canonical field element
// matching what the snapshot adapter writes into the witness leaf —
// without the reduction, roughly half of all SHA-256 outputs would
// produce a user-facing AccountID that doesn't match the leaf input
// for any 32-byte value ≥ fr.Modulus.
//
// The scheme is engine-universal: every customer profile that hashes
// internal user IDs to a 32-byte digest before submission can reuse
// this. R8-A promoted the implementation from profile/binance and
// profile/sea_reference (renamed to profile/t4_reference and
// profile/t1_reference in Phase 3d+), which both held byte-identical
// copies.
const IdentitySchemePassthroughHexBN254ReducedV0 = "passthrough_hex_bn254_reduced.v0"

func init() {
	RegisterIdentity(IdentitySchemePassthroughHexBN254ReducedV0, newPassthroughHexBN254ReducedV0)
}

// passthroughHexBN254ReducedV0 is the AccountIDProvider implementation
// for IdentitySchemePassthroughHexBN254ReducedV0. Stateless — the
// factory may return the same instance every call.
type passthroughHexBN254ReducedV0 struct{}

var passthroughHexBN254ReducedV0Instance = passthroughHexBN254ReducedV0{}

func newPassthroughHexBN254ReducedV0() spec.AccountIDProvider {
	return passthroughHexBN254ReducedV0Instance
}

// DeriveAccountID maps an internal user ID (a 64-hex-char string
// already produced by the customer's pre-hash pipeline) to the
// 32-byte canonical AccountID embedded in the account-tree leaf hash.
//
// Inputs that are not exactly 64 hex chars panic — surfaces ETL bugs
// immediately rather than producing silent garbage downstream.
func (passthroughHexBN254ReducedV0) DeriveAccountID(internalUserID string) [32]byte {
	if len(internalUserID) != 64 {
		panic(fmt.Sprintf("identity %s: expected 64-hex-char internal user id, got len=%d",
			IdentitySchemePassthroughHexBN254ReducedV0, len(internalUserID)))
	}
	raw, err := hex.DecodeString(internalUserID)
	if err != nil {
		panic("identity " + IdentitySchemePassthroughHexBN254ReducedV0 + ": hex decode failed: " + err.Error())
	}
	reduced := new(fr.Element).SetBytes(raw).Marshal()
	var out [32]byte
	copy(out[:], reduced)
	return out
}

func (passthroughHexBN254ReducedV0) Scheme() string {
	return IdentitySchemePassthroughHexBN254ReducedV0
}
