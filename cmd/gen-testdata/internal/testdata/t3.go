package testdata

import "strconv"

// generateT3 synthesises a t3_tiered_haircut_margin_1pool standard-CSV
// snapshot. Adds collateral per asset (single pool) + tier_ratios.csv
// describing the haircut curve.
//
// Uniform distribution: every user holds same per-asset balance,
// collateral = equity, debt = 0. tier_ratios contains one tier per
// asset (ratio=100% = no haircut) — mirrors profile/t3_reference/
// testdata/happy/. Parser pads to corespec.TierCount when needed.
func generateT3(opts Options, symbols []string) error {
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
				strconv.FormatUint(eq, 10),
			})
		}
	}
	if err := writeCSV(opts.OutDir, "accounts.csv", accountsHeader, accountsRows); err != nil {
		return err
	}

	cexHeader := []string{"asset_index", "symbol", "total_equity", "total_debt", "base_price", "collateral"}
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
			strconv.FormatUint(total, 10),
		})
	}
	if err := writeCSV(opts.OutDir, "cex_assets.csv", cexHeader, cexRows); err != nil {
		return err
	}

	// tier_ratios.csv: one tier per asset, ratio=100 (no haircut),
	// boundary = 1e20. Parser pads remaining tiers up to corespec.
	// TierCount with reserved entries.
	tierHeader := []string{"asset_index", "tier_index", "boundary_value", "ratio", "precomputed_value"}
	tierRows := make([][]string, 0, len(symbols))
	for slot := range symbols {
		tierRows = append(tierRows, []string{
			strconv.Itoa(slot),
			"0",
			"100000000000000000000", // 1e20
			"100",
			// precomputed = boundary since ratio=100 (no haircut); matches
			// the audited recipe in core/tierpolicy.BuildTierCurve, which the
			// snapshot parser now validates.
			"100000000000000000000",
		})
	}
	return writeCSV(opts.OutDir, "tier_ratios.csv", tierHeader, tierRows)
}
