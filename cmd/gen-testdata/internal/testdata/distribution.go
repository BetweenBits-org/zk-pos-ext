package testdata

import "fmt"

// assetCatalog returns a symbol list of exactly `target` entries.
// Profile catalog is the primary source; defaultSymbols fills in when
// the profile leaves Catalog.Symbols empty (Binance-style production
// profile). If neither covers `target`, synthetic "asset_N" symbols
// extend the tail.
//
// R11-A measurement use case: target controls the per-user non-empty
// asset count, which in turn drives tier routing (e.g., target=50 →
// Tier 1 50_700, target=500 → Tier 2 500_92). Reserved padding to
// asset_capacity is still the snapshot parser's job.
func assetCatalog(symbols []string, target int) []string {
	if len(symbols) == 0 {
		symbols = defaultSymbols
	}
	if len(symbols) > target {
		symbols = symbols[:target]
	}
	for len(symbols) < target {
		symbols = append(symbols, fmt.Sprintf("asset_%d", len(symbols)))
	}
	return symbols
}

var defaultSymbols = []string{"btc", "eth", "usdt"}

// basePriceFor returns the static base_price for a symbol used to pack
// cex_assets.csv. Values mirror what the existing testdata/happy/
// fixtures use, so derived USD-scaled totals stay in the same range as
// the smoke tests at small scale.
func basePriceFor(symbol string) uint64 {
	switch symbol {
	case "btc":
		return 6500000000000 // 1e8 * 65000 (USD-scaled at 1e8 balance + 1e8 price)
	case "eth":
		return 350000000000
	case "usdt":
		return 100000000
	case "usdc":
		return 100000000
	case "bnb":
		return 60000000000
	default:
		return 1000000000 // 1e9 fallback for unknown symbols
	}
}

// perUserAssetEquity returns the per-user equity for an asset at the
// given slot index (0..numAssets). Uniform distribution: every user
// holds the same balance.
//
// Picks values that make sum equality (Σ user.equity[asset] =
// cex.total_equity[asset]) clean uint64-safe even at 1M users:
//
//	slot 0 = 10      → 10M  at 1M users
//	slot 1 = 100     → 100M at 1M users
//	slot 2 = 10000   → 10G at 1M users
//	slot 3+ = 1      → small filler for higher-index assets
//
// Stays comfortably under 2^64 for any realistic N.
func perUserAssetEquity(slot int) uint64 {
	switch slot {
	case 0:
		return 10
	case 1:
		return 100
	case 2:
		return 10000
	default:
		return 1
	}
}
