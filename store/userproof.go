package store

import (
	"gorm.io/gorm"
)

// UserProof rows are the userproof service's per-account output. One
// row per user, indexed by both AccountIndex (uint32 sequence) and
// AccountId (hex string) — operators query by either depending on
// whether the customer presents an internal index or a public ID.
//
// All large fields are stored as strings:
//   - AccountLeafHash / Proof — base64 or hex per legacy format;
//   - Assets — JSON-marshalled []tier3spec.AccountAsset;
//   - Total{Equity,Debt,Collateral} — big.Int.String();
//   - Config — JSON-marshalled tier3host.UserConfig (what the
//     verifier -user mode reads back).
type UserProof struct {
	AccountIndex    uint32 `gorm:"index:idx_int,unique"`
	AccountId       string `gorm:"index:idx_str,unique"`
	AccountLeafHash string
	TotalEquity     string
	TotalDebt       string
	TotalCollateral string
	Assets          string
	Proof           string
	Config          string
}

const userProofTablePrefix = "userproof"

// UserProofStore wraps a *gorm.DB scoped to a single user-proof table.
type UserProofStore struct {
	db    *gorm.DB
	table string
}

// NewUserProofStore returns a store scoped to "userproof" + suffix.
func NewUserProofStore(db *gorm.DB, suffix string) *UserProofStore {
	return &UserProofStore{db: db, table: userProofTablePrefix + suffix}
}

// CreateTable runs gorm auto-migrate on the user-proof schema.
func (s *UserProofStore) CreateTable() error {
	return s.db.Table(s.table).AutoMigrate(UserProof{})
}

// Create inserts a batch of user-proof rows. Legacy writes in chunks
// of ~100 to amortise round-trip cost; the API accepts arbitrary-size
// slices and leaves the chunking choice to the caller.
func (s *UserProofStore) Create(rows []UserProof) error {
	if len(rows) == 0 {
		return nil
	}
	tx := s.db.Table(s.table).Create(rows)
	if tx.Error != nil {
		return ConvertMySQLErr(tx.Error)
	}
	return nil
}

// GetByIndex fetches the row at a specific account index. ErrNotFound
// when no row matches.
func (s *UserProofStore) GetByIndex(accountIndex uint32) (*UserProof, error) {
	var row UserProof
	tx := s.db.Clauses(MaxExecutionTimeHint).Table(s.table).
		Where("account_index = ?", accountIndex).Limit(1).Find(&row)
	if tx.Error != nil {
		return nil, ConvertMySQLErr(tx.Error)
	}
	if tx.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return &row, nil
}

// Count returns the number of rows in the table — the legacy
// userproof service uses this to skip already-written accounts on
// resume. Core-path callers can ignore the result.
func (s *UserProofStore) Count() (int64, error) {
	var n int64
	if err := s.db.Clauses(MaxExecutionTimeHint).Table(s.table).Count(&n).Error; err != nil {
		return 0, ConvertMySQLErr(err)
	}
	return n, nil
}
