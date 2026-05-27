package t3_tiered_haircut_margin_1pool_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	t3snapshot "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t3_tiered_haircut_margin_1pool"
	t3host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t3_tiered_haircut_margin_1pool/host"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

const accountID0 = "1111111111111111111111111111111111111111111111111111111111111111"

func TestStandardCSVSnapshotT3(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cex_assets.csv"), `asset_index,symbol,total_equity,total_debt,base_price,collateral
0,btc,100,0,60000,100
`)
	writeFile(t, filepath.Join(dir, "tier_ratios.csv"), `asset_index,tier_index,boundary_value,ratio,precomputed_value
0,0,100000000,100,0
`)
	writeFile(t, filepath.Join(dir, "accounts.csv"), `account_id,asset_index,equity,debt,collateral
`+accountID0+`,0,100,0,100
`)
	src := t3snapshot.NewSnapshotCSV(t3snapshot.Config{Dir: dir, SnapshotID: "snap", AssetCapacity: 2})
	assets, err := src.CexAssets(context.Background())
	if err != nil {
		t.Fatalf("CexAssets: %v", err)
	}
	if len(assets) != 2 || len(assets[0].CollateralRatios) != corespec.TierCount {
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

func TestStandardCSVSnapshotT3Registered(t *testing.T) {
	if !contains(t3host.RegisteredSnapshotConnectors(), t3snapshot.ConnectorID) {
		t.Fatalf("registered connectors missing %q", t3snapshot.ConnectorID)
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
