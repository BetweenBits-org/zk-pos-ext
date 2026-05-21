package host_test

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"

	legacyutils "github.com/binance/zkmerkle-proof-of-solvency/src/utils"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// computeRoot walks the same hash chain VerifyMerkleProof walks, so the
// (root, proof, leaf) triple it returns is by construction accepted by
// any correct implementation. Used by tests below to fabricate a
// well-formed proof without standing up a full Merkle tree.
func computeRoot(t *testing.T, accountIndex uint32, proof [][]byte, leaf []byte) []byte {
	t.Helper()
	node := append([]byte(nil), leaf...)
	hasher := poseidon.NewPoseidon()
	for i := range spec.AccountTreeDepth {
		bit := accountIndex & (1 << i)
		if bit == 0 {
			hasher.Write(node)
			hasher.Write(proof[i])
		} else {
			hasher.Write(proof[i])
			hasher.Write(node)
		}
		node = hasher.Sum(nil)
		hasher.Reset()
	}
	return node
}

func fabricateProof(seed uint32) (proof [][]byte, leaf []byte) {
	leaf = modSafeBytes(seed, 0)
	proof = make([][]byte, spec.AccountTreeDepth)
	for i := range spec.AccountTreeDepth {
		proof[i] = modSafeBytes(seed, uint32(i)+1)
	}
	return proof, leaf
}

// modSafeBytes returns a deterministic 32-byte slice whose big-endian
// integer value is strictly less than fr.Modulus(). gnark-crypto's
// bn254 Poseidon Write silently drops inputs >= modulus, so any
// hash-chain test fixture must respect this constraint to exercise
// the algorithm faithfully. Production proofs always satisfy it
// (their bytes come from prior Poseidon outputs, which are fr.Element
// canonical bytes).
func modSafeBytes(a, b uint32) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint32(buf[0:], a)
	binary.BigEndian.PutUint32(buf[4:], b)
	h := sha256.Sum256(buf[:])
	// Clear top 3 bits → value < 2^253 < fr.Modulus() (~2^253.86).
	h[0] &= 0x1F
	return h[:]
}

// TestVerifyMerkleProof_AcceptsValid confirms a well-formed proof
// (root computed by walking the same chain) verifies successfully.
func TestVerifyMerkleProof_AcceptsValid(t *testing.T) {
	const accountIndex uint32 = 0x0a5f3c91 & ((1 << spec.AccountTreeDepth) - 1)
	proof, leaf := fabricateProof(42)
	root := computeRoot(t, accountIndex, proof, leaf)

	if !host.VerifyMerkleProof(root, accountIndex, proof, leaf) {
		t.Fatalf("host.VerifyMerkleProof rejected a well-formed proof")
	}
}

// TestVerifyMerkleProof_RejectsTampered ensures any single mutation
// (proof byte / leaf byte / root byte / index bit / proof length)
// invalidates the proof.
func TestVerifyMerkleProof_RejectsTampered(t *testing.T) {
	const accountIndex uint32 = 0x0a5f3c91 & ((1 << spec.AccountTreeDepth) - 1)
	proof, leaf := fabricateProof(7)
	root := computeRoot(t, accountIndex, proof, leaf)

	t.Run("flip one proof byte", func(t *testing.T) {
		mutProof := cloneProof(proof)
		mutProof[10][0] ^= 0x01
		if host.VerifyMerkleProof(root, accountIndex, mutProof, leaf) {
			t.Fatal("tampered sibling accepted")
		}
	})
	t.Run("flip one leaf byte", func(t *testing.T) {
		mutLeaf := append([]byte(nil), leaf...)
		mutLeaf[0] ^= 0x01
		if host.VerifyMerkleProof(root, accountIndex, proof, mutLeaf) {
			t.Fatal("tampered leaf accepted")
		}
	})
	t.Run("flip one root byte", func(t *testing.T) {
		mutRoot := append([]byte(nil), root...)
		mutRoot[0] ^= 0x01
		if host.VerifyMerkleProof(mutRoot, accountIndex, proof, leaf) {
			t.Fatal("tampered root accepted")
		}
	})
	t.Run("flip one index bit", func(t *testing.T) {
		// Flipping a path bit picks the wrong child at that level, so
		// the recomputed root won't match.
		if host.VerifyMerkleProof(root, accountIndex^1, proof, leaf) {
			t.Fatal("tampered index accepted")
		}
	})
	t.Run("short proof", func(t *testing.T) {
		short := make([][]byte, spec.AccountTreeDepth-1)
		copy(short, proof)
		if host.VerifyMerkleProof(root, accountIndex, short, leaf) {
			t.Fatal("short proof accepted")
		}
	})
}

// TestVerifyMerkleProof_LegacyParity verifies that host.VerifyMerkleProof
// returns the same bool as legacy utils.VerifyMerkleProof for both the
// happy and one tamper case. Locks in byte-level equivalence at the
// algorithm boundary (Poseidon over BN254, same level ordering, same
// path-bit convention).
func TestVerifyMerkleProof_LegacyParity(t *testing.T) {
	const accountIndex uint32 = 0x12345 & ((1 << spec.AccountTreeDepth) - 1)
	proof, leaf := fabricateProof(99)
	root := computeRoot(t, accountIndex, proof, leaf)

	if got, want := host.VerifyMerkleProof(root, accountIndex, proof, leaf), legacyutils.VerifyMerkleProof(root, accountIndex, proof, leaf); got != want {
		t.Fatalf("host vs legacy disagree on valid proof: host=%v legacy=%v", got, want)
	}

	mutProof := cloneProof(proof)
	mutProof[3][1] ^= 0x80
	if got, want := host.VerifyMerkleProof(root, accountIndex, mutProof, leaf), legacyutils.VerifyMerkleProof(root, accountIndex, mutProof, leaf); got != want {
		t.Fatalf("host vs legacy disagree on tampered proof: host=%v legacy=%v", got, want)
	}
}

func cloneProof(p [][]byte) [][]byte {
	out := make([][]byte, len(p))
	for i := range p {
		out[i] = append([]byte(nil), p[i]...)
	}
	return out
}
