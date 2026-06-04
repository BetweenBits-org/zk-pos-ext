module github.com/BetweenBits-org/zk-pos-ext

go 1.22

toolchain go1.23.1

require (
	github.com/bnb-chain/zkbnb-smt v0.0.3-0.20221227064653-7422bfd51aa0
	github.com/consensys/gnark v0.10.0
	github.com/consensys/gnark-crypto v0.14.0
	github.com/go-sql-driver/mysql v1.8.1
	github.com/gocarina/gocsv v0.0.0-20230123225133-763e25b40669
	github.com/klauspost/compress v1.17.10
	github.com/pelletier/go-toml/v2 v2.3.1
	github.com/rs/zerolog v1.33.0
	gorm.io/driver/mysql v1.4.7
	gorm.io/gorm v1.25.0
	gorm.io/hints v1.1.2
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/bits-and-blooms/bitset v1.14.2 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/consensys/bavard v0.1.13 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/ethereum/go-ethereum v1.12.1 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-redis/redis/v8 v8.11.5 // indirect
	github.com/google/pprof v0.0.0-20240727154555-813a5fbdbec8 // indirect
	github.com/hashicorp/golang-lru v0.5.5-0.20221011183528-d4900dc688bf // indirect
	github.com/holiman/uint256 v1.2.3 // indirect
	github.com/ingonyama-zk/icicle v1.1.0 // indirect
	github.com/ingonyama-zk/iciclegnark v0.1.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mmcloughlin/addchain v0.4.0 // indirect
	github.com/onsi/gomega v1.27.10 // indirect
	github.com/panjf2000/ants/v2 v2.5.0 // indirect
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/ronanh/intcomp v1.1.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/crypto v0.26.0 // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
	rsc.io/tmplfunc v0.0.3 // indirect
)

replace (
	github.com/consensys/gnark => github.com/bnb-chain/gnark v0.10.1-0.20240910145009-4b5261061f04
	github.com/consensys/gnark-crypto => github.com/bnb-chain/gnark-crypto v0.14.1-0.20240910145340-609ab3a7eb9b
)
