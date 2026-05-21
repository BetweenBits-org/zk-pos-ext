// Package spec defines the constants and contracts that constitute the
// PoR engine standard. Anything here is invariant across solvency
// models and customer deployments — changing it breaks verifier
// compatibility with proofs produced under the prior definition.
package spec

const (
	// AccountTreeDepth is the depth of the sparse Merkle tree used for
	// the account commitment. Determines maximum user capacity (2^depth)
	// and the trusted setup. MUST NOT differ between proof producer
	// and verifier.
	AccountTreeDepth = 28

	// TierCount is the per-collateral-bucket tier slot count used by
	// tier-based solvency models. MUST be even — circuits in those
	// models pack two TierRatio entries into one field element.
	TierCount = 12

	// AssetCounts is the global capacity for distinct CEX assets in one
	// proof. Deployments with fewer assets pad with "reserved" slots so
	// circuit-instance size is constant.
	AssetCounts = 500

	// DefaultPriceScale is the recommended multiplier from a quote-
	// currency float price to the uint64 BasePrice value embedded in
	// the witness. Deployments MAY override per-asset via
	// PriceScaleProvider to support very low unit-value assets
	// (e.g. SHIB, PEPE).
	DefaultPriceScale int64 = 100_000_000 // 1e8

	// DefaultBalanceScale is the recommended multiplier from a float
	// balance to the uint64 value embedded in the witness. Paired with
	// DefaultPriceScale so the product equals DefaultValueScale.
	DefaultBalanceScale int64 = 100_000_000 // 1e8

	// DefaultValueScale is the product DefaultPriceScale *
	// DefaultBalanceScale. It is the unit of TotalEquity, TotalDebt,
	// and TotalCollateral values in the witness and user proof.
	// PriceScaleProvider implementations MUST report this same
	// ValueScale across all symbols within one proof.
	DefaultValueScale int64 = DefaultPriceScale * DefaultBalanceScale // 1e16
)
