package binance_test

import (
	"slices"
	"testing"

	t4host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/binance"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
)

// TestSnapshotConnector_Registered locks the engine wiring: importing
// profile/binance is sufficient to make the binance_csv.v1 connector
// available in the T4 host's registry. Service startup that reads
// profile.toml needs the binance import path to be in the binary.
func TestSnapshotConnector_Registered(t *testing.T) {
	if !slices.Contains(t4host.RegisteredSnapshotConnectors(), binance.SnapshotConnectorID) {
		t.Fatalf("T4 host registry missing %q (registered=%v)",
			binance.SnapshotConnectorID, t4host.RegisteredSnapshotConnectors())
	}
}

// TestSnapshotConnector_NewSnapshotFromRegistry confirms the registered
// factory produces a non-nil SnapshotSource via the universal
// (dir, id, capacity, pricing) tuple.
func TestSnapshotConnector_NewSnapshotFromRegistry(t *testing.T) {
	pricing, err := declarative.BuildPricing(declarative.Pricing{
		DefaultPriceScale: 1e8, DefaultBalanceScale: 1e8,
	})
	if err != nil {
		t.Fatalf("BuildPricing: %v", err)
	}
	src := t4host.NewSnapshot(binance.SnapshotConnectorID,
		"testdata/happy", "test", 500, pricing)
	if src == nil {
		t.Fatal("NewSnapshot returned nil")
	}
}
