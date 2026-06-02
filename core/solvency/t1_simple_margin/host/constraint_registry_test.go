package host_test

import (
	"slices"
	"testing"

	t1host "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t1_simple_margin/host"
	t1spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t1_simple_margin/spec"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/consensys/gnark/frontend"
)

type fakeModule struct {
	id corespec.ConstraintModuleID
}

func (f fakeModule) ID() corespec.ConstraintModuleID { return f.id }
func (fakeModule) Define(frontend.API, t1spec.ConstraintContext) error {
	return nil
}

func TestNewConstraintModule_EmptyReturnsNoop(t *testing.T) {
	mod := t1host.NewConstraintModule(corespec.NoExtensionID)
	if mod == nil {
		t.Fatal("NewConstraintModule(\"\") returned nil")
	}
	if string(mod.ID()) != corespec.NoExtensionID {
		t.Fatalf("noop ID = %q, want empty", mod.ID())
	}
}

func TestConstraintRegistry_RegisterAndLookup(t *testing.T) {
	const id = "t1host_test_fake.v0"
	t1host.RegisterConstraintModule(id, func() t1spec.ConstraintModule {
		return fakeModule{id: corespec.ConstraintModuleID(id)}
	})

	if !slices.Contains(t1host.RegisteredConstraintModules(), id) {
		t.Fatalf("RegisteredConstraintModules missing %q", id)
	}
	mod := t1host.NewConstraintModule(id)
	if string(mod.ID()) != id {
		t.Fatalf("got ID %q, want %q", mod.ID(), id)
	}
}

func TestConstraintRegistry_NewPanicsOnUnknown(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on unknown module id")
		}
	}()
	t1host.NewConstraintModule("never_registered.v0")
}

func TestConstraintRegistry_FactoryIDMismatchPanics(t *testing.T) {
	const id = "t1host_test_mismatch.v0"
	t1host.RegisterConstraintModule(id, func() t1spec.ConstraintModule {
		return fakeModule{id: corespec.ConstraintModuleID("different.v0")}
	})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on factory id mismatch")
		}
	}()
	t1host.NewConstraintModule(id)
}

func TestConstraintRegistry_RegisterDuplicatePanics(t *testing.T) {
	const id = "t1host_test_dup.v0"
	t1host.RegisterConstraintModule(id, func() t1spec.ConstraintModule {
		return fakeModule{id: corespec.ConstraintModuleID(id)}
	})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	t1host.RegisterConstraintModule(id, func() t1spec.ConstraintModule {
		return fakeModule{id: corespec.ConstraintModuleID(id)}
	})
}

func TestConstraintRegistry_RegisterEmptyIDPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty module id")
		}
	}()
	t1host.RegisterConstraintModule("", func() t1spec.ConstraintModule {
		return fakeModule{id: ""}
	})
}

func TestConstraintRegistry_RegisterNilFactoryPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil factory")
		}
	}()
	t1host.RegisterConstraintModule("nonempty.v0", nil)
}
