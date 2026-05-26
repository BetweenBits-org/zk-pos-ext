// Package store is the zkpor persistence layer for cross-service
// artifacts. The four engine services share three MySQL tables —
// batch witnesses (witness writes, prover reads), proofs (prover
// writes, verifier reads via CSV export), and per-user inclusion
// proofs (userproof writes). gorm.io is the only ORM the engine
// depends on; the connection helper here pins logger and slow-query
// thresholds so service code stays free of gorm boilerplate.
//
// Schema is operational infrastructure, not a frozen engine contract —
// table layout MAY evolve between minor versions. The on-wire witness
// bytes (t4_tiered_haircut_margin_3pool/host.EncodeBatchWitness output) ARE part of the
// witness↔prover contract; the WitnessData column merely transports
// them as a base64 string.
package store

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/hints"
)

// Error sentinels mirror legacy src/utils.DbErr* so caller patterns
// (retry on timeout/interrupted, treat NotFound as a normal control
// flow signal) can carry over verbatim.
var (
	ErrNotFound         = errors.New("sql: no rows in result set")
	ErrTableNotFound    = errors.New("sql: table not found")
	ErrQueryTimeout     = errors.New("sql: query timeout")
	ErrQueryInterrupted = errors.New("sql: query interrupted")
)

// MaxExecutionTimeHint caps long-running SELECTs at the MySQL
// optimiser layer (10s). Attach via .Clauses(MaxExecutionTimeHint) to
// any read that can legitimately time out without crashing the
// service. Mirrors legacy src/utils.MaxExecutionTimeHint.
var MaxExecutionTimeHint = hints.New("MAX_EXECUTION_TIME(10000)")

// Open establishes a gorm connection against the supplied DSN with the
// engine's standard logger profile (silent except for >60s queries,
// no colour, ignore record-not-found). Callers MUST NOT share a single
// *gorm.DB across services that have independent retry policies.
func Open(dsn string) (*gorm.DB, error) {
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             60 * time.Second,
			LogLevel:                  logger.Silent,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
	return gorm.Open(mysqldriver.Open(dsn), &gorm.Config{Logger: gormLogger})
}

// ConvertMySQLErr translates MySQL driver errors with well-known
// numbers into store sentinels. Numbers mirror legacy
// src/utils.ConvertMysqlErrToDbErr (1317=interrupted, 3024=timeout,
// 1146=table missing). Pass-through for any other error.
func ConvertMySQLErr(err error) error {
	if err == nil {
		return nil
	}
	var mErr *mysql.MySQLError
	if !errors.As(err, &mErr) {
		return err
	}
	switch mErr.Number {
	case 1317:
		return ErrQueryInterrupted
	case 3024:
		return ErrQueryTimeout
	case 1146:
		return ErrTableNotFound
	default:
		return err
	}
}
