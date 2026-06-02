// por_port.go declares the model-blind types the non-zk Merkle-sum
// proof-of-liabilities side line (PRODUCTION_ROADMAP Stage MS, gate G19)
// exchanges across the core/pkg boundary: the per-account leaf record a
// model collector emits, the persisted attest row, and the AttestStore
// persistence port. These mirror the userproof equivalents
// (SumLeafRecord ~ AccountInfo projection, AttestProofDTO ~ UserProofDTO,
// AttestStore ~ UserProofStore) so the side line reuses the same
// adapter/injection discipline.

package host

import "math/big"

// SumLeafRecord is one account's contribution to the dense Merkle-sum
// tree, produced by a model's collector (e.g.
// t1_simple_margin/host.CollectSumLeaves) and consumed model-blind by
// pkg/merklepor.
//
//   - Position is the dense, 0-based sum-tree index, assigned in
//     deterministic stream order (NOT the sparse account-tree AccountIndex).
//   - AccountID is the canonical account id bytes.
//   - LeafHash is the frozen 5-input AccountLeafHash output, reused as the
//     sum-tree leaf identity (gate G19, D5 — no separate identity scheme).
//   - Balance is the non-negative net liability the exchange owes the
//     account (TotalEquity - TotalDebt for T1).
type SumLeafRecord struct {
	Position  int
	AccountID []byte
	LeafHash  []byte
	Balance   *big.Int
}

// AttestProofDTO is the gorm-free per-account row the Merkle-sum attest
// service persists. It mirrors UserProofDTO: Index + AccountId keys, hex /
// decimal / JSON string payloads, and an embedded Config — the JSON
// published per-user sum-inclusion artifact the verify path reads back
// (pkg/merklepor.SumUserConfig). Root / RootSum are denormalised onto every
// row so a single row fully witnesses the published commitment.
type AttestProofDTO struct {
	Index     uint32
	AccountId string
	LeafHash  string
	Balance   string
	Proof     string // JSON-encoded sibling chain (leaf-to-root)
	Root      string // hex root hash
	RootSum   string // total liabilities, decimal string
	Config    string // JSON published artifact (verify-path input)
}

// AttestStore is the injected persistence port for the Merkle-sum attest
// service's per-account output. Mirrors UserProofStore; the MySQL adapter
// (store.AttestStoreAdapter) is the shipped backing.
type AttestStore interface {
	// EnsureSchema idempotently provisions the backing schema.
	EnsureSchema() error
	// Create inserts a batch of attest rows. The backing enforces unique
	// indexes on both Index and AccountId.
	Create(rows []AttestProofDTO) error
	// GetByIndex fetches the row at a positional index. Returns
	// corehost.ErrNotFound when no row matches.
	GetByIndex(idx uint32) (*AttestProofDTO, error)
}
