// Package constraint_modules is the v1 catalog root for ConstraintModule
// promotions — model-agnostic add-only zk-constraints that customer
// profiles can compose with one of the v1 solvency models (T1~T4).
//
// STATUS (R7): catalog v1 FROZEN at zero entries. The directory exists
// so the layout + governance is locked; concrete entries land in R7+1
// (first customer module that meets the rule-of-three).
//
// # Why this exists
//
// A solvency model (core/solvency/<id>/) carries the math invariant
// (sum equality, per-user solvency rule). A ConstraintModule layers
// *additional* constraints on top — regulatory caps, business rules,
// fraud-vector guards — without weakening the base. See
// docs/02-module-architecture.md §1-5 for the architecture lock.
//
// # ID prefix scheme (G9 closure)
//
// Modules in this directory MUST use one of two filename-safe prefixes:
//
//	core/constraint_modules/regulator/<jurisdiction>/<rule>_v<v>/
//	    → ID format: regulator.<jurisdiction>.<rule>_v<v>
//	    → example:   regulator.kr.user_limit_v1
//
//	core/constraint_modules/business/<pattern>_v<v>/
//	    → ID format: business.<pattern>_v<v>
//	    → example:   business.spot_only_v1
//
// Customer-specific (non-promoted) modules live in
// profile/<customer>/<module>.go and use the `<exchange>.<rule>_v<v>`
// prefix (e.g. `binance.vip_loan_carveout_v1`).
//
// # Rule-of-three promotion gate
//
// A pattern is promoted from profile/<customer>/ into this catalog
// only after **three independent customer adoptions of the same
// pattern**. Until then, the module stays customer-local.
//
// `noop` is structurally exempt (universal by construction) but at v1
// the per-model ConstraintContext field sets diverge enough that a
// single generic noop type is impractical — each model carries its
// own in-package noop helper (`profile/<customer>/constraint_noop.go`
// today). A type-parametric or interface-dispatched universal noop
// is a v2 candidate.
//
// # v1 freeze governance (R7)
//
// The directory is frozen as the catalog *layout*. Entry additions
// follow rule-of-three; removals follow deprecate-then-remove across
// two version cycles minimum. ID format above is locked — renames
// disallowed in v1.
//
// # Composition
//
// One circuit instance carries at most one alpha-layer
// ConstraintModule slot (BatchCreateUserCircuit.SetConstraintModule).
// N modules are composed via the Composite pattern — see
// docs/02-module-architecture.md §2.3. Composite ID derives from
// canonical-sorted child IDs (§2.4).
//
// Each (model, module-or-composite) pair forks the trusted setup:
// distinct .pk/.vk pair. The .vk-naming scheme is
// `zkpor.<model>.<shape>[.<module>]` (BatchShape.StandardKeyName).
package constraint_modules
