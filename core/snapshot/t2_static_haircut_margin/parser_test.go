package t2_static_haircut_margin_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	t2snapshot "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/t2_static_haircut_margin"
	t2host "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t2_static_haircut_margin/host"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/BetweenBits-org/zk-pos-ext/core/tierpolicy"
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

// TestStandardCSVSnapshotT2PolicyCommitment asserts the snapshot's
// PolicyCommitment equals the digest of the policy as authored
// (proving the parser extracts the real per-asset haircut policy) and
// is independent of the deployment asset capacity.
func TestStandardCSVSnapshotT2PolicyCommitment(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cex_assets.csv"), `asset_index,symbol,total_equity,total_debt,base_price,collateral,haircut_bp
0,btc,100,0,60000,100,9000
1,eth,50,0,3000,50,8500
`)
	writeFile(t, filepath.Join(dir, "accounts.csv"), `account_id,asset_index,equity,debt,collateral
`+accountID0+`,0,100,0,100
`)
	src := t2snapshot.NewSnapshotCSV(t2snapshot.Config{Dir: dir, SnapshotID: "snap", AssetCapacity: 4})
	got, err := src.PolicyCommitment(context.Background())
	if err != nil {
		t.Fatalf("PolicyCommitment: %v", err)
	}
	want, err := tierpolicy.PolicyCommitment(tierpolicy.Policy{
		Model: corespec.T2StaticHaircutMargin,
		Assets: []tierpolicy.AssetPolicy{
			{AssetIndex: 0, Haircut: 9000},
			{AssetIndex: 1, Haircut: 8500},
		},
	})
	if err != nil {
		t.Fatalf("expected PolicyCommitment: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("snapshot digest %x != policy-as-authored %x", got, want)
	}
	// Capacity-independent: same policy at a larger capacity → same digest.
	src2 := t2snapshot.NewSnapshotCSV(t2snapshot.Config{Dir: dir, SnapshotID: "snap", AssetCapacity: 8})
	got2, err := src2.PolicyCommitment(context.Background())
	if err != nil {
		t.Fatalf("PolicyCommitment cap=8: %v", err)
	}
	if !bytes.Equal(got, got2) {
		t.Fatal("digest depends on asset capacity (must be capacity-independent)")
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
