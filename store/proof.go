package store

import (
	"gorm.io/gorm"
)

// Proof rows are the prover→verifier artifact channel. The prover
// writes one row per successfully-proved batch; the verifier-side
// exporter (dbtool / proof_table.csv) reads them in BatchNumber
// order. ProofInfo is base64(groth16.Proof.WriteRawTo);
// CexAssetListCommitments and AccountTreeRoots are JSON arrays of
// base64 strings (size 2: before/after).
type Proof struct {
	gorm.Model
	ProofInfo               string
	CexAssetListCommitments string
	AccountTreeRoots        string
	BatchCommitment         string
	AssetsCount             int
	BatchNumber             int64 `gorm:"index:idx_number,unique"`
}

const proofTablePrefix = "proof"

// ProofStore wraps a *gorm.DB scoped to a single proof table.
type ProofStore struct {
	db    *gorm.DB
	table string
}

// NewProofStore returns a store scoped to "proof" + suffix.
func NewProofStore(db *gorm.DB, suffix string) *ProofStore {
	return &ProofStore{db: db, table: proofTablePrefix + suffix}
}

// CreateTable runs gorm auto-migrate on the proof schema.
func (s *ProofStore) CreateTable() error {
	return s.db.Table(s.table).AutoMigrate(Proof{})
}

// Create inserts one proof row. The unique index on BatchNumber means
// a duplicate Create returns a MySQL-translated error — the prover
// checks GetByBatchNumber first to keep crash-recovery idempotent.
func (s *ProofStore) Create(row *Proof) error {
	tx := s.db.Table(s.table).Create(row)
	if tx.Error != nil {
		return ConvertMySQLErr(tx.Error)
	}
	if tx.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByBatchNumber fetches the proof at a specific batch height.
// Returns ErrNotFound when no row matches — the prover's "have I
// already proved this batch?" probe.
func (s *ProofStore) GetByBatchNumber(batchNumber int64) (*Proof, error) {
	var row Proof
	tx := s.db.Clauses(MaxExecutionTimeHint).Table(s.table).
		Where("batch_number = ?", batchNumber).Limit(1).Find(&row)
	if tx.Error != nil {
		return nil, ConvertMySQLErr(tx.Error)
	}
	if tx.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return &row, nil
}
