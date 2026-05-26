// Package spec declares the data shapes and interfaces specific to
// the t1_simple_margin solvency model: per-user account-level
// (TotalEquity, TotalDebt) with the constraint TotalEquity >= TotalDebt
// (no risk-weighted collateral, no tier table, no haircut).
//
// Spot use case: supply TotalDebt == 0 and the constraint is trivially
// satisfied — a single ceremony serves both spot and simple-margin
// customers. See docs/04-solvency-models.md §8.1 for the absorption
// trail.
//
// Solvency claim:
//
//	∀ user :  user.TotalEquity ≥ user.TotalDebt           (account-level)
//	cex.TotalEquity[asset] == Σ user.Equity[asset]        (per-asset sum)
//	cex.TotalDebt[asset]   == Σ user.Debt[asset]          (per-asset sum)
//
// Account leaf: Poseidon(AccountID, TotalEquity, TotalDebt, 0,
// AssetsCommitment) — the universal 5-input leaf signature (see
// docs/04-solvency-models.md §3). Slot 4 (TotalCollateral) is pinned to
// 0 since T1 carries no risk-weighted collateral.
//
// Differences vs t4_tiered_haircut_margin_3pool/spec (intentional
// simplifications):
//   - AccountAsset has only Index + Equity + Debt (no Loan/Margin/PM)
//   - AccountInfo carries no TotalCollateral
//   - CexAssetInfo carries no collateral aggregates / tier ratios
//   - No RiskPolicy (T1 has no risk-weighted collateral)
//   - ConstraintModule is still present (alpha-layer architectural
//     consistency with t4_tiered_haircut_margin_3pool; typical case is noop)
package spec

import "math/big"

// AccountAsset is the per-user, per-asset record of the
// t1_simple_margin model. Equity is the user's claim against the
// exchange in this asset; Debt is what the user owes (borrowed margin)
// in this asset. Both are non-negative uint64.
//
// Empty entries have Equity == 0 && Debt == 0 and are skipped by the
// witness builder when sizing the in-circuit per-user asset slice.
//
// Spot customers always supply Debt == 0.
type AccountAsset struct {
	Index  uint16
	Equity uint64
	Debt   uint64
}

// AccountInfo is the per-user record in the t1_simple_margin model.
// TotalEquity and TotalDebt are the per-user totals across all
// AccountAsset entries; the circuit enforces TotalEquity >= TotalDebt
// at the account level. No TotalCollateral field — T1 carries no
// risk-weighted collateral.
type AccountInfo struct {
	AccountIndex uint32
	AccountID    []byte
	TotalEquity  *big.Int
	TotalDebt    *big.Int
	Assets       []AccountAsset
}

// CexAssetInfo is the per-asset global state in the t1_simple_margin
// model. TotalEquity is Σ user.Equity[asset] and TotalDebt is
// Σ user.Debt[asset]. BasePrice is retained for user-side USD reporting
// (and folded into the commitment for byte stability) but does not
// participate in any circuit constraint beyond commitment integrity.
type CexAssetInfo struct {
	TotalEquity uint64
	TotalDebt   uint64
	BasePrice   uint64
	Symbol      string
	Index       uint32
}
