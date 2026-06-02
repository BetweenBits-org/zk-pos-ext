package circuit

import (
	"testing"
	"time"

	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	t2spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t2_static_haircut_margin/spec"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// TestSetupSmoke compiles BatchCreateUserCircuit and runs groth16.Setup
// at a deliberately tiny shape (userAssetCounts=5, allAssetCounts=50,
// batchCounts=2). Purpose: surface IR-level defects (bad lookup-table
// shapes, malformed Define signatures, wrong field types, etc.) before
// R3 step 2 (alpha wiring) and step 3 (G1 byte-equivalence).
//
// Production shape (50, 500, 700) is intentionally NOT used here —
// running Setup at that size costs multi-minute compile and multi-GB
// pk, which is wasted under a smoke. Exact-shape Compile + byte
// equivalence is the job of R3 step 3 (G1 closure). The constraint
// count logged below is also not compared to legacy — that comparison
// is meaningful only at production shape and at the same shape on
// both sides.
//
// batchCounts=2 is the minimum that still exercises the user-roots
// chaining check (op[i].After == op[i+1].Before) in Define().
func TestSetupSmoke(t *testing.T) {
	const (
		userAssetCounts uint32 = 5
		allAssetCounts  uint32 = 50
		batchCounts     uint32 = 2
	)

	c := NewBatchCreateUserCircuit(userAssetCounts, allAssetCounts, batchCounts)

	startCompile := time.Now()
	r1csObj, err := frontend.Compile(
		ecc.BN254.ScalarField(),
		r1cs.NewBuilder,
		c,
		frontend.IgnoreUnconstrainedInputs(),
	)
	if err != nil {
		t.Fatalf("frontend.Compile: %v", err)
	}
	t.Logf("compile took %s, NbConstraints=%d (legacy comparison is R3 step 3 at production shape)",
		time.Since(startCompile), r1csObj.GetNbConstraints())

	startSetup := time.Now()
	if _, _, err := groth16.Setup(r1csObj); err != nil {
		t.Fatalf("groth16.Setup: %v", err)
	}
	t.Logf("groth16.Setup took %s", time.Since(startSetup))
}

// testNoopModule is the in-package no-op ConstraintModule used as the
// alpha-wiring regression guard in TestSetupSmoke_AlphaNoopBaseline.
// Defined here (rather than reusing profile/binance.noopModule) so the
// circuit-level test stays import-free of profile packages.
type testNoopModule struct{}

func (testNoopModule) ID() corespec.ConstraintModuleID {
	return corespec.ConstraintModuleID(corespec.NoExtensionID)
}

func (testNoopModule) Define(_ frontend.API, _ t2spec.ConstraintContext) error {
	return nil
}

// TestSetupSmoke_AlphaNoopBaseline compiles the t2_static_haircut_margin circuit
// twice at the same tiny shape — once with no ConstraintModule (the
// R3 step 0 path) and once with a no-op module wired via
// SetConstraintModule — and asserts the two compilations produce the
// same NbConstraints. The exact count is not asserted here (legacy
// byte-equivalence is R3 step 3 / G1); the equality between the two
// paths is what proves the alpha hook adds zero in-circuit cost when
// the module is no-op.
func TestSetupSmoke_AlphaNoopBaseline(t *testing.T) {
	const (
		userAssetCounts uint32 = 5
		allAssetCounts  uint32 = 50
		batchCounts     uint32 = 2
	)

	compile := func(label string, withModule bool) int {
		t.Helper()
		c := NewBatchCreateUserCircuit(userAssetCounts, allAssetCounts, batchCounts)
		if withModule {
			c.SetConstraintModule(testNoopModule{})
		}
		start := time.Now()
		r1csObj, err := frontend.Compile(
			ecc.BN254.ScalarField(),
			r1cs.NewBuilder,
			c,
			frontend.IgnoreUnconstrainedInputs(),
		)
		if err != nil {
			t.Fatalf("%s compile: %v", label, err)
		}
		nb := r1csObj.GetNbConstraints()
		t.Logf("%s compile took %s, NbConstraints=%d", label, time.Since(start), nb)
		return nb
	}

	baseline := compile("nil-module", false)
	withNoop := compile("noop-module", true)
	if baseline != withNoop {
		t.Fatalf("alpha hook changed constraint count under no-op module: baseline=%d, noop=%d", baseline, withNoop)
	}
}
