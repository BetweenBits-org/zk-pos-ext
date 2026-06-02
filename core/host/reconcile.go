// reconcile.go holds the auditor-substitute checks for the non-zk
// Merkle-sum proof-of-liabilities side line (PRODUCTION_ROADMAP Stage MS,
// gate G19). In the zk PoS line, non-negativity and sum-equality are
// enforced inside the circuit for everyone; in the Merkle-sum line a user
// sees only their own inclusion path, so these properties must instead be
// re-established by an auditor who recomputes them over the FULL leaf set.
// These pure functions are that recomputation — the trust substitute for
// the missing zk proof.

package host

import (
	"fmt"
	"math/big"
)

// LiabilityLeaf is one account's contribution to total liabilities: a
// positional Index, a public Id (hex account id) for duplicate detection,
// and the non-negative Balance the exchange owes the account.
type LiabilityLeaf struct {
	Index   uint32
	Id      string
	Balance *big.Int
}

// ViolationKind classifies a reconcile failure.
type ViolationKind string

const (
	// ViolationNegative — a leaf balance is nil or negative (the canonical
	// Merkle-sum forgery: a negative leaf shrinks the apparent total).
	ViolationNegative ViolationKind = "negative_balance"
	// ViolationDuplicate — a leaf Index or Id appears more than once.
	ViolationDuplicate ViolationKind = "duplicate"
	// ViolationOutOfRange — a leaf balance exceeds the allowed maximum.
	ViolationOutOfRange ViolationKind = "out_of_range"
	// ViolationSum — the recomputed total does not equal the published total.
	ViolationSum ViolationKind = "sum_mismatch"
)

// Violation is a single reconcile failure with enough context to locate
// it. Index is the offending leaf's positional index; for ViolationSum it
// is zero (the failure is set-wide, see Detail).
type Violation struct {
	Kind   ViolationKind
	Index  uint32
	Detail string
}

// Report is the aggregate result of Reconcile. Total is the recomputed sum
// of all non-negative balances; OK is true iff there are no violations.
type Report struct {
	Total      *big.Int
	Violations []Violation
}

// OK reports whether reconciliation passed with no violations.
func (r Report) OK() bool { return len(r.Violations) == 0 }

// CheckNonNegative returns a Violation for every leaf whose Balance is nil
// or negative.
func CheckNonNegative(leaves []LiabilityLeaf) []Violation {
	var v []Violation
	for _, lf := range leaves {
		if lf.Balance == nil || lf.Balance.Sign() < 0 {
			v = append(v, Violation{Kind: ViolationNegative, Index: lf.Index, Detail: "balance is nil or negative"})
		}
	}
	return v
}

// CheckNoDuplicate returns a Violation for every leaf whose Index or
// (non-empty) Id has already appeared — each account must contribute
// exactly once.
func CheckNoDuplicate(leaves []LiabilityLeaf) []Violation {
	var v []Violation
	seenIdx := make(map[uint32]bool, len(leaves))
	seenID := make(map[string]bool, len(leaves))
	for _, lf := range leaves {
		if seenIdx[lf.Index] {
			v = append(v, Violation{Kind: ViolationDuplicate, Index: lf.Index, Detail: "duplicate index"})
		} else {
			seenIdx[lf.Index] = true
		}
		if lf.Id != "" {
			if seenID[lf.Id] {
				v = append(v, Violation{Kind: ViolationDuplicate, Index: lf.Index, Detail: "duplicate id " + lf.Id})
			} else {
				seenID[lf.Id] = true
			}
		}
	}
	return v
}

// CheckRange returns a Violation for every leaf whose Balance exceeds max
// (e.g. the per-account uint64 ceiling). A nil max disables the check.
func CheckRange(leaves []LiabilityLeaf, max *big.Int) []Violation {
	if max == nil {
		return nil
	}
	var v []Violation
	for _, lf := range leaves {
		if lf.Balance != nil && lf.Balance.Cmp(max) > 0 {
			v = append(v, Violation{Kind: ViolationOutOfRange, Index: lf.Index, Detail: "balance exceeds max"})
		}
	}
	return v
}

// CheckSumEquality recomputes the total over all non-negative balances and
// compares it to publishedTotal, returning (recomputed, ok). Negative or
// nil balances are excluded from the total (their own CheckNonNegative
// violation flags them). A nil publishedTotal skips the comparison and
// reports ok=true.
func CheckSumEquality(leaves []LiabilityLeaf, publishedTotal *big.Int) (*big.Int, bool) {
	total := big.NewInt(0)
	for _, lf := range leaves {
		if lf.Balance != nil && lf.Balance.Sign() >= 0 {
			total.Add(total, lf.Balance)
		}
	}
	if publishedTotal == nil {
		return total, true
	}
	return total, total.Cmp(publishedTotal) == 0
}

// Reconcile runs the full auditor pass — non-negativity, duplicate, range,
// and sum-equality against publishedTotal — and aggregates every violation
// plus the recomputed total. max bounds per-leaf balances (nil disables);
// publishedTotal is the exchange's claimed total liabilities (nil skips
// the sum check). This is the engine half of the Stage MS RunAudit report;
// the Reserves >= Liabilities comparison is the caller's, with reserves
// supplied as an audited input (gate G19, D4).
func Reconcile(leaves []LiabilityLeaf, max, publishedTotal *big.Int) Report {
	var vs []Violation
	vs = append(vs, CheckNonNegative(leaves)...)
	vs = append(vs, CheckNoDuplicate(leaves)...)
	vs = append(vs, CheckRange(leaves, max)...)
	total, ok := CheckSumEquality(leaves, publishedTotal)
	if !ok {
		vs = append(vs, Violation{Kind: ViolationSum, Detail: fmt.Sprintf("recomputed total %s != published %s", total, publishedTotal)})
	}
	return Report{Total: total, Violations: vs}
}
