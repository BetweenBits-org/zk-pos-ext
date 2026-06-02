package host

import (
	"math/big"
	"testing"
)

func ll(idx uint32, id string, bal int64) LiabilityLeaf {
	return LiabilityLeaf{Index: idx, Id: id, Balance: big.NewInt(bal)}
}

func TestReconcile_Happy(t *testing.T) {
	leaves := []LiabilityLeaf{ll(0, "a", 100), ll(1, "b", 250), ll(2, "c", 50)}
	rep := Reconcile(leaves, big.NewInt(1_000_000), big.NewInt(400))
	if !rep.OK() {
		t.Fatalf("expected clean reconcile, got %+v", rep.Violations)
	}
	if rep.Total.Cmp(big.NewInt(400)) != 0 {
		t.Fatalf("recomputed total = %s, want 400", rep.Total)
	}
}

func TestReconcile_NegativeDetected(t *testing.T) {
	leaves := []LiabilityLeaf{ll(0, "a", 100), {Index: 1, Id: "b", Balance: big.NewInt(-50)}}
	rep := Reconcile(leaves, nil, nil)
	if rep.OK() {
		t.Fatal("expected a negative-balance violation")
	}
	found := false
	for _, v := range rep.Violations {
		if v.Kind == ViolationNegative {
			found = true
		}
	}
	if !found {
		t.Fatalf("no negative violation recorded: %+v", rep.Violations)
	}
}

func TestReconcile_DuplicateDetected(t *testing.T) {
	leaves := []LiabilityLeaf{ll(0, "a", 1), ll(0, "b", 2), ll(2, "a", 3)}
	rep := Reconcile(leaves, nil, nil)
	var dupIdx, dupID bool
	for _, v := range rep.Violations {
		if v.Kind == ViolationDuplicate && v.Detail == "duplicate index" {
			dupIdx = true
		}
		if v.Kind == ViolationDuplicate && v.Detail == "duplicate id a" {
			dupID = true
		}
	}
	if !dupIdx || !dupID {
		t.Fatalf("expected duplicate index and id violations, got %+v", rep.Violations)
	}
}

func TestReconcile_RangeAndSumDetected(t *testing.T) {
	leaves := []LiabilityLeaf{ll(0, "a", 100), ll(1, "b", 250)}
	// max 200 -> 250 is out of range; published 999 -> total 350 mismatches.
	rep := Reconcile(leaves, big.NewInt(200), big.NewInt(999))
	var rangeV, sumV bool
	for _, v := range rep.Violations {
		if v.Kind == ViolationOutOfRange {
			rangeV = true
		}
		if v.Kind == ViolationSum {
			sumV = true
		}
	}
	if !rangeV || !sumV {
		t.Fatalf("expected range and sum violations, got %+v", rep.Violations)
	}
	if rep.Total.Cmp(big.NewInt(350)) != 0 {
		t.Fatalf("recomputed total = %s, want 350", rep.Total)
	}
}

func TestCheckSumEquality_NilPublishedSkips(t *testing.T) {
	total, ok := CheckSumEquality([]LiabilityLeaf{ll(0, "a", 10), ll(1, "b", 20)}, nil)
	if !ok {
		t.Fatal("nil publishedTotal should report ok")
	}
	if total.Cmp(big.NewInt(30)) != 0 {
		t.Fatalf("total = %s, want 30", total)
	}
}
