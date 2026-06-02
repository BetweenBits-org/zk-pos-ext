// merklesum.go is the off-circuit verifier for the dense Merkle sum tree
// behind the non-zk Merkle-sum proof-of-liabilities side line
// (PRODUCTION_ROADMAP Stage MS, gate G19). It is the sum-carrying analogue
// of merkle.go's VerifyMerkleProof: where the account-tree verifier proves
// only "this leaf is at this path under this root", VerifyMerkleSumProof
// additionally proves "this leaf's balance flows into the published total"
// by walking the (hash, sum) sibling chain.

package host

import (
	"bytes"
	"math/big"

	"github.com/BetweenBits-org/zk-pos-ext/core/sumtree"
)

// VerifyMerkleSumProof walks the sibling chain from leaf up to the implied
// root, combining (hash, sum) pairs via sumtree.HashNode, and reports
// whether the result equals (rootHash, rootSum).
//
// index is the leaf's positional path expressed LSB-first: bit i selects
// the running node's side at level i, where 0 means it is the LEFT child
// (sibling on the right) and 1 the RIGHT child. siblings must be ordered
// leaf-to-root (siblings[0] is the leaf's immediate sibling), exactly as
// sumtree.Tree.Proof emits them.
//
// Any negative or nil sum (leaf or sibling) makes the proof invalid: a
// negative balance is the canonical Merkle-sum forgery — it shrinks the
// apparent total — so it is rejected here in addition to the auditor-side
// CheckNonNegative over the full leaf set. A nil rootSum is also rejected.
//
// Universal across solvency models: the Merkle-sum line is T1-only by
// product scope (gate G19, D3), but the verifier itself is model-blind —
// it consumes only (hash, sum) nodes.
func VerifyMerkleSumProof(rootHash []byte, rootSum *big.Int, index int, leaf sumtree.Node, siblings []sumtree.Node) bool {
	if rootSum == nil || leaf.Sum == nil || leaf.Sum.Sign() < 0 || len(leaf.Hash) != 32 {
		return false
	}
	cur := sumtree.Node{Hash: leaf.Hash, Sum: new(big.Int).Set(leaf.Sum)}
	for l, sib := range siblings {
		if sib.Sum == nil || sib.Sum.Sign() < 0 || len(sib.Hash) != 32 {
			return false
		}
		if (index>>l)&1 == 0 {
			cur = sumtree.HashNode(cur, sib)
		} else {
			cur = sumtree.HashNode(sib, cur)
		}
	}
	return bytes.Equal(cur.Hash, rootHash) && cur.Sum.Cmp(rootSum) == 0
}
