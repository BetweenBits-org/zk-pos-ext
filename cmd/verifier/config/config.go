// Package config declares the on-disk configuration shapes consumed by
// the zkpor verifier CLI. R8-D slimmed Config to deployment-secret +
// per-snapshot fields only; per-customer values (asset capacity, tiers,
// verifying-key stems) are derived from profile.toml + the -keys-dir
// flag.
//
// Phase 3d (R10+1): CexAssetsInfo is kept as json.RawMessage so the
// verifier can dispatch to the per-model runner that knows the typed
// schema. Each model's BuildCexCommitments unmarshals into its own
// CexAssetInfo shape (T1: no collateral; T2: Haircut + Collateral;
// T3: 1-pool tiered; T4: 3-pool tiered).
package config

import "encoding/json"

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

	// CexAssetsInfo is the published per-asset global state, kept as
	// raw JSON so the verifier can dispatch to a per-model runner that
	// unmarshals it into its model-typed slice. Its Poseidon commitment
	// must equal the final batch's after-CEX commitment. Entries MUST
	// carry the full per-model schema — operator-supplied incomplete
	// data yields a commitment mismatch, not a silent pass.
	CexAssetsInfo json.RawMessage
}

// UserConfig (per-user inclusion-proof artifact) is defined per model
// in zkpor/core/solvency/<model>/host so the userproof writer and
// verifier reader share one type. The verifier dispatches on
// profile.Model to pick the matching runner.
