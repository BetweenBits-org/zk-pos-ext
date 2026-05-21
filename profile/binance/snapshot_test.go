package binance

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	modelspec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/spec"
)

const happyFixtureDir = "testdata/happy"

func TestCexAssets_HappyPath(t *testing.T) {
	src := NewSnapshotCSV(SnapshotConfig{
		UserDataDir: happyFixtureDir,
		SnapshotID:  "test",
	})
	assets, err := src.CexAssets(context.Background())
	if err != nil {
		t.Fatalf("CexAssets: unexpected error: %v", err)
	}
	if len(assets) != corespec.AssetCounts {
		t.Fatalf("len(assets) = %d, want %d", len(assets), corespec.AssetCounts)
	}

	// Real assets in user-CSV-header order: btc, eth, doge.
	want := []struct {
		sym   string
		index uint32
	}{
		{"btc", 0}, {"eth", 1}, {"doge", 2},
	}
	for _, w := range want {
		a := assets[w.index]
		if a.Symbol != w.sym {
			t.Errorf("assets[%d].Symbol = %q, want %q", w.index, a.Symbol, w.sym)
		}
		if a.Index != w.index {
			t.Errorf("assets[%d].Index = %d, want %d", w.index, a.Index, w.index)
		}
		if len(a.LoanRatios) != corespec.TierCount {
			t.Errorf("assets[%d].LoanRatios len = %d, want %d",
				w.index, len(a.LoanRatios), corespec.TierCount)
		}
		if len(a.MarginRatios) != corespec.TierCount {
			t.Errorf("assets[%d].MarginRatios len = %d, want %d",
				w.index, len(a.MarginRatios), corespec.TierCount)
		}
		if len(a.PortfolioMarginRatios) != corespec.TierCount {
			t.Errorf("assets[%d].PortfolioMarginRatios len = %d, want %d",
				w.index, len(a.PortfolioMarginRatios), corespec.TierCount)
		}
	}

	// btc loan tier 0: boundary hi=100, scaled by ValueScale (1e16); ratio 90.
	wantBoundary := new(big.Int).Mul(big.NewInt(100), big.NewInt(corespec.DefaultValueScale))
	if got := assets[0].LoanRatios[0].BoundaryValue; got.Cmp(wantBoundary) != 0 {
		t.Errorf("btc loan tier 0 boundary = %s, want %s", got, wantBoundary)
	}
	if got := assets[0].LoanRatios[0].Ratio; got != 90 {
		t.Errorf("btc loan tier 0 ratio = %d, want 90", got)
	}

	// Reserved padding from index 3 through AssetCounts-1.
	for i := 3; i < corespec.AssetCounts; i++ {
		if assets[i].Symbol != "reserved" {
			t.Fatalf("assets[%d].Symbol = %q, want %q", i, assets[i].Symbol, "reserved")
		}
		if assets[i].Index != uint32(i) {
			t.Fatalf("assets[%d].Index = %d, want %d", i, assets[i].Index, i)
		}
	}
}

// TestCexAssets_TwoDigitMultiplier verifies the per-symbol multiplier
// path: doge is in the two-digit set, so its BasePrice must use the
// 1e14 multiplier rather than the default 1e8. The two are off by a
// factor of 1e6, so a regression here is unmistakable.
func TestCexAssets_TwoDigitMultiplier(t *testing.T) {
	src := NewSnapshotCSV(SnapshotConfig{
		UserDataDir: happyFixtureDir,
		SnapshotID:  "test",
	})
	assets, err := src.CexAssets(context.Background())
	if err != nil {
		t.Fatalf("CexAssets: unexpected error: %v", err)
	}
	const wantDogePrice uint64 = 7_000_000_000_000 // 0.07 * 1e14
	if got := assets[2].BasePrice; got != wantDogePrice {
		t.Fatalf("doge BasePrice = %d, want %d (default-1e8 path would yield %d)",
			got, wantDogePrice, uint64(7_000_000))
	}
}

func TestCexAssets_MissingSymbol(t *testing.T) {
	// Keep the row count at 3 so the length-equality precheck passes;
	// substitute doge → foo so the user header's doge is unresolvable.
	tampered := `token,asset_usdt_price,collateral_vip_loan_ratio_tiers,collateral_margin_ratio_tiers,collateral_portfolio_margin_ratio_tiers
btc,60000.0,"[0-100:90]","[0-1000:85]","[0-500:95]"
eth,3000.0,"[0-1000:85]","[0-2000:80]",
foo,0.07,"[0-100000:50]","[0-50000:60]",
`
	dir := buildTamperedFixture(t, map[string]string{"cex_assets_info.csv": tampered})
	err := loadFixture(t, dir)
	if err == nil || !strings.Contains(err.Error(), "doge") {
		t.Fatalf("want error referencing missing doge, got: %v", err)
	}
}

func TestCexAssets_MalformedHeader(t *testing.T) {
	// Header column count fails (len-3) % 6 == 0.
	tampered := "rn,id,one,two,three,four,five\n"
	dir := buildTamperedFixture(t, map[string]string{"user_shard.csv": tampered})
	err := loadFixture(t, dir)
	if err == nil || !strings.Contains(err.Error(), "malformed header") {
		t.Fatalf("want malformed-header error, got: %v", err)
	}
}

func TestCexAssets_NonMonotonicBoundary(t *testing.T) {
	// Second tier hi (50) is not strictly greater than first (100).
	tampered := `token,asset_usdt_price,collateral_vip_loan_ratio_tiers,collateral_margin_ratio_tiers,collateral_portfolio_margin_ratio_tiers
btc,60000.0,"[0-100:90,0-50:80]","[0-1000:85]","[0-500:95]"
eth,3000.0,"[0-1000:85]","[0-2000:80]",
doge,0.07,"[0-100000:50]","[0-50000:60]",
`
	dir := buildTamperedFixture(t, map[string]string{"cex_assets_info.csv": tampered})
	err := loadFixture(t, dir)
	if err == nil || !strings.Contains(err.Error(), "strictly greater") {
		t.Fatalf("want non-monotonic boundary error, got: %v", err)
	}
}

// TestCexAssets_BoundaryOverflow checks the boundary-overflow guard.
// The maxTierBoundary cap (~3.32e35) sits above (uint64.Max · 1e16) ≈
// 1.84e35, so the in-tier uint64 conversion always trips first for any
// CSV-sourced value that would exceed the cap. Assert that path — it
// is the one actually reachable from the CSV ingest.
func TestCexAssets_BoundaryOverflow(t *testing.T) {
	tampered := `token,asset_usdt_price,collateral_vip_loan_ratio_tiers,collateral_margin_ratio_tiers,collateral_portfolio_margin_ratio_tiers
btc,60000.0,"[0-1000000000000000000000:90]","[0-1000:85]","[0-500:95]"
eth,3000.0,"[0-1000:85]","[0-2000:80]",
doge,0.07,"[0-100000:50]","[0-50000:60]",
`
	dir := buildTamperedFixture(t, map[string]string{"cex_assets_info.csv": tampered})
	err := loadFixture(t, dir)
	if err == nil || !strings.Contains(err.Error(), "overflows uint64") {
		t.Fatalf("want uint64-overflow error, got: %v", err)
	}
}

// buildTamperedFixture copies the happy-path fixture into a fresh
// TempDir, then overlays each (filename → body) entry. Returns the
// directory path.
func buildTamperedFixture(t *testing.T, overrides map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir(happyFixtureDir)
	if err != nil {
		t.Fatalf("read happy fixture: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		var body []byte
		if override, ok := overrides[name]; ok {
			body = []byte(override)
		} else {
			body, err = os.ReadFile(filepath.Join(happyFixtureDir, name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func loadFixture(t *testing.T, dir string) error {
	t.Helper()
	src := NewSnapshotCSV(SnapshotConfig{UserDataDir: dir, SnapshotID: "test"})
	_, err := src.CexAssets(context.Background())
	return err
}

// TestAccountStream_HappyPath is the R2/2 step 1 smoke test. The
// happy fixture has two data rows: a 3-asset row (btc + eth + doge,
// debt-free, no collateral) and a 1-asset row (doge only). We assert
// the channel yields exactly two AccountInfo records with sequential
// AccountIndex values, correctly decoded AccountID bytes, and the
// expected per-row Assets slice length. Heavier coverage (multi-shard,
// mid-stream parse errors, invalid-account classification) lands in
// R2/2 step 2 / step 3.
func TestAccountStream_HappyPath(t *testing.T) {
	src := NewSnapshotCSV(SnapshotConfig{
		UserDataDir: happyFixtureDir,
		SnapshotID:  "test",
	})
	ch, err := src.AccountStream(context.Background())
	if err != nil {
		t.Fatalf("AccountStream: unexpected start-up error: %v", err)
	}
	var got []modelspec.AccountInfo
	for a := range ch {
		got = append(got, a)
	}
	if len(got) != 2 {
		t.Fatalf("got %d accounts, want 2", len(got))
	}

	// Row 0: account id 0x11..11, all three assets populated with
	// equity > 0 and debt = 0 → Assets has 3 entries.
	if got[0].AccountIndex != 0 {
		t.Errorf("row 0 AccountIndex = %d, want 0", got[0].AccountIndex)
	}
	wantID0 := bytes.Repeat([]byte{0x11}, 32)
	if !bytes.Equal(got[0].AccountID, wantID0) {
		t.Errorf("row 0 AccountID = %x, want %x", got[0].AccountID, wantID0)
	}
	if len(got[0].Assets) != 3 {
		t.Errorf("row 0 Assets len = %d, want 3", len(got[0].Assets))
	}

	// Row 1: account id 0x22..22, only doge populated → Assets has 1
	// entry with Index = 2.
	if got[1].AccountIndex != 1 {
		t.Errorf("row 1 AccountIndex = %d, want 1", got[1].AccountIndex)
	}
	wantID1 := bytes.Repeat([]byte{0x22}, 32)
	if !bytes.Equal(got[1].AccountID, wantID1) {
		t.Errorf("row 1 AccountID = %x, want %x", got[1].AccountID, wantID1)
	}
	if len(got[1].Assets) != 1 {
		t.Fatalf("row 1 Assets len = %d, want 1", len(got[1].Assets))
	}
	if got[1].Assets[0].Index != 2 {
		t.Errorf("row 1 doge asset Index = %d, want 2", got[1].Assets[0].Index)
	}
	// doge equity 50.0 with two-digit balance multiplier 1e2.
	const wantDogeEquity uint64 = 50 * 100
	if got[1].Assets[0].Equity != wantDogeEquity {
		t.Errorf("row 1 doge equity = %d, want %d", got[1].Assets[0].Equity, wantDogeEquity)
	}
	// All zero collateral → TotalCollateral == 0.
	if got[1].TotalCollateral.Sign() != 0 {
		t.Errorf("row 1 TotalCollateral = %s, want 0", got[1].TotalCollateral)
	}
}
