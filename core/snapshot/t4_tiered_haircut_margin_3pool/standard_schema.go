// Package t4_tiered_haircut_margin_3pool defines the standard raw
// snapshot schema for the T4 Tiered-Haircut Margin 3 Pool solvency
// model.
package t4_tiered_haircut_margin_3pool

import (
	snapshotschema "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/schema"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
)

// StandardSchema is the v1 canonical raw data contract for
// t4_tiered_haircut_margin_3pool. It matches the Binance-class model:
// equity/debt plus three independent collateral pools, each with its
// own tier curve.
var StandardSchema = snapshotschema.Schema{
	ModelID: corespec.T4TieredHaircutMargin3Pool,
	Version: "v1",
	Files: []snapshotschema.File{
		{
			Name:       "accounts.csv",
			Required:   true,
			Grain:      "one row per account and asset",
			PrimaryKey: []string{"account_id", "asset_index"},
			SortKey:    []string{"account_index", "asset_index", "account_id"},
			Fields: []snapshotschema.Field{
				{Name: "account_index", Type: snapshotschema.FieldUint32, Required: false, Description: "Dense Merkle leaf order. If omitted, the parser derives it from first-seen valid account order."},
				{Name: "account_id", Type: snapshotschema.FieldAccountID, Required: true, Description: "64-hex account identifier, reduced to BN254 canonical bytes before hashing."},
				{Name: "asset_index", Type: snapshotschema.FieldUint16, Required: true, Description: "Catalog asset slot. Must be less than the deployment asset capacity."},
				{Name: "equity", Type: snapshotschema.FieldUint64, Required: true, Description: "User claim for this asset after balance scaling."},
				{Name: "debt", Type: snapshotschema.FieldUint64, Required: true, Description: "User debt for this asset after balance scaling."},
				{Name: "loan_collateral", Type: snapshotschema.FieldUint64, Required: true, Description: "Collateral balance in the loan pool."},
				{Name: "margin_collateral", Type: snapshotschema.FieldUint64, Required: true, Description: "Collateral balance in the cross-margin pool."},
				{Name: "portfolio_margin_collateral", Type: snapshotschema.FieldUint64, Required: true, Description: "Collateral balance in the portfolio-margin pool."},
			},
			Description: "Canonical account-asset rows used to derive T4 AccountInfo and three-pool tiered collateral totals.",
		},
		{
			Name:       "cex_assets.csv",
			Required:   true,
			Grain:      "one row per asset slot with published exchange totals",
			PrimaryKey: []string{"asset_index"},
			SortKey:    []string{"asset_index"},
			Fields: []snapshotschema.Field{
				{Name: "asset_index", Type: snapshotschema.FieldUint16, Required: true, Description: "Catalog asset slot."},
				{Name: "symbol", Type: snapshotschema.FieldString, Required: true, Description: "Lowercase asset symbol matching the deployment catalog."},
				{Name: "total_equity", Type: snapshotschema.FieldUint64, Required: true, Description: "Published exchange total equity for this asset after balance scaling."},
				{Name: "total_debt", Type: snapshotschema.FieldUint64, Required: true, Description: "Published exchange total debt for this asset after balance scaling."},
				{Name: "base_price", Type: snapshotschema.FieldUint64, Required: true, Description: "Price-scaled reporting value committed with the asset totals."},
				{Name: "loan_collateral", Type: snapshotschema.FieldUint64, Required: true, Description: "Published exchange loan-pool collateral aggregate for this asset."},
				{Name: "margin_collateral", Type: snapshotschema.FieldUint64, Required: true, Description: "Published exchange cross-margin-pool collateral aggregate for this asset."},
				{Name: "portfolio_margin_collateral", Type: snapshotschema.FieldUint64, Required: true, Description: "Published exchange portfolio-margin-pool collateral aggregate for this asset."},
			},
			Description: "Published per-asset totals used for the CEX asset commitment.",
		},
		{
			Name:       "tier_ratios.csv",
			Required:   true,
			Grain:      "one row per asset, collateral pool, and tier",
			PrimaryKey: []string{"asset_index", "collateral_pool", "tier_index"},
			SortKey:    []string{"asset_index", "collateral_pool", "tier_index"},
			Fields: []snapshotschema.Field{
				{Name: "asset_index", Type: snapshotschema.FieldUint16, Required: true, Description: "Catalog asset slot."},
				{Name: "collateral_pool", Type: snapshotschema.FieldEnum, Required: true, Description: "One of loan, margin, portfolio_margin."},
				{Name: "tier_index", Type: snapshotschema.FieldUint16, Required: true, Description: "Dense tier order for this asset and collateral pool."},
				{Name: "boundary_value", Type: snapshotschema.FieldBigInt, Required: true, Description: "Upper boundary for the tier in balance-scaled integer units."},
				{Name: "ratio", Type: snapshotschema.FieldUint8, Required: true, Description: "Tier haircut ratio as used by the audited circuit implementation."},
				{Name: "precomputed_value", Type: snapshotschema.FieldBigInt, Required: true, Description: "Precomputed cumulative value at the tier boundary."},
			},
			Description: "Asset-specific piecewise-linear haircut curves for each collateral pool.",
		},
	},
	Invariants: []string{
		"Rows are canonical after mapping: no decimal strings, no customer column aliases, and no negative values.",
		"(account_id, asset_index) pairs are unique; omitted account-asset pairs are interpreted as zero balances.",
		"loan_collateral + margin_collateral + portfolio_margin_collateral must be less than or equal to equity for the same account-asset row unless the active invalid-account policy explicitly drops the row before witness construction.",
		"collateral_pool is closed over loan, margin, and portfolio_margin; each pool has an independent dense tier_index sequence per asset_index.",
		"boundary_value is strictly increasing per (asset_index, collateral_pool), and precomputed_value must match the cumulative tier function used by the audited T4 circuit.",
		"Per-account TotalCollateral is derived by applying the relevant pool-specific tier curve to each collateral field, then summing pools; the T4 circuit enforces TotalCollateral >= TotalDebt.",
		"cex_assets.csv is padded by the parser to AssetCatalog.Capacity() with reserved zero slots; real rows must match catalog order and capacity.",
	},
}
