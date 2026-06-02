// Package t2_static_haircut_margin defines the standard raw snapshot
// schema for the T2 Static-Haircut Margin solvency model.
package t2_static_haircut_margin

import (
	snapshotschema "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/schema"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
)

// StandardSchema is the v1 canonical raw data contract for
// t2_static_haircut_margin. It extends T1 with one collateral amount
// and a static haircut basis-points value per asset.
var StandardSchema = snapshotschema.Schema{
	ModelID: corespec.T2StaticHaircutMargin,
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
			Description: "Canonical account-asset rows used to derive T2 AccountInfo and static-haircut collateral totals.",
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
				{Name: "haircut_bp", Type: snapshotschema.FieldUint16, Required: true, Description: "Static haircut in basis points. 10000 means no haircut."},
			},
			Description: "Published per-asset totals and static haircut policy used for the CEX asset commitment.",
		},
	},
	Invariants: []string{
		"Rows are canonical after mapping: no decimal strings, no customer column aliases, and no negative values.",
		"(account_id, asset_index) pairs are unique; omitted account-asset pairs are interpreted as zero balances.",
		"collateral must be less than or equal to equity for the same account-asset row unless the active invalid-account policy explicitly drops the row before witness construction.",
		"haircut_bp is in [0, 10000] and is constant for a given asset_index within the snapshot.",
		"Per-account TotalCollateral is derived as sum(collateral * haircut_bp / 10000); the T2 circuit enforces TotalCollateral >= TotalDebt.",
		"cex_assets.csv is padded by the parser to AssetCatalog.Capacity() with reserved zero slots; real rows must match catalog order and capacity.",
	},
}
