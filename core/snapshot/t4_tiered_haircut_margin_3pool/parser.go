package t4_tiered_haircut_margin_3pool

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
	t4host "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t4_tiered_haircut_margin_3pool/host"
	t4spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// ConnectorID is the T4 standard canonical CSV snapshot connector
// registered with the model host registry.
const ConnectorID = "t4_standard_csv.v1"

var (
	maxTierBoundary = new(big.Int).Lsh(big.NewInt(1), 118)
	percentDivisor  = big.NewInt(100)
)

func init() {
	t4host.RegisterSnapshot(ConnectorID, func(src vfs.Opener, snapshotID string, assetCapacity int, _ corespec.PriceScaleProvider) t4spec.SnapshotSource {
		return NewSnapshotCSV(Config{
			Source:        src,
			SnapshotID:    snapshotID,
			AssetCapacity: assetCapacity,
		})
	})
}

// Config constructs a T4 standard CSV SnapshotSource from canonical
// accounts.csv, cex_assets.csv, and tier_ratios.csv inputs.
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
	assets []t4spec.CexAssetInfo
	err    error

	invalidCount atomic.Uint64
}

// NewSnapshotCSV returns a canonical standard CSV SnapshotSource for
// t4_tiered_haircut_margin_3pool.
func NewSnapshotCSV(cfg Config) t4spec.SnapshotSource {
	return &snapshot{cfg: cfg}
}

// AccountStream groups canonical account-asset rows into one
// AccountInfo per account.
func (s *snapshot) AccountStream(ctx context.Context) (<-chan t4spec.AccountInfo, error) {
	assets, err := s.CexAssets(ctx)
	if err != nil {
		return nil, fmt.Errorf("t4 standard snapshot: preload CexAssets: %w", err)
	}
	accounts, err := s.loadAccounts(ctx, assets)
	if err != nil {
		return nil, err
	}
	out := make(chan t4spec.AccountInfo, 1024)
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
func (s *snapshot) CexAssets(ctx context.Context) ([]t4spec.CexAssetInfo, error) {
	s.once.Do(func() {
		s.assets, s.err = s.loadCexAssets(ctx)
	})
	if s.err != nil {
		return nil, s.err
	}
	out := make([]t4spec.CexAssetInfo, len(s.assets))
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

func (s *snapshot) loadCexAssets(ctx context.Context) ([]t4spec.CexAssetInfo, error) {
	if s.cfg.AssetCapacity <= 0 {
		return nil, fmt.Errorf("asset capacity must be > 0, got %d", s.cfg.AssetCapacity)
	}
	opts, err := s.csvOptions()
	if err != nil {
		return nil, err
	}
	ratios, err := s.loadTierRatios(ctx, opts)
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
	out := make([]t4spec.CexAssetInfo, s.cfg.AssetCapacity)
	for i := range out {
		out[i] = reservedCexAsset(uint32(i))
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
		symbol, err := row.Required("symbol")
		if err != nil {
			return nil, err
		}
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
		loan, err := row.Uint64("loan_collateral", 64)
		if err != nil {
			return nil, err
		}
		margin, err := row.Uint64("margin_collateral", 64)
		if err != nil {
			return nil, err
		}
		pm, err := row.Uint64("portfolio_margin_collateral", 64)
		if err != nil {
			return nil, err
		}
		out[idx] = t4spec.CexAssetInfo{
			TotalEquity:               totalEquity,
			TotalDebt:                 totalDebt,
			BasePrice:                 basePrice,
			Symbol:                    symbol,
			Index:                     uint32(idx),
			LoanCollateral:            loan,
			MarginCollateral:          margin,
			PortfolioMarginCollateral: pm,
			LoanRatios:                padRatios(ratios[idx]["loan"]),
			MarginRatios:              padRatios(ratios[idx]["margin"]),
			PortfolioMarginRatios:     padRatios(ratios[idx]["portfolio_margin"]),
		}
	}
}

func (s *snapshot) loadTierRatios(ctx context.Context, opts snapshotcsv.Options) (map[uint16]map[string][]t4spec.TierRatio, error) {
	f, err := s.cfg.source().Open(ctx, "tier_ratios.csv")
	if err != nil {
		return nil, fmt.Errorf("open tier_ratios.csv: %w", err)
	}
	defer f.Close()
	reader, err := snapshotcsv.NewReader(f, schemaFile("tier_ratios.csv"), opts)
	if err != nil {
		return nil, err
	}
	out := map[uint16]map[string][]t4spec.TierRatio{}
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
		pool, err := row.Required("collateral_pool")
		if err != nil {
			return nil, err
		}
		if pool != "loan" && pool != "margin" && pool != "portfolio_margin" {
			return nil, fmt.Errorf("tier_ratios.csv record %d invalid collateral_pool %q", row.RecordNumber, pool)
		}
		tierIndex, err := row.Uint64("tier_index", 16)
		if err != nil {
			return nil, err
		}
		boundary, err := row.BigInt("boundary_value")
		if err != nil {
			return nil, err
		}
		ratio, err := row.Uint64("ratio", 8)
		if err != nil {
			return nil, err
		}
		precomputed, err := row.BigInt("precomputed_value")
		if err != nil {
			return nil, err
		}
		idx := uint16(idx64)
		if out[idx] == nil {
			out[idx] = map[string][]t4spec.TierRatio{}
		}
		list := out[idx][pool]
		if int(tierIndex) != len(list) {
			return nil, fmt.Errorf("tier_ratios.csv record %d tier_index %d is not dense for asset %d pool %s", row.RecordNumber, tierIndex, idx, pool)
		}
		if len(list) > 0 && boundary.Cmp(list[len(list)-1].BoundaryValue) <= 0 {
			return nil, fmt.Errorf("tier_ratios.csv record %d boundary is not increasing", row.RecordNumber)
		}
		list = append(list, t4spec.TierRatio{
			BoundaryValue:    boundary,
			Ratio:            uint8(ratio),
			PrecomputedValue: precomputed,
		})
		out[idx][pool] = list
	}
}

type accountGroup struct {
	account t4spec.AccountInfo
	seenIdx bool
	order   uint32
}

func (s *snapshot) loadAccounts(ctx context.Context, assets []t4spec.CexAssetInfo) ([]t4spec.AccountInfo, error) {
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
				log.Printf("t4 standard snapshot: skip account row: %v", err)
				s.invalidCount.Add(1)
				continue
			}
			return nil, err
		}
		if err := addAccountRow(row, assets, groups, &order); err != nil {
			log.Printf("t4 standard snapshot: skip account row %d: %v", row.RecordNumber, err)
			s.invalidCount.Add(1)
		}
	}
}

func addAccountRow(row snapshotcsv.Row, assets []t4spec.CexAssetInfo, groups map[string]*accountGroup, order *[]string) error {
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
	loan, err := row.Uint64("loan_collateral", 64)
	if err != nil {
		return err
	}
	margin, err := row.Uint64("margin_collateral", 64)
	if err != nil {
		return err
	}
	pm, err := row.Uint64("portfolio_margin_collateral", 64)
	if err != nil {
		return err
	}
	if sum, ok := safeAddU64(loan, margin, pm); !ok || sum > equity {
		return fmt.Errorf("collateral exceeds equity")
	}
	key := string(accountID)
	g, ok := groups[key]
	if !ok {
		g = &accountGroup{
			account: t4spec.AccountInfo{
				AccountID:       accountID,
				TotalEquity:     new(big.Int),
				TotalDebt:       new(big.Int),
				TotalCollateral: new(big.Int),
			},
			order: uint32(len(*order)),
		}
		g.account.AccountIndex = g.order
		groups[key] = g
		*order = append(*order, key)
	}
	if _, ok := row.Value("account_index"); ok {
		idx, err := row.Uint64("account_index", 32)
		if err != nil {
			return err
		}
		if g.seenIdx && g.account.AccountIndex != uint32(idx) {
			return fmt.Errorf("account_index changed for account %s", idHex)
		}
		g.account.AccountIndex = uint32(idx)
		g.seenIdx = true
	}
	if equity != 0 || debt != 0 {
		g.account.Assets = append(g.account.Assets, t4spec.AccountAsset{
			Index:           assetIndex,
			Equity:          equity,
			Debt:            debt,
			Loan:            loan,
			Margin:          margin,
			PortfolioMargin: pm,
		})
	}
	price := new(big.Int).SetUint64(assets[assetIndex].BasePrice)
	addScaled(g.account.TotalEquity, equity, price)
	addScaled(g.account.TotalDebt, debt, price)
	g.account.TotalCollateral.Add(g.account.TotalCollateral, collateralValue(loan, margin, pm, price, &assets[assetIndex]))
	return nil
}

func finalizeAccounts(order []string, groups map[string]*accountGroup) []t4spec.AccountInfo {
	out := make([]t4spec.AccountInfo, 0, len(order))
	for _, key := range order {
		g := groups[key]
		sort.Slice(g.account.Assets, func(i, j int) bool {
			return g.account.Assets[i].Index < g.account.Assets[j].Index
		})
		if g.account.TotalCollateral.Cmp(g.account.TotalDebt) >= 0 {
			out = append(out, g.account)
		}
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
	panic("t4 standard schema missing file " + name)
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

func collateralValue(loan, margin, pm uint64, price *big.Int, info *t4spec.CexAssetInfo) *big.Int {
	haircut := func(amount uint64, tiers []t4spec.TierRatio) *big.Int {
		v := new(big.Int).SetUint64(amount)
		v.Mul(v, price)
		return haircutValue(v, tiers)
	}
	out := haircut(loan, info.LoanRatios)
	out.Add(out, haircut(margin, info.MarginRatios))
	out.Add(out, haircut(pm, info.PortfolioMarginRatios))
	return out
}

func haircutValue(value *big.Int, tiers []t4spec.TierRatio) *big.Int {
	prevBoundary := new(big.Int)
	prevPrecomp := new(big.Int)
	for _, tier := range tiers {
		if value.Cmp(tier.BoundaryValue) <= 0 {
			delta := new(big.Int).Sub(value, prevBoundary)
			delta.Mul(delta, new(big.Int).SetUint64(uint64(tier.Ratio)))
			delta.Div(delta, percentDivisor)
			return delta.Add(delta, prevPrecomp)
		}
		prevBoundary.Set(tier.BoundaryValue)
		prevPrecomp.Set(tier.PrecomputedValue)
	}
	return prevPrecomp
}

func safeAddU64(a, b, c uint64) (uint64, bool) {
	s1 := a + b
	if s1 < a {
		return 0, false
	}
	s2 := s1 + c
	if s2 < s1 {
		return 0, false
	}
	return s2, true
}

func padRatios(in []t4spec.TierRatio) []t4spec.TierRatio {
	out := make([]t4spec.TierRatio, corespec.TierCount)
	var lastPrecomputed *big.Int
	if len(in) > 0 {
		lastPrecomputed = in[len(in)-1].PrecomputedValue
	}
	for i := range out {
		if i < len(in) {
			out[i] = in[i]
			continue
		}
		precomputed := new(big.Int)
		if lastPrecomputed != nil {
			precomputed.Set(lastPrecomputed)
		}
		out[i] = t4spec.TierRatio{
			BoundaryValue:    new(big.Int).Set(maxTierBoundary),
			Ratio:            0,
			PrecomputedValue: precomputed,
		}
	}
	return out
}

func reservedCexAsset(index uint32) t4spec.CexAssetInfo {
	return t4spec.CexAssetInfo{
		Symbol:                "reserved",
		Index:                 index,
		LoanRatios:            padRatios(nil),
		MarginRatios:          padRatios(nil),
		PortfolioMarginRatios: padRatios(nil),
	}
}
