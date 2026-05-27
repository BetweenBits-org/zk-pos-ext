package t1_simple_margin_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	t1snapshot "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t1_simple_margin"
	t1host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/host"
)

const accountID0 = "1111111111111111111111111111111111111111111111111111111111111111"

func TestStandardCSVSnapshotT1(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cex_assets.csv"), `asset_index,symbol,total_equity,total_debt,base_price
0,btc,150,0,60000
1,eth,20,0,3000
`)
	writeFile(t, filepath.Join(dir, "accounts.csv"), `account_id,asset_index,equity,debt
`+accountID0+`,0,100,0
`+accountID0+`,1,20,0
2222222222222222222222222222222222222222222222222222222222222222,0,50,0
`)

	src := t1snapshot.NewSnapshotCSV(t1snapshot.Config{
		Dir:           dir,
		SnapshotID:    "snap",
		AssetCapacity: 3,
	})
	assets, err := src.CexAssets(context.Background())
	if err != nil {
		t.Fatalf("CexAssets: %v", err)
	}
	if len(assets) != 3 || assets[2].Symbol != "reserved" {
		t.Fatalf("assets = %+v", assets)
	}
	stream, err := src.AccountStream(context.Background())
	if err != nil {
		t.Fatalf("AccountStream: %v", err)
	}
	var accounts int
	for account := range stream {
		accounts++
		if len(account.AccountID) != 32 {
			t.Fatalf("AccountID length = %d", len(account.AccountID))
		}
	}
	if accounts != 2 {
		t.Fatalf("accounts = %d, want 2", accounts)
	}
	if got := src.InvalidCount(); got != 0 {
		t.Fatalf("InvalidCount = %d", got)
	}
}

func TestStandardCSVSnapshotT1Registered(t *testing.T) {
	if !contains(t1host.RegisteredSnapshotConnectors(), t1snapshot.ConnectorID) {
		t.Fatalf("registered connectors missing %q", t1snapshot.ConnectorID)
	}
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
