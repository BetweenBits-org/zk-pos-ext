// Package t3_tiered_haircut_margin_1pool is the Pro-tier variant B
// solvency model (T3 in the v1 catalog Tn naming).
//
// STATUS: implementation complete (R6 follow-up — T2/T3 catalog fill).
//
// Solvency: tier-based piecewise-linear haircut (same curve shape
// as t4_tiered_haircut_margin_3pool) with a SINGLE collateral pool —
// the 3-bucket (Loan / Margin / PortfolioMargin) split of T4 is
// collapsed into one.
//
//	∀ user :  Σ_i haircut_curve_i(collateral_i) ≥ user.TotalDebt
//
// AccountLeaf signature is universal across the catalog
// (see core/host.AccountLeafHash):
//
//	Poseidon(AccountID, TotalEquity, TotalDebt, TotalCollateral, AssetsCommitment)
//
// Distinguishes from t4_tiered_haircut_margin_3pool by collapsing the
// Loan / Margin / PortfolioMargin three-bucket split into one bucket.
// Smaller circuit; retains size-tiered risk curves per asset.
//
// Adapter surface (per customer):
//   - core/spec.AssetCatalog
//   - core/spec.PriceScaleProvider
//   - core/spec.AccountIDProvider
//   - core/spec.InvalidAccountPolicy
//   - core/spec.BatchShapeProvider
//   - <this package>/spec.RiskPolicy        (single-bucket tier ratios)
//   - <this package>/spec.SnapshotSource    (single-collateral shape)
//
// Industry reference: dYdX size-tiered IMF curve + single cross-margin
// collateral pool (see docs/04-solvency-models.md §6).
//
// Target customers: derivatives-heavy exchanges that have not
// segmented collateral by business line.
package t3_tiered_haircut_margin_1pool
