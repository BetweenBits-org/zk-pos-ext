package csv_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	snapshotcsv "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/csv"
	t1schema "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/t1_simple_margin"
	t4schema "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/t4_tiered_haircut_margin_3pool"
)

const accountID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestReaderValidatesCanonicalRows(t *testing.T) {
	file := t1schema.StandardSchema.Files[0]
	input := strings.NewReader(
		" account_index, account_id, asset_index, equity, debt\n" +
			" 0, " + accountID + ", 1, 42, 7\n",
	)
	reader, err := snapshotcsv.NewReader(input, file, snapshotcsv.DefaultOptions())
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	row, err := reader.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if row.RecordNumber != 2 {
		t.Fatalf("RecordNumber = %d, want 2", row.RecordNumber)
	}
	equity, err := row.Uint64("equity", 64)
	if err != nil {
		t.Fatalf("Uint64(equity): %v", err)
	}
	if equity != 42 {
		t.Fatalf("equity = %d, want 42", equity)
	}
	if _, err := reader.Read(); !errors.Is(err, io.EOF) {
		t.Fatalf("second Read err = %v, want EOF", err)
	}
}

func TestParseHeaderRejectsUnknownAndMissingColumns(t *testing.T) {
	file := t1schema.StandardSchema.Files[0]
	_, err := snapshotcsv.ParseHeader(file, []string{"account_id", "asset_index", "equity", "debt", "extra"}, snapshotcsv.DefaultOptions())
	if err == nil || !strings.Contains(err.Error(), "unknown header column") {
		t.Fatalf("unknown column err = %v", err)
	}
	_, err = snapshotcsv.ParseHeader(file, []string{"account_id", "asset_index", "equity"}, snapshotcsv.DefaultOptions())
	if err == nil || !strings.Contains(err.Error(), "missing required header column") {
		t.Fatalf("missing column err = %v", err)
	}
}

func TestReaderAllowsOptionalAccountIndexOmission(t *testing.T) {
	file := t1schema.StandardSchema.Files[0]
	input := strings.NewReader(
		"account_id,asset_index,equity,debt\n" +
			accountID + ",1,42,0\n",
	)
	reader, err := snapshotcsv.NewReader(input, file, snapshotcsv.DefaultOptions())
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	row, err := reader.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if _, ok := row.Value("account_index"); ok {
		t.Fatalf("account_index present, want omitted")
	}
}

func TestReaderRejectsScalarTypeViolations(t *testing.T) {
	file := t1schema.StandardSchema.Files[0]
	tests := []struct {
		name string
		row  string
		want string
	}{
		{
			name: "bad account id",
			row:  "0,not_hex,1,42,0\n",
			want: "account id must be 64 hex chars",
		},
		{
			name: "uint16 overflow",
			row:  "0," + accountID + ",65536,42,0\n",
			want: "invalid uint16",
		},
		{
			name: "negative uint",
			row:  "0," + accountID + ",1,-42,0\n",
			want: "invalid unsigned integer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader("account_index,account_id,asset_index,equity,debt\n" + tt.row)
			reader, err := snapshotcsv.NewReader(input, file, snapshotcsv.DefaultOptions())
			if err != nil {
				t.Fatalf("NewReader: %v", err)
			}
			_, err = reader.Read()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Read err = %v, want contains %q", err, tt.want)
			}
			if !errors.Is(err, snapshotcsv.ErrInvalidRow) {
				t.Fatalf("Read err = %v, want ErrInvalidRow classification", err)
			}
		})
	}
}

func TestReaderRejectsDuplicatePrimaryKey(t *testing.T) {
	file := t1schema.StandardSchema.Files[0]
	input := strings.NewReader(
		"account_id,asset_index,equity,debt\n" +
			accountID + ",1,42,0\n" +
			accountID + ",1,43,0\n",
	)
	reader, err := snapshotcsv.NewReader(input, file, snapshotcsv.DefaultOptions())
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	if _, err := reader.Read(); err != nil {
		t.Fatalf("first Read: %v", err)
	}
	if _, err := reader.Read(); err == nil || !strings.Contains(err.Error(), "duplicate primary key") {
		t.Fatalf("second Read err = %v, want duplicate primary key", err)
	} else if !errors.Is(err, snapshotcsv.ErrInvalidRow) {
		t.Fatalf("second Read err = %v, want ErrInvalidRow classification", err)
	}
}

func TestReaderValidatesBigIntTierRows(t *testing.T) {
	file := t4schema.StandardSchema.Files[2]
	input := strings.NewReader(
		"asset_index,collateral_pool,tier_index,boundary_value,ratio,precomputed_value\n" +
			"1,loan,0,100000000000000000000,95,0\n",
	)
	reader, err := snapshotcsv.NewReader(input, file, snapshotcsv.DefaultOptions())
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	row, err := reader.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	boundary, err := row.BigInt("boundary_value")
	if err != nil {
		t.Fatalf("BigInt(boundary_value): %v", err)
	}
	if boundary.String() != "100000000000000000000" {
		t.Fatalf("boundary = %s", boundary)
	}
}

func TestStreamHonorsContextAndReportsCleanEOF(t *testing.T) {
	file := t1schema.StandardSchema.Files[0]
	input := strings.NewReader(
		"account_id,asset_index,equity,debt\n" +
			accountID + ",1,42,0\n",
	)
	rows, errs := snapshotcsv.Stream(context.Background(), input, file, snapshotcsv.DefaultOptions())
	var count int
	for range rows {
		count++
	}
	if count != 1 {
		t.Fatalf("streamed rows = %d, want 1", count)
	}
	if err := <-errs; err != nil {
		t.Fatalf("stream err = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rows, errs = snapshotcsv.Stream(ctx, strings.NewReader("account_id,asset_index,equity,debt\n"), file, snapshotcsv.DefaultOptions())
	for range rows {
		t.Fatalf("unexpected row after cancellation")
	}
	if err := <-errs; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err = %v, want context.Canceled", err)
	}
}
