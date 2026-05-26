// Package config declares the on-disk configuration the zkpor prover
// service consumes. R8-C/3 slimmed this to deployment-secret fields
// only — AssetsCountTiers and ZkKeyName stems are derived from the
// declarative profile.toml + the -keys-dir flag.
//
// Redis is omitted; the core-path prover uses DB-poll
// (ClaimOldestByStatus) instead of a BLPOP queue. A multi-worker
// follow-up slice will re-add the redis field if/when it lands.
package config

// Config drives the prover service.
//
//	MysqlDataSource   DSN for the witness/proof tables (gorm/MySQL).
//	                  Deployment-secret; not in profile.toml.
//	DbSuffix          Optional table-name suffix (production: "").
//
// Unknown JSON fields are tolerated by json.Unmarshal — pre-R8 configs
// that still carry ZkKeyName/AssetsCountTiers will load cleanly but
// those values are ignored.
type Config struct {
	MysqlDataSource string
	DbSuffix        string
}
