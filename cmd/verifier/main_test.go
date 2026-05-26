package main

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"
)

// TestAssetCountTiers locks the verifier's view of the Binance
// deployment's per-batch asset tiers. -user mode pads a user's asset
// list to one of these before recomputing the leaf hash; a drift here
// silently breaks single-user verification.
func TestAssetCountTiers(t *testing.T) {
	got := assetCountTiers()
	want := []int{50, 500}
	if len(got) != len(want) {
		t.Fatalf("assetCountTiers length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("assetCountTiers[%d] = %d, want %d", i, got[i], want[i])
		}
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
