// Package mapping defines the declarative raw-data mapping DSL used by
// standard snapshot parsers.
//
// The DSL describes how customer-owned raw CSV fields are converted to
// the canonical standard schema fields under core/snapshot/<model>.
// It is intentionally data-format level: it knows about column names,
// decimal scaling, constants, and CSV dialect, but it does not build
// model-specific AccountInfo or CexAssetInfo values.
package mapping

import (
	"fmt"
	"strings"

	snapshotcsv "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/csv"
	snapshotschema "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/schema"
)

// Config is the mapping DSL root for one customer snapshot source.
// Each File entry maps one customer raw file or logical stream into one
// canonical standard schema file.
type Config struct {
	// Format configures the raw CSV dialect before column mapping.
	Format Format `toml:"format"`
	// Files maps logical standard schema files to customer raw sources.
	// Empty means the profile still uses a procedural snapshot adapter.
	Files []File `toml:"files"`
}

// Format describes source CSV dialect options. Empty values select the
// standard snapshot defaults: comma delimiter, no comments, trim-space
// enabled, and no null aliases.
type Format struct {
	// Delimiter is the raw CSV delimiter. Empty means comma. Non-empty
	// values must contain exactly one rune.
	Delimiter string `toml:"delimiter"`
	// Comment skips raw CSV lines beginning with this rune. Empty
	// disables comments. Non-empty values must contain exactly one rune.
	Comment string `toml:"comment"`
	// TrimSpace overrides the default trim-space behavior when set.
	// Nil means true.
	TrimSpace *bool `toml:"trim_space"`
	// AllowUnknownColumns permits customer raw columns that no mapping
	// rule consumes. Canonical output never includes unknown columns.
	AllowUnknownColumns bool `toml:"allow_unknown_columns"`
	// NullValues lists raw values treated as absent before required-field
	// validation. Common examples: "", "NA", "null".
	NullValues []string `toml:"null_values"`
}

// File maps one standard schema file. Mode controls row-shape
// semantics: direct means one raw row maps to one canonical row;
// wide_assets means a model parser may expand asset-prefixed columns
// into multiple canonical account-asset rows.
type File struct {
	// Name is the standard schema file name, e.g. accounts.csv.
	Name string `toml:"name"`
	// Source is the customer raw filename or logical stream identifier.
	Source string `toml:"source"`
	// Mode is "direct" or "wide_assets". Empty means direct.
	Mode string `toml:"mode"`
	// Columns maps canonical field names to raw extraction rules.
	Columns map[string]Column `toml:"columns"`
}

// Column describes how to produce one canonical field. Exactly one of
// Source, Constant, or SourcePrefix must be set. SourcePrefix is a
// wildcard rule used by wide_assets parsers; it is validated here but
// expanded by the model parser layer.
type Column struct {
	// Source names a raw column copied into the canonical field.
	Source string `toml:"source"`
	// Constant provides a literal canonical value.
	Constant string `toml:"constant"`
	// SourcePrefix names a raw-column prefix used for wide asset
	// expansion, e.g. "equity_" for equity_btc/equity_eth.
	SourcePrefix string `toml:"source_prefix"`
	// Type optionally repeats the target canonical field type. When set
	// it must match the standard schema field type exactly.
	Type snapshotschema.FieldType `toml:"type"`
	// DecimalScale multiplies a raw decimal string into a canonical
	// integer string. Zero means no decimal scaling.
	DecimalScale int64 `toml:"decimal_scale"`
	// Required overrides the standard schema field's required flag for
	// mapping diagnostics only. It cannot make a schema-required field
	// optional.
	Required *bool `toml:"required"`
}

// Validate checks that cfg can map into the given standard schema. It
// validates file names, required canonical fields, mapping rule shape,
// declared field types, and CSV dialect options. It does not inspect
// customer raw files.
func Validate(schema snapshotschema.Schema, cfg Config) error {
	if err := snapshotschema.Validate(schema); err != nil {
		return err
	}
	if _, err := BuildCSVOptions(cfg.Format); err != nil {
		return err
	}
	filesByName := make(map[string]snapshotschema.File, len(schema.Files))
	for _, file := range schema.Files {
		filesByName[file.Name] = file
	}
	seenFiles := map[string]struct{}{}
	for i, fileCfg := range cfg.Files {
		if fileCfg.Name == "" {
			return fmt.Errorf("mapping files[%d].name is empty", i)
		}
		schemaFile, ok := filesByName[fileCfg.Name]
		if !ok {
			return fmt.Errorf("mapping files[%d].name %q is not in schema %s", i, fileCfg.Name, schema.ModelID)
		}
		if _, dup := seenFiles[fileCfg.Name]; dup {
			return fmt.Errorf("mapping files[%d] duplicates standard file %q", i, fileCfg.Name)
		}
		seenFiles[fileCfg.Name] = struct{}{}
		if fileCfg.Source == "" {
			return fmt.Errorf("mapping file %q source is empty", fileCfg.Name)
		}
		mode := fileCfg.Mode
		if mode == "" {
			mode = ModeDirect
		}
		if mode != ModeDirect && mode != ModeWideAssets {
			return fmt.Errorf("mapping file %q mode %q is invalid", fileCfg.Name, fileCfg.Mode)
		}
		if len(fileCfg.Columns) == 0 {
			return fmt.Errorf("mapping file %q has no columns", fileCfg.Name)
		}
		if err := validateColumns(schemaFile, fileCfg); err != nil {
			return err
		}
	}
	return nil
}

// Mapping modes.
const (
	// ModeDirect maps one raw row to one canonical row.
	ModeDirect = "direct"
	// ModeWideAssets allows model parsers to expand asset-prefixed raw
	// columns into multiple canonical account-asset rows.
	ModeWideAssets = "wide_assets"
)

// BuildCSVOptions converts mapping format settings into the core CSV
// reader options used by R9-B primitives.
func BuildCSVOptions(format Format) (snapshotcsv.Options, error) {
	opts := snapshotcsv.DefaultOptions()
	if format.Delimiter != "" {
		r, err := singleRune("snapshot.format.delimiter", format.Delimiter)
		if err != nil {
			return snapshotcsv.Options{}, err
		}
		opts.Comma = r
	}
	if format.Comment != "" {
		r, err := singleRune("snapshot.format.comment", format.Comment)
		if err != nil {
			return snapshotcsv.Options{}, err
		}
		opts.Comment = r
	}
	if format.TrimSpace != nil {
		opts.TrimSpace = *format.TrimSpace
	}
	opts.AllowUnknownColumns = format.AllowUnknownColumns
	opts.NullValues = append([]string(nil), format.NullValues...)
	return opts, nil
}

func validateColumns(schemaFile snapshotschema.File, fileCfg File) error {
	fields := make(map[string]snapshotschema.Field, len(schemaFile.Fields))
	for _, field := range schemaFile.Fields {
		fields[field.Name] = field
	}
	for name, rule := range fileCfg.Columns {
		field, ok := fields[name]
		if !ok {
			return fmt.Errorf("mapping file %q column %q is not in standard schema", fileCfg.Name, name)
		}
		if err := validateColumnRule(fileCfg.Name, name, field, rule); err != nil {
			return err
		}
	}
	for _, field := range schemaFile.Fields {
		if field.Required {
			rule, ok := fileCfg.Columns[field.Name]
			if !ok {
				return fmt.Errorf("mapping file %q missing required canonical field %q", fileCfg.Name, field.Name)
			}
			if rule.Required != nil && !*rule.Required {
				return fmt.Errorf("mapping file %q cannot mark required schema field %q optional", fileCfg.Name, field.Name)
			}
		}
	}
	return nil
}

func validateColumnRule(fileName, fieldName string, field snapshotschema.Field, rule Column) error {
	var sources int
	if rule.Source != "" {
		sources++
	}
	if rule.Constant != "" {
		sources++
	}
	if rule.SourcePrefix != "" {
		sources++
	}
	if sources != 1 {
		return fmt.Errorf("mapping file %q column %q must set exactly one of source, constant, source_prefix", fileName, fieldName)
	}
	if rule.Type != "" && rule.Type != field.Type {
		return fmt.Errorf("mapping file %q column %q type %q does not match schema type %q", fileName, fieldName, rule.Type, field.Type)
	}
	if rule.DecimalScale < 0 {
		return fmt.Errorf("mapping file %q column %q decimal_scale must be >= 0", fileName, fieldName)
	}
	if rule.DecimalScale > 0 && !decimalScalable(field.Type) {
		return fmt.Errorf("mapping file %q column %q type %q cannot use decimal_scale", fileName, fieldName, field.Type)
	}
	return nil
}

func decimalScalable(t snapshotschema.FieldType) bool {
	switch t {
	case snapshotschema.FieldUint8, snapshotschema.FieldUint16, snapshotschema.FieldUint32, snapshotschema.FieldUint64, snapshotschema.FieldBigInt:
		return true
	default:
		return false
	}
}

func singleRune(name, value string) (rune, error) {
	runes := []rune(value)
	if len(runes) != 1 {
		return 0, fmt.Errorf("%s must be exactly one rune, got %q", name, value)
	}
	r := runes[0]
	if strings.ContainsRune("\r\n", r) {
		return 0, fmt.Errorf("%s cannot be a newline", name)
	}
	return r, nil
}
