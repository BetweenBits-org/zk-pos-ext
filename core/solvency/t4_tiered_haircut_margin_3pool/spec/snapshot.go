package spec

import "context"

// SnapshotSource yields the raw account stream and per-asset CEX
// totals for a single PoR snapshot under the t4_tiered_haircut_margin_3pool model.
//
// Implementations encapsulate the customer's ETL — CSV files,
// database queries, internal RPC, S3 dumps, etc. The witness builder
// consumes the resulting stream and produces circuit-ready
// BatchCreateUserWitness records.
//
// AccountStream MUST yield each account exactly once. Order does not
// matter; the witness builder assigns final account-tree indexes.
//
// CexAssets MUST return the same global totals that the per-account
// stream sums to — the witness builder asserts this invariant before
// proof generation.
type SnapshotSource interface {
	// AccountStream returns a channel of raw accounts. The channel
	// closes when the snapshot is exhausted. Errors are reported via
	// the returned error (synchronous start-up failure) or by closing
	// the channel early (streaming failure — implementations SHOULD
	// log before closing).
	AccountStream(ctx context.Context) (<-chan AccountInfo, error)

	// CexAssets returns global per-asset totals (TotalEquity,
	// TotalDebt, collateral aggregates). Returned slice is indexed by
	// AssetCatalog index.
	CexAssets(ctx context.Context) ([]CexAssetInfo, error)

	// SnapshotID returns a stable identifier for the snapshot point
	// in time (e.g. "2026-01-15T00:00:00Z" or an internal sequence
	// number). Embedded in published artifacts for audit.
	SnapshotID() string

	// InvalidCount returns the number of source rows that were
	// observed but classified as invalid (and therefore not yielded
	// on AccountStream's channel). Implementations MUST update this
	// counter monotonically while streaming and MUST make the value
	// safe to read concurrently with the stream goroutine. After the
	// AccountStream channel closes, the returned value is the final
	// invalid count for the snapshot.
	//
	// Witness builders use this for sanity gates — e.g. asserting
	// that valid + invalid == expected source row count — and for
	// audit-trail metadata.
	InvalidCount() uint64
}
