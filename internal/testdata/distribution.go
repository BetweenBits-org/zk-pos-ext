package testdata

// assetCatalog returns the symbol list for a profile, falling back to
// a tiny default when profile.Catalog.Symbols is empty (Binance-style
// production profile leaves it empty intentionally — the committed
// order comes from cex_assets.csv at snapshot time).
//
// numAssets caps the catalog to at most `capacity` (real slots) so the
// generator emits ≤ capacity per-asset rows. Reserved padding to the
// full capacity is the snapshot parser's job (already implemented).
func assetCatalog(symbols []string, capacity int) []string {
	if len(symbols) == 0 {
		symbols = defaultSymbols
	}
	if len(symbols) > capacity {
		symbols = symbols[:capacity]
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
