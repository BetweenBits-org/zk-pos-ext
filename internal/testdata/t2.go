package testdata

import "strconv"

// generateT2 synthesises a t2_static_haircut_margin standard-CSV
// snapshot. Adds collateral per asset (single pool, no tier curve);
// haircut_bp is static per asset.
//
// Uniform distribution: every user holds the same per-asset balance,
// collateral = equity (no negative net), debt = 0. Satisfies T2's
// account-level invariant Σ collateral × haircut / 10000 ≥ debt
// trivially (debt=0).
func generateT2(opts Options, symbols []string) error {
	accountsHeader := []string{"account_id", "asset_index", "equity", "debt", "collateral"}
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
				strconv.FormatUint(eq, 10), // collateral = equity
			})
		}
	}
	if err := writeCSV(opts.OutDir, "accounts.csv", accountsHeader, accountsRows); err != nil {
		return err
	}

	cexHeader := []string{"asset_index", "symbol", "total_equity", "total_debt", "base_price", "collateral", "haircut_bp"}
	cexRows := make([][]string, 0, len(symbols))
	for slot, sym := range symbols {
		eq := perUserAssetEquity(slot)
		total := eq * uint64(opts.Users)
		cexRows = append(cexRows, []string{
			strconv.Itoa(slot),
			sym,
			strconv.FormatUint(total, 10),
			"0",
			strconv.FormatUint(basePriceFor(sym), 10),
			strconv.FormatUint(total, 10), // collateral = total equity
			"10000",                       // haircut_bp = 100% (no haircut)
		})
	}
	return writeCSV(opts.OutDir, "cex_assets.csv", cexHeader, cexRows)
}
