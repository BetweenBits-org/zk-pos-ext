package testdata

import "strconv"

// generateT4 synthesises a t4_tiered_haircut_margin_3pool standard-CSV
// snapshot. Three collateral pools per asset (loan / margin /
// portfolio_margin), each with its own tier curve in tier_ratios.csv.
//
// Uniform distribution: each user's equity per asset is split into the
// three pools (loan = equity/2, margin = equity/4, pm = equity/4,
// with a hard floor of 0 for tiny equity values). debt = 0 so the
// per-asset invariant loan+margin+pm ≤ equity holds.
func generateT4(opts Options, symbols []string) error {
	accountsHeader := []string{
		"account_id", "asset_index", "equity", "debt",
		"loan_collateral", "margin_collateral", "portfolio_margin_collateral",
	}
	accountsRows := make([][]string, 0, opts.Users*len(symbols))
	for i := 0; i < opts.Users; i++ {
		id := accountIDHex(opts.Seed, i)
		for slot := range symbols {
			eq := perUserAssetEquity(slot)
			loan, margin, pm := splitCollateral(eq)
			accountsRows = append(accountsRows, []string{
				id,
				strconv.Itoa(slot),
				strconv.FormatUint(eq, 10),
				"0",
				strconv.FormatUint(loan, 10),
				strconv.FormatUint(margin, 10),
				strconv.FormatUint(pm, 10),
			})
		}
	}
	if err := writeCSV(opts.OutDir, "accounts.csv", accountsHeader, accountsRows); err != nil {
		return err
	}

	cexHeader := []string{
		"asset_index", "symbol", "total_equity", "total_debt", "base_price",
		"loan_collateral", "margin_collateral", "portfolio_margin_collateral",
	}
	cexRows := make([][]string, 0, len(symbols))
	for slot, sym := range symbols {
		eq := perUserAssetEquity(slot)
		loan, margin, pm := splitCollateral(eq)
		userCount := uint64(opts.Users)
		cexRows = append(cexRows, []string{
			strconv.Itoa(slot),
			sym,
			strconv.FormatUint(eq*userCount, 10),
			"0",
			strconv.FormatUint(basePriceFor(sym), 10),
			strconv.FormatUint(loan*userCount, 10),
			strconv.FormatUint(margin*userCount, 10),
			strconv.FormatUint(pm*userCount, 10),
		})
	}
	if err := writeCSV(opts.OutDir, "cex_assets.csv", cexHeader, cexRows); err != nil {
		return err
	}

	// tier_ratios.csv: one tier per (asset, collateral_pool), ratio=100
	// (no haircut), boundary = 1e20. Parser pads remaining tiers to
	// corespec.TierCount.
	tierHeader := []string{"asset_index", "collateral_pool", "tier_index", "boundary_value", "ratio", "precomputed_value"}
	pools := []string{"loan", "margin", "portfolio_margin"}
	tierRows := make([][]string, 0, len(symbols)*len(pools))
	for slot := range symbols {
		for _, pool := range pools {
			tierRows = append(tierRows, []string{
				strconv.Itoa(slot),
				pool,
				"0",
				"100000000000000000000",
				"100",
				"0",
			})
		}
	}
	return writeCSV(opts.OutDir, "tier_ratios.csv", tierHeader, tierRows)
}

// splitCollateral distributes an equity value into the three T4
// collateral buckets so that loan + margin + pm ≤ equity. For very
// small equity (≤2) the high-resolution split collapses; the function
// guarantees floor-zero so the per-asset invariant is preserved.
func splitCollateral(equity uint64) (loan, margin, pm uint64) {
	if equity == 0 {
		return 0, 0, 0
	}
	loan = equity / 2
	margin = equity / 4
	pm = equity - loan - margin // ensures sum == equity for the small-N case
	return
}
