package spec

import "context"

// SnapshotSource yields the raw account stream and per-asset CEX
// totals for a single PoR snapshot under the t1_simple_margin model.
//
// Same shape as t4_tiered_haircut_margin_3pool/spec.SnapshotSource — only AccountInfo
// and CexAssetInfo differ (spot single-balance vs 5-tuple).
//
// AccountStream MUST yield each account exactly once. Order does not
// matter; the witness builder assigns final account-tree indexes.
//
// CexAssets MUST return the same per-asset Equity totals that the
// per-account stream sums to — the witness builder asserts this
// invariant before proof generation.
type SnapshotSource interface {
	// AccountStream returns a channel of raw accounts. Closes when the
	// snapshot is exhausted. Synchronous start-up failures are returned
	// as the error; streaming failures close the channel early
	// (implementations SHOULD log before closing).
	AccountStream(ctx context.Context) (<-chan AccountInfo, error)

	// CexAssets returns global per-asset totals indexed by AssetCatalog
	// index. Length == capacity (snapshot pads with reserved entries).
	CexAssets(ctx context.Context) ([]CexAssetInfo, error)

	// SnapshotID returns a stable identifier for the snapshot point
	// in time. Embedded in published artifacts for audit.
	SnapshotID() string

	// InvalidCount returns the number of source rows classified as
	// invalid (e.g. malformed hex account id, balance overflow) and
	// therefore not yielded on AccountStream's channel. MUST be safe
	// to read concurrently with the stream goroutine.
	InvalidCount() uint64
}
