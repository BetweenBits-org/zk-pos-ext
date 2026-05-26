// Package config declares the on-disk configuration the zkpor prover
// service consumes. Mirrors legacy src/prover/config so an existing
// config.json keeps working, with two scope changes for the core path:
//
//   - Redis is omitted. The core-path prover uses DB-poll
//     (ClaimOldestByStatus) instead of a BLPOP queue. A multi-worker
//     follow-up slice will re-add the redis field if/when it lands.
package config

// Config drives the prover service.
//
//	MysqlDataSource   DSN for the witness/proof tables (gorm/MySQL).
//	DbSuffix          Optional table-name suffix (production: "").
//	ZkKeyName         File stems for the snark artifacts, one per
//	                  AssetsCountTiers entry (same index). The
//	                  prover appends ".r1cs", ".pk", ".vk".
//	AssetsCountTiers  Per-batch asset-count tiers in the same order
//	                  as ZkKeyName.
type Config struct {
	MysqlDataSource  string
	DbSuffix         string
	ZkKeyName        []string
	AssetsCountTiers []int
}
