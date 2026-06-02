package host

import (
	"context"
	"math/big"
	"testing"

	t1spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t1_simple_margin/spec"
)

// fakeSnapshot is an in-memory t1spec.SnapshotSource yielding a fixed
// account list — enough to exercise CollectSumLeaves without CSV/vfs
// plumbing.
type fakeSnapshot struct {
	accounts []t1spec.AccountInfo
}

func (f *fakeSnapshot) AccountStream(ctx context.Context) (<-chan t1spec.AccountInfo, error) {
	ch := make(chan t1spec.AccountInfo)
	go func() {
		defer close(ch)
		for _, a := range f.accounts {
			select {
			case <-ctx.Done():
				return
			case ch <- a:
			}
		}
	}()
	return ch, nil
}

func (f *fakeSnapshot) CexAssets(ctx context.Context) ([]t1spec.CexAssetInfo, error) {
	return nil, nil
}
func (f *fakeSnapshot) SnapshotID() string   { return "test-snap" }
func (f *fakeSnapshot) InvalidCount() uint64 { return 0 }

func acct(idx uint32, id byte, eq, debt int64) t1spec.AccountInfo {
	aid := make([]byte, 32)
	aid[31] = id
	return t1spec.AccountInfo{
		AccountIndex: idx,
		AccountID:    aid,
		TotalEquity:  big.NewInt(eq),
		TotalDebt:    big.NewInt(debt),
		Assets:       []t1spec.AccountAsset{{Index: 0, Equity: uint64(eq), Debt: uint64(debt)}},
	}
}

func TestCollectSumLeaves_NetAndDensePositions(t *testing.T) {
	snap := &fakeSnapshot{accounts: []t1spec.AccountInfo{
		acct(10, 1, 100, 30), // net 70
		acct(20, 2, 250, 0),  // net 250
	}}
	recs, err := CollectSumLeaves(context.Background(), snap, []int{2})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	if recs[0].Position != 0 || recs[1].Position != 1 {
		t.Fatalf("positions not dense: %d, %d", recs[0].Position, recs[1].Position)
	}
	if recs[0].Balance.Cmp(big.NewInt(70)) != 0 {
		t.Fatalf("net[0] = %s, want 70", recs[0].Balance)
	}
	if recs[1].Balance.Cmp(big.NewInt(250)) != 0 {
		t.Fatalf("net[1] = %s, want 250", recs[1].Balance)
	}
	if len(recs[0].LeafHash) != 32 {
		t.Fatalf("leaf hash len = %d, want 32", len(recs[0].LeafHash))
	}
}
