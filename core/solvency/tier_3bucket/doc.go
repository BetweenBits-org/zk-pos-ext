// Package tier_3bucket is the Enterprise-tier solvency model — the
// reference implementation inherited from the Binance OSS PoR v2
// codebase.
//
// Solvency: three-bucket collateral (Loan / Margin / PortfolioMargin),
// tier-based piecewise-linear haircut per bucket. Full per-user
// solvency:
//
//	per-asset:  Loan + Margin + PortfolioMargin <= Equity
//	per-user:   sum_assets( haircut(collateral) ) >= totalDebt
//	per-asset:  published_total == sum_users(...)
//
// Model-tied interfaces live in this package's spec/ sub-package:
//   - RiskPolicy        : per-symbol tier ratio values
//   - SnapshotSource    : 5-tuple raw account stream
//   - ConstraintModule  : optional zk-constraint extensions (typed)
//   - types.go          : AccountAsset, CexAssetInfo, TierRatio, AccountInfo
//
// The actual circuit will live under this package's circuit/
// sub-package (porting from legacy circuit/ pending).
//
// Target customers: Binance-class exchanges with VIP loan +
// cross margin + portfolio margin business lines.
package tier_3bucket
