package t3_tiered_haircut_margin_1pool_test

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	t3snapshot "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/t3_tiered_haircut_margin_1pool"
	t3host "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t3_tiered_haircut_margin_1pool/host"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/BetweenBits-org/zk-pos-ext/core/tierpolicy"
)

const accountID0 = "1111111111111111111111111111111111111111111111111111111111111111"

func TestStandardCSVSnapshotT3(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cex_assets.csv"), `asset_index,symbol,total_equity,total_debt,base_price,collateral
0,btc,100,0,60000,100
`)
	writeFile(t, filepath.Join(dir, "tier_ratios.csv"), `asset_index,tier_index,boundary_value,ratio,precomputed_value
0,0,100000000,100,100000000
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

// TestStandardCSVSnapshotT3RejectsBadPrecomputed asserts the parser
// enforces the standard_schema precomputed_value invariant: a
// recipe-inconsistent value (0 instead of the audited 1e8 for
// boundary=1e8, ratio=100) is rejected before witness construction.
func TestStandardCSVSnapshotT3RejectsBadPrecomputed(t *testing.T) {
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
	if _, err := src.CexAssets(context.Background()); err == nil {
		t.Fatalf("expected error for recipe-inconsistent precomputed_value, got nil")
	}
}

// TestStandardCSVSnapshotT3PolicyCommitment asserts the snapshot's
// PolicyCommitment equals the digest of the policy as authored,
// proving the parser extracts the real (unpadded) per-asset tier curve.
func TestStandardCSVSnapshotT3PolicyCommitment(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cex_assets.csv"), `asset_index,symbol,total_equity,total_debt,base_price,collateral
0,btc,100,0,60000,100
`)
	writeFile(t, filepath.Join(dir, "tier_ratios.csv"), `asset_index,tier_index,boundary_value,ratio,precomputed_value
0,0,100000000,100,100000000
`)
	writeFile(t, filepath.Join(dir, "accounts.csv"), `account_id,asset_index,equity,debt,collateral
`+accountID0+`,0,100,0,100
`)
	src := t3snapshot.NewSnapshotCSV(t3snapshot.Config{Dir: dir, SnapshotID: "snap", AssetCapacity: 2})
	got, err := src.PolicyCommitment(context.Background())
	if err != nil {
		t.Fatalf("PolicyCommitment: %v", err)
	}
	want, err := tierpolicy.PolicyCommitment(tierpolicy.Policy{
		Model: corespec.T3TieredHaircutMargin1Pool,
		Assets: []tierpolicy.AssetPolicy{
			{AssetIndex: 0, Pools: [][]tierpolicy.Tier{{{Boundary: big.NewInt(100000000), Ratio: 100}}}},
		},
	})
	if err != nil {
		t.Fatalf("expected PolicyCommitment: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("snapshot digest %x != policy-as-authored %x", got, want)
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
