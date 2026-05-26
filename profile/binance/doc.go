// Package binance is Binance's customer-deployment profile.
//
// Post-R8 cleanup: every model-blind adapter (catalog, identity,
// insolvent, pricing, batch_shape, constraint_noop) has been removed.
// Those adapters were byte-equivalent copies of the engine-default
// implementations now owned by core/host (identity, insolvent) and
// profile/declarative (catalog, batch_shape, pricing) plus
// core/solvency/t4_tiered_haircut_margin_3pool/host (snapshot connector
// + constraint module). The remaining files are:
//
//   - snapshot.go              Customer-specific CSV ETL — the only
//                              piece that genuinely varies per
//                              customer. Registers its connector
//                              with t4host under SnapshotConnectorID
//                              via init() (R8-B/2).
//   - snapshot_test.go         ETL fixture coverage (happy + tamper).
//   - snapshot_connector_test.go   Verifies init() registration with
//                                  t4host.
//   - legacy_compare_test.go   R3/3 G1 sample-corpus AccountID
//                              byte-equivalence vs legacy ETL.
//   - testdata/                CSV fixtures.
//   - binance.toml             Declarative profile — the toml the
//                              services load via -profile.
//
// Profile.toml is the single source of truth for the model id +
// asset capacity + batch shapes + pricing + identity scheme +
// insolvent action + snapshot connector. The init() in snapshot.go
// is the only Go-level wiring left.
//
// Multi-customer note: a second customer on t4_tiered_haircut_margin_3pool
// places its profile under zkpor/profile/<customer>/ with its own
// snapshot.go (registering under a new connector id) and its own
// <customer>.toml. The shared model code (circuit + spec + host
// helpers) lives at zkpor/core/solvency/t4_tiered_haircut_margin_3pool/.
package binance
