package mapping_test

import (
	"testing"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/mapping"
	snapshotschema "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/schema"
	t1schema "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t1_simple_margin"
)

func TestValidateDirectMapping(t *testing.T) {
	cfg := mapping.Config{
		Format: mapping.Format{Delimiter: ",", NullValues: []string{"", "NA"}},
		Files: []mapping.File{{
			Name:   "accounts.csv",
			Source: "user_balances.csv",
			Mode:   mapping.ModeDirect,
			Columns: map[string]mapping.Column{
				"account_id":  {Source: "id", Type: snapshotschema.FieldAccountID},
				"asset_index": {Source: "asset_index", Type: snapshotschema.FieldUint16},
				"equity":      {Source: "balance", Type: snapshotschema.FieldUint64, DecimalScale: 100_000_000},
				"debt":        {Constant: "0", Type: snapshotschema.FieldUint64},
			},
		}},
	}
	if err := mapping.Validate(t1schema.StandardSchema, cfg); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	opts, err := mapping.BuildCSVOptions(cfg.Format)
	if err != nil {
		t.Fatalf("BuildCSVOptions: %v", err)
	}
	if opts.Comma != ',' || !opts.TrimSpace || len(opts.NullValues) != 2 {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestValidateWideAssetsMapping(t *testing.T) {
	cfg := mapping.Config{
		Files: []mapping.File{{
			Name:   "accounts.csv",
			Source: "user_shard.csv",
			Mode:   mapping.ModeWideAssets,
			Columns: map[string]mapping.Column{
				"account_id":  {Source: "id"},
				"asset_index": {SourcePrefix: "asset_"},
				"equity":      {SourcePrefix: "equity_", DecimalScale: 100_000_000},
				"debt":        {SourcePrefix: "debt_", DecimalScale: 100_000_000},
			},
		}},
	}
	if err := mapping.Validate(t1schema.StandardSchema, cfg); err != nil {
		t.Fatalf("Validate wide_assets: %v", err)
	}
}

func TestValidateRejectsMissingRequired(t *testing.T) {
	cfg := mapping.Config{
		Files: []mapping.File{{
			Name:   "accounts.csv",
			Source: "user_balances.csv",
			Columns: map[string]mapping.Column{
				"account_id":  {Source: "id"},
				"asset_index": {Source: "asset_index"},
				"equity":      {Source: "balance"},
			},
		}},
	}
	if err := mapping.Validate(t1schema.StandardSchema, cfg); err == nil {
		t.Fatal("Validate accepted missing required debt mapping")
	}
}

func TestValidateRejectsBadColumnRule(t *testing.T) {
	tests := []struct {
		name string
		rule mapping.Column
	}{
		{name: "two sources", rule: mapping.Column{Source: "a", Constant: "0"}},
		{name: "bad decimal scale target", rule: mapping.Column{Source: "id", DecimalScale: 100}},
		{name: "type mismatch", rule: mapping.Column{Source: "id", Type: snapshotschema.FieldUint64}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mapping.Config{
				Files: []mapping.File{{
					Name:   "accounts.csv",
					Source: "user_balances.csv",
					Columns: map[string]mapping.Column{
						"account_id":  tt.rule,
						"asset_index": {Source: "asset_index"},
						"equity":      {Source: "balance"},
						"debt":        {Constant: "0"},
					},
				}},
			}
			if err := mapping.Validate(t1schema.StandardSchema, cfg); err == nil {
				t.Fatal("Validate accepted bad column rule")
			}
		})
	}
}

func TestBuildCSVOptionsRejectsBadDelimiter(t *testing.T) {
	_, err := mapping.BuildCSVOptions(mapping.Format{Delimiter: "::"})
	if err == nil {
		t.Fatal("BuildCSVOptions accepted multi-rune delimiter")
	}
}
