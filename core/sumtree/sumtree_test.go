package sumtree

import (
	"math/big"
	"testing"
)

// leaf builds a 32-byte-hash leaf with the low byte set to b and the given
// sum — enough to make distinct leaves without caring about real account
// commitments.
func leaf(b byte, sum int64) Leaf {
	h := make([]byte, 32)
	h[31] = b
	return Leaf{Hash: h, Sum: big.NewInt(sum)}
}

func TestBuild_RootSumEqualsTotal(t *testing.T) {
	tr, err := Build([]Leaf{leaf(1, 100), leaf(2, 250), leaf(3, 50)})
	if err != nil {
		t.Fatal(err)
	}
	if got := tr.Root().Sum; got.Cmp(big.NewInt(400)) != 0 {
		t.Fatalf("root sum = %s, want 400", got)
	}
	if tr.NumLeaves() != 3 {
		t.Fatalf("NumLeaves = %d, want 3", tr.NumLeaves())
	}
}

func TestBuild_Rejects(t *testing.T) {
	if _, err := Build(nil); err == nil {
		t.Fatal("expected error on empty leaves")
	}
	if _, err := Build([]Leaf{{Hash: make([]byte, 32), Sum: big.NewInt(-1)}}); err == nil {
		t.Fatal("expected error on negative sum")
	}
	if _, err := Build([]Leaf{{Hash: make([]byte, 16), Sum: big.NewInt(1)}}); err == nil {
		t.Fatal("expected error on short hash")
	}
	if _, err := Build([]Leaf{{Hash: make([]byte, 32), Sum: nil}}); err == nil {
		t.Fatal("expected error on nil sum")
	}
}

func TestBuild_Deterministic(t *testing.T) {
	leaves := []Leaf{leaf(1, 100), leaf(2, 250), leaf(3, 50)}
	a, _ := Build(leaves)
	b, _ := Build(leaves)
	if !a.Root().Equal(b.Root()) {
		t.Fatal("root not deterministic across rebuilds")
	}
}

func TestProof_SingleLeaf(t *testing.T) {
	tr, err := Build([]Leaf{leaf(7, 42)})
	if err != nil {
		t.Fatal(err)
	}
	sibs, err := tr.Proof(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(sibs) != 0 {
		t.Fatalf("single-leaf proof should be empty, got %d siblings", len(sibs))
	}
	if tr.Root().Sum.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("single-leaf root sum = %s, want 42", tr.Root().Sum)
	}
}

func TestProof_OutOfRange(t *testing.T) {
	tr, _ := Build([]Leaf{leaf(1, 1), leaf(2, 2)})
	if _, err := tr.Proof(2); err == nil {
		t.Fatal("expected out-of-range error for index past real leaves")
	}
	if _, err := tr.Proof(-1); err == nil {
		t.Fatal("expected out-of-range error for negative index")
	}
}
