// Package config declares the on-disk configuration the zkpor witness
// service consumes. R8-C/2 slimmed this down to deployment-secret +
// runtime-ops fields only; per-customer values (asset capacity, user
// data directory, snapshot id, pricing, batch shapes) flow from the
// declarative profile.toml referenced by the -profile flag instead.
package config

// Config drives the witness service.
//
//	MysqlDataSource    DSN for the batch witness table (gorm/MySQL).
//	                   Deployment-secret; not in profile.toml.
//	DbSuffix           Optional table-name suffix (production: "").
//	TreeDB.Driver      "memory" or "redis" — runtime ops decision,
//	                   independent of customer.
//	TreeDB.Option.Addr Redis endpoint when Driver == "redis".
//
// Unknown JSON fields are tolerated by json.Unmarshal — pre-R8 configs
// that still carry UserDataFile/AssetCapacity will load cleanly but
// those values are ignored. The -profile flag is the new source of
// truth; smoke + production wiring write the slimmed shape.
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
