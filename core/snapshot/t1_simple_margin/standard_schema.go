// Package t1_simple_margin defines the standard raw snapshot schema
// for the T1 Simple Margin solvency model.
package t1_simple_margin

import (
	snapshotschema "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/schema"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
)

// StandardSchema is the v1 canonical raw data contract for
// t1_simple_margin. Amounts are already integer-scaled before they
// reach this schema; customer decimals and column names are handled by
// the mapping layer.
var StandardSchema = snapshotschema.Schema{
	ModelID: corespec.T1SimpleMargin,
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
				{Name: "debt", Type: snapshotschema.FieldUint64, Required: true, Description: "User debt for this asset after balance scaling. Spot deployments set this to zero."},
			},
			Description: "Canonical account-asset rows used to derive T1 AccountInfo and per-account totals.",
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
			},
			Description: "Published per-asset totals used for the CEX asset commitment.",
		},
	},
	Invariants: []string{
		"Rows are canonical after mapping: no decimal strings, no customer column aliases, and no negative values.",
		"(account_id, asset_index) pairs are unique; omitted account-asset pairs are interpreted as zero balances.",
		"account_index, when present, is dense among valid accounts and stable for the snapshot; when absent, parsers derive dense order from first-seen valid account order after deterministic file ordering.",
		"Per-account TotalEquity and TotalDebt are derived as sums over account rows, then the T1 circuit enforces TotalEquity >= TotalDebt.",
		"cex_assets.csv is padded by the parser to AssetCatalog.Capacity() with reserved zero slots; real rows must match catalog order and capacity.",
	},
}
