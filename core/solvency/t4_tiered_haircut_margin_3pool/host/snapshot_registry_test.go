package host_test

import (
	"slices"
	"testing"

	t4host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host"
	t4spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// stubFactory is a dummy SnapshotFactory used by negative tests that
// need a non-nil factory value but never actually invoke it.
func stubFactory(string, string, int, corespec.PriceScaleProvider) t4spec.SnapshotSource {
	return nil
}

// Note: t4_standard_csv.v1 registration is covered by core/snapshot's
// parser package. This file only covers registry mechanics.

// TestSnapshotRegistry_RegisterAndLookup exercises the mechanism with
// a one-off id that no profile claims.
func TestSnapshotRegistry_RegisterAndLookup(t *testing.T) {
	const id = "t4host_registry_test_stub.v0"
	t4host.RegisterSnapshot(id, stubFactory)

	if !slices.Contains(t4host.RegisteredSnapshotConnectors(), id) {
		t.Fatalf("RegisteredSnapshotConnectors missing %q", id)
	}
	if got := t4host.NewSnapshot(id, "/tmp", "snap", 5, nil); got != nil {
		t.Fatalf("stubFactory returned non-nil: %v", got)
	}
}

// TestSnapshotRegistry_NewSnapshotPanicsOnMissing locks the G17
// build-time-omission contract.
func TestSnapshotRegistry_NewSnapshotPanicsOnMissing(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on unknown connector id")
		}
	}()
	t4host.NewSnapshot("never_registered.v0", "/tmp", "snap", 5, nil)
}

// TestSnapshotRegistry_RegisterDuplicatePanics asserts the single-
// owner invariant by retrying the id the previous test took.
func TestSnapshotRegistry_RegisterDuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	t4host.RegisterSnapshot("t4host_registry_test_stub.v0", stubFactory)
}

// TestSnapshotRegistry_RegisterEmptyIDPanics asserts the empty-id
// guard at registration time.
func TestSnapshotRegistry_RegisterEmptyIDPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty connector id")
		}
	}()
	t4host.RegisterSnapshot("", stubFactory)
}

// TestSnapshotRegistry_RegisterNilFactoryPanics asserts nil factories
// are rejected at registration time.
func TestSnapshotRegistry_RegisterNilFactoryPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil factory")
		}
	}()
	t4host.RegisterSnapshot("nonempty.v0", nil)
}
