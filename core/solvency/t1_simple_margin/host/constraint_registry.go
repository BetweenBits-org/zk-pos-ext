package host

import (
	"fmt"
	"sort"
	"sync"

	t1spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t1_simple_margin/spec"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
)

// ConstraintModuleFactory constructs a T1 ConstraintModule. Same
// semantics as T4 — stateless factories registered at build time.
type ConstraintModuleFactory func() t1spec.ConstraintModule

// Constraint module registry (T1 — R8-B/3 / G17).
//
// Same shape as t4_tiered_haircut_margin_3pool/host's registry. The empty-id
// branch (corespec.NoExtensionID) returns the universal T1 noop
// without a lookup.
var (
	constraintRegistryMu sync.RWMutex
	constraintRegistry   = map[string]ConstraintModuleFactory{}
)

// RegisterConstraintModule adds (moduleID → factory). Panics on empty
// id, nil factory, or duplicate registration.
func RegisterConstraintModule(moduleID string, factory ConstraintModuleFactory) {
	if moduleID == "" {
		panic("t1_simple_margin/host: RegisterConstraintModule called with empty module id")
	}
	if factory == nil {
		panic("t1_simple_margin/host: RegisterConstraintModule(" + moduleID + ") called with nil factory")
	}
	constraintRegistryMu.Lock()
	defer constraintRegistryMu.Unlock()
	if _, dup := constraintRegistry[moduleID]; dup {
		panic("t1_simple_margin/host: constraint module " + moduleID + " registered twice")
	}
	constraintRegistry[moduleID] = factory
}

// NewConstraintModule returns the T1 ConstraintModule for the given
// moduleID. Empty ID returns the engine-default noop without a lookup.
// Non-empty IDs MUST be registered or the call panics.
func NewConstraintModule(moduleID string) t1spec.ConstraintModule {
	if moduleID == corespec.NoExtensionID {
		return NewNoopConstraint()
	}
	constraintRegistryMu.RLock()
	factory, ok := constraintRegistry[moduleID]
	constraintRegistryMu.RUnlock()
	if !ok {
		panic(fmt.Sprintf("t1_simple_margin/host: constraint module %q is not registered (known: %v)",
			moduleID, RegisteredConstraintModules()))
	}
	mod := factory()
	if got := string(mod.ID()); got != moduleID {
		panic(fmt.Sprintf(
			"t1_simple_margin/host: module %q factory returned module whose ID()=%q",
			moduleID, got))
	}
	return mod
}

// RegisteredConstraintModules returns the sorted list of non-noop
// module IDs currently in the registry.
func RegisteredConstraintModules() []string {
	constraintRegistryMu.RLock()
	defer constraintRegistryMu.RUnlock()
	out := make([]string, 0, len(constraintRegistry))
	for k := range constraintRegistry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
