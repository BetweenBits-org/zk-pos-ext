// Package config declares the on-disk configuration the zkpor userproof
// service consumes. R8-D slimmed this to deployment-secret + runtime-
// ops fields only; per-customer values (asset capacity, user data
// directory, snapshot id) flow from the declarative profile.toml.
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
