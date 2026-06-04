package estimate

import (
	"testing"

	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
)

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// TestConstraints_MatchesDirectCompile validates the drift guard: the
// extrapolated estimate must agree with a direct compile of the same shape
// (within a small tolerance for any per-batch non-linearity). If the
// circuit changes, both sides move together — but a broken extrapolation
// is caught here. T1 keeps this fast enough for -short.
func TestConstraints_MatchesDirectCompile(t *testing.T) {
	const (
		model    = corespec.SolvencyModelID("t1_simple_margin")
		capacity = 50
	)
	shape := corespec.BatchShape{AssetCountTier: 5, UsersPerBatch: 10}

	actual, err := CompileConstraints(model, shape, capacity)
	if err != nil {
		t.Fatalf("CompileConstraints: %v", err)
	}
	est, err := Constraints(model, shape, capacity)
	if err != nil {
		t.Fatalf("Constraints: %v", err)
	}

	rel := float64(abs(est-actual)) / float64(actual)
	t.Logf("T1 tier5 users10 cap50: actual=%d estimate=%d (%.3f%% off)", actual, est, 100*rel)
	if rel > 0.01 {
		t.Fatalf("extrapolated estimate %d off from direct compile %d by %.2f%% (want <1%%)", est, actual, 100*rel)
	}

	// Cross-check against the box measurement (2026-06, shape 5_10 cap50).
	// A mismatch is informational (a gnark/circuit delta), not a hard fail.
	if actual != 174463 {
		t.Logf("note: local direct compile %d != box-measured 174463", actual)
	}
}

// TestConstraints_SmallShapeIsExact confirms that when usersPerBatch is
// within the probe range the estimate is the direct (exact) compile.
func TestConstraints_SmallShapeIsExact(t *testing.T) {
	const (
		model    = corespec.SolvencyModelID("t1_simple_margin")
		capacity = 50
	)
	shape := corespec.BatchShape{AssetCountTier: 5, UsersPerBatch: probeHi}

	est, err := Constraints(model, shape, capacity)
	if err != nil {
		t.Fatalf("Constraints: %v", err)
	}
	exact, err := CompileConstraints(model, shape, capacity)
	if err != nil {
		t.Fatalf("CompileConstraints: %v", err)
	}
	if est != exact {
		t.Fatalf("small-shape estimate %d != exact %d", est, exact)
	}
}
