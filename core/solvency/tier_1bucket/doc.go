// Package tier_1bucket is the Pro-tier variant B solvency model.
//
// STATUS: catalog-reserved, implementation pending.
//
// Solvency: tier-based piecewise-linear haircut (same curve shape
// as tier_3bucket) with a single collateral bucket.
//
//	for each user:
//	  sum_i( haircut_curve(collateral_i, asset_i) ) >= user.totalDebt
//
// Distinguishes from tier_3bucket by collapsing the Loan / Margin /
// PortfolioMargin three-bucket split into one bucket. Smaller circuit;
// retains size-tiered risk curves.
//
// Adapter surface (when implemented):
//   - core/spec.AssetCatalog
//   - core/spec.PriceScaleProvider
//   - core/spec.AccountIDProvider
//   - core/spec.InvalidAccountPolicy
//   - core/spec.BatchShapeProvider
//   - <this package>/spec.RiskPolicy        (single-bucket tier ratios)
//   - <this package>/spec.SnapshotSource    (single-collateral shape)
//
// Target customers: derivatives-heavy exchanges that have not
// segmented collateral by business line.
package tier_1bucket
