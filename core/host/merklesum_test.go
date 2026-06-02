package host

import (
	"math/big"
	"testing"

	"github.com/BetweenBits-org/zk-pos-ext/core/sumtree"
)

func ml(b byte, sum int64) sumtree.Leaf {
	h := make([]byte, 32)
	h[31] = b
	return sumtree.Leaf{Hash: h, Sum: big.NewInt(sum)}
}

func TestVerifyMerkleSumProof_AllLeavesPass(t *testing.T) {
	leaves := []sumtree.Leaf{ml(1, 100), ml(2, 250), ml(3, 50), ml(4, 75), ml(5, 25)}
	tr, err := sumtree.Build(leaves)
	if err != nil {
		t.Fatal(err)
	}
	root := tr.Root()
	for i, lf := range leaves {
		sibs, err := tr.Proof(i)
		if err != nil {
			t.Fatal(err)
		}
		leafNode := sumtree.Node{Hash: lf.Hash, Sum: lf.Sum}
		if !VerifyMerkleSumProof(root.Hash, root.Sum, i, leafNode, sibs) {
			t.Fatalf("leaf %d failed to verify against the published root", i)
		}
	}
}

func TestVerifyMerkleSumProof_TamperFails(t *testing.T) {
	leaves := []sumtree.Leaf{ml(1, 100), ml(2, 250), ml(3, 50)}
	tr, _ := sumtree.Build(leaves)
	root := tr.Root()
	sibs, _ := tr.Proof(0)
	leafNode := sumtree.Node{Hash: leaves[0].Hash, Sum: leaves[0].Sum}

	// Sanity: the untampered proof verifies.
	if !VerifyMerkleSumProof(root.Hash, root.Sum, 0, leafNode, sibs) {
		t.Fatal("baseline proof should verify")
	}

	// Tampered leaf sum.
	if VerifyMerkleSumProof(root.Hash, root.Sum, 0, sumtree.Node{Hash: leaves[0].Hash, Sum: big.NewInt(999)}, sibs) {
		t.Fatal("tampered leaf sum should fail")
	}
	// Wrong published root sum.
	if VerifyMerkleSumProof(root.Hash, big.NewInt(1), 0, leafNode, sibs) {
		t.Fatal("wrong root sum should fail")
	}
	// Negative sibling sum (the canonical Merkle-sum forgery).
	if len(sibs) > 0 {
		forged := append([]sumtree.Node{{Hash: sibs[0].Hash, Sum: big.NewInt(-5)}}, sibs[1:]...)
		if VerifyMerkleSumProof(root.Hash, root.Sum, 0, leafNode, forged) {
			t.Fatal("negative sibling sum should fail")
		}
	}
	// Wrong index (siblings no longer match the path).
	if VerifyMerkleSumProof(root.Hash, root.Sum, 1, leafNode, sibs) {
		t.Fatal("wrong index should fail")
	}
	// Nil root sum.
	if VerifyMerkleSumProof(root.Hash, nil, 0, leafNode, sibs) {
		t.Fatal("nil root sum should fail")
	}
}
