// Package sea_reference is a hypothetical Southeast Asia (SEA)
// spot-only exchange profile, used to validate the model→customer flow
// for the t1_simple_margin solvency model end-to-end.
//
// Status: synthetic reference, NOT a real customer integration. Once
// a concrete SEA partner (Indodax / Tokocrypto / Pintu / Bitkub /
// other) is confirmed, rename this directory to the customer's name
// and adjust the snapshot ETL + sea_reference.toml to match their
// actual asset list and CSV layout.
//
// Model: t1_simple_margin — single-balance-per-asset user state, no
// debt, no collateral. Per-asset sum equality:
//
//	cex.TotalEquity[asset] == Σ user.Equity[asset]
//
// Post-R8 cleanup: every model-blind adapter (catalog, identity,
// insolvent, pricing, batch_shape, constraint_noop) has been removed.
// The engine-default implementations now own those shapes — core/host
// for identity + insolvent, profile/declarative for catalog +
// batch_shape + pricing, core/solvency/t1_simple_margin/host for
// snapshot connector + constraint module. The remaining files are:
//
//   - snapshot.go              T1 spot-CSV ETL — customer-specific
//                              and registered with t1host under
//                              SnapshotConnectorID via init().
//   - snapshot_test.go         Fixture coverage.
//   - snapshot_connector_test.go   Verifies init() registration.
//   - sea_reference.toml       Declarative profile.
//   - testdata/                CSV fixtures.
package sea_reference
