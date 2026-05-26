// Package host contains off-circuit (native) helpers used by zkpor
// services to compute hashes and verify proofs without going through
// gnark's frontend. Helpers in this package are universal across
// solvency models. Model-specific off-circuit helpers (e.g. t4_tiered_haircut_margin_3pool
// commitment packing) live under each model's sibling host subpackage
// such as zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host.
package host

import (
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// VerifyMerkleProof traverses the sibling chain in proof from the leaf
// node up to the implied root and reports whether it matches root.
// accountIndex is the leaf's path expressed LSB-first: bit i selects
// the sibling at level i, where 0 means the node is the left child and
// 1 the right.
//
// proof MUST have length spec.AccountTreeDepth — a shorter or longer
// proof returns false without hashing. Byte-equivalent to the legacy
// src/utils/account_tree.go VerifyMerkleProof, modulo the legacy's
// per-level fmt.Println debug output (dropped here).
//
// Universal across solvency models — the sparse Merkle tree shape is
// part of the engine standard (spec.AccountTreeDepth) and Poseidon over
// BN254 is the engine's only host hasher.
func VerifyMerkleProof(root []byte, accountIndex uint32, proof [][]byte, node []byte) bool {
	if len(proof) != spec.AccountTreeDepth {
		return false
	}
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
	return string(node) == string(root)
}
