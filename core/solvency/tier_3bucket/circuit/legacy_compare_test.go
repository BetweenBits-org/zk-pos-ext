package circuit

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"testing"
	"time"

	legacy "github.com/binance/zkmerkle-proof-of-solvency/circuit"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint"
	cs_bn254 "github.com/consensys/gnark/constraint/bn254"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// TestLegacyCompare_R1CSStructure is the G1 byte-equivalence regression
// gate at tiny shape. The test compiles the legacy
// `github.com/binance/zkmerkle-proof-of-solvency/circuit`
// BatchCreateUserCircuit and the zkpor tier_3bucket
// BatchCreateUserCircuit at the same shape (5, 50, 2), then hashes
// each compiled R1CS's L·R == O constraint matrices — exactly the
// data groth16.Setup consumes to derive .pk / .vk. Equal hashes ⇒
// the legacy trusted-setup ceremony output can be reused with the
// zkpor circuit byte-for-byte, since (R1CS, toxic waste) → (pk, vk)
// is deterministic in gnark.
//
// What is NOT covered, deliberately:
//   - hint identifiers in the instruction stream's calldata. Hint
//     IDs are derived from the Go reflect path of the hint function,
//     so the legacy `circuit.IntegerDivision` (at
//     `github.com/binance/zkmerkle-proof-of-solvency/circuit`) and
//     the zkpor `corecircuit.IntegerDivision` (at
//     `…/zkpor/core/circuit`) carry different IDs. Hints are
//     solver-side metadata — they label which wire is computed by
//     which Go function at witness-solving time — and do not
//     contribute to A·s ∘ B·s = C·s. Each service that proves with
//     the new circuit MUST register zkpor's IntegerDivision with
//     gnark's solver; that wiring lands in R3 step 4.
//   - gnark's debug metadata (SymbolTable, DebugInfo, MDebug, Logs),
//     which encodes source file paths / line numbers and would
//     wrongly diverge here even though it does not affect .pk / .vk.
//
// Identical L/R/O term streams ⇒ identical R1CS matrices ⇒ identical
// .pk / .vk under any common toxic waste. Production shape
// (50, 500, 700) is NOT exercised in this permanent test — compile
// cost there is multi-minute and multi-GB — but equality at the
// tiny shape is sufficient evidence that Define is structurally
// identical, since Define is shape-agnostic (loop-driven) and the
// same logic emits the same per-iteration constraint pattern at
// every size. The one-shot production-shape comparison procedure
// is documented in PRODUCTION_ROADMAP G1.
//
// Skipped under -short to keep the inner-loop fast.
func TestLegacyCompare_R1CSStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping legacy-compare; rerun without -short")
	}
	const (
		userAssetCounts uint32 = 5
		allAssetCounts  uint32 = 50
		batchCounts     uint32 = 2
	)

	compile := func(label string, circ frontend.Circuit) constraint.ConstraintSystem {
		t.Helper()
		start := time.Now()
		cs, err := frontend.Compile(
			ecc.BN254.ScalarField(),
			r1cs.NewBuilder,
			circ,
			frontend.IgnoreUnconstrainedInputs(),
		)
		if err != nil {
			t.Fatalf("%s compile: %v", label, err)
		}
		t.Logf("%s compile took %s, NbConstraints=%d", label, time.Since(start), cs.GetNbConstraints())
		return cs
	}

	legacyCS := compile("legacy", legacy.NewBatchCreateUserCircuit(userAssetCounts, allAssetCounts, batchCounts))
	zkporCS := compile("zkpor", NewBatchCreateUserCircuit(userAssetCounts, allAssetCounts, batchCounts))

	// Fail-fast on the cheap counts before walking the instruction stream.
	requireCountEqual(t, "NbConstraints", legacyCS.GetNbConstraints(), zkporCS.GetNbConstraints())
	requireCountEqual(t, "NbInstructions", legacyCS.GetNbInstructions(), zkporCS.GetNbInstructions())
	requireCountEqual(t, "NbInternalVariables", legacyCS.GetNbInternalVariables(), zkporCS.GetNbInternalVariables())
	requireCountEqual(t, "NbSecretVariables", legacyCS.GetNbSecretVariables(), zkporCS.GetNbSecretVariables())
	requireCountEqual(t, "NbPublicVariables", legacyCS.GetNbPublicVariables(), zkporCS.GetNbPublicVariables())
	requireCountEqual(t, "NbCoefficients", legacyCS.GetNbCoefficients(), zkporCS.GetNbCoefficients())

	legacyR1Cs := bn254R1Cs(t, legacyCS)
	zkporR1Cs := bn254R1Cs(t, zkporCS)
	requireCountEqual(t, "len(GetR1Cs)", len(legacyR1Cs), len(zkporR1Cs))

	legacyR1csHash := hashR1Cs(legacyR1Cs)
	zkporR1csHash := hashR1Cs(zkporR1Cs)
	legacyCoeffHash := hashCoefficients(legacyCS)
	zkporCoeffHash := hashCoefficients(zkporCS)
	t.Logf("legacy R1CS sha256         = %s", hex.EncodeToString(legacyR1csHash))
	t.Logf("zkpor  R1CS sha256         = %s", hex.EncodeToString(zkporR1csHash))
	t.Logf("legacy coefficients sha256 = %s", hex.EncodeToString(legacyCoeffHash))
	t.Logf("zkpor  coefficients sha256 = %s", hex.EncodeToString(zkporCoeffHash))

	r1csMatch := hex.EncodeToString(legacyR1csHash) == hex.EncodeToString(zkporR1csHash)
	coeffMatch := hex.EncodeToString(legacyCoeffHash) == hex.EncodeToString(zkporCoeffHash)
	if !r1csMatch {
		reportFirstR1CDiff(t, legacyR1Cs, zkporR1Cs)
	}
	if !coeffMatch {
		reportFirstCoefficientDiff(t, legacyCS, zkporCS)
	}
	if !r1csMatch || !coeffMatch {
		t.Fatalf("legacy and zkpor diverge — port has drifted from reference (r1cs match=%v, coeff match=%v)", r1csMatch, coeffMatch)
	}
}

// bn254R1Cs unwraps a compiled ConstraintSystem into its underlying
// bn254 R1CS and returns the L·R == O constraint slice. Fails the
// test with a clear message if the system isn't a bn254 R1CS (e.g.
// because the field was changed).
func bn254R1Cs(t *testing.T, cs constraint.ConstraintSystem) []constraint.R1C {
	t.Helper()
	r, ok := cs.(*cs_bn254.R1CS)
	if !ok {
		t.Fatalf("ConstraintSystem is not *bn254.R1CS (got %T)", cs)
	}
	return r.GetR1Cs()
}

// hashR1Cs is the canonical R1CS hash. Each constraint contributes
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

func reportFirstR1CDiff(t *testing.T, legacy, zkpor []constraint.R1C) {
	t.Helper()
	n := len(legacy)
	if len(zkpor) < n {
		n = len(zkpor)
	}
	for i := 0; i < n; i++ {
		if !linearExprEqual(legacy[i].L, zkpor[i].L) ||
			!linearExprEqual(legacy[i].R, zkpor[i].R) ||
			!linearExprEqual(legacy[i].O, zkpor[i].O) {
			t.Logf("first R1C diff at index %d:", i)
			t.Logf("  legacy: L=%v R=%v O=%v", legacy[i].L, legacy[i].R, legacy[i].O)
			t.Logf("  zkpor : L=%v R=%v O=%v", zkpor[i].L, zkpor[i].R, zkpor[i].O)
			return
		}
	}
}

func linearExprEqual(a, b constraint.LinearExpression) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].CID != b[i].CID || a[i].VID != b[i].VID {
			return false
		}
	}
	return true
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

func reportFirstCoefficientDiff(t *testing.T, legacyCS, zkporCS constraint.ConstraintSystem) {
	t.Helper()
	nCoeff := legacyCS.GetNbCoefficients()
	if zkporCS.GetNbCoefficients() < nCoeff {
		nCoeff = zkporCS.GetNbCoefficients()
	}
	for i := 0; i < nCoeff; i++ {
		l := legacyCS.GetCoefficient(i)
		z := zkporCS.GetCoefficient(i)
		lb := l.Bytes()
		zb := z.Bytes()
		if lb != zb {
			t.Logf("first coefficient diff at index %d: legacy=%x zkpor=%x", i, lb[:], zb[:])
			return
		}
	}
}


// requireCountEqual fails the test fast with both values labelled when
// a simple integer-count check across the two systems diverges. Used
// before the (more expensive) structural hash so divergences land on
// the smallest, most diagnosable signal.
func requireCountEqual(t *testing.T, name string, legacy, zkpor int) {
	t.Helper()
	if legacy != zkpor {
		t.Fatalf("%s diverged: legacy=%d, zkpor=%d", name, legacy, zkpor)
	}
}


// writeUint64 writes one little-endian uint64 into h, panicking on
// the (impossible) hash.Hash write failure path.
func writeUint64(h hash.Hash, v uint64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	if _, err := h.Write(buf[:]); err != nil {
		panic(err)
	}
}
