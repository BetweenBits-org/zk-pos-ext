package tree_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/BetweenBits-org/zk-pos-ext/core/host"
	"github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/BetweenBits-org/zk-pos-ext/core/tree"
)

// TestNewAccountTree_MemoryRoundTrip exercises the smallest meaningful
// path: build a memory-backed tree, write one leaf, fetch its proof,
// commit a version, and verify the proof with the engine's off-circuit
// Merkle verifier. Ties tree construction and proof verification at
// the same SMT shape (corespec.AccountTreeDepth, EmptyAccountLeafHash).
func TestNewAccountTree_MemoryRoundTrip(t *testing.T) {
	at, err := tree.NewAccountTree("memory", "")
	if err != nil {
		t.Fatalf("NewAccountTree memory: %v", err)
	}

	const accountIndex uint64 = 42
	leaf := modSafeLeaf(0xabcd1234)
	if err := at.Set(accountIndex, leaf); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := at.Commit(nil); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	root := at.Root()
	if len(root) != 32 {
		t.Fatalf("root length = %d, want 32", len(root))
	}
	if bytes.Equal(root, tree.EmptyAccountLeafHash) {
		t.Fatalf("root equals empty leaf; expected a non-trivial root after Set")
	}

	proof, err := at.GetProof(accountIndex)
	if err != nil {
		t.Fatalf("GetProof: %v", err)
	}
	if len(proof) != spec.AccountTreeDepth {
		t.Fatalf("proof length = %d, want %d", len(proof), spec.AccountTreeDepth)
	}

	if !host.VerifyMerkleProof(root, uint32(accountIndex), proof, leaf) {
		t.Fatal("host.VerifyMerkleProof rejected a tree-produced proof")
	}
}

// TestNewAccountTree_UnknownDriver confirms unknown drivers return an
// error instead of silently producing a misconfigured tree (legacy
// fell through with a nil backend).
func TestNewAccountTree_UnknownDriver(t *testing.T) {
	if _, err := tree.NewAccountTree("postgres", ""); err == nil {
		t.Fatal("expected error for unknown driver, got nil")
	}
}

// TestEmptyAccountLeafHash_Length confirms the engine's empty-leaf
// hash is the canonical 32-byte BN254 field element representation —
// the SMT depends on this shape.
func TestEmptyAccountLeafHash_Length(t *testing.T) {
	if len(tree.EmptyAccountLeafHash) != 32 {
		t.Fatalf("EmptyAccountLeafHash length = %d, want 32", len(tree.EmptyAccountLeafHash))
	}
}

// modSafeLeaf returns a deterministic 32-byte slice whose big-endian
// integer is < fr.Modulus(). The tree's poseidon hasher silently
// drops inputs >= modulus, so the test leaf must respect the
// constraint (production leaves do — they come from poseidon outputs).
func modSafeLeaf(seed uint64) []byte {
	var out [32]byte
	binary.BigEndian.PutUint64(out[24:], seed)
	// out[0..23] are zero → value == seed → well under fr.Modulus.
	return out[:]
}
