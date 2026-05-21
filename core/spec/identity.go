package spec

// AccountIDProvider derives the 32-byte account identifier embedded
// in the account-tree leaf hash from an exchange-internal user ID.
//
// PoR requires that a user can independently reproduce their own
// AccountID — otherwise they cannot verify their own proof. The
// derivation algorithm (salting, hashing scheme) is therefore part
// of the published proof artifacts; the secret material used for
// salting is not, but the *algorithm* MUST be public.
//
// Model-independent — every solvency model uses the same 32-byte
// AccountID slot in the account-tree leaf.
type AccountIDProvider interface {
	// DeriveAccountID maps an exchange-internal user identifier to
	// the 32-byte AccountID used in the witness. Implementations
	// MUST be deterministic: same input -> same output.
	DeriveAccountID(internalUserID string) [32]byte

	// Scheme returns a stable, human-readable identifier for the
	// derivation algorithm (e.g. "hmac-sha256.v1"). Embedded in
	// published proof artifacts so users can audit how their ID is
	// produced.
	Scheme() string
}
