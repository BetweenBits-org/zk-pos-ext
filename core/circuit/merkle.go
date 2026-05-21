package circuit

import (
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/consensys/gnark/std/hash/poseidon"
)

// VerifyMerkleProof asserts that `node` is the leaf at the path
// described by `helper` inside the sparse Merkle tree rooted at
// `merkleRoot`. `helper` MUST be AccountIndexToMerkleHelper(idx).
// `proofSet` is the sibling at each level (len == AccountTreeDepth).
//
// Universal across all solvency models.
func VerifyMerkleProof(api API, merkleRoot Variable, node Variable, proofSet, helper []Variable) {
	for i := 0; i < len(proofSet); i++ {
		api.AssertIsBoolean(helper[i])
		d1 := api.Select(helper[i], proofSet[i], node)
		d2 := api.Select(helper[i], node, proofSet[i])
		node = poseidon.Poseidon(api, d1, d2)
	}
	api.AssertIsEqual(merkleRoot, node)
}

// UpdateMerkleProof returns the new tree root after replacing the
// leaf at the path described by `helper` with `node`. `helper` MUST
// be AccountIndexToMerkleHelper(idx).
//
// Universal across all solvency models.
func UpdateMerkleProof(api API, node Variable, proofSet, helper []Variable) (root Variable) {
	for i := 0; i < len(proofSet); i++ {
		api.AssertIsBoolean(helper[i])
		d1 := api.Select(helper[i], proofSet[i], node)
		d2 := api.Select(helper[i], node, proofSet[i])
		node = poseidon.Poseidon(api, d1, d2)
	}
	return node
}

// AccountIndexToMerkleHelper converts an account index into the
// boolean direction-bit vector consumed by VerifyMerkleProof /
// UpdateMerkleProof. Length equals spec.AccountTreeDepth.
//
// Universal across all solvency models.
func AccountIndexToMerkleHelper(api API, accountIndex Variable) []Variable {
	return api.ToBinary(accountIndex, spec.AccountTreeDepth)
}
