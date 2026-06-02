package t2_static_haircut_margin

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/BetweenBits-org/zk-pos-ext/core/io/vfs"
	"github.com/BetweenBits-org/zk-pos-ext/core/io/vfs/osvfs"
	snapshotcsv "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/csv"
	snapshotmapping "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/mapping"
	snapshotschema "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/schema"
	t2host "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t2_static_haircut_margin/host"
	t2spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t2_static_haircut_margin/spec"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// ConnectorID is the T2 standard canonical CSV snapshot connector.
const ConnectorID = "t2_standard_csv.v1"

var basisPointsDivisor = big.NewInt(10_000)

func init() {
	t2host.RegisterSnapshot(ConnectorID, func(src vfs.Opener, snapshotID string, assetCapacity int, _ corespec.PriceScaleProvider) t2spec.SnapshotSource {
		return NewSnapshotCSV(Config{Source: src, SnapshotID: snapshotID, AssetCapacity: assetCapacity})
	})
}

// Config constructs a T2 standard CSV SnapshotSource.
//
// Source is the snapshot input opener. When Source is nil, the parser
// falls back to Dir, opening inputs as local files under that directory
// — a convenience used mainly by tests; production callers inject
// Source via the model host's NewSnapshot.
type Config struct {
	Source        vfs.Opener
	Dir           string
	SnapshotID    string
	AssetCapacity int
	Mapping       snapshotmapping.Config
}

// source returns the configured input opener, defaulting to a local
// osvfs.Dir(Dir) opener when Source is unset.
func (c Config) source() vfs.Opener {
	if c.Source != nil {
		return c.Source
	}
	return osvfs.Dir(c.Dir)
}

type snapshot struct {
	cfg Config

	once   sync.Once
	assets []t2spec.CexAssetInfo
	err    error

	invalidCount atomic.Uint64
}

// NewSnapshotCSV returns a canonical standard CSV SnapshotSource for T2.
func NewSnapshotCSV(cfg Config) t2spec.SnapshotSource { return &snapshot{cfg: cfg} }

// AccountStream streams account rows grouped by account_id.
func (s *snapshot) AccountStream(ctx context.Context) (<-chan t2spec.AccountInfo, error) {
	assets, err := s.CexAssets(ctx)
	if err != nil {
		return nil, err
	}
	accounts, err := s.loadAccounts(ctx, assets)
	if err != nil {
		return nil, err
	}
	out := make(chan t2spec.AccountInfo, 1024)
	go func() {
		defer close(out)
		for _, account := range accounts {
			select {
			case <-ctx.Done():
				return
			case out <- account:
			}
		}
	}()
	return out, nil
}

// CexAssets returns the capacity-padded canonical per-asset state.
func (s *snapshot) CexAssets(ctx context.Context) ([]t2spec.CexAssetInfo, error) {
	s.once.Do(func() { s.assets, s.err = s.loadCexAssets(ctx) })
	if s.err != nil {
		return nil, s.err
	}
	out := make([]t2spec.CexAssetInfo, len(s.assets))
	copy(out, s.assets)
	return out, nil
}

// SnapshotID returns the configured snapshot identifier.
func (s *snapshot) SnapshotID() string { return s.cfg.SnapshotID }

// InvalidCount returns the number of rejected account rows.
func (s *snapshot) InvalidCount() uint64 { return s.invalidCount.Load() }

func (s *snapshot) opts() (snapshotcsv.Options, error) {
	return snapshotmapping.BuildCSVOptions(s.cfg.Mapping.Format)
}

func (s *snapshot) loadCexAssets(ctx context.Context) ([]t2spec.CexAssetInfo, error) {
	if s.cfg.AssetCapacity <= 0 {
		return nil, fmt.Errorf("asset capacity must be > 0")
	}
	opts, err := s.opts()
	if err != nil {
		return nil, err
	}
	f, err := s.cfg.source().Open(ctx, "cex_assets.csv")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	reader, err := snapshotcsv.NewReader(f, schemaFile("cex_assets.csv"), opts)
	if err != nil {
		return nil, err
	}
	out := make([]t2spec.CexAssetInfo, s.cfg.AssetCapacity)
	for i := range out {
		out[i] = t2spec.CexAssetInfo{Symbol: "reserved", Index: uint32(i)}
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		idx64, err := row.Uint64("asset_index", 16)
		if err != nil {
			return nil, err
		}
		if int(idx64) >= s.cfg.AssetCapacity {
			return nil, fmt.Errorf("asset_index %d exceeds capacity", idx64)
		}
		idx := uint16(idx64)
		totalEquity, _ := row.Uint64("total_equity", 64)
		totalDebt, _ := row.Uint64("total_debt", 64)
		basePrice, _ := row.Uint64("base_price", 64)
		collateral, _ := row.Uint64("collateral", 64)
		haircut, _ := row.Uint64("haircut_bp", 16)
		symbol, _ := row.Required("symbol")
		out[idx] = t2spec.CexAssetInfo{
			TotalEquity: totalEquity,
			TotalDebt:   totalDebt,
			BasePrice:   basePrice,
			Symbol:      symbol,
			Index:       uint32(idx),
			Collateral:  collateral,
			Haircut:     uint16(haircut),
		}
	}
}

func (s *snapshot) loadAccounts(ctx context.Context, assets []t2spec.CexAssetInfo) ([]t2spec.AccountInfo, error) {
	opts, err := s.opts()
	if err != nil {
		return nil, err
	}
	f, err := s.cfg.source().Open(ctx, "accounts.csv")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	reader, err := snapshotcsv.NewReader(f, schemaFile("accounts.csv"), opts)
	if err != nil {
		return nil, err
	}
	groups := map[string]*t2spec.AccountInfo{}
	order := []string{}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			out := make([]t2spec.AccountInfo, 0, len(order))
			for i, key := range order {
				account := groups[key]
				account.AccountIndex = uint32(i)
				if account.TotalCollateral.Cmp(account.TotalDebt) >= 0 {
					out = append(out, *account)
				} else {
					s.invalidCount.Add(1)
				}
			}
			return out, nil
		}
		if err != nil {
			if errors.Is(err, snapshotcsv.ErrInvalidRow) {
				s.invalidCount.Add(1)
				continue
			}
			return nil, err
		}
		if err := addRow(row, assets, groups, &order); err != nil {
			s.invalidCount.Add(1)
		}
	}
}

func addRow(row snapshotcsv.Row, assets []t2spec.CexAssetInfo, groups map[string]*t2spec.AccountInfo, order *[]string) error {
	id, _ := row.Required("account_id")
	accountID, err := canonicalAccountID(id)
	if err != nil {
		return err
	}
	idx64, err := row.Uint64("asset_index", 16)
	if err != nil {
		return err
	}
	if int(idx64) >= len(assets) {
		return fmt.Errorf("asset_index %d exceeds assets", idx64)
	}
	idx := uint16(idx64)
	equity, _ := row.Uint64("equity", 64)
	debt, _ := row.Uint64("debt", 64)
	collateral, _ := row.Uint64("collateral", 64)
	if collateral > equity {
		return fmt.Errorf("collateral exceeds equity")
	}
	key := string(accountID)
	account := groups[key]
	if account == nil {
		account = &t2spec.AccountInfo{AccountID: accountID, TotalEquity: new(big.Int), TotalDebt: new(big.Int), TotalCollateral: new(big.Int)}
		groups[key] = account
		*order = append(*order, key)
	}
	if equity != 0 || debt != 0 {
		account.Assets = append(account.Assets, t2spec.AccountAsset{Index: idx, Equity: equity, Debt: debt, Collateral: collateral})
	}
	price := new(big.Int).SetUint64(assets[idx].BasePrice)
	addScaled(account.TotalEquity, equity, price)
	addScaled(account.TotalDebt, debt, price)
	c := new(big.Int).SetUint64(collateral)
	c.Mul(c, price)
	c.Mul(c, new(big.Int).SetUint64(uint64(assets[idx].Haircut)))
	c.Div(c, basisPointsDivisor)
	account.TotalCollateral.Add(account.TotalCollateral, c)
	return nil
}

func schemaFile(name string) snapshotschema.File {
	for _, file := range StandardSchema.Files {
		if file.Name == name {
			return file
		}
	}
	panic("t2 standard schema missing file " + name)
}

func canonicalAccountID(raw string) ([]byte, error) {
	accountID, err := hex.DecodeString(raw)
	if err != nil || len(accountID) != 32 {
		return nil, fmt.Errorf("invalid account_id")
	}
	return new(fr.Element).SetBytes(accountID).Marshal(), nil
}

func addScaled(dst *big.Int, amount uint64, price *big.Int) {
	tmp := new(big.Int).SetUint64(amount)
	tmp.Mul(tmp, price)
	dst.Add(dst, tmp)
}
