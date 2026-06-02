// Package t3_tiered_haircut_margin_1pool defines the standard raw
// snapshot schema for the T3 Tiered-Haircut Margin 1 Pool solvency
// model.
package t3_tiered_haircut_margin_1pool

import (
	snapshotschema "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/schema"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
)

// StandardSchema is the v1 canonical raw data contract for
// t3_tiered_haircut_margin_1pool. It extends T2 by moving haircut
// policy from a static basis-points field into an asset-specific tier
// curve.
var StandardSchema = snapshotschema.Schema{
	ModelID: corespec.T3TieredHaircutMargin1Pool,
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
				{Name: "collateral", Type: snapshotschema.FieldUint64, Required: true, Description: "Collateral balance for the model's single collateral pool."},
			},
			Description: "Canonical account-asset rows used to derive T3 AccountInfo and tiered collateral totals.",
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
				{Name: "collateral", Type: snapshotschema.FieldUint64, Required: true, Description: "Published exchange collateral aggregate for this asset."},
			},
			Description: "Published per-asset totals used for the CEX asset commitment.",
		},
		{
			Name:       "tier_ratios.csv",
			Required:   true,
			Grain:      "one row per asset and tier",
			PrimaryKey: []string{"asset_index", "tier_index"},
			SortKey:    []string{"asset_index", "tier_index"},
			Fields: []snapshotschema.Field{
				{Name: "asset_index", Type: snapshotschema.FieldUint16, Required: true, Description: "Catalog asset slot."},
				{Name: "tier_index", Type: snapshotschema.FieldUint16, Required: true, Description: "Dense tier order for this asset's collateral curve."},
				{Name: "boundary_value", Type: snapshotschema.FieldBigInt, Required: true, Description: "Upper boundary for the tier in balance-scaled integer units."},
				{Name: "ratio", Type: snapshotschema.FieldUint8, Required: true, Description: "Tier haircut ratio as used by the audited circuit implementation."},
				{Name: "precomputed_value", Type: snapshotschema.FieldBigInt, Required: true, Description: "Precomputed cumulative value at the tier boundary."},
			},
			Description: "Asset-specific piecewise-linear haircut curves for the single collateral pool.",
		},
	},
	Invariants: []string{
		"Rows are canonical after mapping: no decimal strings, no customer column aliases, and no negative values.",
		"(account_id, asset_index) pairs are unique; omitted account-asset pairs are interpreted as zero balances.",
		"collateral must be less than or equal to equity for the same account-asset row unless the active invalid-account policy explicitly drops the row before witness construction.",
		"tier_index is dense per asset_index, and boundary_value is strictly increasing in tier order.",
		"precomputed_value must match the cumulative tier function used by the audited T3 circuit; parser validation rejects mismatches before witness construction.",
		"Per-account TotalCollateral is derived by applying the asset's tier curve to collateral; the T3 circuit enforces TotalCollateral >= TotalDebt.",
		"cex_assets.csv is padded by the parser to AssetCatalog.Capacity() with reserved zero slots; real rows must match catalog order and capacity.",
	},
}
