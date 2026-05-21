package binance

import (
	"strings"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// catalog is constructed from an ordered symbol list — typically the
// header row of cex_assets_info.csv as parsed at snapshot time.
type catalog struct {
	symbols []string          // index -> lowercase symbol
	index   map[string]uint16 // lowercase symbol -> index
}

// NewCatalog returns a Binance AssetCatalog over the given ordered
// symbol list. Symbols are lower-cased on entry; lookup is
// case-insensitive.
func NewCatalog(orderedSymbols []string) spec.AssetCatalog {
	syms := make([]string, len(orderedSymbols))
	idx := make(map[string]uint16, len(orderedSymbols))
	for i, s := range orderedSymbols {
		ls := strings.ToLower(s)
		syms[i] = ls
		idx[ls] = uint16(i)
	}
	return &catalog{symbols: syms, index: idx}
}

func (c *catalog) Capacity() int { return spec.AssetCounts }

func (c *catalog) Symbols() []string {
	out := make([]string, len(c.symbols))
	copy(out, c.symbols)
	return out
}

func (c *catalog) IndexOf(symbol string) (uint16, bool) {
	i, ok := c.index[strings.ToLower(symbol)]
	return i, ok
}
