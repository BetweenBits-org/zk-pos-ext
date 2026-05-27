package schema

import "fmt"

// Alpha sidecar constants define the model-neutral transport schema
// for extension inputs consumed by non-noop ConstraintModules.
const (
	// AlphaSchemaVersion is the frozen v1 identifier for the generic
	// alpha sidecar file contract.
	AlphaSchemaVersion = "alpha_sidecar.v1"

	// AlphaManifestFile is the canonical manifest filename. It
	// declares module-local alpha fields, their scopes, and scalar
	// types before values are read.
	AlphaManifestFile = "alpha_manifest.csv"

	// AlphaValuesFile is the canonical value filename. It stores
	// entity-scoped alpha values in EAV form, allowing modules to carry
	// arbitrary field names without changing the base model CSV schema.
	AlphaValuesFile = "alpha_values.csv"

	// AlphaScopeSnapshot addresses one value for the whole snapshot.
	AlphaScopeSnapshot = "snapshot"
	// AlphaScopeAsset addresses one value for one asset slot.
	AlphaScopeAsset = "asset"
	// AlphaScopeAccount addresses one value for one account.
	AlphaScopeAccount = "account"
	// AlphaScopeAccountAsset addresses one value for one account-asset
	// pair.
	AlphaScopeAccountAsset = "account_asset"
)

// AlphaSchema is the model-neutral standard schema for alpha sidecar
// files. It deliberately avoids fixed business field names; modules
// declare field names in alpha_manifest.csv and provide values in
// alpha_values.csv.
type AlphaSchema struct {
	// Version identifies the sidecar contract. Changing filename,
	// scope, subject, or value semantics requires a new version.
	Version string
	// Files lists the logical alpha files and their metadata. Parsers
	// may require the files only when a non-noop module declares that it
	// consumes alpha inputs.
	Files []File
	// Invariants are deterministic validation rules that a module-aware
	// alpha parser must enforce before values cross into witness or
	// ConstraintContext state.
	Invariants []string
}

// StandardAlphaSchema is the v1 model-neutral alpha input sidecar.
// It is an EAV transport layer: arbitrary module field names live in
// data rows, not in CSV headers, so standard readers and auditors keep
// a stable file shape.
var StandardAlphaSchema = AlphaSchema{
	Version: AlphaSchemaVersion,
	Files: []File{
		{
			Name:       AlphaManifestFile,
			Required:   false,
			Grain:      "one row per module, scope, and alpha field",
			PrimaryKey: []string{"module_id", "scope", "field_name"},
			SortKey:    []string{"module_id", "scope", "field_name"},
			Fields: []Field{
				{Name: "module_id", Type: FieldString, Required: true, Description: "ConstraintModule ID that owns this alpha field."},
				{Name: "scope", Type: FieldEnum, Required: true, Description: "One of snapshot, asset, account, account_asset."},
				{Name: "field_name", Type: FieldString, Required: true, Description: "Module-local canonical field name."},
				{Name: "field_type", Type: FieldEnum, Required: true, Description: "One of the canonical FieldType values accepted by this package."},
				{Name: "required", Type: FieldUint8, Required: true, Description: "0 or 1; whether the module requires this field for every subject in scope."},
				{Name: "description", Type: FieldString, Required: false, Description: "Human-readable audit description."},
			},
			Description: "Declares the arbitrary alpha fields consumed by registered ConstraintModules.",
		},
		{
			Name:       AlphaValuesFile,
			Required:   false,
			Grain:      "one row per module, scope, subject, and alpha field value",
			PrimaryKey: []string{"module_id", "scope", "subject", "field_name"},
			SortKey:    []string{"module_id", "scope", "subject", "field_name"},
			Fields: []Field{
				{Name: "module_id", Type: FieldString, Required: true, Description: "ConstraintModule ID that owns this value."},
				{Name: "scope", Type: FieldEnum, Required: true, Description: "One of snapshot, asset, account, account_asset."},
				{Name: "subject", Type: FieldString, Required: true, Description: "snapshot, asset_index, account_id, or account_id:asset_index depending on scope."},
				{Name: "field_name", Type: FieldString, Required: true, Description: "Module-local canonical field name declared in alpha_manifest.csv."},
				{Name: "value", Type: FieldString, Required: true, Description: "Canonical scalar encoded as string, parsed according to the manifest field_type."},
			},
			Description: "Carries arbitrary module-owned alpha input values without extending base model CSV headers.",
		},
	},
	Invariants: []string{
		"scope is closed over snapshot, asset, account, account_asset.",
		"field_type in alpha_manifest.csv must be one of the canonical FieldType values.",
		"required is 0 or 1.",
		"Every alpha_values.csv row must have a matching (module_id, scope, field_name) row in alpha_manifest.csv.",
		"value must parse according to the manifest field_type before entering witness or ConstraintContext state.",
		"subject grammar is scope-dependent: snapshot uses \"snapshot\"; asset uses decimal asset_index; account uses 64-hex account_id; account_asset uses \"<account_id>:<asset_index>\".",
		"Rows are canonical after preprocessing: no raw decimals, customer column aliases, or negative numeric values.",
	},
}

// ValidateAlpha checks alpha sidecar schema metadata consistency. It
// validates the sidecar contract itself, not individual alpha data
// rows; module-aware parsers enforce row-level invariants.
func ValidateAlpha(s AlphaSchema) error {
	if s.Version == "" {
		return fmt.Errorf("alpha schema version is empty")
	}
	if len(s.Files) == 0 {
		return fmt.Errorf("alpha schema has no files")
	}
	return validateFiles("alpha schema", s.Files)
}
