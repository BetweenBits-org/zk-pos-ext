package t1_simple_margin

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/BetweenBits-org/zk-pos-ext/core/io/vfs"
	"github.com/BetweenBits-org/zk-pos-ext/core/io/vfs/osvfs"
	snapshotcsv "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/csv"
	snapshotmapping "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/mapping"
	snapshotschema "github.com/BetweenBits-org/zk-pos-ext/core/snapshot/schema"
	t1host "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t1_simple_margin/host"
	t1spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t1_simple_margin/spec"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// ConnectorID is the T1 standard canonical CSV snapshot connector
// registered with the model host registry.
const ConnectorID = "t1_standard_csv.v1"

func init() {
	t1host.RegisterSnapshot(ConnectorID, func(src vfs.Opener, snapshotID string, assetCapacity int, _ corespec.PriceScaleProvider) t1spec.SnapshotSource {
		return NewSnapshotCSV(Config{
			Source:        src,
			SnapshotID:    snapshotID,
			AssetCapacity: assetCapacity,
		})
	})
}

// Config constructs a T1 standard CSV SnapshotSource. The source is
// expected to open canonical accounts.csv and cex_assets.csv inputs
// matching StandardSchema. Mapping.Format may override CSV dialect
// options; mapping column rules are intentionally applied before this
// parser layer.
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
	assets []t1spec.CexAssetInfo
	err    error

	invalidCount atomic.Uint64
}

// NewSnapshotCSV returns a canonical standard CSV SnapshotSource for
// t1_simple_margin. Heavy I/O is deferred to AccountStream/CexAssets.
func NewSnapshotCSV(cfg Config) t1spec.SnapshotSource {
	return &snapshot{cfg: cfg}
}

// AccountStream groups canonical account-asset rows into one
// AccountInfo per account and streams them in deterministic
// account_index order.
func (s *snapshot) AccountStream(ctx context.Context) (<-chan t1spec.AccountInfo, error) {
	assets, err := s.CexAssets(ctx)
	if err != nil {
		return nil, fmt.Errorf("t1 standard snapshot: preload CexAssets: %w", err)
	}
	accounts, err := s.loadAccounts(ctx, assets)
	if err != nil {
		return nil, err
	}
	out := make(chan t1spec.AccountInfo, 1024)
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
func (s *snapshot) CexAssets(ctx context.Context) ([]t1spec.CexAssetInfo, error) {
	s.once.Do(func() {
		s.assets, s.err = s.loadCexAssets(ctx)
	})
	if s.err != nil {
		return nil, s.err
	}
	out := make([]t1spec.CexAssetInfo, len(s.assets))
	copy(out, s.assets)
	return out, nil
}

// SnapshotID returns the configured snapshot identifier.
func (s *snapshot) SnapshotID() string { return s.cfg.SnapshotID }

// InvalidCount returns the number of canonical account rows rejected
// while building the stream.
func (s *snapshot) InvalidCount() uint64 { return s.invalidCount.Load() }

func (s *snapshot) csvOptions() (snapshotcsv.Options, error) {
	return snapshotmapping.BuildCSVOptions(s.cfg.Mapping.Format)
}

func (s *snapshot) loadCexAssets(ctx context.Context) ([]t1spec.CexAssetInfo, error) {
	if s.cfg.AssetCapacity <= 0 {
		return nil, fmt.Errorf("asset capacity must be > 0, got %d", s.cfg.AssetCapacity)
	}
	opts, err := s.csvOptions()
	if err != nil {
		return nil, err
	}
	f, err := s.cfg.source().Open(ctx, "cex_assets.csv")
	if err != nil {
		return nil, fmt.Errorf("open cex_assets.csv: %w", err)
	}
	defer f.Close()
	reader, err := snapshotcsv.NewReader(f, schemaFile("cex_assets.csv"), opts)
	if err != nil {
		return nil, err
	}
	out := make([]t1spec.CexAssetInfo, s.cfg.AssetCapacity)
	for i := range out {
		out[i] = t1spec.CexAssetInfo{Symbol: "reserved", Index: uint32(i)}
	}
	seen := map[uint16]struct{}{}
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
		idx := uint16(idx64)
		if int(idx) >= s.cfg.AssetCapacity {
			return nil, fmt.Errorf("cex_assets.csv record %d asset_index %d exceeds capacity %d", row.RecordNumber, idx, s.cfg.AssetCapacity)
		}
		if _, dup := seen[idx]; dup {
			return nil, fmt.Errorf("cex_assets.csv duplicate asset_index %d", idx)
		}
		seen[idx] = struct{}{}
		totalEquity, err := row.Uint64("total_equity", 64)
		if err != nil {
			return nil, err
		}
		totalDebt, err := row.Uint64("total_debt", 64)
		if err != nil {
			return nil, err
		}
		basePrice, err := row.Uint64("base_price", 64)
		if err != nil {
			return nil, err
		}
		symbol, err := row.Required("symbol")
		if err != nil {
			return nil, err
		}
		out[idx] = t1spec.CexAssetInfo{
			TotalEquity: totalEquity,
			TotalDebt:   totalDebt,
			BasePrice:   basePrice,
			Symbol:      symbol,
			Index:       uint32(idx),
		}
	}
}

type accountGroup struct {
	account t1spec.AccountInfo
	seenIdx bool
	order   uint32
}

func (s *snapshot) loadAccounts(ctx context.Context, assets []t1spec.CexAssetInfo) ([]t1spec.AccountInfo, error) {
	opts, err := s.csvOptions()
	if err != nil {
		return nil, err
	}
	f, err := s.cfg.source().Open(ctx, "accounts.csv")
	if err != nil {
		return nil, fmt.Errorf("open accounts.csv: %w", err)
	}
	defer f.Close()
	reader, err := snapshotcsv.NewReader(f, schemaFile("accounts.csv"), opts)
	if err != nil {
		return nil, err
	}
	groups := map[string]*accountGroup{}
	order := make([]string, 0)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return finalizeAccounts(order, groups), nil
		}
		if err != nil {
			if errors.Is(err, snapshotcsv.ErrInvalidRow) {
				log.Printf("t1 standard snapshot: skip account row: %v", err)
				s.invalidCount.Add(1)
				continue
			}
			return nil, err
		}
		if err := addT1AccountRow(row, assets, groups, &order); err != nil {
			log.Printf("t1 standard snapshot: skip account row %d: %v", row.RecordNumber, err)
			s.invalidCount.Add(1)
		}
	}
}

func addT1AccountRow(row snapshotcsv.Row, assets []t1spec.CexAssetInfo, groups map[string]*accountGroup, order *[]string) error {
	idHex, err := row.Required("account_id")
	if err != nil {
		return err
	}
	accountID, err := canonicalAccountID(idHex)
	if err != nil {
		return err
	}
	assetIndex64, err := row.Uint64("asset_index", 16)
	if err != nil {
		return err
	}
	assetIndex := uint16(assetIndex64)
	if int(assetIndex) >= len(assets) {
		return fmt.Errorf("asset_index %d exceeds cex asset length %d", assetIndex, len(assets))
	}
	equity, err := row.Uint64("equity", 64)
	if err != nil {
		return err
	}
	debt, err := row.Uint64("debt", 64)
	if err != nil {
		return err
	}
	key := string(accountID)
	g, ok := groups[key]
	if !ok {
		g = &accountGroup{
			account: t1spec.AccountInfo{
				AccountID:   accountID,
				TotalEquity: new(big.Int),
				TotalDebt:   new(big.Int),
			},
			order: uint32(len(*order)),
		}
		g.account.AccountIndex = g.order
		groups[key] = g
		*order = append(*order, key)
	}
	if rawIdx, ok := row.Value("account_index"); ok {
		idx, err := row.Uint64("account_index", 32)
		if err != nil {
			return err
		}
		if g.seenIdx && g.account.AccountIndex != uint32(idx) {
			return fmt.Errorf("account_index changed for account %s", idHex)
		}
		g.account.AccountIndex = uint32(idx)
		g.seenIdx = true
		_ = rawIdx
	}
	if equity != 0 || debt != 0 {
		g.account.Assets = append(g.account.Assets, t1spec.AccountAsset{
			Index:  assetIndex,
			Equity: equity,
			Debt:   debt,
		})
	}
	price := new(big.Int).SetUint64(assets[assetIndex].BasePrice)
	addScaled(g.account.TotalEquity, equity, price)
	addScaled(g.account.TotalDebt, debt, price)
	return nil
}

func finalizeAccounts(order []string, groups map[string]*accountGroup) []t1spec.AccountInfo {
	out := make([]t1spec.AccountInfo, 0, len(order))
	for _, key := range order {
		g := groups[key]
		sort.Slice(g.account.Assets, func(i, j int) bool {
			return g.account.Assets[i].Index < g.account.Assets[j].Index
		})
		out = append(out, g.account)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].AccountIndex < out[j].AccountIndex
	})
	return out
}

func schemaFile(name string) snapshotschema.File {
	for _, file := range StandardSchema.Files {
		if file.Name == name {
			return file
		}
	}
	panic("t1 standard schema missing file " + name)
}

func canonicalAccountID(raw string) ([]byte, error) {
	accountID, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("hex decode account_id: %w", err)
	}
	if len(accountID) != 32 {
		return nil, fmt.Errorf("account_id has %d bytes, want 32", len(accountID))
	}
	return new(fr.Element).SetBytes(accountID).Marshal(), nil
}

func addScaled(dst *big.Int, amount uint64, price *big.Int) {
	tmp := new(big.Int).SetUint64(amount)
	tmp.Mul(tmp, price)
	dst.Add(dst, tmp)
}
