// Package config declares the on-disk configuration the zkpor prover
// engine consumes. R8-C/3 slimmed this to deployment-secret fields
// only — AssetsCountTiers and ZkKeyName stems are derived from the
// declarative profile.toml + the keys-dir option.
//
// Redis is omitted; the core-path prover uses DB-poll
// (ClaimOldestByStatus) instead of a BLPOP queue. A multi-worker
// follow-up slice will re-add the redis field if/when it lands.
//
// R12-A library extraction: this schema moved out from
// zkpor/cmd/prover/config so other in-process clients can import the
// prover engine without dragging in cmd/main wiring.
package config

import (
	"encoding/json"
	"fmt"
)

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

// Parse unmarshals raw JSON config bytes into a *Config. It is the
// injection seam that lets callers supply config as a value rather than
// reading a path: the engine no longer needs to know where the bytes
// came from (file, env, embedded fixture). Unknown JSON fields are
// tolerated; see the Config doc for the slimmed R8 schema.
func Parse(raw []byte) (*Config, error) {
	cfg := &Config{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("prover config: parse: %w", err)
	}
	return cfg, nil
}
