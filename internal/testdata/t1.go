package testdata

import (
	"strconv"
)

// generateT1 synthesises a t1_simple_margin standard-CSV snapshot.
// Uniform distribution: every user holds the same balance across every
// catalog asset, debt always 0 (spot use case).
//
// Sum equality is automatic — cex_assets.csv totals are user_count ×
// per_user_equity for each slot. Per-asset rows in accounts.csv are
// emitted in strictly-increasing asset_index order, which matches the
// parser's per-user grouping + per-user circuit-side uniqueness check.
func generateT1(opts Options, symbols []string) error {
	// accounts.csv rows: numUsers × numAssets
	accountsHeader := []string{"account_id", "asset_index", "equity", "debt"}
	accountsRows := make([][]string, 0, opts.Users*len(symbols))
	for i := 0; i < opts.Users; i++ {
		id := accountIDHex(opts.Seed, i)
		for slot := range symbols {
			eq := perUserAssetEquity(slot)
			accountsRows = append(accountsRows, []string{
				id,
				strconv.Itoa(slot),
				strconv.FormatUint(eq, 10),
				"0",
			})
		}
	}
	if err := writeCSV(opts.OutDir, "accounts.csv", accountsHeader, accountsRows); err != nil {
		return err
	}

	// cex_assets.csv: one row per real asset. Reserved padding to
	// capacity is the parser's job (not the generator's).
	cexHeader := []string{"asset_index", "symbol", "total_equity", "total_debt", "base_price"}
	cexRows := make([][]string, 0, len(symbols))
	for slot, sym := range symbols {
		eq := perUserAssetEquity(slot)
		totalEquity := eq * uint64(opts.Users)
		cexRows = append(cexRows, []string{
			strconv.Itoa(slot),
			sym,
			strconv.FormatUint(totalEquity, 10),
			"0",
			strconv.FormatUint(basePriceFor(sym), 10),
		})
	}
	return writeCSV(opts.OutDir, "cex_assets.csv", cexHeader, cexRows)
}
