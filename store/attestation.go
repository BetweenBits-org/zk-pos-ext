package store

import (
	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
	"gorm.io/gorm"
)

// Attestation rows are the merklepor (Merkle-sum proof-of-liabilities)
// attest service's per-account output. One row per user, indexed by both
// LeafIndex (the dense positional sum-tree index) and AccountId (hex).
// Root / RootSum are denormalised onto every row so a single row fully
// witnesses the published commitment.
//
// LeafIndex (not Index) avoids the SQL reserved word `index`; the column
// is `leaf_index`. All large fields are strings: LeafHash hex, Balance /
// RootSum decimal, Proof JSON sibling chain, Config JSON SumUserConfig
// (what verify-user reads back).
type Attestation struct {
	LeafIndex uint32 `gorm:"index:idx_int,unique"`
	AccountId string `gorm:"index:idx_str,unique"`
	LeafHash  string
	Balance   string
	Proof     string
	Root      string
	RootSum   string
	Config    string
}

const attestationTablePrefix = "attestation"

// AttestationStore wraps a *gorm.DB scoped to a single attest table.
type AttestationStore struct {
	db    *gorm.DB
	table string
}

// NewAttestationStore returns a store scoped to "attestation" + suffix.
func NewAttestationStore(db *gorm.DB, suffix string) *AttestationStore {
	return &AttestationStore{db: db, table: attestationTablePrefix + suffix}
}

// CreateTable runs gorm auto-migrate on the attest schema.
func (s *AttestationStore) CreateTable() error {
	return s.db.Table(s.table).AutoMigrate(Attestation{})
}

// Create inserts a batch of attest rows.
func (s *AttestationStore) Create(rows []Attestation) error {
	if len(rows) == 0 {
		return nil
	}
	tx := s.db.Table(s.table).Create(rows)
	if tx.Error != nil {
		return ConvertMySQLErr(tx.Error)
	}
	return nil
}

// GetByIndex fetches the row at a positional leaf index. ErrNotFound when
// no row matches.
func (s *AttestationStore) GetByIndex(leafIndex uint32) (*Attestation, error) {
	var row Attestation
	tx := s.db.Clauses(MaxExecutionTimeHint).Table(s.table).
		Where("leaf_index = ?", leafIndex).Limit(1).Find(&row)
	if tx.Error != nil {
		return nil, ConvertMySQLErr(tx.Error)
	}
	if tx.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return &row, nil
}

// --- adapter -------------------------------------------------------------

// AttestStoreAdapter adapts a *AttestationStore to the corehost.AttestStore
// port (gorm-free DTOs). Mapping is a verbatim field copy; LeafIndex maps
// to the DTO's Index.
type AttestStoreAdapter struct {
	inner *AttestationStore
}

// Compile-time assertion that the adapter satisfies the port.
var _ corehost.AttestStore = (*AttestStoreAdapter)(nil)

// NewAttestStoreAdapter wraps the given *AttestationStore as a
// corehost.AttestStore.
func NewAttestStoreAdapter(inner *AttestationStore) *AttestStoreAdapter {
	return &AttestStoreAdapter{inner: inner}
}

func attestationFromDTO(dto corehost.AttestProofDTO) Attestation {
	return Attestation{
		LeafIndex: dto.Index,
		AccountId: dto.AccountId,
		LeafHash:  dto.LeafHash,
		Balance:   dto.Balance,
		Proof:     dto.Proof,
		Root:      dto.Root,
		RootSum:   dto.RootSum,
		Config:    dto.Config,
	}
}

func attestationToDTO(row *Attestation) corehost.AttestProofDTO {
	return corehost.AttestProofDTO{
		Index:     row.LeafIndex,
		AccountId: row.AccountId,
		LeafHash:  row.LeafHash,
		Balance:   row.Balance,
		Proof:     row.Proof,
		Root:      row.Root,
		RootSum:   row.RootSum,
		Config:    row.Config,
	}
}

// EnsureSchema delegates to the inner store's CreateTable. Idempotent.
func (a *AttestStoreAdapter) EnsureSchema() error {
	return a.inner.CreateTable()
}

// Create maps each DTO to a gorm row verbatim and inserts the batch.
func (a *AttestStoreAdapter) Create(rows []corehost.AttestProofDTO) error {
	out := make([]Attestation, len(rows))
	for i := range rows {
		out[i] = attestationFromDTO(rows[i])
	}
	return a.inner.Create(out)
}

// GetByIndex fetches the row at the given positional index, remapping a
// missing-row result to corehost.ErrNotFound.
func (a *AttestStoreAdapter) GetByIndex(idx uint32) (*corehost.AttestProofDTO, error) {
	row, err := a.inner.GetByIndex(idx)
	if err != nil {
		return nil, notFound(err)
	}
	dto := attestationToDTO(row)
	return &dto, nil
}
