package testdata

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// accountIDHex generates a deterministic 64-hex account_id from an
// index and seed. Output is the SHA-256 of "user-<seed>-<idx>" rendered
// as 64 hex chars — what the standard-CSV parser's canonicalAccountID
// then decodes + BN254-reduces.
//
// SHA-256 output > fr.Modulus for ~50% of seeds (top bit ≥ 0xc...);
// that's fine — the parser reduces. We only need the 64-hex string to
// be deterministic, unique across users, and BN254-fr.Element-safe
// after reduction.
func accountIDHex(seed int64, idx int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("zkpor-testdata/%d/%d", seed, idx)))
	return hex.EncodeToString(h[:])
}
