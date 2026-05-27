package t2_static_haircut_margin_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	t2snapshot "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t2_static_haircut_margin"
	t2host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t2_static_haircut_margin/host"
)

const accountID0 = "1111111111111111111111111111111111111111111111111111111111111111"

func TestStandardCSVSnapshotT2(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cex_assets.csv"), `asset_index,symbol,total_equity,total_debt,base_price,collateral,haircut_bp
0,btc,100,0,60000,100,10000
`)
	writeFile(t, filepath.Join(dir, "accounts.csv"), `account_id,asset_index,equity,debt,collateral
`+accountID0+`,0,100,0,100
`)
	src := t2snapshot.NewSnapshotCSV(t2snapshot.Config{Dir: dir, SnapshotID: "snap", AssetCapacity: 2})
	assets, err := src.CexAssets(context.Background())
	if err != nil {
		t.Fatalf("CexAssets: %v", err)
	}
	if len(assets) != 2 || assets[1].Symbol != "reserved" {
		t.Fatalf("assets = %+v", assets)
	}
	stream, err := src.AccountStream(context.Background())
	if err != nil {
		t.Fatalf("AccountStream: %v", err)
	}
	var accounts int
	for range stream {
		accounts++
	}
	if accounts != 1 {
		t.Fatalf("accounts = %d, want 1", accounts)
	}
}

func TestStandardCSVSnapshotT2Registered(t *testing.T) {
	if !contains(t2host.RegisteredSnapshotConnectors(), t2snapshot.ConnectorID) {
		t.Fatalf("registered connectors missing %q", t2snapshot.ConnectorID)
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
