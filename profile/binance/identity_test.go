package binance

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// TestScheme_FrozenV1 locks the G2 frozen identifier so a rename
// can't ship without a deliberate edit + a deployment ceremony to
// re-publish artifacts under the new name.
func TestScheme_FrozenV1(t *testing.T) {
	got := NewIdentity().Scheme()
	const want = "passthrough_hex_bn254_reduced.v0"
	if got != want {
		t.Fatalf("Scheme() = %q, want %q (G2 freeze)", got, want)
	}
}

// TestDeriveAccountID_BelowModulusPassesThrough confirms the
// non-reducing happy path: an input strictly below fr.Modulus
// round-trips byte-for-byte (top 3 bits clear → value < 2^253 <
// modulus, so SetBytes→Marshal is the identity).
func TestDeriveAccountID_BelowModulusPassesThrough(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	raw[0] &= 0x1F

	got := NewIdentity().DeriveAccountID(hex.EncodeToString(raw))
	if !bytes.Equal(got[:], raw) {
		t.Fatalf("below-modulus input not passed through:\n  raw=%x\n  got=%x", raw, got[:])
	}
}

// TestDeriveAccountID_AboveModulusReduces confirms the reducing
// branch: all-FF input is well above fr.Modulus and MUST come out
// reduced (= (2^256 - 1) mod fr.Modulus). Without reduction, the
// returned bytes would not match the value the snapshot adapter
// embeds into the leaf hash, and the user's local AccountID would
// fail inclusion verification.
func TestDeriveAccountID_AboveModulusReduces(t *testing.T) {
	allFF := strings.Repeat("ff", 32)
	got := NewIdentity().DeriveAccountID(allFF)

	want := new(fr.Element).SetBytes(bytes.Repeat([]byte{0xff}, 32)).Marshal()
	if !bytes.Equal(got[:], want) {
		t.Fatalf("all-FF input not reduced:\n  got=%x\n  want=%x", got[:], want)
	}
	if bytes.Equal(got[:], bytes.Repeat([]byte{0xff}, 32)) {
		t.Fatal("all-FF input passed through unchanged — fr.Element reduction not applied")
	}
}

// TestDeriveAccountID_BadInputPanics confirms the length and
// hex-decode guards still surface ETL bugs loudly.
func TestDeriveAccountID_BadInputPanics(t *testing.T) {
	t.Run("wrong length", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic on wrong-length input")
			}
		}()
		NewIdentity().DeriveAccountID("abcd")
	})
	t.Run("non-hex chars", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic on non-hex input")
			}
		}()
		NewIdentity().DeriveAccountID(strings.Repeat("z", 64))
	})
}
