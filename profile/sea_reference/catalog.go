package sea_reference

import (
	"fmt"
	"strings"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// catalog is constructed from an ordered symbol list — typically the
// header row of the customer's cex_assets_info.csv as parsed at
// snapshot time. Capacity is supplied at construction time (matches
// the trusted-setup ceremony's asset slot count for this deployment).
type catalog struct {
	symbols  []string          // index -> lowercase symbol
	index    map[string]uint16 // lowercase symbol -> index
	capacity int
}

// NewCatalog returns a sea_reference AssetCatalog over the given
// ordered symbol list and per-deployment capacity. Symbols are
// lower-cased on entry; lookup is case-insensitive.
//
// Capacity MUST be >= len(orderedSymbols) and is part of the trusted
// setup contract — keygen, witness, prover, verifier, userproof MUST
// agree on it.
func NewCatalog(orderedSymbols []string, capacity int) spec.AssetCatalog {
	if capacity < len(orderedSymbols) {
		panic(fmt.Sprintf("sea_reference.NewCatalog: capacity %d < len(orderedSymbols) %d",
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
