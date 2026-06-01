// Package config declares the on-disk configuration the zkpor userproof
// engine consumes. R8-D slimmed this to deployment-secret + runtime-
// ops fields only; per-customer values (asset capacity, user data
// directory, snapshot id) flow from the declarative profile.toml.
//
// R12-A library extraction: this schema moved out from
// zkpor/cmd/userproof/config so other in-process clients can import
// the userproof engine without dragging in cmd/main wiring.
package config

import (
	"encoding/json"
	"fmt"
)

// Config drives the userproof service.
//
//	MysqlDataSource    DSN for the user-proof table (gorm/MySQL).
//	                   Deployment-secret; not in profile.toml.
//	DbSuffix           Optional table-name suffix (production: "").
//	TreeDB.Driver      "memory" or "redis" — runtime ops decision.
//	TreeDB.Option.Addr Redis endpoint when Driver == "redis".
//
// Unknown JSON fields are tolerated by json.Unmarshal — pre-R8 configs
// that still carry UserDataFile/AssetCapacity load cleanly but those
// values are ignored.
type Config struct {
	MysqlDataSource string
	DbSuffix        string
	TreeDB          struct {
		Driver string
		Option struct {
			Addr string
		}
	}
}

// Parse unmarshals raw JSON config bytes into a *Config. It is the
// injection seam that lets callers supply config as a value rather than
// reading a path: the engine no longer needs to know where the bytes
// came from (file, env, embedded fixture). Unknown JSON fields are
// tolerated; see the Config doc for the slimmed R8 schema.
func Parse(raw []byte) (*Config, error) {
	cfg := &Config{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("userproof config: parse: %w", err)
	}
	return cfg, nil
}
