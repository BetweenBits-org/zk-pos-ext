package sea_reference

import (
	"bytes"
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	modelspec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

const (
	happyFixtureDir   = "testdata/happy"
	testAssetCapacity = 5
)

func TestCexAssets_HappyPath(t *testing.T) {
	src := NewSnapshotCSV(SnapshotConfig{
		UserDataDir:   happyFixtureDir,
		SnapshotID:    "test",
		AssetCapacity: testAssetCapacity,
	})
	assets, err := src.CexAssets(context.Background())
	if err != nil {
		t.Fatalf("CexAssets: unexpected error: %v", err)
	}
	if len(assets) != testAssetCapacity {
		t.Fatalf("len(assets) = %d, want %d", len(assets), testAssetCapacity)
	}
	want := []struct {
		sym       string
		index     uint32
		basePrice uint64
	}{
		// usdt_price * DefaultPriceScale (1e8)
		{"btc", 0, 65000 * 100_000_000},
		{"eth", 1, 3500 * 100_000_000},
		{"usdt", 2, 1 * 100_000_000},
	}
	for _, w := range want {
		got := assets[w.index]
		if got.Symbol != w.sym {
			t.Errorf("assets[%d].Symbol = %q, want %q", w.index, got.Symbol, w.sym)
		}
		if got.BasePrice != w.basePrice {
			t.Errorf("assets[%d].BasePrice = %d, want %d", w.index, got.BasePrice, w.basePrice)
		}
		if got.Index != w.index {
			t.Errorf("assets[%d].Index = %d, want %d", w.index, got.Index, w.index)
		}
	}
	for i := 3; i < testAssetCapacity; i++ {
		if assets[i].Symbol != "reserved" {
			t.Errorf("assets[%d].Symbol = %q, want reserved", i, assets[i].Symbol)
		}
	}
}

func TestAccountStream_HappyPath(t *testing.T) {
	src := NewSnapshotCSV(SnapshotConfig{
		UserDataDir:   happyFixtureDir,
		SnapshotID:    "test",
		AssetCapacity: testAssetCapacity,
	})
	ch, err := src.AccountStream(context.Background())
	if err != nil {
		t.Fatalf("AccountStream: unexpected error: %v", err)
	}
	var accounts []modelspec.AccountInfo
	for a := range ch {
		accounts = append(accounts, a)
	}
	if len(accounts) != 2 {
		t.Fatalf("got %d accounts, want 2", len(accounts))
	}
	if accounts[0].AccountIndex != 0 || accounts[1].AccountIndex != 1 {
		t.Fatalf("AccountIndex sequence = [%d, %d], want [0, 1]",
			accounts[0].AccountIndex, accounts[1].AccountIndex)
	}
	// AccountID must be bn254-fr reduced (G13 invariant). Compute the
	// reduction of the raw 32-byte 0x11..11 / 0x22..22 patterns and
	// verify match.
	raw1, _ := hex.DecodeString(strings.Repeat("11", 32))
	want1 := new(fr.Element).SetBytes(raw1).Marshal()
	if !bytes.Equal(accounts[0].AccountID, want1) {
		t.Errorf("accounts[0].AccountID not fr-reduced: got %x", accounts[0].AccountID)
	}
	// Row 0: btc=1.0 (1e8 units), eth=10.0 (1e9 units), usdt=50000.0 (5e12 units). 3 non-empty.
	if len(accounts[0].Assets) != 3 {
		t.Errorf("row 0 Assets length = %d, want 3", len(accounts[0].Assets))
	}
	if accounts[0].Assets[0].Equity != 1*100_000_000 {
		t.Errorf("row 0 btc Equity = %d, want 1e8", accounts[0].Assets[0].Equity)
	}
	if src.InvalidCount() != 0 {
		t.Errorf("InvalidCount = %d, want 0", src.InvalidCount())
	}
}

func TestAccountStream_InvalidHex(t *testing.T) {
	dir := buildTamperedFixture(t, map[string]string{
		"user_shard.csv": "rn,id,btc,eth,usdt,sum\n" +
			"0,zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz,1.0,10.0,50000.0,0.0\n" +
			"1,2222222222222222222222222222222222222222222222222222222222222222,1.5,20.0,100000.0,0.0\n",
	})
	src := NewSnapshotCSV(SnapshotConfig{UserDataDir: dir, SnapshotID: "t", AssetCapacity: testAssetCapacity})
	ch, err := src.AccountStream(context.Background())
	if err != nil {
		t.Fatalf("AccountStream: %v", err)
	}
	var got []modelspec.AccountInfo
	for a := range ch {
		got = append(got, a)
	}
	if len(got) != 1 {
		t.Fatalf("got %d valid accounts, want 1 (invalid hex row should be skipped)", len(got))
	}
	if src.InvalidCount() != 1 {
		t.Errorf("InvalidCount = %d, want 1", src.InvalidCount())
	}
}

func TestAccountStream_BalanceOverflow(t *testing.T) {
	dir := buildTamperedFixture(t, map[string]string{
		"user_shard.csv": "rn,id,btc,eth,usdt,sum\n" +
			// 2e11 * 1e8 multiplier = 2e19 (above uint64.Max 1.844e19) → invalid
			"0,1111111111111111111111111111111111111111111111111111111111111111,200000000000.0,10.0,50000.0,0.0\n" +
			"1,2222222222222222222222222222222222222222222222222222222222222222,1.5,20.0,100000.0,0.0\n",
	})
	src := NewSnapshotCSV(SnapshotConfig{UserDataDir: dir, SnapshotID: "t", AssetCapacity: testAssetCapacity})
	ch, err := src.AccountStream(context.Background())
	if err != nil {
		t.Fatalf("AccountStream: %v", err)
	}
	var got []modelspec.AccountInfo
	for a := range ch {
		got = append(got, a)
	}
	if len(got) != 1 {
		t.Fatalf("got %d valid accounts, want 1 (overflow row should be skipped)", len(got))
	}
	if src.InvalidCount() != 1 {
		t.Errorf("InvalidCount = %d, want 1", src.InvalidCount())
	}
}

func TestCexAssets_MissingSymbol(t *testing.T) {
	dir := buildTamperedFixture(t, map[string]string{
		"cex_assets_info.csv": "symbol,usdt_price,total_equity\n" +
			"btc,65000.00,2.5\n" +
			"eth,3500.00,30.0\n" +
			// "usdt" missing — user header expects it
			"foo,1.00,150000.0\n",
	})
	src := NewSnapshotCSV(SnapshotConfig{UserDataDir: dir, SnapshotID: "t", AssetCapacity: testAssetCapacity})
	_, err := src.CexAssets(context.Background())
	if err == nil || !strings.Contains(err.Error(), "usdt") {
		t.Fatalf("expected missing-usdt error, got %v", err)
	}
}

// buildTamperedFixture copies the happy fixture to TempDir and
// overlays the supplied filename→body map.
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
