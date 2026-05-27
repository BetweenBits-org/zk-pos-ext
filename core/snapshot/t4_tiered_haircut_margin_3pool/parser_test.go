package t4_tiered_haircut_margin_3pool_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	t4snapshot "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t4_tiered_haircut_margin_3pool"
	t4host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

const accountID0 = "1111111111111111111111111111111111111111111111111111111111111111"

func TestStandardCSVSnapshotT4(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cex_assets.csv"), `asset_index,symbol,total_equity,total_debt,base_price,loan_collateral,margin_collateral,portfolio_margin_collateral
0,btc,100,0,60000,100,0,0
`)
	writeFile(t, filepath.Join(dir, "tier_ratios.csv"), `asset_index,collateral_pool,tier_index,boundary_value,ratio,precomputed_value
0,loan,0,100000000,100,0
0,margin,0,100000000,100,0
0,portfolio_margin,0,100000000,100,0
`)
	writeFile(t, filepath.Join(dir, "accounts.csv"), `account_id,asset_index,equity,debt,loan_collateral,margin_collateral,portfolio_margin_collateral
`+accountID0+`,0,100,0,100,0,0
`)

	src := t4snapshot.NewSnapshotCSV(t4snapshot.Config{
		Dir:           dir,
		SnapshotID:    "snap",
		AssetCapacity: 2,
	})
	assets, err := src.CexAssets(context.Background())
	if err != nil {
		t.Fatalf("CexAssets: %v", err)
	}
	if len(assets) != 2 || assets[1].Symbol != "reserved" {
		t.Fatalf("assets = %+v", assets)
	}
	if len(assets[0].LoanRatios) != corespec.TierCount {
		t.Fatalf("LoanRatios length = %d", len(assets[0].LoanRatios))
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
	if accounts != 1 {
		t.Fatalf("accounts = %d, want 1", accounts)
	}
	if got := src.InvalidCount(); got != 0 {
		t.Fatalf("InvalidCount = %d", got)
	}
}

func TestStandardCSVSnapshotT4Registered(t *testing.T) {
	if !contains(t4host.RegisteredSnapshotConnectors(), t4snapshot.ConnectorID) {
		t.Fatalf("registered connectors missing %q", t4snapshot.ConnectorID)
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
