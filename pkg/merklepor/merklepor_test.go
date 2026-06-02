package merklepor

import (
	"encoding/json"
	"math/big"
	"testing"

	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
)

// memAttestStore is an in-memory corehost.AttestStore for testing the
// build → persist → verify round trip without MySQL.
type memAttestStore struct{ rows map[uint32]corehost.AttestProofDTO }

func newMemStore() *memAttestStore { return &memAttestStore{rows: map[uint32]corehost.AttestProofDTO{}} }

func (m *memAttestStore) EnsureSchema() error { return nil }
func (m *memAttestStore) Create(rows []corehost.AttestProofDTO) error {
	for _, r := range rows {
		m.rows[r.Index] = r
	}
	return nil
}
func (m *memAttestStore) GetByIndex(i uint32) (*corehost.AttestProofDTO, error) {
	r, ok := m.rows[i]
	if !ok {
		return nil, corehost.ErrNotFound
	}
	return &r, nil
}

func b32(b byte) []byte {
	out := make([]byte, 32)
	out[31] = b
	return out
}

func sampleLeaves() []corehost.SumLeafRecord {
	return []corehost.SumLeafRecord{
		{Position: 0, AccountID: b32(1), LeafHash: b32(11), Balance: big.NewInt(100)},
		{Position: 1, AccountID: b32(2), LeafHash: b32(12), Balance: big.NewInt(250)},
		{Position: 2, AccountID: b32(3), LeafHash: b32(13), Balance: big.NewInt(50)},
	}
}

func TestBuildAndPersist_RoundTrip(t *testing.T) {
	store := newMemStore()
	root, written, err := buildAndPersist(sampleLeaves(), store, big.NewInt(400), big.NewInt(1_000_000))
	if err != nil {
		t.Fatal(err)
	}
	if written != 3 {
		t.Fatalf("written = %d, want 3", written)
	}
	if root.Sum.Cmp(big.NewInt(400)) != 0 {
		t.Fatalf("root sum = %s, want 400", root.Sum)
	}
	for i := range 3 {
		row, err := store.GetByIndex(uint32(i))
		if err != nil {
			t.Fatalf("GetByIndex(%d): %v", i, err)
		}
		ok, err := verifyUserConfig([]byte(row.Config))
		if err != nil {
			t.Fatalf("verify %d: %v", i, err)
		}
		if !ok {
			t.Fatalf("user %d failed to verify against published root", i)
		}
	}
}

func TestBuildAndPersist_RejectsNegative(t *testing.T) {
	bad := []corehost.SumLeafRecord{{Position: 0, AccountID: b32(1), LeafHash: b32(11), Balance: big.NewInt(-1)}}
	if _, _, err := buildAndPersist(bad, newMemStore(), nil, nil); err == nil {
		t.Fatal("expected reconcile failure on negative balance")
	}
}

func TestBuildAndPersist_RejectsSumMismatch(t *testing.T) {
	if _, _, err := buildAndPersist(sampleLeaves(), newMemStore(), big.NewInt(999), nil); err == nil {
		t.Fatal("expected reconcile failure on wrong published total")
	}
}

func TestVerifyUserConfig_TamperFails(t *testing.T) {
	store := newMemStore()
	if _, _, err := buildAndPersist(sampleLeaves(), store, nil, nil); err != nil {
		t.Fatal(err)
	}
	row, err := store.GetByIndex(0)
	if err != nil {
		t.Fatal(err)
	}
	var cfg SumUserConfig
	if err := json.Unmarshal([]byte(row.Config), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Balance = "999" // tamper the claimed balance
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := verifyUserConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("tampered balance should fail verification")
	}
}
