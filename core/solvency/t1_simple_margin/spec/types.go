// Package spec declares the data shapes and interfaces specific to
// the t1_simple_margin solvency model: single-balance-per-asset user state
// (equity only — no debt, no collateral), no tier table, no haircut.
// The model's invariant reduces to per-asset sum equality:
//
//	cex.TotalEquity[asset] == Σ user.Equity[asset]
//
// Account leaf is Poseidon(AccountID, AssetsCommitment); user-level
// solvency is trivially satisfied (no liabilities → no per-user
// invariant beyond presence in the tree).
//
// Differences vs t4_tiered_haircut_margin_3pool/spec (intentional simplifications):
//   - AccountAsset has only Index + Equity (no Debt/Loan/Margin/PM)
//   - AccountInfo carries no TotalDebt / TotalCollateral
//   - CexAssetInfo carries no Debt / collateral aggregates / tier ratios
//   - No RiskPolicy (spot has no risk-weighted collateral)
//   - ConstraintModule is still present (alpha-layer architectural
//     consistency with t4_tiered_haircut_margin_3pool; typical case is noop)
package spec

import "math/big"

// AccountAsset is the per-user, per-asset 1-tuple of the t1_simple_margin
// model. Empty entries have Equity == 0 and are skipped by the witness
// builder when sizing the in-circuit per-user asset slice.
type AccountAsset struct {
	Index  uint16
	Equity uint64
}

// AccountInfo is the per-user record in the t1_simple_margin model. No debt
// or collateral fields — the model assumes users carry no liabilities
// to the exchange.
type AccountInfo struct {
	AccountIndex uint32
	AccountID    []byte
	TotalEquity  *big.Int
	Assets       []AccountAsset
}

// CexAssetInfo is the per-asset global state in the t1_simple_margin model.
// BasePrice is retained for user-side USD reporting (and folded into
// the commitment for byte stability) but does not participate in any
// circuit constraint beyond commitment integrity.
type CexAssetInfo struct {
	TotalEquity uint64
	BasePrice   uint64
	Symbol      string
	Index       uint32
}
