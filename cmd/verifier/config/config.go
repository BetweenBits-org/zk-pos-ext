// Package config declares the on-disk configuration shapes consumed by
// the zkpor verifier CLI. It is the zkpor-native replacement for legacy
// src/verifier/config — the structs mirror the legacy fields so an
// existing config.json / user_config.json keeps working, but the asset
// types are the zkpor tier_3bucket spec types (no src/utils import).
package config

import (
	tier3spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/spec"
)

// Config drives the batch-verification mode of the verifier: it points
// at the prover's proof table, the per-tier verifying keys, and the
// published CEX asset totals whose commitment the proofs must match.
//
// Proof rows can come from either a CSV file (ProofTable) or the
// prover's MySQL proof table directly (MysqlDataSource + DbSuffix).
// When MysqlDataSource is set, ProofTable is ignored. The DB path is
// the zkpor-preferred mode — no CSV intermediate, no separate exporter.
type Config struct {
	// ProofTable is the path to the prover-produced proof CSV. Ignored
	// when MysqlDataSource is non-empty.
	ProofTable string

	// MysqlDataSource (optional) is a gorm/MySQL DSN pointing at the
	// prover's proof table. When set, the verifier loads proof rows
	// directly via ProofStore.ListAllInOrder instead of parsing
	// ProofTable.
	MysqlDataSource string

	// DbSuffix (optional) is the table-name suffix shared with the
	// prover service. Empty in production, e.g. "_test" in CI. Used
	// only when MysqlDataSource is set.
	DbSuffix string

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

// UserConfig (per-user inclusion-proof artifact) is defined in
// zkpor/core/solvency/tier_3bucket/host so the userproof writer and
// verifier reader share one type. Import tier3host.UserConfig.
