// Package config declares the on-disk configuration shapes consumed by
// the zkpor verifier CLI. It is the zkpor-native replacement for legacy
// src/verifier/config — the structs mirror the legacy fields so an
// existing config.json / user_config.json keeps working, but the asset
// types are the zkpor tier_3bucket spec types (no src/utils import).
package config

import (
	"math/big"

	tier3spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/spec"
)

// Config drives the batch-verification mode of the verifier: it points
// at the prover's proof table, the per-tier verifying keys, and the
// published CEX asset totals whose commitment the proofs must match.
type Config struct {
	// ProofTable is the path to the prover-produced proof CSV.
	ProofTable string

	// ZkKeyName lists the verifying-key file stems, one per entry of
	// AssetsCountTiers (same index). The verifier appends ".vk".
	ZkKeyName []string

	// AssetsCountTiers lists the per-batch asset-count tiers in the
	// same order as ZkKeyName. A proof row's assets_count selects the
	// matching verifying key by position.
	AssetsCountTiers []int

	// CexAssetsInfo is the published per-asset global state. Its
	// Poseidon commitment must equal the final batch's after-CEX
	// commitment. Entries MUST carry full corespec.TierCount-length
	// ratio tables — an operator-supplied incomplete table yields a
	// commitment mismatch, not a silent pass.
	CexAssetsInfo []tier3spec.CexAssetInfo
}

// UserConfig drives the single-user inclusion-verification mode
// (verifier -user). It is the userproof service's per-user output: the
// account's position, balances, asset list, and Merkle path.
type UserConfig struct {
	AccountIndex    uint32
	AccountIdHash   string
	TotalEquity     big.Int
	TotalDebt       big.Int
	TotalCollateral big.Int
	Root            string
	Assets          []tier3spec.AccountAsset
	Proof           []string
}
