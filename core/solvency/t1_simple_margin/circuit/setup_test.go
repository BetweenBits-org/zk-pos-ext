package circuit

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"testing"
	"time"

	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	t1spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/spec"
	"github.com/consensys/gnark-crypto/ecc"
	cs_bn254 "github.com/consensys/gnark/constraint/bn254"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// TestSetupSmoke compiles BatchCreateUserCircuit and runs groth16.Setup
// at a deliberately tiny shape (userAssetCounts=5, allAssetCounts=10,
// batchCounts=2). Purpose: surface IR-level defects (bad lookup-table
// shapes, malformed Define signatures, wrong field types, etc.) and
// record a baseline R1CS hash for the t1_simple_margin circuit.
//
// Unlike t4_tiered_haircut_margin_3pool's setup_test, there's no legacy reference to
// compare against — t1_simple_margin is a zkpor-native model. The R1CS hash
// logged here is the deployment artifact a future regression test (or
// audit) would lock against.
//
// batchCounts=2 is the minimum that still exercises the user-roots
// chaining check (op[i].After == op[i+1].Before) in Define().
func TestSetupSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping setup smoke in -short mode")
	}

	const (
		userAssetCounts uint32 = 5
		allAssetCounts  uint32 = 10
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
	nbConstraints := r1csObj.GetNbConstraints()
	t.Logf("compile took %s, NbConstraints=%d", time.Since(startCompile), nbConstraints)
	if nbConstraints == 0 {
		t.Fatal("Compile produced 0 constraints — circuit Define() likely didn't emit any AssertIsEqual")
	}

	// R1CS + coefficient hashes — baseline for future regression tests
	// or audit lock. No comparison target; just log them.
	r1cs := bn254R1Cs(t, r1csObj)
	t.Logf("R1CS sha256         = %s", hex.EncodeToString(hashR1Cs(r1cs)))
	t.Logf("coefficients sha256 = %s", hex.EncodeToString(hashCoefficients(r1csObj)))

	startSetup := time.Now()
	if _, _, err := groth16.Setup(r1csObj); err != nil {
		t.Fatalf("groth16.Setup: %v", err)
	}
	t.Logf("groth16.Setup took %s", time.Since(startSetup))
}

// TestSetupSmokeWithNoopModule sets a no-op ConstraintModule on the
// circuit and verifies the resulting NbConstraints matches the
// baseline (no module attached). The noopModule must emit zero
// in-circuit constraints — anything else is a regression.
//
// This is the alpha-layer regression guard that R3 step 0 did for
// t4_tiered_haircut_margin_3pool; same intent, model-typed.
func TestSetupSmokeWithNoopModule(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping setup smoke in -short mode")
	}

	const (
		userAssetCounts uint32 = 5
		allAssetCounts  uint32 = 10
		batchCounts     uint32 = 2
	)

	// Baseline: no module.
	cBaseline := NewBatchCreateUserCircuit(userAssetCounts, allAssetCounts, batchCounts)
	csBaseline, err := frontend.Compile(
		ecc.BN254.ScalarField(), r1cs.NewBuilder, cBaseline,
		frontend.IgnoreUnconstrainedInputs(),
	)
	if err != nil {
		t.Fatalf("baseline Compile: %v", err)
	}
	baseline := csBaseline.GetNbConstraints()

	// With noop module attached.
	cNoop := NewBatchCreateUserCircuit(userAssetCounts, allAssetCounts, batchCounts)
	cNoop.SetConstraintModule(noopModule{})
	csNoop, err := frontend.Compile(
		ecc.BN254.ScalarField(), r1cs.NewBuilder, cNoop,
		frontend.IgnoreUnconstrainedInputs(),
	)
	if err != nil {
		t.Fatalf("noop Compile: %v", err)
	}
	withNoop := csNoop.GetNbConstraints()

	if baseline != withNoop {
		t.Fatalf("NbConstraints drift with noop module: baseline=%d, withNoop=%d (must be equal)",
			baseline, withNoop)
	}
	t.Logf("noop module is zero-cost: NbConstraints=%d", baseline)
}

// noopModule is the in-package no-op ConstraintModule used to verify
// the alpha-layer hook is zero-cost. Mirrors profile/binance's
// equivalent type — to be promoted to core/constraint_modules/noop/
// in R4-4.
type noopModule struct{}

func (noopModule) ID() corespec.ConstraintModuleID { return corespec.ConstraintModuleID("test.noop") }
func (noopModule) Define(api frontend.API, ctx t1spec.ConstraintContext) error {
	_ = api
	_ = ctx
	return nil
}

// bn254R1Cs unwraps a compiled ConstraintSystem into its underlying
// bn254 R1CS and returns the L·R == O constraint slice.
func bn254R1Cs(t *testing.T, cs constraint.ConstraintSystem) []constraint.R1C {
	t.Helper()
	r, ok := cs.(*cs_bn254.R1CS)
	if !ok {
		t.Fatalf("ConstraintSystem is not *bn254.R1CS (got %T)", cs)
	}
	return r.GetR1Cs()
}

// hashR1Cs is the canonical R1CS hash: each constraint contributes
// three length-prefixed term streams (L, R, O), and each term writes
// its (CoeffID, WireID) pair. Identical bytes ⇔ identical A, B, C
// matrices over the same coefficient table ⇔ identical .pk / .vk
// under any common toxic waste.
func hashR1Cs(r1cs []constraint.R1C) []byte {
	h := sha256.New()
	writeUint64(h, uint64(len(r1cs)))
	for i := range r1cs {
		writeLinearExpr(h, r1cs[i].L)
		writeLinearExpr(h, r1cs[i].R)
		writeLinearExpr(h, r1cs[i].O)
	}
	return h.Sum(nil)
}

func writeLinearExpr(h hash.Hash, le constraint.LinearExpression) {
	writeUint64(h, uint64(len(le)))
	for _, t := range le {
		writeUint64(h, uint64(t.CID))
		writeUint64(h, uint64(t.VID))
	}
}

func hashCoefficients(cs constraint.ConstraintSystem) []byte {
	h := sha256.New()
	nCoeff := cs.GetNbCoefficients()
	writeUint64(h, uint64(nCoeff))
	for i := 0; i < nCoeff; i++ {
		el := cs.GetCoefficient(i)
		b := el.Bytes()
		writeUint64(h, uint64(len(b)))
		h.Write(b[:])
	}
	return h.Sum(nil)
}

func writeUint64(h hash.Hash, v uint64) {
	var b [8]byte
	for i := 0; i < 8; i++ {
		b[i] = byte(v >> (8 * (7 - i)))
	}
	h.Write(b[:])
}
