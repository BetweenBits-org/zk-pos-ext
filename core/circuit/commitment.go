package circuit

import (
	"github.com/consensys/gnark/std/hash/poseidon"
)

// ComputeFlatUint64Commitment packs an array of uint64-bounded
// Variables (3 per field element, using 2^128 / 2^64 / 1 as positional
// weights) and returns the Poseidon hash of the packed array.
//
// Universal — works for any flat layout of uint64 values. Each
// solvency model decides which fields go into `flatValues` and in
// what order. For t4_tiered_haircut_margin_3pool the layout is
// {Index, Equity, Debt, Loan, Margin, PM} per asset; for t1_simple_margin
// it might be {Index, Balance} per asset.
//
// Migrated verbatim (modulo constant references) from the legacy
// circuit/utils.go:computeUserAssetsCommitment.
func ComputeFlatUint64Commitment(api API, flatValues []Variable) Variable {
	nEles := (len(flatValues) + 2) / 3
	quotientEles := len(flatValues) / 3
	remainderEles := len(flatValues) % 3
	tmp := make([]Variable, nEles)
	for i := 0; i < quotientEles; i++ {
		tmp[i] = api.Add(
			api.Mul(flatValues[3*i], TwoToTheOneTwentyEight),
			api.Mul(flatValues[3*i+1], TwoToTheSixtyFour),
			flatValues[3*i+2],
		)
	}
	// Trailing partial field — fill high-to-low (matching the quotient
	// loop's {2^128, 2^64, 1} weighting). Missing low/mid entries
	// pad with 0. Pre-fix this branch discarded its computed value
	// (`_ = last`) and left tmp[nEles-1] nil — invisible bug under
	// t4_tiered_haircut_margin_3pool's 6-field-per-asset layout (always a multiple of 3),
	// but it panics in Poseidon under any layout whose flatten length
	// is not 3-divisible (e.g. t1_simple_margin's 2-field-per-asset).
	if remainderEles > 0 {
		var last Variable = 0
		for i := 0; i < remainderEles; i++ {
			last = api.Add(api.Mul(last, TwoToTheSixtyFour), flatValues[3*quotientEles+i])
		}
		for i := remainderEles; i < 3; i++ {
			last = api.Mul(last, TwoToTheSixtyFour)
		}
		tmp[nEles-1] = last
	}
	return poseidon.Poseidon(api, tmp...)
}

// BatchCommitment returns the four-input Poseidon hash that becomes
// the public input of the batch proof:
//
//	Poseidon(beforeAccountRoot, afterAccountRoot, beforeCexCommit, afterCexCommit)
//
// Universal across all solvency models — the 4-input shape is the
// standard verifier interface. Models vary in *what*
// beforeCexCommit / afterCexCommit commit to, but the batch-
// commitment shape itself is invariant.
func BatchCommitment(api API, beforeAccountRoot, afterAccountRoot, beforeCexCommit, afterCexCommit Variable) Variable {
	return poseidon.Poseidon(api, beforeAccountRoot, afterAccountRoot, beforeCexCommit, afterCexCommit)
}
