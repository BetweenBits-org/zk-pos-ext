package host

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"

	corecircuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/circuit"
	t1circuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/circuit"
	t1spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/tree"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// TestT1ProverEndToEnd reproduces the EC2 smoke fail path locally
// without going through the standard CSV connector / DB store. Builds
// a minimal in-memory testdata equivalent to the
// profile/t1_reference/testdata/happy/ shape, then:
//
//   build SMT → buildBatch → SetBatchCreateUserCircuitWitness →
//   frontend.NewWitness → groth16.Setup → groth16.Prove + Verify.
//
// On EC2 the smoke fails with "constraint #16677 is not satisfied" at
// Prove. This test exercises the same circuit + host code path so a
// local fail = R4+ regression we can git bisect; a local pass = the
// regression is in the snapshot/parser layer (account-id reduction,
// asset ordering, etc).
//
// Suppressed under -short — full Setup+Prove takes ~10-20s.
func TestT1ProverEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("end-to-end Setup+Prove smoke is heavy under -short")
	}

	const capacity = 5
	const userAssetCounts = 5
	const usersPerBatch = 10

	// 1. Build cex_assets: 3 real (btc/eth/usdt) + 2 reserved padding.
	//    Sum equality: per-asset Σ user.equity equals these.
	cexAssets := []t1spec.CexAssetInfo{
		{TotalEquity: 100, TotalDebt: 0, BasePrice: 6500000000000, Symbol: "btc", Index: 0},
		{TotalEquity: 1000, TotalDebt: 0, BasePrice: 350000000000, Symbol: "eth", Index: 1},
		{TotalEquity: 100000, TotalDebt: 0, BasePrice: 100000000, Symbol: "usdt", Index: 2},
		{TotalEquity: 0, TotalDebt: 0, BasePrice: 0, Symbol: "reserved", Index: 3},
		{TotalEquity: 0, TotalDebt: 0, BasePrice: 0, Symbol: "reserved", Index: 4},
	}

	// 2. Build 10 accounts × 3 assets each, mirroring the testdata
	//    pattern (account_id = 0x11..11 .. 0xaa..aa). AccountID is
	//    fr.Modulus-reduced (same as canonicalAccountID in the parser).
	accountIDPatterns := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa}
	accounts := make([]t1spec.AccountInfo, len(accountIDPatterns))
	for i, p := range accountIDPatterns {
		raw := make([]byte, 32)
		for k := range raw {
			raw[k] = p
		}
		canonical := new(fr.Element).SetBytes(raw).Marshal()
		accounts[i] = t1spec.AccountInfo{
			AccountIndex: uint32(i),
			AccountID:    canonical,
			TotalEquity:  big.NewInt(10 + 100 + 10000), // per-asset sum
			TotalDebt:    big.NewInt(0),
			Assets: []t1spec.AccountAsset{
				{Index: 0, Equity: 10, Debt: 0},
				{Index: 1, Equity: 100, Debt: 0},
				{Index: 2, Equity: 10000, Debt: 0},
			},
		}
	}
	t.Logf("synthesized %d accounts × 3 assets each", len(accounts))

	// 3. Pad to batch boundary (no-op here: 10 accounts == usersPerBatch).
	assetCountTiers := []int{userAssetCounts}
	_, accounts = PaddingAccounts(accounts, userAssetCounts, len(accounts), usersPerBatch)
	if len(accounts) != usersPerBatch {
		t.Fatalf("after padding: got %d accounts, want %d", len(accounts), usersPerBatch)
	}

	// 4. Build SMT (memory driver).
	accountTree, err := tree.NewAccountTree("memory", "")
	if err != nil {
		t.Fatalf("NewAccountTree: %v", err)
	}
	t.Logf("empty tree root: %s", hex.EncodeToString(accountTree.Root()))

	// Make a defensive copy of cexAssets because buildBatch mutates it
	// (sum equality accumulation into the running AfterCex state).
	cexAssetsForBatch := make([]t1spec.CexAssetInfo, len(cexAssets))
	copy(cexAssetsForBatch, cexAssets)

	// 5. buildBatch — the same internal helper RunWitness uses for each
	//    real batch (skipping the DB store).
	_ = context.Background() // future helpers may need ctx
	witness, err := buildBatch(accounts, cexAssetsForBatch, accountTree, assetCountTiers)
	if err != nil {
		t.Fatalf("buildBatch: %v", err)
	}
	t.Logf("batch built: before=%x", witness.BeforeAccountTreeRoot)
	t.Logf("              after=%x", witness.AfterAccountTreeRoot)
	t.Logf("              commit=%x", witness.BatchCommitment)

	// 6. Convert to in-circuit witness shape.
	circuitWitness, err := t1circuit.SetBatchCreateUserCircuitWitness(witness, assetCountTiers)
	if err != nil {
		t.Fatalf("SetBatchCreateUserCircuitWitness: %v", err)
	}

	// 7. Compile + Setup the circuit shape.
	circuitShape := t1circuit.NewBatchCreateUserCircuit(userAssetCounts, capacity, usersPerBatch)
	r1csObj, err := frontend.Compile(
		ecc.BN254.ScalarField(),
		r1cs.NewBuilder,
		circuitShape,
		frontend.IgnoreUnconstrainedInputs(),
	)
	if err != nil {
		t.Fatalf("frontend.Compile: %v", err)
	}
	t.Logf("compiled: NbConstraints=%d", r1csObj.GetNbConstraints())

	pk, vk, err := groth16.Setup(r1csObj)
	if err != nil {
		t.Fatalf("groth16.Setup: %v", err)
	}

	// 8. Register IntegerDivision hint (G1 service-side resolution).
	solver.RegisterHint(corecircuit.IntegerDivision)

	// 9. Build full + public witness.
	w, err := frontend.NewWitness(circuitWitness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("frontend.NewWitness: %v", err)
	}
	verifyCircuit := t1circuit.NewVerifyBatchCreateUserCircuit(witness.BatchCommitment)
	vw, err := frontend.NewWitness(verifyCircuit, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatalf("frontend.NewWitness (public): %v", err)
	}

	// 10. The defining check.
	proof, err := groth16.Prove(r1csObj, pk, w)
	if err != nil {
		t.Fatalf("groth16.Prove: %v", err)
	}
	t.Logf("proof generated successfully")

	if err := groth16.Verify(proof, vk, vw); err != nil {
		t.Fatalf("groth16.Verify: %v", err)
	}
	t.Log("verify pass")
}
