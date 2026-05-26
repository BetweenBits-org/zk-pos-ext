package host

import (
	"fmt"
	"sort"
	"sync"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// InsolventPolicyFactory constructs an InvalidAccountPolicy for a
// given action ID. Stateless factories — service startup calls the
// factory once and shares the returned policy across goroutines, so
// the implementation MUST be safe for concurrent OnInsolventAccount
// calls.
type InsolventPolicyFactory func() spec.InvalidAccountPolicy

// InvalidAccountPolicy registry — R8-A / G17.
//
// Layer ownership: core/host owns the table because the disposition
// of an invalid account (drop / abort / quarantine) is engine-
// universal vocabulary — every solvency model has a notion of
// "invalid input" and chooses one of the three actions.
//
// ID format convention (G17): "<action_id>.v<version>". Matches the
// identity-registry convention so all engine-universal registries
// have the same key shape. First entry: "drop_and_log.v0".
//
// Lifetime: registration at build time via init(). Read-only after
// package init. Misses panic — every action referenced by a
// profile.toml MUST be linked into the binary.
var (
	insolventRegistryMu sync.RWMutex
	insolventRegistry   = map[string]InsolventPolicyFactory{}
)

// RegisterInsolventPolicy adds (action → factory) to the registry.
// Intended to be called from package init(); double-registration
// panics.
//
// Panics:
//   - empty action ID.
//   - nil factory.
//   - action already registered — universal contracts have a single
//     owner per engine binary.
func RegisterInsolventPolicy(action string, factory InsolventPolicyFactory) {
	if action == "" {
		panic("core/host: RegisterInsolventPolicy called with empty action id")
	}
	if factory == nil {
		panic("core/host: RegisterInsolventPolicy(" + action + ") called with nil factory")
	}
	insolventRegistryMu.Lock()
	defer insolventRegistryMu.Unlock()
	if _, dup := insolventRegistry[action]; dup {
		panic("core/host: insolvent action " + action + " registered twice")
	}
	insolventRegistry[action] = factory
}

// NewInsolventPolicy returns the InvalidAccountPolicy registered
// under action. Service startup calls this after parsing profile.toml.
//
// Panics if the action is not registered (build-time omission — a
// referenced action MUST be linked into the engine binary).
func NewInsolventPolicy(action string) spec.InvalidAccountPolicy {
	insolventRegistryMu.RLock()
	factory, ok := insolventRegistry[action]
	insolventRegistryMu.RUnlock()
	if !ok {
		panic(fmt.Sprintf("core/host: insolvent action %q is not registered (known: %v)",
			action, RegisteredInsolventActions()))
	}
	return factory()
}

// RegisteredInsolventActions returns the sorted list of action IDs
// currently in the registry. Diagnostic helper for startup logging.
func RegisteredInsolventActions() []string {
	insolventRegistryMu.RLock()
	defer insolventRegistryMu.RUnlock()
	out := make([]string, 0, len(insolventRegistry))
	for k := range insolventRegistry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
