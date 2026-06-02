// Package config declares the on-disk configuration the zkpor merklepor
// (Merkle-sum proof-of-liabilities) engine consumes. The dense sum tree is
// built fresh in-memory each run, so there is no TreeDB block (unlike the
// zk userproof service): the only persistent backing is the attest table.
package config

import (
	"encoding/json"
	"fmt"
)

// Config drives the merklepor attest/audit services.
//
//	MysqlDataSource  DSN for the attest table (gorm/MySQL). Deployment-
//	                 secret; not in profile.toml.
//	DbSuffix         Optional table-name suffix (production: "").
//
// Unknown JSON fields are tolerated by json.Unmarshal.
type Config struct {
	MysqlDataSource string
	DbSuffix        string
}

// Parse unmarshals raw JSON config bytes into a *Config. Injection seam so
// callers supply config as a value rather than a path.
func Parse(raw []byte) (*Config, error) {
	cfg := &Config{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("merklepor config: parse: %w", err)
	}
	return cfg, nil
}
