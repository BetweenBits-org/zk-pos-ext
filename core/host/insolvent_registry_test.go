package host

import (
	"slices"
	"testing"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// TestDropAndLogV0_AutoRegistered confirms importing core/host
// alone is sufficient to find the engine-default insolvent policy.
func TestDropAndLogV0_AutoRegistered(t *testing.T) {
	policy := NewInsolventPolicy(InsolventActionDropAndLogV0)
	got := policy.OnInsolventAccount("user-id", "test reason")
	if got != spec.InvalidActionDrop {
		t.Fatalf("OnInsolventAccount returned %v, want InvalidActionDrop", got)
	}
}

// TestNewInsolventPolicy_UnknownPanics asserts the build-time-omission
// contract.
func TestNewInsolventPolicy_UnknownPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on unknown insolvent action")
		}
	}()
	NewInsolventPolicy("not_a_registered_action.v0")
}

// TestRegisterInsolventPolicy_DuplicatePanics asserts the single-owner
// invariant.
func TestRegisterInsolventPolicy_DuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate insolvent registration")
		}
	}()
	RegisterInsolventPolicy(InsolventActionDropAndLogV0, newDropAndLogV0)
}

// TestRegisterInsolventPolicy_EmptyPanics asserts inputs that would
// produce malformed registry keys panic at registration time.
func TestRegisterInsolventPolicy_EmptyPanics(t *testing.T) {
	t.Run("empty id", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic on empty action id")
			}
		}()
		RegisterInsolventPolicy("", newDropAndLogV0)
	})
	t.Run("nil factory", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic on nil factory")
			}
		}()
		RegisterInsolventPolicy("nonempty.v0", nil)
	})
}

// TestRegisteredInsolventActions confirms the diagnostic helper
// surfaces the canonical action.
func TestRegisteredInsolventActions(t *testing.T) {
	actions := RegisteredInsolventActions()
	if !slices.Contains(actions, InsolventActionDropAndLogV0) {
		t.Fatalf("RegisteredInsolventActions() = %v, missing %q",
			actions, InsolventActionDropAndLogV0)
	}
}
