package host

// ProofDTO is the gorm-free projection of a proof row. It mirrors the
// exported data fields of store.Proof with gorm.Model dropped:
// ProofInfo is base64(groth16.Proof.WriteRawTo); CexAssetListCommitments
// and AccountTreeRoots are JSON arrays of base64 strings (size 2:
// before/after); AssetsCount is the per-batch asset count; BatchNumber
// is the batch height (unique). All string payloads are carried verbatim
// — adapters copy them with no re-encoding.
type ProofDTO struct {
	ProofInfo               string
	CexAssetListCommitments string
	AccountTreeRoots        string
	BatchCommitment         string
	AssetsCount             int
	BatchNumber             int64
}

// ProofStore is the injected persistence port for the prover→verifier
// proof channel. The MySQL adapter (store.ProofStoreAdapter) is the one
// shipped backing.
type ProofStore interface {
	// EnsureSchema idempotently provisions the backing schema. Safe to
	// call on every service startup.
	EnsureSchema() error

	// Create inserts one proof row. The backing enforces a unique index
	// on BatchNumber: a duplicate Create is rejected (translated backing
	// error), which the prover relies on for idempotent crash recovery.
	Create(row *ProofDTO) error

	// GetByBatchNumber fetches the proof at a specific batch height.
	// Returns corehost.ErrNotFound when no row matches — the prover's
	// "have I already proved this batch?" probe.
	GetByBatchNumber(n int64) (*ProofDTO, error)

	// ListAllInOrder returns every proof row sorted by BatchNumber
	// ascending. Returns an empty slice (nil error) when the store is
	// empty.
	ListAllInOrder() ([]ProofDTO, error)
}
