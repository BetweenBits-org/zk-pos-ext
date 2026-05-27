package sea_reference

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	t1standard "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/t1_simple_margin"
	t1host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/host"
	modelspec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/shopspring/decimal"
)

// SnapshotConnectorID is the G17 registry key under which this
// profile's CSV ETL is registered with the T1 snapshot connector
// registry (R8-B/2). Service startup that loads sea_reference.toml
// will see this exact string at [snapshot].source_type.
const SnapshotConnectorID = "sea_csv.v1"

func init() {
	t1host.RegisterSnapshot(SnapshotConnectorID, snapshotFactory)
}

// snapshotFactory is the registry-facing constructor — same shape
// as profile/binance.snapshotFactory but typed against the t1
// SnapshotSource.
func snapshotFactory(
	userDataDir, snapshotID string,
	assetCapacity int,
	pricing corespec.PriceScaleProvider,
) modelspec.SnapshotSource {
	return NewSnapshotCSV(SnapshotConfig{
		UserDataDir:   userDataDir,
		SnapshotID:    snapshotID,
		AssetCapacity: assetCapacity,
		Pricing:       pricing,
	})
}

// cexAssetsCSVName is the filename the sea_reference pipeline uses to
// publish per-asset global state alongside the user shard CSVs.
const cexAssetsCSVName = "cex_assets_info.csv"

// reservedSymbol is the sentinel symbol stamped onto unused
// CexAssetInfo slots when the deployment uses fewer than its
// per-deployment asset capacity. Same literal value as binance —
// universal padding convention.
const reservedSymbol = "reserved"

// SnapshotConfig is the static input needed to construct a CSV-backed
// SnapshotSource for the sea_reference profile.
//
// CSV layout (spot-simple — one Equity column per asset, no
// debt/collateral):
//
//	user_shard.csv  header: rn, id, <asset1>, <asset2>, ..., sum
//	                 data:  <int>, <64-hex>, <float>, <float>, ..., <float>
//
//	cex_assets_info.csv  header: symbol, usdt_price, total_equity
//	                      data:  <symbol>, <float>, <float>
type SnapshotConfig struct {
	UserDataDir   string
	SnapshotID    string
	AssetCapacity int
	// Pricing scales raw float prices / balances into the uint64
	// values embedded in the witness. R8-E supplied via declarative;
	// nil is rejected by NewSnapshotCSV.
	Pricing corespec.PriceScaleProvider
}

type csvSnapshot struct {
	cfg SnapshotConfig

	once     sync.Once
	standard modelspec.SnapshotSource
	err      error

	invalidCount atomic.Uint64
}

// NewSnapshotCSV constructs a SnapshotSource backed by the
// sea_reference CSV layout. CexAssets reads cex_assets_info.csv and
// the first user-shard CSV's header on first call; the result is
// cached for the lifetime of the returned source. AccountStream walks
// every user shard sequentially, yielding one AccountInfo per CSV
// data row.
func NewSnapshotCSV(cfg SnapshotConfig) modelspec.SnapshotSource {
	if cfg.Pricing == nil {
		panic("sea_reference.NewSnapshotCSV: cfg.Pricing must be non-nil")
	}
	return &csvSnapshot{cfg: cfg}
}

const accountStreamBuffer = 1024

var errInvalidRow = errors.New("invalid account row")

func invalidf(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, errInvalidRow)...)
}

// AccountStream yields one AccountInfo per CSV data row across all
// user-shard files in UserDataDir (sorted lexically, sequential).
// Per-row failures (malformed hex id, balance overflow) are
// classified invalid: logged, counted via InvalidCount, and skipped
// without breaking the stream. Stream-fatal errors (header malformed,
// CSV IO) close the channel early.
func (c *csvSnapshot) AccountStream(ctx context.Context) (<-chan modelspec.AccountInfo, error) {
	src, err := c.standardSource()
	if err != nil {
		return nil, err
	}
	return src.AccountStream(ctx)
}

func (c *csvSnapshot) standardSource() (modelspec.SnapshotSource, error) {
	c.once.Do(func() {
		dir, err := materializeStandardSnapshot(c.cfg)
		if err != nil {
			c.err = err
			return
		}
		c.standard = t1standard.NewSnapshotCSV(t1standard.Config{
			Dir:           dir,
			SnapshotID:    c.cfg.SnapshotID,
			AssetCapacity: c.cfg.AssetCapacity,
		})
	})
	return c.standard, c.err
}

func (c *csvSnapshot) streamAccounts(
	ctx context.Context,
	shards []string,
	assets []modelspec.CexAssetInfo,
	prov interface{ BalanceMultiplier(string) int64 },
	out chan<- modelspec.AccountInfo,
) {
	defer close(out)
	var validIndex uint32
	for _, path := range shards {
		if err := c.streamShard(ctx, path, assets, prov, &validIndex, out); err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				log.Printf("sea_reference snapshot: stream %s: %v", path, err)
			}
			return
		}
	}
}

// streamShard parses one user-shard CSV. Header must be `rn, id,
// <asset1>, <asset2>, ..., <assetN>, sum` — exactly len(assets) of
// the inner asset columns must match the cex_assets_info.csv symbol
// list in order.
func (c *csvSnapshot) streamShard(
	ctx context.Context,
	path string,
	assets []modelspec.CexAssetInfo,
	prov interface{ BalanceMultiplier(string) int64 },
	validIndex *uint32,
	out chan<- modelspec.AccountInfo,
) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return fmt.Errorf("read header: %w", err)
	}
	// Minimum: rn + id + at least one asset + sum.
	if len(header) < 4 {
		return fmt.Errorf("malformed header column count %d", len(header))
	}
	assetCount := len(header) - 3 // strip rn, id, sum
	if assetCount > len(assets) {
		return fmt.Errorf(
			"shard has %d assets but cached snapshot has only %d slots",
			assetCount, len(assets),
		)
	}
	// Sanity: header asset names should match the cached order.
	for j := range assetCount {
		want := strings.ToLower(strings.TrimSpace(header[j+2]))
		if assets[j].Symbol != want {
			return fmt.Errorf(
				"header asset[%d] = %q, cached snapshot asset[%d] = %q",
				j, want, j, assets[j].Symbol,
			)
		}
	}

	var rawIndex uint32
	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read row at raw index %d: %w", rawIndex, err)
		}
		account, err := parseAccountRow(row, assets, assetCount, *validIndex, prov)
		if err != nil {
			if errors.Is(err, errInvalidRow) {
				log.Printf("sea_reference snapshot: skip row %d in %s: %v", rawIndex, path, err)
				c.invalidCount.Add(1)
				rawIndex++
				continue
			}
			return fmt.Errorf("parse row at raw index %d: %w", rawIndex, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- account:
		}
		*validIndex++
		rawIndex++
	}
}

// parseAccountRow converts a single CSV data row into AccountInfo.
// Layout: row[0]=rn, row[1]=id (64 hex chars), row[2..2+assetCount]
// are per-asset Equity floats, row[2+assetCount] is the sum column
// (currently ignored — present for human inspection only).
//
// AccountID normalization is the bn254 fr.Element round-trip
// (SetBytes→Marshal) matching profile/binance's G13 convention so
// AccountInfo.AccountID is byte-equal to the in-circuit field input
// regardless of whether the raw 32-byte ID exceeds fr.Modulus.
func parseAccountRow(
	row []string,
	assets []modelspec.CexAssetInfo,
	assetCount int,
	index uint32,
	prov interface{ BalanceMultiplier(string) int64 },
) (modelspec.AccountInfo, error) {
	minCols := 2 + assetCount
	if len(row) < minCols {
		return modelspec.AccountInfo{}, fmt.Errorf(
			"row has %d columns, want at least %d", len(row), minCols,
		)
	}
	accountID, err := hex.DecodeString(row[1])
	if err != nil {
		return modelspec.AccountInfo{}, invalidf("hex decode account id %q: %v", row[1], err)
	}
	if len(accountID) != 32 {
		return modelspec.AccountInfo{}, invalidf(
			"account id has %d bytes, want 32", len(accountID),
		)
	}
	// fr.Element reduce — G13 universal convention.
	accountID = new(fr.Element).SetBytes(accountID).Marshal()

	// Spot deployment under T1 supplies TotalDebt = 0 — the per-user
	// solvency assertion TotalEquity >= TotalDebt becomes trivial.
	account := modelspec.AccountInfo{
		AccountIndex: index,
		AccountID:    accountID,
		TotalEquity:  new(big.Int),
		TotalDebt:    new(big.Int),
	}
	for j := range assetCount {
		symbol := assets[j].Symbol
		mult := prov.BalanceMultiplier(symbol)
		equity, err := convertFloatStrToUint64(row[j+2], mult)
		if err != nil {
			return modelspec.AccountInfo{}, invalidf("asset %q equity: %v", symbol, err)
		}
		if equity == 0 {
			continue
		}
		account.Assets = append(account.Assets, modelspec.AccountAsset{
			Index:  uint16(j),
			Equity: equity,
		})
		// TotalEquity in USD value units (Equity * BasePrice). Reported
		// for user-facing summary, not enforced by circuit.
		price := new(big.Int).SetUint64(assets[j].BasePrice)
		tmp := new(big.Int).SetUint64(equity)
		tmp.Mul(tmp, price)
		account.TotalEquity.Add(account.TotalEquity, tmp)
	}
	return account, nil
}

// CexAssets returns the per-asset CEX totals slice indexed by
// AssetCatalog index. Symbol order is derived from the first
// user-shard CSV's header. Returned slice has length cfg.AssetCapacity
// with unused slots filled by reservedSymbol entries.
func (c *csvSnapshot) CexAssets(ctx context.Context) ([]modelspec.CexAssetInfo, error) {
	src, err := c.standardSource()
	if err != nil {
		return nil, err
	}
	return src.CexAssets(ctx)
}

func (c *csvSnapshot) SnapshotID() string { return c.cfg.SnapshotID }

func (c *csvSnapshot) InvalidCount() uint64 {
	src, err := c.standardSource()
	if err != nil {
		return 0
	}
	return src.InvalidCount()
}

func materializeStandardSnapshot(cfg SnapshotConfig) (string, error) {
	assets, err := loadCSVSnapshot(cfg.UserDataDir, cfg.AssetCapacity, cfg.Pricing)
	if err != nil {
		return "", err
	}

	dir, err := os.MkdirTemp("", "zkpor-sea-reference-standard-*")
	if err != nil {
		return "", fmt.Errorf("sea_reference snapshot: create standard temp dir: %w", err)
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(dir)
		}
	}()
	if err := writeStandardCexAssets(dir, assets); err != nil {
		return "", err
	}
	if err := writeStandardAccounts(cfg, dir, assets); err != nil {
		return "", err
	}
	ok = true
	return dir, nil
}

func writeStandardCexAssets(dir string, assets []modelspec.CexAssetInfo) error {
	f, err := os.Create(filepath.Join(dir, "cex_assets.csv"))
	if err != nil {
		return fmt.Errorf("sea_reference snapshot: create standard cex_assets.csv: %w", err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"asset_index", "symbol", "total_equity", "total_debt", "base_price"}); err != nil {
		return err
	}
	for _, asset := range assets {
		if err := w.Write([]string{
			fmt.Sprint(asset.Index),
			asset.Symbol,
			fmt.Sprint(asset.TotalEquity),
			fmt.Sprint(asset.TotalDebt),
			fmt.Sprint(asset.BasePrice),
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func writeStandardAccounts(cfg SnapshotConfig, dir string, assets []modelspec.CexAssetInfo) error {
	f, err := os.Create(filepath.Join(dir, "accounts.csv"))
	if err != nil {
		return fmt.Errorf("sea_reference snapshot: create standard accounts.csv: %w", err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"account_index", "account_id", "asset_index", "equity", "debt"}); err != nil {
		return err
	}

	shards, err := listUserShards(cfg.UserDataDir)
	if err != nil {
		return err
	}
	var validIndex uint32
	for _, path := range shards {
		if err := writeStandardAccountShard(w, path, assets, &validIndex, cfg.Pricing); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func writeStandardAccountShard(
	w *csv.Writer,
	path string,
	assets []modelspec.CexAssetInfo,
	validIndex *uint32,
	prov corespec.PriceScaleProvider,
) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return fmt.Errorf("read header: %w", err)
	}
	if len(header) < 4 {
		return fmt.Errorf("malformed header column count %d", len(header))
	}
	assetCount := len(header) - 3
	if assetCount > len(assets) {
		return fmt.Errorf(
			"shard has %d assets but cached snapshot has only %d slots",
			assetCount, len(assets),
		)
	}
	for j := range assetCount {
		want := strings.ToLower(strings.TrimSpace(header[j+2]))
		if assets[j].Symbol != want {
			return fmt.Errorf(
				"header asset[%d] = %q, cached snapshot asset[%d] = %q",
				j, want, j, assets[j].Symbol,
			)
		}
	}

	var rawIndex uint32
	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			if rawIndex > 0 {
				return nil
			}
			return fmt.Errorf("read row at raw index %d: %w", rawIndex, err)
		}
		account, err := parseAccountRow(row, assets, assetCount, *validIndex, prov)
		if err != nil {
			if errors.Is(err, errInvalidRow) {
				if err := writeInvalidStandardAccountRow(w, row, *validIndex); err != nil {
					return err
				}
				rawIndex++
				continue
			}
			if rawIndex > 0 {
				return nil
			}
			return fmt.Errorf("parse row at raw index %d: %w", rawIndex, err)
		}
		if err := writeValidStandardAccountRows(w, row[1], account); err != nil {
			return err
		}
		*validIndex++
		rawIndex++
	}
}

func writeInvalidStandardAccountRow(w *csv.Writer, row []string, index uint32) error {
	accountID := "invalid"
	if len(row) > 1 {
		accountID = row[1]
	}
	return w.Write([]string{
		fmt.Sprint(index),
		accountID,
		"0",
		"invalid",
		"0",
	})
}

func writeValidStandardAccountRows(w *csv.Writer, rawAccountID string, account modelspec.AccountInfo) error {
	if len(account.Assets) == 0 {
		return w.Write([]string{
			fmt.Sprint(account.AccountIndex),
			rawAccountID,
			"0", "0", "0",
		})
	}
	for _, asset := range account.Assets {
		if err := w.Write([]string{
			fmt.Sprint(account.AccountIndex),
			rawAccountID,
			fmt.Sprint(asset.Index),
			fmt.Sprint(asset.Equity),
			fmt.Sprint(asset.Debt),
		}); err != nil {
			return err
		}
	}
	return nil
}

func loadCSVSnapshot(dir string, capacity int, pricing corespec.PriceScaleProvider) ([]modelspec.CexAssetInfo, error) {
	if dir == "" {
		return nil, errors.New("sea_reference snapshot: UserDataDir is empty")
	}
	if capacity <= 0 {
		return nil, errors.New("sea_reference snapshot: AssetCapacity must be > 0")
	}
	order, err := readUserAssetOrder(dir)
	if err != nil {
		return nil, err
	}
	if len(order) > capacity {
		return nil, fmt.Errorf(
			"sea_reference snapshot: user CSV has %d assets, exceeds deployment capacity %d",
			len(order), capacity,
		)
	}
	bySymbol, err := readCexAssetsCSV(filepath.Join(dir, cexAssetsCSVName), pricing)
	if err != nil {
		return nil, err
	}
	if len(order) != len(bySymbol) {
		return nil, fmt.Errorf(
			"sea_reference snapshot: user CSV header has %d assets but %s has %d",
			len(order), cexAssetsCSVName, len(bySymbol),
		)
	}

	assets := make([]modelspec.CexAssetInfo, capacity)
	for i, sym := range order {
		info, ok := bySymbol[sym]
		if !ok {
			return nil, fmt.Errorf(
				"sea_reference snapshot: user CSV references asset %q absent from %s",
				sym, cexAssetsCSVName,
			)
		}
		info.Index = uint32(i)
		assets[i] = info
	}
	for i := len(order); i < capacity; i++ {
		assets[i] = modelspec.CexAssetInfo{Symbol: reservedSymbol, Index: uint32(i)}
	}
	return assets, nil
}

func listUserShards(dir string) ([]string, error) {
	if dir == "" {
		return nil, errors.New("sea_reference snapshot: UserDataDir is empty")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("sea_reference snapshot: read user data dir %q: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".csv") || name == cexAssetsCSVName {
			continue
		}
		paths = append(paths, filepath.Join(dir, name))
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf(
			"sea_reference snapshot: no user CSV files found in %q (expected at least one .csv besides %s)",
			dir, cexAssetsCSVName,
		)
	}
	sort.Strings(paths)
	return paths, nil
}

// readUserAssetOrder extracts the asset symbol order from the first
// user-shard's header. Layout: `rn, id, <asset1>, <asset2>, ...,
// <assetN>, sum`. The middle N columns are the lower-cased symbols.
func readUserAssetOrder(dir string) ([]string, error) {
	shards, err := listUserShards(dir)
	if err != nil {
		return nil, err
	}
	path := shards[0]

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("sea_reference snapshot: open user CSV %q: %w", path, err)
	}
	defer f.Close()

	header, err := csv.NewReader(f).Read()
	if err != nil {
		return nil, fmt.Errorf("sea_reference snapshot: read user CSV header %q: %w", path, err)
	}
	if len(header) < 4 {
		return nil, fmt.Errorf(
			"sea_reference snapshot: user CSV %q has malformed header column count %d",
			path, len(header),
		)
	}
	assetCount := len(header) - 3
	out := make([]string, assetCount)
	for i := range assetCount {
		sym := strings.ToLower(strings.TrimSpace(header[i+2]))
		if sym == "" {
			return nil, fmt.Errorf(
				"sea_reference snapshot: user CSV %q has empty asset name at column %d",
				path, i+2,
			)
		}
		out[i] = sym
	}
	return out, nil
}

// readCexAssetsCSV parses cex_assets_info.csv into a per-symbol
// bundle. Layout: header `symbol, usdt_price, total_equity`, then
// data rows. BasePrice uses PriceMultiplier from this profile's
// pricing provider. TotalEquity uses BalanceMultiplier.
func readCexAssetsCSV(path string, prov corespec.PriceScaleProvider) (map[string]modelspec.CexAssetInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("sea_reference snapshot: open %q: %w", path, err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("sea_reference snapshot: read %q: %w", path, err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("sea_reference snapshot: %q has no data rows", path)
	}
	rows = rows[1:] // drop header

	out := make(map[string]modelspec.CexAssetInfo, len(rows))
	for i, row := range rows {
		if len(row) != 3 {
			return nil, fmt.Errorf(
				"sea_reference snapshot: %q row %d has %d columns, expected 3",
				path, i+2, len(row),
			)
		}
		symbol := strings.ToLower(strings.TrimSpace(row[0]))
		if symbol == "" {
			return nil, fmt.Errorf("sea_reference snapshot: %q row %d: empty symbol", path, i+2)
		}
		if _, dup := out[symbol]; dup {
			return nil, fmt.Errorf("sea_reference snapshot: %q duplicate symbol %q", path, symbol)
		}
		basePrice, err := convertFloatStrToUint64(row[1], prov.PriceMultiplier(symbol))
		if err != nil {
			return nil, fmt.Errorf(
				"sea_reference snapshot: %q row %d (symbol %q): parse price: %w",
				path, i+2, symbol, err,
			)
		}
		totalEquity, err := convertFloatStrToUint64(row[2], prov.BalanceMultiplier(symbol))
		if err != nil {
			return nil, fmt.Errorf(
				"sea_reference snapshot: %q row %d (symbol %q): parse total_equity: %w",
				path, i+2, symbol, err,
			)
		}
		out[symbol] = modelspec.CexAssetInfo{
			Symbol:      symbol,
			BasePrice:   basePrice,
			TotalEquity: totalEquity,
		}
	}
	return out, nil
}

// convertFloatStrToUint64 mirrors binance/snapshot.go's helper of the
// same name — duplicated locally to keep the profile self-contained
// (R6 promotion candidate per rule-of-three).
func convertFloatStrToUint64(s string, multiplier int64) (uint64, error) {
	if s == "0.0" {
		return 0, nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return 0, err
	}
	d = d.Mul(decimal.NewFromInt(multiplier))
	b := d.BigInt()
	if !b.IsUint64() {
		return 0, fmt.Errorf("value %s overflows uint64", b.String())
	}
	return b.Uint64(), nil
}
