package host_test

import (
	"slices"
	"testing"

	t1host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/host"
	t1spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

func stubFactory(string, string, int, corespec.PriceScaleProvider) t1spec.SnapshotSource {
	return nil
}

// Note: sea_csv.v1 registration is verified in profile/sea_reference's
// own test package, where its init() actually runs. This file only
// covers registry mechanics.

// TestSnapshotRegistry_RegisterAndLookup exercises the mechanism with
// a one-off id.
func TestSnapshotRegistry_RegisterAndLookup(t *testing.T) {
	const id = "t1host_registry_test_stub.v0"
	t1host.RegisterSnapshot(id, stubFactory)

	if !slices.Contains(t1host.RegisteredSnapshotConnectors(), id) {
		t.Fatalf("RegisteredSnapshotConnectors missing %q", id)
	}
	if got := t1host.NewSnapshot(id, "/tmp", "snap", 5, nil); got != nil {
		t.Fatalf("stubFactory returned non-nil: %v", got)
	}
}

func TestSnapshotRegistry_NewSnapshotPanicsOnMissing(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on unknown connector id")
		}
	}()
	t1host.NewSnapshot("never_registered.v0", "/tmp", "snap", 5, nil)
}

func TestSnapshotRegistry_RegisterDuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	t1host.RegisterSnapshot("t1host_registry_test_stub.v0", stubFactory)
}

func TestSnapshotRegistry_RegisterEmptyIDPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty connector id")
		}
	}()
	t1host.RegisterSnapshot("", stubFactory)
}

func TestSnapshotRegistry_RegisterNilFactoryPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil factory")
		}
	}()
	t1host.RegisterSnapshot("nonempty.v0", nil)
}
