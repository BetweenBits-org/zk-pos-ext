package binance

import (
	"fmt"
	"strings"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// catalog is constructed from an ordered symbol list — typically the
// header row of cex_assets_info.csv as parsed at snapshot time — plus
// a per-deployment capacity baked into the trusted setup ceremony.
type catalog struct {
	symbols  []string          // index -> lowercase symbol
	index    map[string]uint16 // lowercase symbol -> index
	capacity int               // trusted-setup asset slot count
}

// NewCatalog returns a Binance AssetCatalog over the given ordered
// symbol list and per-deployment capacity. Symbols are lower-cased on
// entry; lookup is case-insensitive.
//
// Capacity MUST be >= len(orderedSymbols) and is part of the trusted
// setup contract — keygen, witness, prover, verifier, userproof MUST
// agree on it.
func NewCatalog(orderedSymbols []string, capacity int) spec.AssetCatalog {
	if capacity < len(orderedSymbols) {
		panic(fmt.Sprintf("binance.NewCatalog: capacity %d < len(orderedSymbols) %d",
			capacity, len(orderedSymbols)))
	}
	syms := make([]string, len(orderedSymbols))
	idx := make(map[string]uint16, len(orderedSymbols))
	for i, s := range orderedSymbols {
		ls := strings.ToLower(s)
		syms[i] = ls
		idx[ls] = uint16(i)
	}
	return &catalog{symbols: syms, index: idx, capacity: capacity}
}

func (c *catalog) Capacity() int { return c.capacity }

func (c *catalog) Symbols() []string {
	out := make([]string, len(c.symbols))
	copy(out, c.symbols)
	return out
}

func (c *catalog) IndexOf(symbol string) (uint16, bool) {
	i, ok := c.index[strings.ToLower(symbol)]
	return i, ok
}
