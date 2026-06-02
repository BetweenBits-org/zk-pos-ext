// Package sumtree implements a dense Merkle Sum Tree over Poseidon/BN254 —
// the off-circuit data structure behind the engine's non-zk Merkle-sum
// proof-of-liabilities side line (PRODUCTION_ROADMAP Stage MS, gate G19).
//
// Unlike core/tree (the sparse depth-28 account SMT used by the zk PoS
// line), this tree is DENSE and POSITIONAL: leaves are placed left to
// right in the order supplied and padded up to the next power of two with
// zero-sum padding leaves. Every internal node carries the running sum of
// its subtree, so the root commits to BOTH the full leaf set (hash) AND
// the total liabilities (sum). A user's inclusion proof therefore proves
// not only "my leaf is in the set" but "my balance flows into the
// published total" — the property a plain inclusion tree cannot give
// without zk.
//
// Construction freeze (G19): the parent of two children is
//
//	node.Sum  = left.Sum + right.Sum
//	node.Hash = Poseidon(left.Hash, fe(left.Sum), right.Hash, fe(right.Sum))
//
// where fe(s) is the 32-byte big-endian BN254 field encoding of the
// non-negative sum s. Leaf sums MUST be non-negative; a negative sum is
// rejected at Build, because the field encoding of a negative value is
// meaningless and a negative leaf is the canonical Merkle-sum forgery (it
// shrinks the apparent total). The auditor-side guard over the full set
// lives in core/host.CheckNonNegative.
//
// The hash function and field are fixed to Poseidon over BN254 to match
// the rest of the engine (core/host.AccountLeafHash leaf commitments feed
// straight in as leaf hashes). There is no fixed depth: a dense tree's
// height is ceil(log2(n)).
package sumtree

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// Node is one (hash, sum) pair in the tree — a leaf, an internal node, the
// root, or a sibling in an inclusion proof. Hash is a 32-byte BN254 field
// element; Sum is the non-negative running total of the subtree the node
// roots (for a leaf, the account balance).
type Node struct {
	Hash []byte
	Sum  *big.Int
}

// Leaf is a caller-supplied input to Build: a leaf commitment Hash (e.g. a
// core/host.AccountLeafHash output, 32 bytes) paired with the account's
// non-negative Balance, carried as the leaf's Sum.
type Leaf struct {
	Hash []byte
	Sum  *big.Int
}

// emptyLeafHash is the canonical hash of a zero-sum padding leaf: Poseidon
// of a single zero field element. Deterministic so Build and any rebuild
// from the same leaves agree on the padded root.
var emptyLeafHash []byte

func init() {
	z := fr.Element{}
	h := poseidon.Poseidon(&z).Bytes()
	emptyLeafHash = h[:]
}

// Tree is a built dense Merkle sum tree. It retains every level so Proof
// can emit sibling chains without rehashing.
type Tree struct {
	levels  [][]Node // levels[0] = padded leaves; last level = [root]
	numReal int      // count of real (non-padding) leaves
}

// feBytes returns the 32-byte big-endian BN254 field encoding of a
// non-negative big.Int. The caller guarantees s >= 0 and s < the field
// modulus (sums of uint64 balances over fewer than 2^28 accounts cannot
// reach it).
func feBytes(s *big.Int) [32]byte {
	var e fr.Element
	e.SetBigInt(s)
	return e.Bytes()
}

// HashNode combines two child nodes into their parent, summing the subtree
// totals and binding both child hashes AND sums into the parent hash.
// Shared by Build and core/host.VerifyMerkleSumProof so the construction
// has a single source of truth.
func HashNode(left, right Node) Node {
	lb := feBytes(left.Sum)
	rb := feBytes(right.Sum)
	h := poseidon.PoseidonBytes(left.Hash, lb[:], right.Hash, rb[:])
	return Node{Hash: h, Sum: new(big.Int).Add(left.Sum, right.Sum)}
}

// Build constructs the tree from leaves in the given positional order.
// Leaves are padded up to the next power of two with zero-sum padding
// leaves. Returns an error if leaves is empty, or any leaf has a nil or
// negative Sum or a non-32-byte Hash.
func Build(leaves []Leaf) (*Tree, error) {
	if len(leaves) == 0 {
		return nil, fmt.Errorf("sumtree: at least one leaf required")
	}
	size := nextPow2(len(leaves))
	level := make([]Node, size)
	for i := range level {
		if i < len(leaves) {
			lf := leaves[i]
			if len(lf.Hash) != 32 {
				return nil, fmt.Errorf("sumtree: leaf %d hash len %d, want 32", i, len(lf.Hash))
			}
			if lf.Sum == nil || lf.Sum.Sign() < 0 {
				return nil, fmt.Errorf("sumtree: leaf %d sum must be non-negative", i)
			}
			level[i] = Node{Hash: lf.Hash, Sum: new(big.Int).Set(lf.Sum)}
		} else {
			level[i] = Node{Hash: emptyLeafHash, Sum: big.NewInt(0)}
		}
	}
	levels := [][]Node{level}
	for len(level) > 1 {
		next := make([]Node, len(level)/2)
		for i := 0; i < len(level); i += 2 {
			next[i/2] = HashNode(level[i], level[i+1])
		}
		levels = append(levels, next)
		level = next
	}
	return &Tree{levels: levels, numReal: len(leaves)}, nil
}

// Root returns the tree root: the hash committing to the full padded leaf
// set and the total sum of all leaf balances. The returned Sum is a fresh
// copy the caller may retain and mutate.
func (t *Tree) Root() Node {
	r := t.levels[len(t.levels)-1][0]
	return Node{Hash: r.Hash, Sum: new(big.Int).Set(r.Sum)}
}

// NumLeaves returns the count of real (non-padding) leaves.
func (t *Tree) NumLeaves() int { return t.numReal }

// Proof returns the sibling chain proving the leaf at positional index is
// included, ordered leaf-to-root (siblings[0] is the leaf's immediate
// sibling). index must address a real leaf: 0 <= index < NumLeaves.
func (t *Tree) Proof(index int) ([]Node, error) {
	if index < 0 || index >= t.numReal {
		return nil, fmt.Errorf("sumtree: index %d out of range [0,%d)", index, t.numReal)
	}
	sibs := make([]Node, 0, len(t.levels)-1)
	idx := index
	for l := 0; l < len(t.levels)-1; l++ {
		sib := t.levels[l][idx^1]
		sibs = append(sibs, Node{Hash: sib.Hash, Sum: new(big.Int).Set(sib.Sum)})
		idx >>= 1
	}
	return sibs, nil
}

// Equal reports whether two nodes have identical hash and sum.
func (n Node) Equal(o Node) bool {
	return bytes.Equal(n.Hash, o.Hash) && n.Sum.Cmp(o.Sum) == 0
}

func nextPow2(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}
