package host_test

import (
	"slices"
	"testing"

	t4host "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t4_tiered_haircut_margin_3pool/host"
	t4spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/consensys/gnark/frontend"
)

// fakeModule is a non-noop constraint module used to exercise the
// registry mechanism without committing a real production module.
type fakeModule struct {
	id corespec.ConstraintModuleID
}

func (f fakeModule) ID() corespec.ConstraintModuleID { return f.id }
func (fakeModule) Define(frontend.API, t4spec.ConstraintContext) error {
	return nil
}

// TestNewConstraintModule_EmptyReturnsNoop locks the engine-default
// fast path: profile.toml's blank constraint.module yields the noop
// without any registry round-trip.
func TestNewConstraintModule_EmptyReturnsNoop(t *testing.T) {
	mod := t4host.NewConstraintModule(corespec.NoExtensionID)
	if mod == nil {
		t.Fatal("NewConstraintModule(\"\") returned nil")
	}
	if string(mod.ID()) != corespec.NoExtensionID {
		t.Fatalf("noop ID = %q, want empty", mod.ID())
	}
}

// TestConstraintRegistry_RegisterAndLookup exercises the non-noop
// path with a fake module.
func TestConstraintRegistry_RegisterAndLookup(t *testing.T) {
	const id = "t4host_test_fake.v0"
	t4host.RegisterConstraintModule(id, func() t4spec.ConstraintModule {
		return fakeModule{id: corespec.ConstraintModuleID(id)}
	})

	if !slices.Contains(t4host.RegisteredConstraintModules(), id) {
		t.Fatalf("RegisteredConstraintModules missing %q", id)
	}
	mod := t4host.NewConstraintModule(id)
	if string(mod.ID()) != id {
		t.Fatalf("got ID %q, want %q", mod.ID(), id)
	}
}

// TestConstraintRegistry_NewPanicsOnUnknown locks G17 (c).
func TestConstraintRegistry_NewPanicsOnUnknown(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on unknown module id")
		}
	}()
	t4host.NewConstraintModule("never_registered.v0")
}

// TestConstraintRegistry_FactoryIDMismatchPanics locks the audit
// invariant: a factory's returned ID must match its registration key.
func TestConstraintRegistry_FactoryIDMismatchPanics(t *testing.T) {
	const id = "t4host_test_mismatch.v0"
	t4host.RegisterConstraintModule(id, func() t4spec.ConstraintModule {
		return fakeModule{id: corespec.ConstraintModuleID("different.v0")}
	})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on factory id mismatch")
		}
	}()
	t4host.NewConstraintModule(id)
}

// TestConstraintRegistry_RegisterDuplicatePanics asserts single-owner.
func TestConstraintRegistry_RegisterDuplicatePanics(t *testing.T) {
	const id = "t4host_test_dup.v0"
	t4host.RegisterConstraintModule(id, func() t4spec.ConstraintModule {
		return fakeModule{id: corespec.ConstraintModuleID(id)}
	})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	t4host.RegisterConstraintModule(id, func() t4spec.ConstraintModule {
		return fakeModule{id: corespec.ConstraintModuleID(id)}
	})
}

// TestConstraintRegistry_RegisterEmptyIDPanics asserts the noop id is
// reserved (cannot be re-registered explicitly).
func TestConstraintRegistry_RegisterEmptyIDPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty (noop-reserved) module id")
		}
	}()
	t4host.RegisterConstraintModule("", func() t4spec.ConstraintModule {
		return fakeModule{id: ""}
	})
}

// TestConstraintRegistry_RegisterNilFactoryPanics asserts nil
// factories are rejected at registration time.
func TestConstraintRegistry_RegisterNilFactoryPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil factory")
		}
	}()
	t4host.RegisterConstraintModule("nonempty.v0", nil)
}
