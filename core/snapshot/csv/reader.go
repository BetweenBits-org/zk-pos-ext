// Package csv provides CSV parsing primitives for standard raw
// snapshot schemas.
//
// The package validates the canonical CSV layer produced after
// customer mapping: headers must match a model schema, scalar values
// must be type-valid, primary keys must be unique, and rows can be
// streamed with context cancellation. Model-specific aggregation into
// AccountInfo / CexAssetInfo is intentionally left to the R9 model
// parser layer.
package csv

import (
	"context"
	stdcsv "encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"

	snapshotschema "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/schema"
)

// ErrInvalidRow classifies row-level data violations in standard raw
// CSV input. Model parsers can use errors.Is(err, ErrInvalidRow) to
// route a bad row through the active invalid-account policy, while
// header and IO errors remain stream-fatal.
var ErrInvalidRow = errors.New("invalid standard snapshot csv row")

// RowError describes a row-level validation failure. It preserves the
// file, record, and optional field context needed for deterministic
// audit logs and invalid-row counters.
type RowError struct {
	// File is the canonical schema file name that rejected the row.
	File string
	// RecordNumber is the one-based CSV record number including the
	// header.
	RecordNumber int
	// Field is the canonical field name when the error is field-scoped.
	// Empty means the row as a whole failed validation.
	Field string
	// Cause is the underlying validation failure.
	Cause error
}

// Error formats a deterministic validation message for logs and tests.
func (e *RowError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("csv %s record %d: %v", e.File, e.RecordNumber, e.Cause)
	}
	return fmt.Sprintf("csv %s record %d field %q: %v", e.File, e.RecordNumber, e.Field, e.Cause)
}

// Unwrap returns the underlying validation failure.
func (e *RowError) Unwrap() error {
	return e.Cause
}

// Is reports ErrInvalidRow for every RowError so policy code can
// distinguish bad rows from stream-fatal errors.
func (e *RowError) Is(target error) bool {
	return target == ErrInvalidRow
}

// Options configures CSV dialect handling before schema validation.
// Defaults are strict comma-separated CSV with quoted fields enabled by
// encoding/csv. TrimSpace is enabled by DefaultOptions because mapping
// output should not depend on incidental whitespace around scalar
// values.
type Options struct {
	// Comma is the field delimiter. Zero means comma.
	Comma rune
	// Comment skips lines beginning with this rune, matching
	// encoding/csv.Reader.Comment. Zero disables comments.
	Comment rune
	// TrimSpace trims leading and trailing spaces around every header
	// and row value before schema validation.
	TrimSpace bool
	// AllowUnknownColumns permits columns not declared by the schema.
	// Unknown columns are ignored in Row.Values. The default rejects
	// them so mapped CSV stays audit-friendly.
	AllowUnknownColumns bool
	// NullValues lists string values that should be treated as empty.
	// Required fields still reject them; optional fields are omitted
	// from Row.Values.
	NullValues []string
}

// DefaultOptions returns the canonical CSV dialect for standard raw
// snapshot files.
func DefaultOptions() Options {
	return Options{TrimSpace: true}
}

// Header is a validated CSV header bound to one schema file. It maps
// canonical field names to CSV column positions and field metadata.
type Header struct {
	file      snapshotschema.File
	columns   []column
	fields    map[string]snapshotschema.Field
	positions map[string]int
}

type column struct {
	name  string
	field snapshotschema.Field
	known bool
}

// File returns the schema file this header was validated against.
func (h Header) File() snapshotschema.File {
	return h.file
}

// Has reports whether the canonical field appears in the CSV header.
func (h Header) Has(name string) bool {
	_, ok := h.positions[name]
	return ok
}

// Position returns the zero-based CSV column position for a canonical
// field, or false when the field is not present in the header.
func (h Header) Position(name string) (int, bool) {
	pos, ok := h.positions[name]
	return pos, ok
}

// ParseHeader validates a raw CSV header against a standard schema
// file. Required schema fields must be present, duplicate columns are
// rejected, and undeclared columns are rejected unless
// Options.AllowUnknownColumns is set.
func ParseHeader(file snapshotschema.File, raw []string, opts Options) (Header, error) {
	fieldByName := make(map[string]snapshotschema.Field, len(file.Fields))
	for _, field := range file.Fields {
		fieldByName[field.Name] = field
	}

	columns := make([]column, len(raw))
	positions := map[string]int{}
	for i, name := range raw {
		name = normalize(name, opts)
		if name == "" {
			return Header{}, fmt.Errorf("csv %s header column %d is empty", file.Name, i)
		}
		if _, exists := positions[name]; exists {
			return Header{}, fmt.Errorf("csv %s duplicate header column %q", file.Name, name)
		}
		field, ok := fieldByName[name]
		if !ok && !opts.AllowUnknownColumns {
			return Header{}, fmt.Errorf("csv %s unknown header column %q", file.Name, name)
		}
		positions[name] = i
		columns[i] = column{name: name, field: field, known: ok}
	}

	for _, field := range file.Fields {
		if field.Required {
			if _, ok := positions[field.Name]; !ok {
				return Header{}, fmt.Errorf("csv %s missing required header column %q", file.Name, field.Name)
			}
		}
	}
	return Header{file: file, columns: columns, fields: fieldByName, positions: positions}, nil
}

// Row is one validated canonical CSV record. Values contains only
// schema-declared fields that were present and non-empty after null
// handling. Unknown columns are never exposed.
type Row struct {
	// File is the schema file that validated the row.
	File snapshotschema.File
	// RecordNumber is the one-based CSV record number including the
	// header. The first data row is record 2.
	RecordNumber int
	// Values maps canonical field names to normalized string values.
	// Typed parser helpers validate and convert these strings without
	// changing canonical ordering.
	Values map[string]string
}

// Value returns a normalized field value and whether it was present.
func (r Row) Value(name string) (string, bool) {
	v, ok := r.Values[name]
	return v, ok
}

// Required returns the normalized field value or a row-scoped error
// when the value is absent.
func (r Row) Required(name string) (string, error) {
	v, ok := r.Value(name)
	if !ok {
		return "", rowError(r.File.Name, r.RecordNumber, name, fmt.Errorf("missing field"))
	}
	return v, nil
}

// Uint64 parses a non-negative decimal integer from the row using the
// provided bit size. It is useful for model parsers that need one
// helper for uint8, uint16, uint32, and uint64 fields.
func (r Row) Uint64(name string, bitSize int) (uint64, error) {
	v, err := r.Required(name)
	if err != nil {
		return 0, err
	}
	n, err := parseUint(v, bitSize)
	if err != nil {
		return 0, rowError(r.File.Name, r.RecordNumber, name, err)
	}
	return n, nil
}

// BigInt parses a non-negative base-10 integer from the row.
func (r Row) BigInt(name string) (*big.Int, error) {
	v, err := r.Required(name)
	if err != nil {
		return nil, err
	}
	n, ok := new(big.Int).SetString(v, 10)
	if !ok || n.Sign() < 0 {
		return nil, rowError(r.File.Name, r.RecordNumber, name, fmt.Errorf("invalid non-negative bigint %q", v))
	}
	return n, nil
}

// Reader streams validated rows from one schema-bound CSV file. It
// keeps primary-key state so duplicate canonical rows are rejected
// before model parsers build account or asset commitments.
type Reader struct {
	r      *stdcsv.Reader
	header Header
	opts   Options
	record int
	seenPK map[string]struct{}
}

// NewReader creates a schema-bound CSV reader and validates the first
// record as the header. Row records start at RecordNumber 2.
func NewReader(src io.Reader, file snapshotschema.File, opts Options) (*Reader, error) {
	r := stdcsv.NewReader(src)
	if opts.Comma != 0 {
		r.Comma = opts.Comma
	}
	if opts.Comment != 0 {
		r.Comment = opts.Comment
	}
	r.TrimLeadingSpace = opts.TrimSpace

	rawHeader, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("csv %s read header: %w", file.Name, err)
	}
	header, err := ParseHeader(file, rawHeader, opts)
	if err != nil {
		return nil, err
	}
	return &Reader{
		r:      r,
		header: header,
		opts:   opts,
		record: 1,
		seenPK: map[string]struct{}{},
	}, nil
}

// Header returns the validated header metadata for this reader.
func (r *Reader) Header() Header {
	return r.header
}

// Read returns the next validated row. It returns io.EOF after the
// last row. Validation includes required value presence, scalar type
// checks, and primary-key duplicate detection.
func (r *Reader) Read() (Row, error) {
	record, err := r.r.Read()
	if err != nil {
		return Row{}, err
	}
	r.record++
	row, err := buildRow(r.header, record, r.record, r.opts)
	if err != nil {
		return Row{}, err
	}
	if err := r.checkPrimaryKey(row); err != nil {
		return Row{}, err
	}
	return row, nil
}

// Stream reads rows in a goroutine and sends either all validated rows
// or the first fatal error. The error channel receives nil on clean EOF.
func Stream(ctx context.Context, src io.Reader, file snapshotschema.File, opts Options) (<-chan Row, <-chan error) {
	rows := make(chan Row)
	errs := make(chan error, 1)
	go func() {
		defer close(rows)
		reader, err := NewReader(src, file, opts)
		if err != nil {
			errs <- err
			return
		}
		for {
			if err := ctx.Err(); err != nil {
				errs <- err
				return
			}
			row, err := reader.Read()
			if errors.Is(err, io.EOF) {
				errs <- nil
				return
			}
			if err != nil {
				errs <- err
				return
			}
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			case rows <- row:
			}
		}
	}()
	return rows, errs
}

func buildRow(header Header, record []string, recordNumber int, opts Options) (Row, error) {
	if len(record) != len(header.columns) {
		return Row{}, rowError(
			header.file.Name,
			recordNumber,
			"",
			fmt.Errorf("has %d columns, want %d", len(record), len(header.columns)),
		)
	}
	values := map[string]string{}
	for i, col := range header.columns {
		if !col.known {
			continue
		}
		value := normalize(record[i], opts)
		if isNull(value, opts) {
			value = ""
		}
		if value == "" {
			if col.field.Required {
				return Row{}, rowError(header.file.Name, recordNumber, col.name, fmt.Errorf("is required"))
			}
			continue
		}
		if err := validateScalar(col.field, value); err != nil {
			return Row{}, rowError(header.file.Name, recordNumber, col.name, err)
		}
		values[col.name] = value
	}
	for _, field := range header.file.Fields {
		if field.Required {
			if _, ok := values[field.Name]; !ok {
				return Row{}, rowError(header.file.Name, recordNumber, field.Name, fmt.Errorf("is required"))
			}
		}
	}
	return Row{File: header.file, RecordNumber: recordNumber, Values: values}, nil
}

func (r *Reader) checkPrimaryKey(row Row) error {
	if len(r.header.file.PrimaryKey) == 0 {
		return nil
	}
	parts := make([]string, len(r.header.file.PrimaryKey))
	for i, key := range r.header.file.PrimaryKey {
		value, ok := row.Value(key)
		if !ok {
			return rowError(r.header.file.Name, row.RecordNumber, key, fmt.Errorf("primary key field is missing"))
		}
		parts[i] = value
	}
	joined := strings.Join(parts, "\x00")
	if _, exists := r.seenPK[joined]; exists {
		return rowError(r.header.file.Name, row.RecordNumber, "", fmt.Errorf("duplicate primary key %v", r.header.file.PrimaryKey))
	}
	r.seenPK[joined] = struct{}{}
	return nil
}

func rowError(file string, recordNumber int, field string, cause error) error {
	return &RowError{
		File:         file,
		RecordNumber: recordNumber,
		Field:        field,
		Cause:        cause,
	}
}

func validateScalar(field snapshotschema.Field, value string) error {
	switch field.Type {
	case snapshotschema.FieldUint8:
		_, err := parseUint(value, 8)
		return err
	case snapshotschema.FieldUint16:
		_, err := parseUint(value, 16)
		return err
	case snapshotschema.FieldUint32:
		_, err := parseUint(value, 32)
		return err
	case snapshotschema.FieldUint64:
		_, err := parseUint(value, 64)
		return err
	case snapshotschema.FieldBigInt:
		n, ok := new(big.Int).SetString(value, 10)
		if !ok || n.Sign() < 0 {
			return fmt.Errorf("invalid non-negative bigint %q", value)
		}
		return nil
	case snapshotschema.FieldString, snapshotschema.FieldEnum:
		return nil
	case snapshotschema.FieldAccountID:
		if len(value) != 64 {
			return fmt.Errorf("account id must be 64 hex chars, got %d", len(value))
		}
		for _, c := range value {
			if !isHex(c) {
				return fmt.Errorf("account id contains non-hex character %q", c)
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported field type %q", field.Type)
	}
}

func parseUint(value string, bitSize int) (uint64, error) {
	if strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		return 0, fmt.Errorf("invalid unsigned integer %q", value)
	}
	n, err := strconv.ParseUint(value, 10, bitSize)
	if err != nil {
		return 0, fmt.Errorf("invalid uint%d %q", bitSize, value)
	}
	return n, nil
}

func normalize(value string, opts Options) string {
	if opts.TrimSpace {
		return strings.TrimSpace(value)
	}
	return value
}

func isNull(value string, opts Options) bool {
	for _, nullValue := range opts.NullValues {
		if value == normalize(nullValue, opts) {
			return true
		}
	}
	return false
}

func isHex(r rune) bool {
	return ('0' <= r && r <= '9') ||
		('a' <= r && r <= 'f') ||
		('A' <= r && r <= 'F')
}
