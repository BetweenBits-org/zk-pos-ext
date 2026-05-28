package witness

import (
	"os"
	"testing"

	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
)

// TestTiersFromShapes_T4Reference locks the T4 reference profile's
// production tier view at {50, 500} via the declarative profile.
// A drift would silently change witness bucketing without an obvious
// failure surface.
func TestTiersFromShapes_T4Reference(t *testing.T) {
	os.Unsetenv("ZKPOR_BATCH_SHAPE_OVERRIDE") // never inherit env override
	prof, err := declarative.Load("../../profile/t4_reference/t4_reference.toml")
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	provider, err := declarative.BuildBatchShapeProvider(
		corespec.SolvencyModelID(prof.Profile.Model), prof.BatchShapes)
	if err != nil {
		t.Fatalf("BuildBatchShapeProvider: %v", err)
	}
	got := tiersFromShapes(provider.Shapes())
	want := []int{50, 500}
	if len(got) != len(want) {
		t.Fatalf("tiers length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tiers[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

// streamAndBucket bucket-routing test moved to the runner packages
// (Phase 3b refactor): the model-typed loop now lives at
// core/solvency/<model>/host/witness_runner.go, where each model can
// assert its own AccountInfo + AccountAsset shape.
