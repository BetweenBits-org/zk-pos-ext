package spec

import "slices"

// SolvencyModelID identifies one of the audited math patterns
// (=circuit + per-user solvency rules) supported by the PoR engine.
// Embedded into .pk/.vk filenames so verifiers can locate the right
// .vk and audit which constraints back a given proof.
//
// Stable, filesystem-safe (lowercase letters, digits, underscores;
// no dots).
//
// Naming convention (R6 freeze): "t<N>_<math_dimension>_margin[_pool_suffix]".
// T<N> is the catalog tier (1..4 in ascending verification richness /
// circuit cost). All entries share the same 5-input Poseidon account
// leaf — see docs/04-solvency-models.md §3.
type SolvencyModelID string

// Solvency-model catalog (R6, 4-entry).
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
//
// Spot use case: T1 with all user.TotalDebt = 0 (constraint trivially
// satisfied). The former separate `t1_simple_margin` entry was absorbed
// into T1 in R6 — see docs/04-solvency-models.md §8.1.
const (
	// T1SimpleMargin — Basic / Standard tier (spot absorbed).
	//
	// Solvency: per-user TotalEquity >= TotalDebt (account-level only;
	// no risk-weighted collateral). Spot-only deployments supply
	// TotalDebt = 0 and the constraint is trivially satisfied — one
	// ceremony serves both spot and simple-margin customers.
	//
	// Reference: Binance OSS PoR v2 (non-negative net constraint),
	// Bybit / KuCoin / HTX off-chain Merkle PoR (in-circuit hoist),
	// OKX zk-STARK Merkle Sum Tree (3 constraints: Total / Non-neg /
	// Inclusion).
	//
	// Target customers: regulated spot exchanges (Korea, EU, Japan,
	// SEA) + mid-tier margin exchanges (Bybit, KuCoin, HTX class)
	// upgrading from legacy Merkle-only PoR.
	T1SimpleMargin SolvencyModelID = "t1_simple_margin"

	// T2StaticHaircutMargin — Pro-A tier.
	//
	// Solvency: per-user sum_i(collateral_i * haircut_i) >= TotalDebt,
	// where haircut_i is a per-asset constant supplied by RiskPolicy.
	//
	// Reference: Aave V3 LTV / Liquidation Threshold per-asset
	// configuration.
	//
	// Target customers: margin exchanges with simple risk models —
	// one collateral pool, asset-level (not size-tiered) haircuts.
	T2StaticHaircutMargin SolvencyModelID = "t2_static_haircut_margin"

	// T3TieredHaircutMargin1Pool — Pro-B tier.
	//
	// Solvency: per-user sum_i haircut_curve_i(collateral_i) >= TotalDebt,
	// where haircut_curve_i is a piecewise-linear, size-tiered function
	// of the collateral amount in asset i. Single collateral pool.
	//
	// Reference: dYdX IMF curve (piecewise-linear over open notional),
	// Bitget / Gate tiered MMR.
	//
	// Target customers: derivatives-heavy exchanges that have not
	// segmented collateral by business line.
	T3TieredHaircutMargin1Pool SolvencyModelID = "t3_tiered_haircut_margin_1pool"

	// T4TieredHaircutMargin3Pool — Enterprise tier.
	//
	// Solvency: per-user sum_{b in {Loan,Margin,PortfolioMargin}}
	//                       sum_i haircut_curve_b_i(collateral_b_i)
	//                       >= TotalDebt
	// — three independent collateral pools, each with its own
	// tier-haircut curve table.
	//
	// Reference implementation: Binance OSS PoR v2 (zkpor R1-R3 is
	// the productization of this circuit). Also: OKX zk-STARK PoR v2.
	//
	// Target customers: Binance / OKX class exchanges with VIP loan +
	// cross margin + portfolio margin business lines.
	T4TieredHaircutMargin3Pool SolvencyModelID = "t4_tiered_haircut_margin_3pool"
)

// CatalogedModels lists every model in the audited catalog in tier
// order (T1 → T4 = ascending verification richness / circuit cost).
// Adding to this list is a versioned, audited change.
var CatalogedModels = []SolvencyModelID{
	T1SimpleMargin,
	T2StaticHaircutMargin,
	T3TieredHaircutMargin1Pool,
	T4TieredHaircutMargin3Pool,
}

// ModelDisplay maps the audited catalog ID to a human-readable
// marketing label. Service code MUST use the ID (lowercase, filename-
// safe); display strings are for UI / docs / report headers only.
//
// The display label intentionally exposes the catalog-tier (Basic /
// Pro / Enterprise) framing from docs/01-project-context.md so the
// marketing surface stays consistent with the engineering catalog
// while the on-disk identifiers stay stable.
var ModelDisplay = map[SolvencyModelID]string{
	T1SimpleMargin:             "T1 · Simple Margin (spot-friendly)",
	T2StaticHaircutMargin:      "T2 · Static-Haircut Margin",
	T3TieredHaircutMargin1Pool: "T3 · Tiered-Haircut Margin · 1 Pool",
	T4TieredHaircutMargin3Pool: "T4 · Tiered-Haircut Margin · 3 Pool",
}

// IsCataloged reports whether the given ID is in the audited catalog.
// Services SHOULD reject proof requests for non-cataloged IDs.
func IsCataloged(id SolvencyModelID) bool {
	return slices.Contains(CatalogedModels, id)
}
