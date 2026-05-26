package main

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"
)

// TestResolveFromProfile_Binance locks the verifier's derivation of
// asset capacity / tiers / .vk stems from the binance reference
// profile. -user mode uses plan.assetCountTiers to pad a user's
// asset list; batch mode uses plan.zkKeyStems + plan.assetCapacity.
func TestResolveFromProfile_Binance(t *testing.T) {
	plan, err := resolveFromProfile(&pflags{
		profilePath: "../../profile/binance/binance.toml",
		keysDir:     "/keys",
	})
	if err != nil {
		t.Fatalf("resolveFromProfile: %v", err)
	}
	if plan.assetCapacity != 500 {
		t.Errorf("assetCapacity = %d, want 500", plan.assetCapacity)
	}
	wantTiers := []int{50, 500}
	for i := range wantTiers {
		if plan.assetCountTiers[i] != wantTiers[i] {
			t.Errorf("tier[%d] = %d, want %d", i, plan.assetCountTiers[i], wantTiers[i])
		}
	}
	wantStems := []string{
		"/keys/zkpor.t4_tiered_haircut_margin_3pool.50_700",
		"/keys/zkpor.t4_tiered_haircut_margin_3pool.500_92",
	}
	for i := range wantStems {
		if plan.zkKeyStems[i] != wantStems[i] {
			t.Errorf("stem[%d] = %q, want %q", i, plan.zkKeyStems[i], wantStems[i])
		}
	}
}

// TestResolveFromProfile_CapacityOverride confirms -asset-capacity
// supersedes profile.asset_capacity (smoke harness behaviour).
func TestResolveFromProfile_CapacityOverride(t *testing.T) {
	plan, err := resolveFromProfile(&pflags{
		profilePath: "../../profile/binance/binance.toml",
		keysDir:     "/keys",
		capacity:    5,
	})
	if err != nil {
		t.Fatalf("resolveFromProfile: %v", err)
	}
	if plan.assetCapacity != 5 {
		t.Errorf("override ignored: assetCapacity = %d, want 5", plan.assetCapacity)
	}
}

// TestDecodeBatchMetadata confirms the base64 account-tree-root and
// cex-commitment pairs of a proof row round-trip back to their raw
// bytes in order.
func TestDecodeBatchMetadata(t *testing.T) {
	rootBefore := []byte("account-root-before-32-bytes....")
	rootAfter := []byte("account-root-after-32-bytes.....")
	commitBefore := []byte("cex-commit-before-32-bytes......")
	commitAfter := []byte("cex-commit-after-32-bytes.......")

	row := proofRow{
		AccountTreeRoots: []string{
			base64.StdEncoding.EncodeToString(rootBefore),
			base64.StdEncoding.EncodeToString(rootAfter),
		},
		CexAssetCommitment: []string{
			base64.StdEncoding.EncodeToString(commitBefore),
			base64.StdEncoding.EncodeToString(commitAfter),
		},
	}

	roots, commits := decodeBatchMetadata(row)
	if string(roots[0]) != string(rootBefore) || string(roots[1]) != string(rootAfter) {
		t.Fatalf("account tree roots mismatch: got %q,%q", roots[0], roots[1])
	}
	if string(commits[0]) != string(commitBefore) || string(commits[1]) != string(commitAfter) {
		t.Fatalf("cex commitments mismatch: got %q,%q", commits[0], commits[1])
	}
}

// TestConvertStoredProof confirms a store.Proof row maps to the proof
// row shape the verifier downstream consumes — including JSON decode
// of the two [][]byte → []base64-string fields the prover writes.
func TestConvertStoredProof(t *testing.T) {
	rootBefore := []byte("account-root-before-32-bytes....")
	rootAfter := []byte("account-root-after-32-bytes.....")
	commitBefore := []byte("cex-commit-before-32-bytes......")
	commitAfter := []byte("cex-commit-after-32-bytes.......")

	cexJSON, err := json.Marshal([][]byte{commitBefore, commitAfter})
	if err != nil {
		t.Fatalf("marshal cex: %v", err)
	}
	rootsJSON, err := json.Marshal([][]byte{rootBefore, rootAfter})
	if err != nil {
		t.Fatalf("marshal roots: %v", err)
	}

	row := store.Proof{
		ProofInfo:               "proof-blob-base64",
		BatchCommitment:         "batch-commit-base64",
		AssetsCount:             5,
		BatchNumber:             7,
		CexAssetListCommitments: string(cexJSON),
		AccountTreeRoots:        string(rootsJSON),
	}

	got, err := convertStoredProof(row)
	if err != nil {
		t.Fatalf("convertStoredProof: %v", err)
	}
	if got.BatchNumber != 7 || got.AssetsCount != 5 {
		t.Fatalf("scalar fields: got %+v", got)
	}
	if got.ZkProof != "proof-blob-base64" || got.BatchCommitment != "batch-commit-base64" {
		t.Fatalf("string fields: got %+v", got)
	}
	if len(got.CexAssetCommitment) != 2 || len(got.AccountTreeRoots) != 2 {
		t.Fatalf("slice lengths: cex=%d roots=%d", len(got.CexAssetCommitment), len(got.AccountTreeRoots))
	}

	// json.Marshal of [][]byte encodes each entry as a base64 string.
	// The verifier's decodeBatchMetadata base64-decodes the same — i.e.
	// CSV path and DB path are byte-equivalent at the proofRow seam.
	if got.CexAssetCommitment[0] != base64.StdEncoding.EncodeToString(commitBefore) {
		t.Fatalf("cex[0]: got %q", got.CexAssetCommitment[0])
	}
	if got.AccountTreeRoots[1] != base64.StdEncoding.EncodeToString(rootAfter) {
		t.Fatalf("roots[1]: got %q", got.AccountTreeRoots[1])
	}
}

// TestConvertStoredProofRejectsBadJSON ensures a corrupted row surfaces
// a parse error rather than silently producing an empty slice.
func TestConvertStoredProofRejectsBadJSON(t *testing.T) {
	row := store.Proof{CexAssetListCommitments: "{not json"}
	if _, err := convertStoredProof(row); err == nil {
		t.Fatal("expected error for bad cex commitments JSON")
	}
	row = store.Proof{
		CexAssetListCommitments: "[]",
		AccountTreeRoots:        "{not json",
	}
	if _, err := convertStoredProof(row); err == nil {
		t.Fatal("expected error for bad account roots JSON")
	}
}
