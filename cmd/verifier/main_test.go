package main

import (
	"encoding/base64"
	"testing"
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
