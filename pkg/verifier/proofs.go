// Proof loading + on-wire decoding. Two ingestion paths share one
// ProofRow seam:
//
//   - CSV file (legacy / smoke harness): cfg.ProofTable.
//   - MySQL proof table (production): cfg.MysqlDataSource + cfg.DbSuffix.
//
// convertStoredProof keeps the two paths byte-equivalent at the
// ProofRow boundary so verify.go does not need to know which source
// the row came from.

package verifier

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	vconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/verifier/config"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"
	"github.com/gocarina/gocsv"
)

// loadProofs reads proof rows either from the prover's MySQL proof
// table (when cfg.MysqlDataSource is set) or from the legacy CSV at
// cfg.ProofTable. In both cases the returned slice is indexed by
// BatchNumber — i.e. result[i] is the proof for batch i, assuming
// batch numbers are a dense 0..N-1 sequence as the prover produces.
func loadProofs(cfg *vconfig.Config) ([]corehost.ProofRow, error) {
	if cfg.MysqlDataSource != "" {
		return loadProofsFromDB(cfg)
	}
	return loadProofsFromCSV(cfg.ProofTable)
}

// loadProofsFromCSV unmarshals the CSV at path and re-indexes the
// resulting slice so result[i] is the proof for batch i.
func loadProofsFromCSV(path string) ([]corehost.ProofRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open proof table %q: %w", path, err)
	}
	defer f.Close()

	tmp := []*corehost.ProofRow{}
	if err := gocsv.UnmarshalFile(f, &tmp); err != nil {
		return nil, fmt.Errorf("parse proof table %q: %w", path, err)
	}
	out := make([]corehost.ProofRow, len(tmp))
	for i := range tmp {
		out[tmp[i].BatchNumber] = *tmp[i]
	}
	return out, nil
}

// loadProofsFromDB reads every proof row from the prover's proof table
// in BatchNumber order and converts each store.Proof into the ProofRow
// shape the verifier downstream consumes. The conversion mirrors the
// CSV column layout: ProofInfo / BatchCommitment / AssetsCount /
// BatchNumber are direct copies; CexAssetListCommitments and
// AccountTreeRoots are unmarshalled from JSON (the prover writes them
// as JSON-encoded [][]byte → []base64-string arrays).
func loadProofsFromDB(cfg *vconfig.Config) ([]corehost.ProofRow, error) {
	db, err := store.Open(cfg.MysqlDataSource)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	proofStore := store.NewProofStore(db, cfg.DbSuffix)
	rows, err := proofStore.ListAllInOrder()
	if err != nil {
		return nil, fmt.Errorf("list proofs: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("proof table is empty (suffix %q)", cfg.DbSuffix)
	}
	out := make([]corehost.ProofRow, len(rows))
	for _, row := range rows {
		converted, err := convertStoredProof(row)
		if err != nil {
			return nil, fmt.Errorf("batch %d: %w", row.BatchNumber, err)
		}
		if row.BatchNumber < 0 || int(row.BatchNumber) >= len(rows) {
			return nil, fmt.Errorf("batch number %d out of dense range [0,%d)", row.BatchNumber, len(rows))
		}
		out[row.BatchNumber] = converted
	}
	return out, nil
}

// convertStoredProof maps one store.Proof into the ProofRow shape the
// verifier uses. The two JSON-encoded slices are decoded directly into
// []string — json.Marshal of [][]byte writes base64-encoded strings,
// which is the same on-wire shape the CSV path produces.
func convertStoredProof(row store.Proof) (corehost.ProofRow, error) {
	var cexCommits []string
	if err := json.Unmarshal([]byte(row.CexAssetListCommitments), &cexCommits); err != nil {
		return corehost.ProofRow{}, fmt.Errorf("unmarshal cex commitments: %w", err)
	}
	var treeRoots []string
	if err := json.Unmarshal([]byte(row.AccountTreeRoots), &treeRoots); err != nil {
		return corehost.ProofRow{}, fmt.Errorf("unmarshal account tree roots: %w", err)
	}
	return corehost.ProofRow{
		BatchNumber:        row.BatchNumber,
		ZkProof:            row.ProofInfo,
		CexAssetCommitment: cexCommits,
		AccountTreeRoots:   treeRoots,
		BatchCommitment:    row.BatchCommitment,
		AssetsCount:        row.AssetsCount,
	}, nil
}

// decodeBatchMetadata base64-decodes the account-tree-roots and
// cex-asset-commitment pairs of one proof row.
func decodeBatchMetadata(p corehost.ProofRow) (roots [][]byte, commits [][]byte) {
	roots = make([][]byte, 2)
	commits = make([][]byte, 2)
	for i := 0; i < len(p.AccountTreeRoots) && i < 2; i++ {
		v, err := base64.StdEncoding.DecodeString(p.AccountTreeRoots[i])
		if err != nil {
			panic("decode account tree root failed")
		}
		roots[i] = v
	}
	for i := 0; i < len(p.CexAssetCommitment) && i < 2; i++ {
		v, err := base64.StdEncoding.DecodeString(p.CexAssetCommitment[i])
		if err != nil {
			panic("decode cex asset commitment failed")
		}
		commits[i] = v
	}
	return roots, commits
}
