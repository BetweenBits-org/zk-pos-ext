// Package config declares the on-disk configuration the zkpor userproof
// service consumes. Mirrors legacy src/userproof/config so an existing
// config.json keeps working.
package config

// Config drives the userproof service.
//
//	MysqlDataSource    DSN for the user-proof table (gorm/MySQL).
//	UserDataFile       Path to the customer's snapshot directory
//	                   (CSV files; the binance profile's
//	                   snapshot adapter consumes it).
//	DbSuffix           Optional table-name suffix (production: "").
//	TreeDB.Driver      "memory" or "redis".
//	TreeDB.Option.Addr Redis endpoint when Driver == "redis".
type Config struct {
	MysqlDataSource string
	UserDataFile    string
	DbSuffix        string
	TreeDB          struct {
		Driver string
		Option struct {
			Addr string
		}
	}
}
