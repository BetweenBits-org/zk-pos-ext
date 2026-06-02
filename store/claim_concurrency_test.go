package store_test

import (
	"os"
	"sync"
	"testing"

	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
	"github.com/BetweenBits-org/zk-pos-ext/store"
)

// TestClaimOldestByStatus_ConcurrentNoDoubleClaim verifies the R13-D
// multi-instance-safety property: N parallel provers claiming from one
// queue each get a DISTINCT batch — no row claimed twice, no row lost.
//
// Requires a real MySQL 8.0+ (for FOR UPDATE SKIP LOCKED). Skipped unless
// ZKPOR_TEST_MYSQL_DSN is set, so `go test -short ./...` stays DB-free.
func TestClaimOldestByStatus_ConcurrentNoDoubleClaim(t *testing.T) {
	dsn := os.Getenv("ZKPOR_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set ZKPOR_TEST_MYSQL_DSN to run the concurrent-claim test against a real MySQL")
	}

	db, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	ws := store.NewWitnessStore(db, "_claimtest")
	_ = db.Migrator().DropTable("witness_claimtest") // fresh table each run
	if err := ws.CreateTable(); err != nil {
		t.Fatalf("create table: %v", err)
	}

	const n = 200
	rows := make([]store.BatchWitness, n)
	for i := range rows {
		rows[i] = store.BatchWitness{Height: int64(i), WitnessData: "x", Status: corehost.StatusPublished}
	}
	if err := ws.Create(rows); err != nil {
		t.Fatalf("seed rows: %v", err)
	}

	const workers = 8
	var (
		mu      sync.Mutex
		claimed = make(map[int64]int)
		wg      sync.WaitGroup
	)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				row, err := ws.ClaimOldestByStatus(corehost.StatusPublished, corehost.StatusReceived)
				if store.IsNotFound(err) {
					return
				}
				if err != nil {
					t.Errorf("claim: %v", err)
					return
				}
				mu.Lock()
				claimed[row.Height]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(claimed) != n {
		t.Fatalf("claimed %d distinct heights, want %d (early-quit lost work?)", len(claimed), n)
	}
	for h, c := range claimed {
		if c != 1 {
			t.Fatalf("height %d claimed %d times (double-claim under concurrency)", h, c)
		}
	}
}
