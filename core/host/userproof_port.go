package host

// UserProofDTO is the gorm-free projection of a user-proof row. It
// mirrors the exported fields of store.UserProof exactly (store.UserProof
// already carries no gorm.Model, so this is a straight rename):
// AccountIndex is the uint32 sequence key; AccountId is the hex string
// key; the remaining fields are string payloads — AccountLeafHash and
// Proof in base64/hex per legacy format, Assets as JSON-marshalled
// []AccountAsset, Total{Equity,Debt,Collateral} as big.Int.String(), and
// Config as JSON-marshalled UserConfig. All string payloads are carried
// verbatim.
type UserProofDTO struct {
	AccountIndex    uint32
	AccountId       string
	AccountLeafHash string
	TotalEquity     string
	TotalDebt       string
	TotalCollateral string
	Assets          string
	Proof           string
	Config          string
}

// UserProofStore is the injected persistence port for the userproof
// service's per-account output. The MySQL adapter
// (store.UserProofStoreAdapter) is the one shipped backing.
type UserProofStore interface {
	// EnsureSchema idempotently provisions the backing schema. Safe to
	// call on every service startup.
	EnsureSchema() error

	// Create inserts a batch of user-proof rows. The backing enforces
	// unique indexes on both AccountIndex and AccountId: a duplicate key
	// is rejected (translated backing error). Payload strings are stored
	// verbatim.
	Create(rows []UserProofDTO) error

	// GetByIndex fetches the row at a specific account index. Returns
	// corehost.ErrNotFound when no row matches.
	GetByIndex(idx uint32) (*UserProofDTO, error)
}
