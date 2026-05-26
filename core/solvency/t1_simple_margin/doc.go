// Package t1_simple_margin is the Basic-tier solvency model.
//
// STATUS: catalog-reserved, implementation pending.
//
// Solvency:
//
//	published_total[asset] == sum_users(user[asset])
//
// No user-level debt or collateral logic. Assumes users carry no
// liabilities to the exchange. Account leaf reduces to
// Poseidon(AccountID, AssetsCommitment).
//
// Adapter surface (when implemented):
//   - core/spec.AssetCatalog
//   - core/spec.PriceScaleProvider
//   - core/spec.AccountIDProvider
//   - core/spec.InvalidAccountPolicy
//   - core/spec.BatchShapeProvider
//   - <this package>/spec.SnapshotSource  (single-balance-per-asset shape)
//
// Notably absent (vs t4_tiered_haircut_margin_3pool): RiskPolicy, ConstraintModule,
// per-user solvency check.
//
// Target customers: regulated spot-only exchanges, stablecoin
// issuers, custodians.
package t1_simple_margin
