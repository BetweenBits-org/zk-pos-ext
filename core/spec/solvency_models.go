package spec

import "slices"

// SolvencyModelID identifies one of the audited math patterns
// (=circuit + per-user solvency rules) supported by the PoR engine.
// Embedded into .pk/.vk filenames so verifiers can locate the right
// .vk and audit which constraints back a given proof.
//
// Stable, filesystem-safe (lowercase letters, digits, underscores;
// no dots).
type SolvencyModelID string

// Solvency-model catalog.
//
// This is the public, audited list of models supported by the engine.
// Each entry maps to a directory under zkpor/core/solvency/<id>/ and
// to a published .pk/.vk pair per BatchShape.
//
// Catalog governance:
//
//   - Adding a model requires its circuit to be audited and its
//     trusted-setup ceremony completed.
//   - Removing a model is a breaking change for any customer whose
//     proofs reference it. Deprecate before removal.
//   - Each batch proof targets exactly one model (single-selection —
//     composition is intentionally not supported at v1).
const (
	// ModelSpotSimple — Basic tier.
	//
	// Solvency: published_total[asset] == sum_users(user[asset]).
	// No user-level debt or collateral logic; assumes users carry no
	// liabilities to the exchange.
	//
	// Target customers: regulated spot-only exchanges (Korea, EU,
	// Japan), stablecoin issuers, custodians.
	ModelSpotSimple SolvencyModelID = "spot_simple"

	// ModelMerkleClassic — Standard tier.
	//
	// Solvency: spot_simple + per-account Merkle inclusion proof in
	// the zk circuit. Equivalent to the historical off-chain Merkle-PoR
	// pattern (Bybit, KuCoin, HTX) but composed end-to-end in zk.
	//
	// Target customers: mid-tier margin exchanges upgrading from
	// legacy Merkle-only PoR.
	ModelMerkleClassic SolvencyModelID = "merkle_classic"

	// ModelOverCollateralSimple — Pro tier (variant A).
	//
	// Solvency: single per-user collateral bucket, fixed (non-tiered)
	// haircut per asset.
	//   for each user: sum_i(collateral_i * haircut_i) >= user.totalDebt
	//
	// Target customers: margin exchanges with simple risk models —
	// one collateral pool, asset-level (not size-tiered) haircuts.
	ModelOverCollateralSimple SolvencyModelID = "over_collateral_simple"

	// ModelTier1Bucket — Pro tier (variant B).
	//
	// Solvency: tier-based piecewise-linear haircut with a single
	// collateral bucket. Same curve shape as tier_3bucket without the
	// Loan/Margin/PortfolioMargin split.
	//
	// Target customers: derivatives-heavy exchanges that have not
	// segmented collateral by business line.
	ModelTier1Bucket SolvencyModelID = "tier_1bucket"

	// ModelTier3Bucket — Enterprise tier.
	//
	// Solvency: three-bucket collateral (Loan / Margin /
	// PortfolioMargin), tier-based piecewise-linear haircut per
	// bucket. Full per-user solvency.
	//
	// Reference implementation in core/solvency/tier_3bucket/ —
	// circuit inherited from the Binance OSS PoR v2 codebase.
	//
	// Target customers: Binance-class exchanges with VIP loan +
	// cross margin + portfolio margin business lines.
	ModelTier3Bucket SolvencyModelID = "tier_3bucket"
)

// CatalogedModels lists every model in the audited catalog in tier
// order (Basic → Enterprise). Adding to this list is a versioned,
// audited change.
var CatalogedModels = []SolvencyModelID{
	ModelSpotSimple,
	ModelMerkleClassic,
	ModelOverCollateralSimple,
	ModelTier1Bucket,
	ModelTier3Bucket,
}

// IsCataloged reports whether the given ID is in the audited catalog.
// Services SHOULD reject proof requests for non-cataloged IDs.
func IsCataloged(id SolvencyModelID) bool {
	return slices.Contains(CatalogedModels, id)
}
