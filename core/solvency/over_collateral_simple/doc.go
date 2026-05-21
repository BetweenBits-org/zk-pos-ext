// Package over_collateral_simple is the Pro-tier variant A solvency model.
//
// STATUS: catalog-reserved, implementation pending.
//
// Solvency: single per-user collateral bucket, fixed (non-tiered)
// haircut per asset.
//
//	for each user:
//	  sum_i( collateral_i * haircut_i ) >= user.totalDebt
//
// Distinguishes from tier_1bucket only in using a single haircut
// constant per asset rather than a piecewise-linear curve.
// Substantially smaller circuit than tier-based.
//
// Adapter surface (when implemented):
//   - core/spec.AssetCatalog
//   - core/spec.PriceScaleProvider
//   - core/spec.AccountIDProvider
//   - core/spec.InvalidAccountPolicy
//   - core/spec.BatchShapeProvider
//   - <this package>/spec.HaircutPolicy   (asset -> fixed haircut)
//   - <this package>/spec.SnapshotSource
//
// Target customers: margin exchanges with simple risk models —
// one collateral pool, asset-level (not size-tiered) haircuts.
package over_collateral_simple
