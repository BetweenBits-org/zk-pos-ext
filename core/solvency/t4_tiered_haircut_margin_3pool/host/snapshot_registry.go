package host

import (
	"fmt"
	"sort"
	"sync"

	t4spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// SnapshotFactory constructs a t4_tiered_haircut_margin_3pool SnapshotSource from
// the universal arguments captured in the declarative profile.
//
// Arguments:
//   - userDataDir, snapshotID: the profile's [snapshot] table.
//   - assetCapacity: trusted-setup asset slot count (toml or override).
//   - pricing: PriceScaleProvider built from [pricing] — the ETL uses
//     it to scale raw float prices/balances into the uint64 values
//     embedded in the witness. R8-E surfaced this as a missing factory
//     argument: customer ETL code previously imported a per-profile
//     pricing struct, which the R8-E cleanup removed.
//
// Factories MUST be cheap (no I/O) — heavy work (file open, CSV
// parse) is deferred to AccountStream/CexAssets.
//
// One factory per connector ID. Connectors are registered at build
// time via init() in the package that owns the canonical snapshot
// implementation.
type SnapshotFactory func(
	userDataDir, snapshotID string,
	assetCapacity int,
	pricing corespec.PriceScaleProvider,
) t4spec.SnapshotSource

// Snapshot connector registry (T4 — R8-B/2 / G17).
//
// Layer ownership: each model's host package owns its own snapshot
// registry because SnapshotSource is model-typed (t4spec.SnapshotSource
// differs from t1spec.SnapshotSource). Universal factory arguments
// (dir, snapshotID, capacity) keep the registration shape consistent
// across models even though the returned interface type does not.
//
// ID format convention (G17): "<connector_id>.v<version>". The V1
// product connector is "t4_standard_csv.v1" from core/snapshot.
//
// Lifetime: registration at build time via init(). Read-only after
// package init. Misses panic — every connector referenced by a
// profile.toml MUST be linked into the binary.
var (
	snapshotRegistryMu sync.RWMutex
	snapshotRegistry   = map[string]SnapshotFactory{}
)

// RegisterSnapshot adds (connectorID → factory) to the T4 snapshot
// registry. Intended to be called from package init(); double-
// registration panics.
//
// Panics:
//   - empty connectorID.
//   - nil factory.
//   - connectorID already registered.
func RegisterSnapshot(connectorID string, factory SnapshotFactory) {
	if connectorID == "" {
		panic("t4_tiered_haircut_margin_3pool/host: RegisterSnapshot called with empty connector id")
	}
	if factory == nil {
		panic("t4_tiered_haircut_margin_3pool/host: RegisterSnapshot(" + connectorID + ") called with nil factory")
	}
	snapshotRegistryMu.Lock()
	defer snapshotRegistryMu.Unlock()
	if _, dup := snapshotRegistry[connectorID]; dup {
		panic("t4_tiered_haircut_margin_3pool/host: snapshot connector " + connectorID + " registered twice")
	}
	snapshotRegistry[connectorID] = factory
}

// NewSnapshot returns the T4 SnapshotSource built by the connector
// registered under connectorID. Service startup calls this after
// resolving the model from profile.toml.
//
// Panics if connectorID is not registered (build-time omission).
func NewSnapshot(
	connectorID, userDataDir, snapshotID string,
	assetCapacity int,
	pricing corespec.PriceScaleProvider,
) t4spec.SnapshotSource {
	snapshotRegistryMu.RLock()
	factory, ok := snapshotRegistry[connectorID]
	snapshotRegistryMu.RUnlock()
	if !ok {
		panic(fmt.Sprintf("t4_tiered_haircut_margin_3pool/host: snapshot connector %q is not registered (known: %v)",
			connectorID, RegisteredSnapshotConnectors()))
	}
	return factory(userDataDir, snapshotID, assetCapacity, pricing)
}

// RegisteredSnapshotConnectors returns the sorted list of T4
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
