// Package t2_static_haircut_margin is the Pro-tier variant A solvency
// model (T2 in the v1 catalog Tn naming).
//
// STATUS: implementation complete (R6 follow-up — T2/T3 catalog fill).
//
// Solvency: per-asset *static* (non-tiered) haircut over a SINGLE
// collateral pool. Each asset has one Haircut basis-points constant
// (10000 = 100% = no haircut, 9000 = 90% = 10% haircut).
//
//	∀ user :  Σ_i (collateral_i × haircut_i / 10000) ≥ user.TotalDebt
//
// AccountLeaf signature is universal across the catalog
// (see core/host.AccountLeafHash):
//
//	Poseidon(AccountID, TotalEquity, TotalDebt, TotalCollateral, AssetsCommitment)
//
// Distinguishes from t3_tiered_haircut_margin_1pool by collapsing the
// piecewise-linear tier curve to a single Haircut constant per asset.
// One multiply + one division per asset, no tier-table lookup —
// cheaper circuit than T3.
//
// Adapter surface (per customer):
//   - core/spec.AssetCatalog
//   - core/spec.PriceScaleProvider
//   - core/spec.AccountIDProvider
//   - core/spec.InvalidAccountPolicy
//   - core/spec.BatchShapeProvider
//   - <this package>/spec.RiskPolicy        (per-asset static haircut)
//   - <this package>/spec.SnapshotSource    (single-collateral shape)
//
// Industry reference: Aave V3 per-asset LTV / Liquidation Threshold
// constants (see docs/04-solvency-models.md §5).
//
// Target customers: margin exchanges with simple risk models —
// one collateral pool, asset-level (not size-tiered) haircuts.
package t2_static_haircut_margin
