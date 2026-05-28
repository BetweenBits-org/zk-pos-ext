// Package config declares the on-disk configuration the zkpor userproof
// engine consumes. R8-D slimmed this to deployment-secret + runtime-
// ops fields only; per-customer values (asset capacity, user data
// directory, snapshot id) flow from the declarative profile.toml.
//
// R12-A library extraction: this schema moved out from
// zkpor/cmd/userproof/config so other in-process clients can import
// the userproof engine without dragging in cmd/main wiring.
package config

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
