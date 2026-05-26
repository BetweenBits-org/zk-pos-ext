// Package sea_reference is a hypothetical Southeast Asia (SEA)
// spot-only exchange profile, used to validate the model→customer flow
// for the spot_simple solvency model end-to-end (R5).
//
// Status: synthetic reference, NOT a real customer integration. Once
// a concrete SEA partner (Indodax / Tokocrypto / Pintu / Bitkub /
// other) is confirmed, rename this directory to the customer's name
// and adjust catalog + snapshot to match their actual asset list and
// CSV layout. The adapter shapes here are stable across that rename.
//
// Model: spot_simple — single-balance-per-asset user state, no debt,
// no collateral. The model invariant is per-asset sum equality:
//
//	cex.TotalEquity[asset] == Σ user.Equity[asset]
//
// Adapter surface (one Go file each):
//
//   - core/spec.AssetCatalog            → catalog.go
//   - core/spec.PriceScaleProvider      → pricing.go
//   - core/spec.AccountIDProvider       → identity.go
//   - core/spec.InvalidAccountPolicy    → insolvent.go
//   - core/spec.BatchShapeProvider      → batch_shape.go
//   - core/solvency/spot_simple/spec.ConstraintModule (noop choice)
//                                       → constraint_noop.go
//   - core/solvency/spot_simple/spec.SnapshotSource
//                                       → snapshot.go (R5-2)
//
// Notably absent (vs profile/binance): risk.go — spot_simple has no
// RiskPolicy interface (no haircut math).
package sea_reference

import (
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// SolvencyModel is the SolvencyModelID this profile targets.
// Re-exported so wiring code doesn't need to import core/spec just
// for the constant.
const SolvencyModel = corespec.ModelSpotSimple
