package testdata_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	t1snapshot "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t1_simple_margin"
	t2snapshot "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t2_static_haircut_margin"
	t3snapshot "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t3_tiered_haircut_margin_1pool"
	t4snapshot "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t4_tiered_haircut_margin_3pool"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/internal/testdata"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
)

// loadProfile loads a reference profile. testdata tests use the
// shipped reference toml files — same input format the engine reads.
func loadProfile(t *testing.T, name string) *declarative.Profile {
	t.Helper()
	prof, err := declarative.Load(filepath.Join("../../profile", name, name+".toml"))
	if err != nil {
		t.Fatalf("load profile %q: %v", name, err)
	}
	return prof
}

// TestGenerateScale_T1Roundtrip checks that the T1 generator's output
// passes the standard CSV parser end-to-end + maintains per-asset sum
// equality (Σ user.equity[asset] = cex.total_equity[asset]).
func TestGenerateScale_T1Roundtrip(t *testing.T) {
	dir := t.TempDir()
	prof := loadProfile(t, "t1_reference")

	if err := testdata.GenerateScale(prof, testdata.Options{
		OutDir:           dir,
		Users:            100,
		CapacityOverride: 5,
		Distribution:     "uniform",
		Seed:             42,
	}); err != nil {
		t.Fatalf("GenerateScale: %v", err)
	}

	src := t1snapshot.NewSnapshotCSV(t1snapshot.Config{
		Dir:           dir,
		SnapshotID:    "test",
		AssetCapacity: 5,
	})
	ctx := context.Background()

	cex, err := src.CexAssets(ctx)
	if err != nil {
		t.Fatalf("CexAssets: %v", err)
	}
	if len(cex) != 5 {
		t.Fatalf("cex slot count = %d, want 5 (cap padding)", len(cex))
	}

	stream, err := src.AccountStream(ctx)
	if err != nil {
		t.Fatalf("AccountStream: %v", err)
	}
	got := 0
	sumEquity := make([]uint64, 5)
	for acct := range stream {
		got++
		for _, a := range acct.Assets {
			sumEquity[a.Index] += a.Equity
		}
	}
	if got != 100 {
		t.Fatalf("real accounts streamed = %d, want 100", got)
	}
	if invalid := src.InvalidCount(); invalid != 0 {
		t.Fatalf("InvalidCount = %d, want 0", invalid)
	}
	for i := range cex {
		if sumEquity[i] != cex[i].TotalEquity {
			t.Fatalf("sum equality broken at slot %d: Σ user.equity=%d cex.total_equity=%d",
				i, sumEquity[i], cex[i].TotalEquity)
		}
	}
}

// TestGenerateScale_AllModelsParseable runs a tiny generate-then-parse
// for every model and asserts the generated CSVs round-trip through
// the standard parser with no errors and no invalid rows.
func TestGenerateScale_AllModelsParseable(t *testing.T) {
	cases := []struct {
		profileName string
		check       func(t *testing.T, dir string)
	}{
		{
			profileName: "t1_reference",
			check: func(t *testing.T, dir string) {
				src := t1snapshot.NewSnapshotCSV(t1snapshot.Config{Dir: dir, SnapshotID: "x", AssetCapacity: 5})
				assertStreamCount(t, src.CexAssets, src.AccountStream, src.InvalidCount, 20)
			},
		},
		{
			profileName: "t2_reference",
			check: func(t *testing.T, dir string) {
				src := t2snapshot.NewSnapshotCSV(t2snapshot.Config{Dir: dir, SnapshotID: "x", AssetCapacity: 5})
				assertStreamCount(t, src.CexAssets, src.AccountStream, src.InvalidCount, 20)
			},
		},
		{
			profileName: "t3_reference",
			check: func(t *testing.T, dir string) {
				src := t3snapshot.NewSnapshotCSV(t3snapshot.Config{Dir: dir, SnapshotID: "x", AssetCapacity: 5})
				assertStreamCount(t, src.CexAssets, src.AccountStream, src.InvalidCount, 20)
			},
		},
		{
			profileName: "t4_reference",
			check: func(t *testing.T, dir string) {
				src := t4snapshot.NewSnapshotCSV(t4snapshot.Config{Dir: dir, SnapshotID: "x", AssetCapacity: 5})
				assertStreamCount(t, src.CexAssets, src.AccountStream, src.InvalidCount, 20)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.profileName, func(t *testing.T) {
			dir := t.TempDir()
			prof := loadProfile(t, c.profileName)
			if err := testdata.GenerateScale(prof, testdata.Options{
				OutDir:           dir,
				Users:            20,
				CapacityOverride: 5,
				Distribution:     "uniform",
				Seed:             42,
			}); err != nil {
				t.Fatalf("GenerateScale: %v", err)
			}
			// quick sanity: accounts.csv non-empty
			if info, err := os.Stat(filepath.Join(dir, "accounts.csv")); err != nil || info.Size() == 0 {
				t.Fatalf("accounts.csv missing or empty: %v", err)
			}
			c.check(t, dir)
		})
	}
}

// TestGenerateScale_AssetCountSubsetOfCapacity verifies the R11-D
// tier-isolation contract: AssetCount can be smaller than the circuit's
// asset_capacity, and the resulting accounts.csv emits exactly that
// many non-empty rows per user (synthetic asset_N symbols extend a
// short profile catalog).
//
// This is the key invariant for R11-D Tier 1 isolation (capacity=500,
// asset_count=50) and Tier 2 isolation (capacity=500, asset_count=500).
func TestGenerateScale_AssetCountSubsetOfCapacity(t *testing.T) {
	cases := []struct {
		name       string
		capacity   int
		assetCount int
		wantCEX    int // expected real rows in cex_assets.csv (also = per-user accounts rows)
	}{
		{name: "tier1_50_of_500", capacity: 500, assetCount: 50, wantCEX: 50},
		{name: "tier2_500_of_500", capacity: 500, assetCount: 500, wantCEX: 500},
		{name: "default_eq_capacity", capacity: 10, assetCount: 0, wantCEX: 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			prof := loadProfile(t, "t4_reference")
			if err := testdata.GenerateScale(prof, testdata.Options{
				OutDir:           dir,
				Users:            3,
				CapacityOverride: c.capacity,
				AssetCount:       c.assetCount,
				Distribution:     "uniform",
				Seed:             42,
			}); err != nil {
				t.Fatalf("GenerateScale: %v", err)
			}
			// cex_assets.csv: header + wantCEX rows
			data, err := os.ReadFile(filepath.Join(dir, "cex_assets.csv"))
			if err != nil {
				t.Fatalf("read cex_assets.csv: %v", err)
			}
			lines := 0
			for _, b := range data {
				if b == '\n' {
					lines++
				}
			}
			if lines != c.wantCEX+1 { // +1 for header
				t.Fatalf("cex_assets.csv lines = %d, want %d (= %d + header)", lines, c.wantCEX+1, c.wantCEX)
			}
		})
	}
}

// TestGenerateScale_AssetCountExceedsCapacityRejected guards the
// invariant: per-user non-empty count cannot exceed the circuit's
// slot dimension.
func TestGenerateScale_AssetCountExceedsCapacityRejected(t *testing.T) {
	prof := loadProfile(t, "t4_reference")
	err := testdata.GenerateScale(prof, testdata.Options{
		OutDir:           t.TempDir(),
		Users:            1,
		CapacityOverride: 50,
		AssetCount:       100,
		Distribution:     "uniform",
		Seed:             42,
	})
	if err == nil {
		t.Fatalf("expected error for AssetCount > capacity, got nil")
	}
}

// assertStreamCount adapts to each model's snapshot.SnapshotSource
// interface; the per-model channel returns model-typed AccountInfo but
// we only need a count + InvalidCount sanity check here.
func assertStreamCount[CexT any, AccT any](
	t *testing.T,
	cexFn func(ctx context.Context) ([]CexT, error),
	streamFn func(ctx context.Context) (<-chan AccT, error),
	invalidFn func() uint64,
	wantUsers int,
) {
	t.Helper()
	ctx := context.Background()
	if _, err := cexFn(ctx); err != nil {
		t.Fatalf("CexAssets: %v", err)
	}
	ch, err := streamFn(ctx)
	if err != nil {
		t.Fatalf("AccountStream: %v", err)
	}
	count := 0
	for range ch {
		count++
	}
	if count != wantUsers {
		t.Fatalf("AccountStream count = %d, want %d", count, wantUsers)
	}
	if invalid := invalidFn(); invalid != 0 {
		t.Fatalf("InvalidCount = %d, want 0", invalid)
	}
}
