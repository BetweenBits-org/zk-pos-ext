package host

import (
	"fmt"
	"sort"
	"sync"

	t4spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
)

// ConstraintModuleFactory constructs a T4 ConstraintModule. Factories
// MUST be cheap (no I/O) and SHOULD return stateless modules so a
// single instance is reused across batches.
//
// Module registration happens at build time via init() in whichever
// package owns the module implementation (a profile, a regulator-
// specific module package under core/constraint_modules/, etc.).
type ConstraintModuleFactory func() t4spec.ConstraintModule

// Constraint module registry (T4 — R8-B/3 / G17).
//
// Layer ownership: each model's host package owns its own table
// because ConstraintModule is model-typed (T4's ConstraintContext has
// collateral/tier-ratio fields that don't exist on T1).
//
// ID format convention (G17): "<module_id>.v<version>". The engine-
// default no-op is special-cased — when profile.toml's
// constraint.module is empty (corespec.NoExtensionID), NewConstraintModule
// returns the universal noop without a registry lookup. Non-empty IDs
// MUST be registered or the lookup panics.
//
// v1 catalog has zero non-noop entries (R7-C governance freeze).
// Customer-local or regulator-specific modules added later follow the
// same G17 lock as the snapshot connector registry.
var (
	constraintRegistryMu sync.RWMutex
	constraintRegistry   = map[string]ConstraintModuleFactory{}
)

// RegisterConstraintModule adds (moduleID → factory). Intended to be
// called from package init().
//
// Panics:
//   - empty moduleID.
//   - nil factory.
//   - duplicate moduleID — single-owner invariant per engine binary.
//
// Note: corespec.NoExtensionID ("") is reserved for the engine-default
// noop and CANNOT be registered (the empty-id panic guards this).
func RegisterConstraintModule(moduleID string, factory ConstraintModuleFactory) {
	if moduleID == "" {
		panic("t4_tiered_haircut_margin_3pool/host: RegisterConstraintModule called with empty module id")
	}
	if factory == nil {
		panic("t4_tiered_haircut_margin_3pool/host: RegisterConstraintModule(" + moduleID + ") called with nil factory")
	}
	constraintRegistryMu.Lock()
	defer constraintRegistryMu.Unlock()
	if _, dup := constraintRegistry[moduleID]; dup {
		panic("t4_tiered_haircut_margin_3pool/host: constraint module " + moduleID + " registered twice")
	}
	constraintRegistry[moduleID] = factory
}

// NewConstraintModule returns the T4 ConstraintModule corresponding to
// the given moduleID. Empty ID is the engine-default noop (no registry
// lookup); non-empty IDs MUST be registered or the call panics.
//
// Service startup invokes this with profile.toml's constraint.module
// value verbatim — empty string in the TOML maps cleanly to noop.
func NewConstraintModule(moduleID string) t4spec.ConstraintModule {
	if moduleID == corespec.NoExtensionID {
		return NewNoopConstraint()
	}
	constraintRegistryMu.RLock()
	factory, ok := constraintRegistry[moduleID]
	constraintRegistryMu.RUnlock()
	if !ok {
		panic(fmt.Sprintf("t4_tiered_haircut_margin_3pool/host: constraint module %q is not registered (known: %v)",
			moduleID, RegisteredConstraintModules()))
	}
	mod := factory()
	if got := string(mod.ID()); got != moduleID {
		panic(fmt.Sprintf(
			"t4_tiered_haircut_margin_3pool/host: module %q factory returned module whose ID()=%q",
			moduleID, got))
	}
	return mod
}

// RegisteredConstraintModules returns the sorted list of non-noop
// module IDs currently in the registry. The noop is intentionally
// omitted (it isn't a registry entry — see NewConstraintModule).
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
