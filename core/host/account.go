// Package host: see merkle.go for the package docstring.
//
// This file holds the universal AccountLeafHash — the 5-input
// Poseidon leaf signature shared by every solvency model in the v1
// catalog (see docs/04-solvency-models.md §3). Each model's
// per-model host helper wraps this with model-typed inputs and
// supplies the slot semantics:
//
//	T1 simple_margin            : (AccountID, TotalEquity, TotalDebt, 0,                AssetsCommitment)
//	T2 static_haircut_margin    : (AccountID, TotalEquity, TotalDebt, TotalCollateral,  AssetsCommitment)
//	T3 tiered_haircut_margin_1p : (AccountID, TotalEquity, TotalDebt, TotalCollateral,  AssetsCommitment)
//	T4 tiered_haircut_margin_3p : (AccountID, TotalEquity, TotalDebt, TotalCollateral,  AssetsCommitment)
//
// Promoted in R6/FU (G11 first entry) — see PRODUCTION_ROADMAP.md G11.
package host

import (
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// AccountLeafHash returns the universal 5-input Poseidon SMT leaf
// value for one user account. All v1 catalog models share this
// signature (see docs/04-solvency-models.md §3); per-model semantics
// are encoded in which slots carry zero versus computed values.
//
// nil byte slices are converted to fr.Element{0,0,0,0} by
// PoseidonBytes — caller MAY pass nil for slots they don't use (e.g.
// totalCollateral for T1, which carries no risk-weighted collateral).
//
// The output is byte-equivalent to the in-circuit
// `poseidon.Poseidon(api, accountID, totalEquity, totalDebt,
// totalCollateral, assetsCommitment)` over the same inputs — every
// model's Define() emits exactly this 5-input call.
func AccountLeafHash(accountID, totalEquity, totalDebt, totalCollateral, assetsCommitment []byte) []byte {
	return poseidon.PoseidonBytes(
		accountID,
		totalEquity,
		totalDebt,
		totalCollateral,
		assetsCommitment,
	)
}
