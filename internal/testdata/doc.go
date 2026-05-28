// Package testdata is the synthesis backend for cmd/gen-testdata (R11-A).
// Generates real-scale standard-CSV snapshots that the smoke harness
// (R11-B) feeds into the engine for measurement (R11-D).
//
// Internal package — engine production binaries (cmd/{keygen,witness,
// prover,verifier,userproof}) do NOT depend on this. Only dev tools
// link it.
//
// Contract:
//
//   - Per-asset sum equality: Σ user.equity[asset] == cex.total_equity[asset]
//     (also for debt, collateral fields). The CexAssetInfo emitted by
//     the generator already aggregates the synthesised user rows.
//   - Account IDs are 64-hex strings reduced to canonical BN254
//     fr.Element form (same as core/snapshot/<model>/parser.go
//     canonicalAccountID). Generated row hex matches the on-leaf bytes.
//   - Per-user invariants per model (e.g. T2/T3/T4: Σ collateral ≤
//     equity at user level) are honoured by the distribution sampler.
//
// File layout per output directory:
//
//   <out>/
//     accounts.csv       (all models)
//     cex_assets.csv     (all models)
//     tier_ratios.csv    (T3, T4 only)
//
// Each file follows the standard-CSV schema declared by
// core/snapshot/<model>/standard_schema.go — same parser path the
// engine binaries use, so byte-equivalent.
//
// Public API (planned):
//
//	type Options struct { ... }
//	func GenerateScale(profile *declarative.Profile, users int, opts Options) error
//
// Dispatched internally to per-model generators (t1.go, t2.go, t3.go,
// t4.go), each consuming/emitting its own model-typed CexAssetInfo
// and AccountInfo shapes via core/solvency/<model>/spec.
package testdata
