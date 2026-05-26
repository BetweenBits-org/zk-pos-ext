package host

import (
	"bytes"
	"encoding/hex"
	"slices"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// TestPassthroughHexBN254ReducedV0_AutoRegistered confirms that
// importing core/host alone is sufficient to find the engine-frozen
// identity scheme — services do not need to import any customer
// profile package to look it up.
func TestPassthroughHexBN254ReducedV0_AutoRegistered(t *testing.T) {
	provider := NewIdentity(IdentitySchemePassthroughHexBN254ReducedV0)
	if provider.Scheme() != IdentitySchemePassthroughHexBN254ReducedV0 {
		t.Fatalf("Scheme() = %q, want %q",
			provider.Scheme(), IdentitySchemePassthroughHexBN254ReducedV0)
	}
}

// TestPassthroughHexBN254ReducedV0_AboveModulusReduces locks the
// reducing branch — promoted from profile/binance/identity_test.go.
// All-FF input MUST come out reduced (= (2^256-1) mod fr.Modulus),
// matching what the snapshot adapter writes into the leaf hash.
func TestPassthroughHexBN254ReducedV0_AboveModulusReduces(t *testing.T) {
	allFF := strings.Repeat("ff", 32)
	got := NewIdentity(IdentitySchemePassthroughHexBN254ReducedV0).DeriveAccountID(allFF)

	want := new(fr.Element).SetBytes(bytes.Repeat([]byte{0xff}, 32)).Marshal()
	if !bytes.Equal(got[:], want) {
		t.Fatalf("all-FF input not reduced:\n  got=%x\n  want=%x", got[:], want)
	}
}

// TestPassthroughHexBN254ReducedV0_BelowModulusPassesThrough confirms
// the non-reducing happy path: top-3-bit-cleared input round-trips
// byte-for-byte.
func TestPassthroughHexBN254ReducedV0_BelowModulusPassesThrough(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	raw[0] &= 0x1F

	got := NewIdentity(IdentitySchemePassthroughHexBN254ReducedV0).
		DeriveAccountID(hex.EncodeToString(raw))
	if !bytes.Equal(got[:], raw) {
		t.Fatalf("below-modulus input not passed through:\n  raw=%x\n  got=%x", raw, got[:])
	}
}

// TestNewIdentity_UnknownPanics asserts the build-time-omission
// contract: a scheme not linked into the binary is a panic at
// startup, not silent fall-through.
func TestNewIdentity_UnknownPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on unknown identity scheme")
		}
	}()
	NewIdentity("not_a_registered_scheme.v0")
}

// TestRegisterIdentity_DuplicatePanics asserts the single-owner
// invariant: a second registration under an existing scheme MUST
// surface as a panic rather than silently overwriting.
func TestRegisterIdentity_DuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate identity registration")
		}
	}()
	RegisterIdentity(IdentitySchemePassthroughHexBN254ReducedV0,
		newPassthroughHexBN254ReducedV0)
}

// TestRegisterIdentity_EmptyPanics asserts inputs that would
// produce malformed registry keys panic at registration time.
func TestRegisterIdentity_EmptyPanics(t *testing.T) {
	t.Run("empty id", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic on empty scheme id")
			}
		}()
		RegisterIdentity("", newPassthroughHexBN254ReducedV0)
	})
	t.Run("nil factory", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic on nil factory")
			}
		}()
		RegisterIdentity("nonempty.v0", nil)
	})
}

// TestRegisteredIdentitySchemes confirms the diagnostic helper
// surfaces the canonical scheme — services log this on startup so
// operators can audit which schemes are linked.
func TestRegisteredIdentitySchemes(t *testing.T) {
	schemes := RegisteredIdentitySchemes()
	if !slices.Contains(schemes, IdentitySchemePassthroughHexBN254ReducedV0) {
		t.Fatalf("RegisteredIdentitySchemes() = %v, missing %q",
			schemes, IdentitySchemePassthroughHexBN254ReducedV0)
	}
}
