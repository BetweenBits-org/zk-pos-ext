// Package binance is Binance's complete customer-deployment profile.
//
// This profile selects the tier_3bucket solvency model and supplies
// every adapter implementation the PoR engine needs to run end-to-end
// against Binance's snapshot ETL:
//
//   - core/spec  : AssetCatalog, PriceScaleProvider, AccountIDProvider,
//                  InvalidAccountPolicy, BatchShapeProvider
//   - core/solvency/tier_3bucket/spec : RiskPolicy, SnapshotSource,
//                                        ConstraintModule
//
// Each adapter lives in its own file. Construct what a service needs
// via the New* functions and wire them into the witness / prover /
// verifier entry points.
//
// Multi-customer note: if a second customer adopts tier_3bucket, place
// its profile under zkpor/profile/<customer>/ as a sibling of this
// package. The shared model code (circuit + spec) lives at
// zkpor/core/solvency/tier_3bucket/.
package binance

import "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"

// SolvencyModel is the SolvencyModelID this profile targets.
// Re-exported so wiring code doesn't need to import core/spec just
// for the constant.
const SolvencyModel = spec.ModelTier3Bucket
