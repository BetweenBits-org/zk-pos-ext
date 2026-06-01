// Proof loading + on-wire decoding. Two ingestion paths share one
// ProofRow seam:
//
//   - CSV (legacy / smoke harness): an injected vfs.ByteSource — cmd/verifier
//     wraps cfg.ProofTable via osvfs.File so the engine reads no path itself.
//   - MySQL proof table (production): the injected corehost.ProofStore port.
//
// convertStoredProof keeps the two paths byte-equivalent at the
// ProofRow boundary so verify.go does not need to know which source
// the row came from.

package verifier

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/io/vfs"
	vconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/verifier/config"
	"github.com/gocarina/gocsv"
)

// loadProofs reads proof rows either from the injected ProofStore (when
// cfg.MysqlDataSource is set — the store is wired by cmd/verifier) or
// from the injected ProofCSV byte source (the legacy / smoke path; cmd
// wraps cfg.ProofTable via osvfs.File). In both cases the returned slice
// is indexed by BatchNumber — i.e. result[i] is the proof for batch i,
// assuming batch numbers are a dense 0..N-1 sequence as the prover
// produces. The engine selects the path from config but performs no IO
// itself; both sources are injected.
func loadProofs(ctx context.Context, cfg *vconfig.Config, proofs corehost.ProofStore, proofCSV vfs.ByteSource) ([]corehost.ProofRow, error) {
	if cfg.MysqlDataSource != "" {
		if proofs == nil {
			return nil, fmt.Errorf("Proofs port is required when Config.MysqlDataSource is set")
		}
		return loadProofsFromStore(proofs, cfg.DbSuffix)
	}
	if proofCSV == nil {
		return nil, fmt.Errorf("ProofCSV source is required for the CSV proof path")
	}
	return loadProofsFromCSV(ctx, proofCSV)
}

// loadProofsFromCSV reads the proof CSV bytes from the injected source
// and re-indexes the resulting slice so result[i] is the proof for
// batch i.
func loadProofsFromCSV(ctx context.Context, src vfs.ByteSource) ([]corehost.ProofRow, error) {
	raw, err := src.ReadAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("read proof table: %w", err)
	}
	tmp := []*corehost.ProofRow{}
	if err := gocsv.UnmarshalBytes(raw, &tmp); err != nil {
		return nil, fmt.Errorf("parse proof table: %w", err)
	}
	out := make([]corehost.ProofRow, len(tmp))
	for i := range tmp {
		out[tmp[i].BatchNumber] = *tmp[i]
	}
	return out, nil
}

// loadProofsFromStore reads every proof row from the injected
// ProofStore in BatchNumber order and converts each ProofDTO into the
// ProofRow shape the verifier downstream consumes. The conversion
// mirrors the CSV column layout: ProofInfo / BatchCommitment /
// AssetsCount / BatchNumber are direct copies; CexAssetListCommitments
// and AccountTreeRoots are unmarshalled from JSON (the prover writes
// them as JSON-encoded [][]byte → []base64-string arrays).
func loadProofsFromStore(proofs corehost.ProofStore, dbSuffix string) ([]corehost.ProofRow, error) {
	rows, err := proofs.ListAllInOrder()
	if err != nil {
		return nil, fmt.Errorf("list proofs: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("proof table is empty (suffix %q)", dbSuffix)
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

// convertStoredProof maps one corehost.ProofDTO into the ProofRow shape
// the verifier uses. The two JSON-encoded slices are decoded directly
// into []string — json.Marshal of [][]byte writes base64-encoded
// strings, which is the same on-wire shape the CSV path produces.
func convertStoredProof(row corehost.ProofDTO) (corehost.ProofRow, error) {
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
// cex-asset-commitment pairs of one proof row. Returns an error
// describing the first decode failure encountered; callers propagate
// it with the batch context.
func decodeBatchMetadata(p corehost.ProofRow) (roots [][]byte, commits [][]byte, err error) {
	roots = make([][]byte, 2)
	commits = make([][]byte, 2)
	for i := 0; i < len(p.AccountTreeRoots) && i < 2; i++ {
		v, decErr := base64.StdEncoding.DecodeString(p.AccountTreeRoots[i])
		if decErr != nil {
			return nil, nil, fmt.Errorf("decode account tree root[%d]: %w", i, decErr)
		}
		roots[i] = v
	}
	for i := 0; i < len(p.CexAssetCommitment) && i < 2; i++ {
		v, decErr := base64.StdEncoding.DecodeString(p.CexAssetCommitment[i])
		if decErr != nil {
			return nil, nil, fmt.Errorf("decode cex asset commitment[%d]: %w", i, decErr)
		}
		commits[i] = v
	}
	return roots, commits, nil
}
