// Package config declares the on-disk configuration the zkpor witness
// service consumes. Mirrors legacy src/witness/config so an existing
// config.json keeps working.
package config

// Config drives the witness service.
//
//	MysqlDataSource    DSN for the batch witness table (gorm/MySQL).
//	UserDataFile       Path to the customer's snapshot directory
//	                   (CSV files; the binance profile's
//	                   snapshot adapter consumes it).
//	DbSuffix           Optional table-name suffix (production: "").
//	AssetCapacity      Per-deployment asset slot count baked into the
//	                   trusted setup. Must match keygen, prover,
//	                   verifier, and userproof for this deployment.
//	TreeDB.Driver      "memory" or "redis".
//	TreeDB.Option.Addr Redis endpoint when Driver == "redis".
type Config struct {
	MysqlDataSource string
	UserDataFile    string
	DbSuffix        string
	AssetCapacity   int
	TreeDB          struct {
		Driver string
		Option struct {
			Addr string
		}
	}
}
