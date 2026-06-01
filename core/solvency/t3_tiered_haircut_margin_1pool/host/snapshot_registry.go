package host

import (
	"fmt"
	"sort"
	"sync"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/io/vfs"
	t3spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t3_tiered_haircut_margin_1pool/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// SnapshotFactory constructs a t3_tiered_haircut_margin_1pool
// SnapshotSource from the universal declarative profile arguments. src
// is the snapshot input opener; the connector reads its CSVs through
// src.Open(ctx, name).
type SnapshotFactory func(src vfs.Opener, snapshotID string, assetCapacity int, pricing corespec.PriceScaleProvider) t3spec.SnapshotSource

var (
	snapshotRegistryMu sync.RWMutex
	snapshotRegistry   = map[string]SnapshotFactory{}
)

// RegisterSnapshot adds a T3 snapshot connector factory. Panics on an
// empty id, nil factory, or duplicate registration.
func RegisterSnapshot(connectorID string, factory SnapshotFactory) {
	if connectorID == "" {
		panic("t3_tiered_haircut_margin_1pool/host: RegisterSnapshot called with empty connector id")
	}
	if factory == nil {
		panic("t3_tiered_haircut_margin_1pool/host: RegisterSnapshot(" + connectorID + ") called with nil factory")
	}
	snapshotRegistryMu.Lock()
	defer snapshotRegistryMu.Unlock()
	if _, dup := snapshotRegistry[connectorID]; dup {
		panic("t3_tiered_haircut_margin_1pool/host: snapshot connector " + connectorID + " registered twice")
	}
	snapshotRegistry[connectorID] = factory
}

// NewSnapshot returns the T3 SnapshotSource registered under
// connectorID, reading its inputs through src. Panics when the
// connector is not linked into the binary.
func NewSnapshot(connectorID string, src vfs.Opener, snapshotID string, assetCapacity int, pricing corespec.PriceScaleProvider) t3spec.SnapshotSource {
	snapshotRegistryMu.RLock()
	factory, ok := snapshotRegistry[connectorID]
	snapshotRegistryMu.RUnlock()
	if !ok {
		panic(fmt.Sprintf("t3_tiered_haircut_margin_1pool/host: snapshot connector %q is not registered (known: %v)",
			connectorID, RegisteredSnapshotConnectors()))
	}
	return factory(src, snapshotID, assetCapacity, pricing)
}

// RegisteredSnapshotConnectors returns sorted T3 snapshot connector IDs.
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
