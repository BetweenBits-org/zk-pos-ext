package binance

import (
	"context"
	"errors"

	modelspec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/spec"
)

// SnapshotConfig is the static input needed to construct a CSV-backed
// SnapshotSource. Mirrors the legacy witness/config.json:UserDataFile.
type SnapshotConfig struct {
	// UserDataDir is the directory containing per-shard user CSV
	// files plus the cex_assets_info.csv summary file.
	UserDataDir string

	// SnapshotID is a stable identifier embedded in published
	// artifacts (e.g. "2026-01-15T00:00:00Z").
	SnapshotID string
}

type csvSnapshot struct {
	cfg SnapshotConfig
}

// NewSnapshotCSV constructs a SnapshotSource backed by the legacy CSV
// directory layout. STUB: AccountStream and CexAssets currently return
// errStubSnapshot until the legacy parsers
// (src/utils/utils.go:ParseUserDataSet) are absorbed.
func NewSnapshotCSV(cfg SnapshotConfig) modelspec.SnapshotSource {
	return &csvSnapshot{cfg: cfg}
}

var errStubSnapshot = errors.New("binance snapshot CSV loader not yet absorbed from src/utils/utils.go")

func (c *csvSnapshot) AccountStream(ctx context.Context) (<-chan modelspec.AccountInfo, error) {
	return nil, errStubSnapshot
}

func (c *csvSnapshot) CexAssets(ctx context.Context) ([]modelspec.CexAssetInfo, error) {
	return nil, errStubSnapshot
}

func (c *csvSnapshot) SnapshotID() string { return c.cfg.SnapshotID }
