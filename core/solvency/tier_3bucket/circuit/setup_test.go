package circuit

import (
	"testing"
	"time"

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
