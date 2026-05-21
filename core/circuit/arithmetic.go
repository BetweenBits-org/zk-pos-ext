package circuit

import (
	"math/big"

	"github.com/consensys/gnark/frontend"
)

// IntegerDivision is a gnark hint that computes (quotient, remainder)
// of in[0] divided by in[1].
//
// Universal arithmetic primitive. Model-specific circuits compose it
// via CheckedDivByConstant or directly via api.NewHint.
//
// Migrated verbatim from the legacy circuit/utils.go:IntegerDivision.
func IntegerDivision(_ *big.Int, in []*big.Int, out []*big.Int) error {
	out[0].DivMod(in[0], in[1], out[1])
	return nil
}

// CheckedDivByConstant verifies integer division of `dividend` by a
// compile-time-known field constant `divisor` and returns the quotient.
//
// The hint returns (quotient, remainder); the circuit asserts:
//
//	quotient * divisor + remainder == dividend
//	remainder < divisor
//
// `quotientBits` / `remainderBits` are the range-check widths the
// caller expects for each — model-specific tuning parameters.
//
// Universal arithmetic primitive — the original tier-haircut math
// used divisor=100 (PercentageMultiplier). Other models may use
// other divisors (e.g. 10000 for basis points).
func CheckedDivByConstant(
	api API, r frontend.Rangechecker,
	dividend Variable, divisor Variable,
	quotientBits, remainderBits int,
) (quotient Variable) {
	res, err := api.NewHint(IntegerDivision, 2, dividend, divisor)
	if err != nil {
		panic(err)
	}
	r.Check(res[0], quotientBits)
	r.Check(res[1], remainderBits)
	api.AssertIsLessOrEqualNOp(res[1], divisor, remainderBits, true)
	api.AssertIsEqual(api.Add(api.Mul(res[0], divisor), res[1]), dividend)
	return res[0]
}
