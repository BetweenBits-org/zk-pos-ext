package snapshot_test

import (
	"testing"

	snapshotschema "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/schema"
	t1schema "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t1_simple_margin"
	t2schema "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t2_static_haircut_margin"
	t3schema "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t3_tiered_haircut_margin_1pool"
	t4schema "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t4_tiered_haircut_margin_3pool"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

func TestStandardSchemasValidate(t *testing.T) {
	schemas := []snapshotschema.Schema{
		t1schema.StandardSchema,
		t2schema.StandardSchema,
		t3schema.StandardSchema,
		t4schema.StandardSchema,
	}
	seen := map[corespec.SolvencyModelID]bool{}
	for _, schema := range schemas {
		if err := snapshotschema.Validate(schema); err != nil {
			t.Fatalf("Validate(%s): %v", schema.ModelID, err)
		}
		if seen[schema.ModelID] {
			t.Fatalf("duplicate standard schema for %s", schema.ModelID)
		}
		seen[schema.ModelID] = true
	}
	for _, model := range corespec.CatalogedModels {
		if !seen[model] {
			t.Fatalf("missing standard schema for catalog model %s", model)
		}
	}
}

func TestStandardSchemaModelShapes(t *testing.T) {
	tests := []struct {
		name       string
		schema     snapshotschema.Schema
		accountLen int
		fileCount  int
	}{
		{name: "t1", schema: t1schema.StandardSchema, accountLen: 5, fileCount: 2},
		{name: "t2", schema: t2schema.StandardSchema, accountLen: 6, fileCount: 2},
		{name: "t3", schema: t3schema.StandardSchema, accountLen: 6, fileCount: 3},
		{name: "t4", schema: t4schema.StandardSchema, accountLen: 8, fileCount: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.schema.Files) != tt.fileCount {
				t.Fatalf("file count = %d, want %d", len(tt.schema.Files), tt.fileCount)
			}
			accounts := tt.schema.Files[0]
			if accounts.Name != "accounts.csv" {
				t.Fatalf("first file = %q, want accounts.csv", accounts.Name)
			}
			if len(accounts.Fields) != tt.accountLen {
				t.Fatalf("accounts.csv field count = %d, want %d", len(accounts.Fields), tt.accountLen)
			}
			for _, field := range []string{"account_id", "asset_index", "equity", "debt"} {
				if !hasField(accounts, field) {
					t.Fatalf("accounts.csv missing common field %q", field)
				}
			}
		})
	}
}

func hasField(file snapshotschema.File, name string) bool {
	for _, field := range file.Fields {
		if field.Name == name {
			return true
		}
	}
	return false
}
