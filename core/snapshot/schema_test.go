package snapshot_test

import (
	"strings"
	"testing"

	snapshotcsv "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/csv"
	snapshotschema "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/schema"
	t1schema "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/t1_simple_margin"
	t2schema "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/t2_static_haircut_margin"
	t3schema "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/t3_tiered_haircut_margin_1pool"
	t4schema "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/t4_tiered_haircut_margin_3pool"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
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

func TestStandardAlphaSchemaValidate(t *testing.T) {
	if err := snapshotschema.ValidateAlpha(snapshotschema.StandardAlphaSchema); err != nil {
		t.Fatalf("ValidateAlpha(StandardAlphaSchema): %v", err)
	}
	if got := len(snapshotschema.StandardAlphaSchema.Files); got != 2 {
		t.Fatalf("alpha file count = %d, want 2", got)
	}
	if snapshotschema.StandardAlphaSchema.Files[0].Name != snapshotschema.AlphaManifestFile {
		t.Fatalf("first alpha file = %q, want %q",
			snapshotschema.StandardAlphaSchema.Files[0].Name,
			snapshotschema.AlphaManifestFile)
	}
	if snapshotschema.StandardAlphaSchema.Files[1].Name != snapshotschema.AlphaValuesFile {
		t.Fatalf("second alpha file = %q, want %q",
			snapshotschema.StandardAlphaSchema.Files[1].Name,
			snapshotschema.AlphaValuesFile)
	}
}

func TestStandardAlphaSchemaCSVShape(t *testing.T) {
	manifest := snapshotschema.StandardAlphaSchema.Files[0]
	manifestCSV := `module_id,scope,field_name,field_type,required,description
regulator.kr.user_limit_v1,account,daily_limit,uint64,1,per-account daily limit
`
	if _, err := snapshotcsv.NewReader(strings.NewReader(manifestCSV), manifest, snapshotcsv.DefaultOptions()); err != nil {
		t.Fatalf("alpha manifest reader: %v", err)
	}

	values := snapshotschema.StandardAlphaSchema.Files[1]
	valuesCSV := `module_id,scope,subject,field_name,value
regulator.kr.user_limit_v1,account,0000000000000000000000000000000000000000000000000000000000000001,daily_limit,100000000
`
	reader, err := snapshotcsv.NewReader(strings.NewReader(valuesCSV), values, snapshotcsv.DefaultOptions())
	if err != nil {
		t.Fatalf("alpha values reader: %v", err)
	}
	row, err := reader.Read()
	if err != nil {
		t.Fatalf("alpha values read: %v", err)
	}
	if got, _ := row.Value("subject"); got != "0000000000000000000000000000000000000000000000000000000000000001" {
		t.Fatalf("alpha subject = %q", got)
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
