package host

import (
	"fmt"
	"sort"
	"sync"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// IdentityFactory constructs an AccountIDProvider for a given scheme.
// Factories MUST be cheap (no I/O) — they run during service startup
// after the declarative profile is loaded and before any account is
// processed.
//
// One factory per scheme ID. The returned provider's Scheme() value
// MUST equal the registry ID under which the factory is registered;
// the registry asserts this on lookup.
type IdentityFactory func() spec.AccountIDProvider

// Identity scheme registry — R8-A / G17.
//
// Layer ownership: core/host owns the table because identity is engine-
// universal (every solvency model uses the same 32-byte AccountID
// slot). Customer profiles do NOT carry their own identity packages;
// they reference an engine-built-in scheme by ID from profile.toml.
//
// ID format convention (G17): "<id>.v<version>". The version is part
// of the identifier rather than a separate field so a customer
// upgrading derivation semantics (e.g. HMAC v1 → v2) gets a distinct
// registry key and a distinct stem in published artifacts.
//
// Lifetime: registration happens at build time via init() in the
// implementation file (e.g. identity_passthrough.go). The registry
// is read-only after package init. Lookups outside init() that miss
// panic — every scheme referenced by a profile.toml MUST be linked
// into the binary.
var (
	identityRegistryMu sync.RWMutex
	identityRegistry   = map[string]IdentityFactory{}
)

// RegisterIdentity adds (scheme → factory) to the registry. Intended
// to be called from package init(); double-registration panics.
//
// Panics:
//   - empty scheme ID.
//   - scheme already registered (regardless of whether the factory
//     value is identical) — universal contracts MUST have a single
//     owner in the engine binary.
func RegisterIdentity(scheme string, factory IdentityFactory) {
	if scheme == "" {
		panic("core/host: RegisterIdentity called with empty scheme id")
	}
	if factory == nil {
		panic("core/host: RegisterIdentity(" + scheme + ") called with nil factory")
	}
	identityRegistryMu.Lock()
	defer identityRegistryMu.Unlock()
	if _, dup := identityRegistry[scheme]; dup {
		panic("core/host: identity scheme " + scheme + " registered twice")
	}
	identityRegistry[scheme] = factory
}

// NewIdentity returns the AccountIDProvider registered under scheme.
// Service startup calls this after parsing profile.toml.
//
// Panics:
//   - scheme not registered (build-time omission — a referenced
//     scheme MUST be linked into the engine binary).
//   - factory returns a provider whose Scheme() does not equal the
//     registry key (audit invariant: stem ID == artifact ID).
func NewIdentity(scheme string) spec.AccountIDProvider {
	identityRegistryMu.RLock()
	factory, ok := identityRegistry[scheme]
	identityRegistryMu.RUnlock()
	if !ok {
		panic(fmt.Sprintf("core/host: identity scheme %q is not registered (known: %v)",
			scheme, RegisteredIdentitySchemes()))
	}
	provider := factory()
	if got := provider.Scheme(); got != scheme {
		panic(fmt.Sprintf("core/host: identity scheme %q factory returned provider whose Scheme()=%q",
			scheme, got))
	}
	return provider
}

// RegisteredIdentitySchemes returns the sorted list of scheme IDs
// currently in the registry. Diagnostic helper for startup logging
// and lookup-failure messages.
func RegisteredIdentitySchemes() []string {
	identityRegistryMu.RLock()
	defer identityRegistryMu.RUnlock()
	out := make([]string, 0, len(identityRegistry))
	for k := range identityRegistry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
