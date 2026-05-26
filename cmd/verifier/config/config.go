// Package config declares the on-disk configuration shapes consumed by
// the zkpor verifier CLI. R8-D slimmed Config to deployment-secret +
// per-snapshot fields only; per-customer values (asset capacity, tiers,
// verifying-key stems) are derived from profile.toml + the -keys-dir
// flag.
package config

import (
	t4spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec"
)

// Config drives the batch-verification mode of the verifier: it points
// at the prover's proof table and carries the published CEX asset
// totals whose commitment the proofs must match.
//
// Proof rows can come from either a CSV file (ProofTable) or the
// prover's MySQL proof table directly (MysqlDataSource + DbSuffix).
// When MysqlDataSource is set, ProofTable is ignored.
//
// Unknown JSON fields are tolerated by json.Unmarshal — pre-R8 configs
// that still carry ZkKeyName/AssetsCountTiers/AssetCapacity will load
// cleanly but those values are ignored (the verifier derives them).
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

	// CexAssetsInfo is the published per-asset global state. Its
	// Poseidon commitment must equal the final batch's after-CEX
	// commitment. Entries MUST carry full corespec.TierCount-length
	// ratio tables — an operator-supplied incomplete table yields a
	// commitment mismatch, not a silent pass.
	CexAssetsInfo []t4spec.CexAssetInfo
}

// UserConfig (per-user inclusion-proof artifact) is defined in
// zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host so the userproof writer and
// verifier reader share one type. Import t4host.UserConfig.
