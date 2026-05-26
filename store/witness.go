package store

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// BatchWitness rows are the witness↔prover artifact channel. The
// witness service writes one row per batch (Height is the batch
// number, WitnessData is base64(s2(gob(BatchCreateUserWitness)))).
// The prover transitions Status through Published → Received →
// Finished as it picks rows up, runs groth16.Prove, and exports.
type BatchWitness struct {
	gorm.Model
	Height      int64 `gorm:"index:idx_height,unique"`
	WitnessData string
	Status      int64 `gorm:"index"`
}

// Status values used by the prover state machine. Numeric values are
// pinned so a partially-migrated cluster can still read each other's
// rows.
const (
	StatusPublished int64 = 0 // written by witness, awaiting prover pickup
	StatusReceived  int64 = 1 // prover picked up, proving in progress
	StatusFinished  int64 = 2 // prover finished, exported to proof table
)

// witnessTablePrefix is the legacy table-name prefix; the actual
// table name is prefix + suffix where suffix is operator-supplied
// (typically empty in production, "_test" in CI).
const witnessTablePrefix = "witness"

// WitnessStore wraps a *gorm.DB scoped to a single batch-witness
// table. Construct with NewWitnessStore.
type WitnessStore struct {
	db    *gorm.DB
	table string
}

// NewWitnessStore returns a store scoped to "witness" + suffix.
func NewWitnessStore(db *gorm.DB, suffix string) *WitnessStore {
	return &WitnessStore{db: db, table: witnessTablePrefix + suffix}
}

// CreateTable runs gorm auto-migrate on the batch witness schema.
// Idempotent; safe to call on service startup.
func (s *WitnessStore) CreateTable() error {
	return s.db.Table(s.table).AutoMigrate(BatchWitness{})
}

// Create inserts a batch of witness rows. Pass a slice (typical
// witness service writes one at a time, but the API accepts batches
// for future throughput tuning).
func (s *WitnessStore) Create(rows []BatchWitness) error {
	if len(rows) == 0 {
		return nil
	}
	if err := s.db.Table(s.table).Create(rows).Error; err != nil {
		return ConvertMySQLErr(err)
	}
	return nil
}

// Latest returns the row with the largest Height. Returns ErrNotFound
// (not nil) when the table is empty — the witness service's
// fresh-start branch keys off this exact sentinel.
func (s *WitnessStore) Latest() (*BatchWitness, error) {
	var height int64
	tx := s.db.Clauses(MaxExecutionTimeHint).Table(s.table).
		Select("height").Order("height desc").Limit(1).Find(&height)
	if tx.Error != nil {
		return nil, ConvertMySQLErr(tx.Error)
	}
	if tx.RowsAffected == 0 {
		return nil, ErrNotFound
	}

	var row BatchWitness
	tx = s.db.Clauses(MaxExecutionTimeHint).Table(s.table).
		Where("height = ?", height).Limit(1).Find(&row)
	if tx.Error != nil {
		return nil, ConvertMySQLErr(tx.Error)
	}
	if tx.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return &row, nil
}

// Touch updates a row's UpdatedAt without changing other fields.
// Useful for status-machine probes; unused in the witness core path.
func (s *WitnessStore) Touch(height int64) error {
	tx := s.db.Table(s.table).Where("height = ?", height).
		Updates(BatchWitness{Model: gorm.Model{UpdatedAt: time.Now()}})
	if tx.Error != nil {
		return ConvertMySQLErr(tx.Error)
	}
	if tx.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// IsNotFound is a small convenience for callers that want to write
// `if store.IsNotFound(err)` instead of importing errors and
// reaching for errors.Is(err, store.ErrNotFound).
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }
