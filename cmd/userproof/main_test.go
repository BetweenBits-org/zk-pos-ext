package main

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	t4host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host"
	t4spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
)

// TestTiersFromShapes_T4Reference locks the T4 reference tier view via
// the declarative profile (same source-of-truth as the witness; a drift
// breaks the tree-root parity that ties the two services together).
func TestTiersFromShapes_T4Reference(t *testing.T) {
	os.Unsetenv("ZKPOR_BATCH_SHAPE_OVERRIDE")
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

// TestBuildUserProofRow_ConfigRoundTrip locks the userproof→verifier
// contract for T4: the JSON payload embedded in UserProof.Config
// unmarshals back into t4host.UserConfig with every field preserved
// (including raw-byte Proof entries and *big.Int totals). Phase 3e
// moved BuildUserProofRow into the runner — test calls the runner
// directly.
func TestBuildUserProofRow_ConfigRoundTrip(t *testing.T) {
	account := &t4spec.AccountInfo{
		AccountIndex:    42,
		AccountID:       bytesPattern(0xab, 32),
		TotalEquity:     new(big.Int).SetUint64(1_000_000),
		TotalDebt:       new(big.Int).SetUint64(250_000),
		TotalCollateral: new(big.Int).SetUint64(500_000),
		Assets: []t4spec.AccountAsset{
			{Index: 0, Equity: 100, Debt: 30, Loan: 20, Margin: 0, PortfolioMargin: 10},
			{Index: 5, Equity: 5_000_000, Loan: 800_000, PortfolioMargin: 50},
		},
	}
	leaf := bytesPattern(0xcd, 32)
	proof := [][]byte{
		bytesPattern(0x01, 32),
		bytesPattern(0x02, 32),
		bytesPattern(0x03, 32),
	}
	rootHex := "deadbeef00000000000000000000000000000000000000000000000000000000"

	row, err := t4host.BuildUserProofRow(account, leaf, proof, rootHex)
	if err != nil {
		t.Fatalf("BuildUserProofRow: %v", err)
	}

	var got t4host.UserConfig
	if err := json.Unmarshal([]byte(row.Config), &got); err != nil {
		t.Fatalf("unmarshal Config: %v", err)
	}

	if got.AccountIndex != account.AccountIndex {
		t.Fatalf("AccountIndex = %d, want %d", got.AccountIndex, account.AccountIndex)
	}
	if got.Root != rootHex {
		t.Fatalf("Root = %q, want %q", got.Root, rootHex)
	}
	if got.TotalEquity.Cmp(account.TotalEquity) != 0 ||
		got.TotalDebt.Cmp(account.TotalDebt) != 0 ||
		got.TotalCollateral.Cmp(account.TotalCollateral) != 0 {
		t.Fatalf("totals round-trip mismatch: got=(%s,%s,%s) want=(%s,%s,%s)",
			got.TotalEquity, got.TotalDebt, got.TotalCollateral,
			account.TotalEquity, account.TotalDebt, account.TotalCollateral)
	}
	if len(got.Proof) != len(proof) {
		t.Fatalf("Proof length = %d, want %d", len(got.Proof), len(proof))
	}
	for i := range proof {
		if !bytes.Equal(got.Proof[i], proof[i]) {
			t.Fatalf("Proof[%d] mismatch", i)
		}
	}
	if len(got.Assets) != len(account.Assets) {
		t.Fatalf("Assets length = %d, want %d", len(got.Assets), len(account.Assets))
	}
	for i := range account.Assets {
		if got.Assets[i] != account.Assets[i] {
			t.Fatalf("Assets[%d] mismatch: got=%+v want=%+v", i, got.Assets[i], account.Assets[i])
		}
	}
}

func bytesPattern(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}
