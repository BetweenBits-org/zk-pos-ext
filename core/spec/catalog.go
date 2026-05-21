package spec

// AssetCatalog enumerates the assets supported by a given customer
// deployment. Model-independent — every solvency model needs to know
// which symbols exist and what numeric indexes they occupy.
//
// Catalog identity (the ordered symbol list) is part of the published
// proof artifacts: a verifier MUST receive the same ordered list to
// interpret per-asset commitments.
//
// Implementations MUST be deterministic and immutable for the lifetime
// of one PoR snapshot. Adding or reordering symbols changes catalog
// identity and invalidates prior snapshots' commitments.
type AssetCatalog interface {
	// Capacity returns the maximum number of distinct assets the
	// circuit instance reserves slots for. MUST be >= len(Symbols()).
	// SHOULD equal spec.AssetCounts for engine-standard compliance.
	Capacity() int

	// Symbols returns the asset symbols in index order (index 0..N-1
	// where N == len(Symbols())). Symbols are returned lower-cased.
	Symbols() []string

	// IndexOf returns the zero-based index of a symbol, or (0, false)
	// if the symbol is not in the catalog. Lookup is case-insensitive.
	IndexOf(symbol string) (uint16, bool)
}
