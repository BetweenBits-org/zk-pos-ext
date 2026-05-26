package host

import (
	"fmt"
	"sort"
	"sync"

	t1spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// SnapshotFactory constructs a t1_simple_margin SnapshotSource from
// the universal arguments captured in the declarative profile. Same
// signature as t4_tiered_haircut_margin_3pool's factory — see that file for
// argument semantics (R8-E added the PriceScaleProvider tail
// argument so customer ETL can fully drop its in-package pricing
// adapter).
type SnapshotFactory func(
	userDataDir, snapshotID string,
	assetCapacity int,
	pricing corespec.PriceScaleProvider,
) t1spec.SnapshotSource

// Snapshot connector registry (T1 — R8-B/2 / G17).
//
// Identical shape to t4_tiered_haircut_margin_3pool/host's registry — the table
// lives per-model because SnapshotSource is model-typed. ID format
// "<connector_id>.v<version>". First v1 entry: "sea_csv.v1"
// (implementation in profile/sea_reference/snapshot.go).
var (
	snapshotRegistryMu sync.RWMutex
	snapshotRegistry   = map[string]SnapshotFactory{}
)

// RegisterSnapshot adds (connectorID → factory). Panics on empty ID,
// nil factory, or duplicate registration.
func RegisterSnapshot(connectorID string, factory SnapshotFactory) {
	if connectorID == "" {
		panic("t1_simple_margin/host: RegisterSnapshot called with empty connector id")
	}
	if factory == nil {
		panic("t1_simple_margin/host: RegisterSnapshot(" + connectorID + ") called with nil factory")
	}
	snapshotRegistryMu.Lock()
	defer snapshotRegistryMu.Unlock()
	if _, dup := snapshotRegistry[connectorID]; dup {
		panic("t1_simple_margin/host: snapshot connector " + connectorID + " registered twice")
	}
	snapshotRegistry[connectorID] = factory
}

// NewSnapshot returns the T1 SnapshotSource built by the connector
// registered under connectorID. Panics if not registered.
func NewSnapshot(
	connectorID, userDataDir, snapshotID string,
	assetCapacity int,
	pricing corespec.PriceScaleProvider,
) t1spec.SnapshotSource {
	snapshotRegistryMu.RLock()
	factory, ok := snapshotRegistry[connectorID]
	snapshotRegistryMu.RUnlock()
	if !ok {
		panic(fmt.Sprintf("t1_simple_margin/host: snapshot connector %q is not registered (known: %v)",
			connectorID, RegisteredSnapshotConnectors()))
	}
	return factory(userDataDir, snapshotID, assetCapacity, pricing)
}

// RegisteredSnapshotConnectors returns the sorted list of T1
// snapshot connector IDs currently in the registry.
func RegisteredSnapshotConnectors() []string {
	snapshotRegistryMu.RLock()
	defer snapshotRegistryMu.RUnlock()
	out := make([]string, 0, len(snapshotRegistry))
	for k := range snapshotRegistry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
