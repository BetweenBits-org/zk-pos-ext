// Package merkle_classic is the Standard-tier solvency model.
//
// STATUS: catalog-reserved, implementation pending.
//
// Solvency: spot_simple + the classical per-account Merkle inclusion
// proof, hoisted into the zk circuit. Equivalent to the Merkle-PoR
// pattern most non-Binance margin exchanges currently publish off-
// chain (Bybit, KuCoin, HTX), but composed in-circuit so the entire
// batch is a single verifiable artifact.
//
// Adapter surface (when implemented):
//   - core/spec.AssetCatalog
//   - core/spec.PriceScaleProvider
//   - core/spec.AccountIDProvider
//   - core/spec.InvalidAccountPolicy
//   - core/spec.BatchShapeProvider
//   - <this package>/spec.SnapshotSource
//
// Notably absent (vs tier_3bucket): RiskPolicy, per-user debt-vs-
// collateral arithmetic.
//
// Target customers: mid-tier margin exchanges upgrading from
// legacy off-chain Merkle PoR.
package merkle_classic
