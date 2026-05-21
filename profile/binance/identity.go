package binance

import (
	"encoding/hex"
	"fmt"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// The reference snapshot ETL pre-hashes user IDs into 32-byte values
// stored directly in the user CSVs. This implementation preserves
// that contract — DeriveAccountID is a passthrough hex-decode for
// callers that already have the 32-byte form.
//
// A future iteration will absorb the actual HMAC/salt derivation so
// users can independently reproduce their AccountID from their
// internal user ID. The identityScheme constant MUST be updated when
// that change ships.
const identityScheme = "passthrough_hex.v0"

type identity struct{}

// NewIdentity returns Binance's AccountIDProvider.
func NewIdentity() spec.AccountIDProvider { return identity{} }

// DeriveAccountID treats internalUserID as a hex-encoded 32-byte
// AccountID. Inputs that are not exactly 64 hex chars panic —
// surfaces ETL bugs immediately rather than producing silent garbage.
func (identity) DeriveAccountID(internalUserID string) [32]byte {
	if len(internalUserID) != 64 {
		panic(fmt.Sprintf("binance identity: expected 64-hex-char internal user id, got len=%d", len(internalUserID)))
	}
	raw, err := hex.DecodeString(internalUserID)
	if err != nil {
		panic("binance identity: hex decode failed: " + err.Error())
	}
	var out [32]byte
	copy(out[:], raw)
	return out
}

func (identity) Scheme() string { return identityScheme }
