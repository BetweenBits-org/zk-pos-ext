package verifier

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
)

// loadProfileFixture reads a reference profile.toml for the resolve
// tests. R12-EF: the engine takes a parsed *declarative.Profile, so the
// path read lives in the test (mirroring cmd/verifier's declarative.Load).
func loadProfileFixture(t *testing.T, path string) *declarative.Profile {
	t.Helper()
	prof, err := declarative.Load(path)
	if err != nil {
		t.Fatalf("declarative.Load(%q): %v", path, err)
	}
	return prof
}

// TestResolveFromProfile_T4Reference locks the verifier's derivation of
// asset capacity / tiers / .vk stems from the T4 reference profile.
// User mode uses plan.AssetCountTiers to pad a user's asset list;
// batch mode uses plan.ZkKeyStems + plan.AssetCapacity. Phase 3d adds
// the model selector — t4_reference.toml resolves to T4.
//
// R12-EF: the stems are now LOGICAL identifiers (provider.KeyName only);
// the directory join moved into the vfs.KeyOpener (covered by the osvfs
// test). So the plan carries no "/keys" prefix.
func TestResolveFromProfile_T4Reference(t *testing.T) {
	r, err := resolveFromProfile(Options{
		Profile: loadProfileFixture(t, "../../profile/t4_reference/t4_reference.toml"),
	})
	if err != nil {
		t.Fatalf("resolveFromProfile: %v", err)
	}
	if r.model != "t4_tiered_haircut_margin_3pool" {
		t.Errorf("model = %q, want t4_tiered_haircut_margin_3pool", r.model)
	}
	if r.plan.AssetCapacity != 500 {
		t.Errorf("AssetCapacity = %d, want 500", r.plan.AssetCapacity)
	}
	wantTiers := []int{50, 500}
	for i := range wantTiers {
		if r.plan.AssetCountTiers[i] != wantTiers[i] {
			t.Errorf("tier[%d] = %d, want %d", i, r.plan.AssetCountTiers[i], wantTiers[i])
		}
	}
	wantStems := []string{
		"zkpor.t4_tiered_haircut_margin_3pool.50_700",
		"zkpor.t4_tiered_haircut_margin_3pool.500_92",
	}
	for i := range wantStems {
		if r.plan.ZkKeyStems[i] != wantStems[i] {
			t.Errorf("stem[%d] = %q, want %q", i, r.plan.ZkKeyStems[i], wantStems[i])
		}
	}
}

// TestResolveFromProfile_CapacityOverride confirms CapacityOverride
// supersedes profile.asset_capacity (smoke harness behaviour).
func TestResolveFromProfile_CapacityOverride(t *testing.T) {
	r, err := resolveFromProfile(Options{
		Profile:          loadProfileFixture(t, "../../profile/t4_reference/t4_reference.toml"),
		CapacityOverride: 5,
	})
	if err != nil {
		t.Fatalf("resolveFromProfile: %v", err)
	}
	if r.plan.AssetCapacity != 5 {
		t.Errorf("override ignored: AssetCapacity = %d, want 5", r.plan.AssetCapacity)
	}
}

// TestResolveFromProfile_T1Reference locks T1 dispatch via the T1
// reference profile. Confirms Phase 3d removed the T4-only guard.
func TestResolveFromProfile_T1Reference(t *testing.T) {
	r, err := resolveFromProfile(Options{
		Profile: loadProfileFixture(t, "../../profile/t1_reference/t1_reference.toml"),
	})
	if err != nil {
		t.Fatalf("resolveFromProfile: %v", err)
	}
	if r.model != "t1_simple_margin" {
		t.Errorf("model = %q, want t1_simple_margin", r.model)
	}
}

// TestResolveFromProfile_NilProfile confirms the engine rejects a
// missing profile rather than panicking — cmd/verifier loads it lazily,
// so a programming error (nil Profile) must surface as an error.
func TestResolveFromProfile_NilProfile(t *testing.T) {
	if _, err := resolveFromProfile(Options{}); err == nil {
		t.Fatal("expected error for nil Profile")
	}
}

// TestRunHash_NoProfileRequired locks the -hash adversarial contract:
// RunHash is model-blind and consumes neither Profile, Keys, Config nor
// any IO. `verifier -hash A B` with an EMPTY -profile must still
// succeed, so the engine entry point must not touch Options at all.
func TestRunHash_NoProfileRequired(t *testing.T) {
	// Two valid base64 inputs that decode to small (valid) bn254 field
	// elements; no Options, no profile, no keys.
	a := base64.StdEncoding.EncodeToString([]byte{0x01})
	b := base64.StdEncoding.EncodeToString([]byte{0x02})
	if err := RunHash(a, b); err != nil {
		t.Fatalf("RunHash with no profile: %v", err)
	}
}

// TestRunHash_RejectsBadBase64 ensures a non-base64 argument surfaces an
// error rather than hashing garbage.
func TestRunHash_RejectsBadBase64(t *testing.T) {
	if err := RunHash("!not-base64!", "AAAA"); err == nil {
		t.Fatal("expected error for bad arg0 base64")
	}
}

// TestRunBatch_MissingKeys confirms RunBatch rejects a nil Keys opener
// before doing any work — cmd/verifier always injects one, so a nil is a
// wiring error that must surface as an error, not a nil-deref.
func TestRunBatch_MissingKeys(t *testing.T) {
	err := RunBatch(context.Background(), Options{
		Profile: loadProfileFixture(t, "../../profile/t4_reference/t4_reference.toml"),
	})
	if err == nil {
		t.Fatal("expected error for nil Keys")
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

	row := corehost.ProofRow{
		AccountTreeRoots: []string{
			base64.StdEncoding.EncodeToString(rootBefore),
			base64.StdEncoding.EncodeToString(rootAfter),
		},
		CexAssetCommitment: []string{
			base64.StdEncoding.EncodeToString(commitBefore),
			base64.StdEncoding.EncodeToString(commitAfter),
		},
	}

	roots, commits, err := decodeBatchMetadata(row)
	if err != nil {
		t.Fatalf("decodeBatchMetadata: %v", err)
	}
	if string(roots[0]) != string(rootBefore) || string(roots[1]) != string(rootAfter) {
		t.Fatalf("account tree roots mismatch: got %q,%q", roots[0], roots[1])
	}
	if string(commits[0]) != string(commitBefore) || string(commits[1]) != string(commitAfter) {
		t.Fatalf("cex commitments mismatch: got %q,%q", commits[0], commits[1])
	}
}

// TestDecodeBatchMetadataRejectsBadBase64 ensures a corrupted row
// surfaces a decode error rather than silently producing empty bytes.
func TestDecodeBatchMetadataRejectsBadBase64(t *testing.T) {
	row := corehost.ProofRow{
		AccountTreeRoots: []string{"!not-base64!", "AAAA"},
	}
	if _, _, err := decodeBatchMetadata(row); err == nil {
		t.Fatal("expected error for bad account tree root base64")
	}
	row = corehost.ProofRow{
		AccountTreeRoots:   []string{"AAAA", "AAAA"},
		CexAssetCommitment: []string{"!not-base64!", "AAAA"},
	}
	if _, _, err := decodeBatchMetadata(row); err == nil {
		t.Fatal("expected error for bad cex commitment base64")
	}
}

// TestConvertStoredProof confirms a corehost.ProofDTO row maps to the
// proof row shape the verifier downstream consumes — including JSON
// decode of the two [][]byte → []base64-string fields the prover writes.
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

	row := corehost.ProofDTO{
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
	// CSV path and DB path are byte-equivalent at the ProofRow seam.
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
	row := corehost.ProofDTO{CexAssetListCommitments: "{not json"}
	if _, err := convertStoredProof(row); err == nil {
		t.Fatal("expected error for bad cex commitments JSON")
	}
	row = corehost.ProofDTO{
		CexAssetListCommitments: "[]",
		AccountTreeRoots:        "{not json",
	}
	if _, err := convertStoredProof(row); err == nil {
		t.Fatal("expected error for bad account roots JSON")
	}
}
