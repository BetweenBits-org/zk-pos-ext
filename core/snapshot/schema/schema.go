// Package schema defines metadata for model-specific standard raw
// snapshot schemas.
//
// A standard raw schema is the canonical file/data-format layer that
// sits above customer-specific CSV/JSONL exports and below a model's
// SnapshotSource. Customer adapters or mapping config convert exchange
// data into these fields; model parsers then validate the schema and
// build deterministic AccountInfo / CexAssetInfo values for the zk
// boundary.
//
// The package also defines a model-neutral alpha sidecar schema. Alpha
// sidecars carry extension-module inputs in a stable EAV shape so
// customer-specific fields do not require ad hoc columns in the base
// model CSV files.
package schema

import (
	"fmt"

	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
)

// FieldType identifies a canonical scalar type in a standard raw
// snapshot schema. The type names describe values after customer
// mapping and scaling have run; raw decimals and customer-specific
// column names are intentionally not represented here.
type FieldType string

// Canonical field types accepted by v1 standard raw schemas.
const (
	// FieldUint8 is an unsigned integer in [0, 255].
	FieldUint8 FieldType = "uint8"
	// FieldUint16 is an unsigned integer in [0, 65535].
	FieldUint16 FieldType = "uint16"
	// FieldUint32 is an unsigned integer in [0, 2^32-1].
	FieldUint32 FieldType = "uint32"
	// FieldUint64 is an unsigned integer in [0, 2^64-1].
	FieldUint64 FieldType = "uint64"
	// FieldBigInt is a non-negative base-10 integer with no fixed
	// upper bound at the schema layer.
	FieldBigInt FieldType = "bigint"
	// FieldString is a UTF-8 string. Individual schema fields may
	// narrow this further in their descriptions.
	FieldString FieldType = "string"
	// FieldAccountID is a 64-hex-character account identifier that
	// is reduced to a BN254 field element before account-leaf hashing.
	FieldAccountID FieldType = "account_id_hex_bn254"
	// FieldEnum is a closed string set documented by the field
	// description. The parser must reject unknown values.
	FieldEnum FieldType = "enum"
)

// Field describes one canonical column or JSONL property in a standard
// raw snapshot file. Required=false means the parser may derive a
// deterministic value when the field is absent; derivation rules must
// be stated by the model schema invariants.
type Field struct {
	// Name is the canonical column name or JSONL property name after
	// customer mapping. Parsers use this exact identifier in errors and
	// mapping diagnostics.
	Name string
	// Type is the canonical scalar type accepted for the field after
	// customer-specific casting and decimal scaling.
	Type FieldType
	// Required reports whether the mapped input must contain the field.
	// Optional fields need deterministic derivation rules in Schema
	// invariants before a parser may omit them.
	Required bool
	// Description explains the field's role at the PoR boundary,
	// including any range or catalog relationship not captured by Type.
	Description string
}

// File describes one logical input file in a standard raw snapshot
// schema. CSV uses the field order exactly as listed. JSONL uses the
// same names as object keys and ignores order.
type File struct {
	// Name is the canonical logical filename. CSV parsers use this as
	// the default expected filename; JSONL parsers use it as the
	// logical stream name.
	Name string
	// Required reports whether the logical file must be present for the
	// model parser to build a complete SnapshotSource.
	Required bool
	// Grain describes what one row/object represents, e.g. "one row per
	// account and asset". It is documentation for auditors and parser
	// diagnostics.
	Grain string
	// PrimaryKey lists fields that must be unique together inside this
	// file after mapping. Empty means uniqueness is not specified at the
	// metadata layer.
	PrimaryKey []string
	// SortKey lists fields that define canonical ordering before values
	// are committed into account leaves or CEX asset commitments.
	SortKey []string
	// Fields are the canonical columns/properties in CSV order. JSONL
	// parsers use the same names as object keys and ignore order.
	Fields []Field
	// Description explains how the file participates in witness
	// construction or public statement validation.
	Description string
}

// Schema is the versioned standard raw data contract for one solvency
// model. Invariants document deterministic behavior that a parser must
// enforce before values cross into the SnapshotSource / witness
// boundary.
type Schema struct {
	// ModelID identifies the audited solvency model that owns this raw
	// schema. It must be one of core/spec.CatalogedModels.
	ModelID corespec.SolvencyModelID
	// Version is the raw schema version. v1 is frozen by G18 once R9
	// closes; additive-compatible changes require a new minor version.
	Version string
	// Files lists every logical input required or accepted by the
	// schema. Parsers must reject undeclared fields unless a later
	// mapping layer explicitly permits pass-through metadata.
	Files []File
	// Invariants are deterministic validation rules that cannot be
	// captured by field types alone, such as duplicate policy,
	// omitted-zero behavior, padding, and tier monotonicity.
	Invariants []string
}

// MustValidate panics if s is not internally well-formed. Standard
// schemas call it from package init tests or direct tests to catch
// duplicate fields, unknown field types, and missing required metadata
// before a parser consumes the contract.
func MustValidate(s Schema) {
	if err := Validate(s); err != nil {
		panic(err)
	}
}

// Validate checks schema metadata consistency. It does not validate
// snapshot rows; row-level validation belongs to the R9 parser layer.
func Validate(s Schema) error {
	if s.ModelID == "" {
		return fmt.Errorf("schema model id is empty")
	}
	if !corespec.IsCataloged(s.ModelID) {
		return fmt.Errorf("schema model %q is not in catalog", s.ModelID)
	}
	if s.Version == "" {
		return fmt.Errorf("schema %s version is empty", s.ModelID)
	}
	if len(s.Files) == 0 {
		return fmt.Errorf("schema %s has no files", s.ModelID)
	}
	return validateFiles(fmt.Sprintf("schema %s", s.ModelID), s.Files)
}

func validateFiles(prefix string, files []File) error {
	fileNames := map[string]struct{}{}
	for _, file := range files {
		if file.Name == "" {
			return fmt.Errorf("%s has file with empty name", prefix)
		}
		if _, ok := fileNames[file.Name]; ok {
			return fmt.Errorf("%s duplicate file %q", prefix, file.Name)
		}
		fileNames[file.Name] = struct{}{}
		if len(file.Fields) == 0 {
			return fmt.Errorf("%s file %q has no fields", prefix, file.Name)
		}
		fields := map[string]struct{}{}
		for _, field := range file.Fields {
			if field.Name == "" {
				return fmt.Errorf("%s file %q has field with empty name", prefix, file.Name)
			}
			if _, ok := fields[field.Name]; ok {
				return fmt.Errorf("%s file %q duplicate field %q", prefix, file.Name, field.Name)
			}
			fields[field.Name] = struct{}{}
			if !knownFieldType(field.Type) {
				return fmt.Errorf("%s file %q field %q has unknown type %q", prefix, file.Name, field.Name, field.Type)
			}
		}
		for _, key := range append(append([]string{}, file.PrimaryKey...), file.SortKey...) {
			if _, ok := fields[key]; !ok {
				return fmt.Errorf("%s file %q key field %q is not declared", prefix, file.Name, key)
			}
		}
	}
	return nil
}

func knownFieldType(t FieldType) bool {
	switch t {
	case FieldUint8, FieldUint16, FieldUint32, FieldUint64, FieldBigInt, FieldString, FieldAccountID, FieldEnum:
		return true
	default:
		return false
	}
}
