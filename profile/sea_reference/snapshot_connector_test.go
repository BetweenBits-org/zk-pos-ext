package sea_reference_test

import (
	"slices"
	"testing"

	t1host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/host"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
	sea_reference "github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/sea_reference"
)

// TestSnapshotConnector_Registered locks the engine wiring: importing
// profile/sea_reference makes the sea_csv.v1 connector available in
// the T1 host's registry.
func TestSnapshotConnector_Registered(t *testing.T) {
	if !slices.Contains(t1host.RegisteredSnapshotConnectors(), sea_reference.SnapshotConnectorID) {
		t.Fatalf("T1 host registry missing %q (registered=%v)",
			sea_reference.SnapshotConnectorID, t1host.RegisteredSnapshotConnectors())
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
	src := t1host.NewSnapshot(sea_reference.SnapshotConnectorID,
		"testdata/happy", "test", 50, pricing)
	if src == nil {
		t.Fatal("NewSnapshot returned nil")
	}
}
