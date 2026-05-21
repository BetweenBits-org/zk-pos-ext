package binance

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

	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	modelspec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/shopspring/decimal"
)

// cexAssetsCSVName is the fixed filename Binance's PoR pipeline uses to
// publish per-asset global state alongside the user shard CSVs.
const cexAssetsCSVName = "cex_assets_info.csv"

// reservedSymbol is the sentinel symbol stamped onto unused
// CexAssetInfo slots when the deployment uses fewer than
// corespec.AssetCounts assets. Mirrors legacy src/utils behaviour and
// matches the literal value the circuit-instance commitment is built
// against — DO NOT change without a trusted-setup re-ceremony.
const reservedSymbol = "reserved"

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

	once   sync.Once
	assets []modelspec.CexAssetInfo
	err    error

	// invalidCount accumulates the number of source rows classified
	// as invalid (collateral > equity, debt-uncovered, malformed
	// data) during AccountStream. Read concurrently with the stream
	// goroutine via the InvalidCount() method.
	invalidCount atomic.Uint64
}

// NewSnapshotCSV constructs a SnapshotSource backed by the legacy CSV
// directory layout. CexAssets reads cex_assets_info.csv and the first
// user-shard CSV's header on first call; the result is cached for the
// lifetime of the returned source. AccountStream walks every user
// shard sequentially, yielding one AccountInfo per CSV data row.
func NewSnapshotCSV(cfg SnapshotConfig) modelspec.SnapshotSource {
	return &csvSnapshot{cfg: cfg}
}

// accountStreamBuffer is the channel buffer size used by AccountStream.
// Sized for the typical witness-builder consumer that pulls accounts in
// batch-sized chunks (~700 by default) — a buffer of 1024 keeps the
// producer from blocking inside a single batch fill.
const accountStreamBuffer = 1024

// errInvalidRow marks per-row data problems that the snapshot
// classifies as "invalid account": the row is logged, increments
// InvalidCount, and is skipped without being yielded on the channel.
// Stream-fatal errors (column-count mismatch, IO failure) are
// returned without this wrap so they propagate to streamShard's
// channel-close path.
var errInvalidRow = errors.New("invalid account row")

// invalidf wraps an errInvalidRow with a formatted reason. Reserved
// for per-row data problems; stream-fatal errors MUST NOT use it.
func invalidf(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, errInvalidRow)...)
}

// safeAddU64 returns a+b+c when the sum fits in uint64, and (0, false)
// on overflow. Mirrors legacy SafeAdd but reports overflow instead of
// panicking — overflow becomes an invalid-row classification.
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

// AccountStream yields one AccountInfo per CSV data row across all
// user-shard files in UserDataDir (sorted lexically, processed
// sequentially in a single goroutine).
//
// Step 1 scope (happy-path): every successfully-parsed row is emitted
// regardless of solvency invariants (TotalCollateral vs TotalDebt,
// per-asset collateral vs equity). Invalid-account classification is
// deferred to R2/2 step 2. Per-row parse failures (malformed hex
// account id, malformed numeric field) close the channel early after
// logging — matching the spec's mid-stream error contract.
//
// AccountID handling: the raw 32-byte hex-decoded value is normalized
// via a bn254 fr.Element SetBytes→Marshal round-trip inside
// parseAccountRow. This mirrors legacy src/utils/utils.go:553 and
// keeps AccountInfo.AccountID == userproof.AccountID == in-circuit
// field input as a single canonical form. The normalization couples
// this snapshot to bn254 — every model currently in
// corespec.solvency_models.go is bn254, so the coupling is real but
// not yet a conflict. It is an R6 helper-promotion candidate when a
// second curve enters the catalog (see G13 in PRODUCTION_ROADMAP.md).
func (c *csvSnapshot) AccountStream(ctx context.Context) (<-chan modelspec.AccountInfo, error) {
	assets, err := c.CexAssets(ctx)
	if err != nil {
		return nil, fmt.Errorf("binance snapshot: preload CexAssets: %w", err)
	}
	shards, err := listUserShards(c.cfg.UserDataDir)
	if err != nil {
		return nil, err
	}
	out := make(chan modelspec.AccountInfo, accountStreamBuffer)
	go c.streamAccounts(ctx, shards, assets, pricing{}, out)
	return out, nil
}

// streamAccounts walks shards sequentially, parsing each row into an
// AccountInfo and pushing it onto out. Closes out on completion, ctx
// cancellation, or shard-level error. Fatal errors (header malformed,
// CSV IO) are logged before the channel closes; per-row "invalid"
// classifications are logged at the row level and counted via
// c.invalidCount.
func (c *csvSnapshot) streamAccounts(
	ctx context.Context,
	shards []string,
	assets []modelspec.CexAssetInfo,
	prov pricing,
	out chan<- modelspec.AccountInfo,
) {
	defer close(out)
	var validIndex uint32
	for _, path := range shards {
		if err := c.streamShard(ctx, path, assets, prov, &validIndex, out); err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				log.Printf("binance snapshot: stream %s: %v", path, err)
			}
			return
		}
	}
}

// streamShard reads one user-shard CSV, validating its header against
// the same (len-3) % 6 == 0 rule used by readUserAssetOrder. Each data
// row is parsed via parseAccountRow; rows wrapped as errInvalidRow are
// logged + counted + skipped (channel stays open), other errors close
// the channel. validIndex increments only on successful yield, so
// AccountInfo.AccountIndex is dense across the valid stream.
func (c *csvSnapshot) streamShard(
	ctx context.Context,
	path string,
	assets []modelspec.CexAssetInfo,
	prov pricing,
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
	if len(header) < 3 || (len(header)-3)%6 != 0 {
		return fmt.Errorf("malformed header column count %d", len(header))
	}
	assetCount := (len(header) - 3) / 6
	if assetCount > len(assets) {
		return fmt.Errorf(
			"shard has %d assets but cached snapshot has only %d slots",
			assetCount, len(assets),
		)
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
				log.Printf("binance snapshot: skip row %d in %s: %v", rawIndex, path, err)
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

// parseAccountRow converts a single CSV row into an AccountInfo. The
// per-asset block layout mirrors legacy ReadUserDataFromCsvFile:
// equity (j*6+2), debt (j*6+3), loan (j*6+5), margin (j*6+6),
// portfolio_margin (j*6+7). Column j*6+4 (asset name in the header)
// is intentionally not read. Per-asset values use the BalanceMultiplier
// from the pricing provider (1e8 default, 1e2 for two-digit assets).
// An AccountAsset entry is emitted only when equity or debt is non-zero,
// matching legacy semantics.
//
// Errors with errInvalidRow attached classify the row as invalid —
// streamShard logs, counts, and skips it. Errors without that wrap
// are stream-fatal (e.g. column-count mismatch).
func parseAccountRow(
	row []string,
	assets []modelspec.CexAssetInfo,
	assetCount int,
	index uint32,
	prov pricing,
) (modelspec.AccountInfo, error) {
	minCols := 2 + assetCount*6
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
	// bn254 fr.Element normalization — mirrors legacy
	// src/utils/utils.go:553. About half of SHA256-derived 32-byte IDs
	// exceed the bn254 modulus; without this round-trip the snapshot
	// and the in-circuit field input would diverge byte-for-byte on
	// roughly half of accounts. This single line keeps
	// AccountInfo.AccountID == userproof.AccountID == circuit field
	// input. The dependency on bn254 is intentional at this layer for
	// G13 (R3 step 1 closure) — promote to a curve-agnostic helper in
	// core/circuit/ when a second curve enters the catalog (R6 / G11).
	accountID = new(fr.Element).SetBytes(accountID).Marshal()
	account := modelspec.AccountInfo{
		AccountIndex:    index,
		AccountID:       accountID,
		TotalEquity:     new(big.Int),
		TotalDebt:       new(big.Int),
		TotalCollateral: new(big.Int),
	}
	for j := range assetCount {
		symbol := assets[j].Symbol
		mult := prov.BalanceMultiplier(symbol)
		equity, err := convertFloatStrToUint64(row[j*6+2], mult)
		if err != nil {
			return modelspec.AccountInfo{}, invalidf("asset %q equity: %v", symbol, err)
		}
		debt, err := convertFloatStrToUint64(row[j*6+3], mult)
		if err != nil {
			return modelspec.AccountInfo{}, invalidf("asset %q debt: %v", symbol, err)
		}
		loan, err := convertFloatStrToUint64(row[j*6+5], mult)
		if err != nil {
			return modelspec.AccountInfo{}, invalidf("asset %q loan: %v", symbol, err)
		}
		margin, err := convertFloatStrToUint64(row[j*6+6], mult)
		if err != nil {
			return modelspec.AccountInfo{}, invalidf("asset %q margin: %v", symbol, err)
		}
		pm, err := convertFloatStrToUint64(row[j*6+7], mult)
		if err != nil {
			return modelspec.AccountInfo{}, invalidf("asset %q portfolio margin: %v", symbol, err)
		}
		if equity == 0 && debt == 0 {
			continue
		}
		// Per-asset solvency invariant: loan + margin + pm <= equity.
		// Mirrors legacy line 615 (collateral sum > equity → invalid).
		sumCollateral, ok := safeAddU64(loan, margin, pm)
		if !ok {
			return modelspec.AccountInfo{}, invalidf(
				"asset %q collateral sum overflows uint64", symbol,
			)
		}
		if sumCollateral > equity {
			return modelspec.AccountInfo{}, invalidf(
				"asset %q collateral %d > equity %d", symbol, sumCollateral, equity,
			)
		}
		account.Assets = append(account.Assets, modelspec.AccountAsset{
			Index:           uint16(j),
			Equity:          equity,
			Debt:            debt,
			Loan:            loan,
			Margin:          margin,
			PortfolioMargin: pm,
		})
		price := new(big.Int).SetUint64(assets[j].BasePrice)
		addScaled(account.TotalEquity, equity, price)
		addScaled(account.TotalDebt, debt, price)
		account.TotalCollateral.Add(
			account.TotalCollateral,
			assetCollateralValue(loan, margin, pm, price, &assets[j]),
		)
	}
	// Account-level solvency invariant: TotalCollateral >= TotalDebt.
	// Mirrors legacy line 634 (debt-uncovered account → invalid).
	if account.TotalCollateral.Cmp(account.TotalDebt) < 0 {
		return modelspec.AccountInfo{}, invalidf(
			"account TotalCollateral %s < TotalDebt %s",
			account.TotalCollateral, account.TotalDebt,
		)
	}
	return account, nil
}

// addScaled accumulates (val * factor) into acc in place.
func addScaled(acc *big.Int, val uint64, factor *big.Int) {
	tmp := new(big.Int).SetUint64(val)
	tmp.Mul(tmp, factor)
	acc.Add(acc, tmp)
}

// percentageDivisor is the denominator used by the haircut math
// (TierRatio.Ratio is a /100 percentage). Matches legacy
// PercentageMultiplier verbatim.
var percentageDivisor = big.NewInt(100)

// assetCollateralValue mirrors legacy
// src/utils/utils.go:CalculateAssetValueForCollateral. Each of the
// three collateral types is priced (amount * BasePrice), passed
// through its tier-ratio haircut table, and the three results summed.
func assetCollateralValue(
	loan, margin, pm uint64,
	price *big.Int,
	info *modelspec.CexAssetInfo,
) *big.Int {
	haircut := func(amount uint64, tiers []modelspec.TierRatio) *big.Int {
		v := new(big.Int).SetUint64(amount)
		v.Mul(v, price)
		return haircutValue(v, tiers)
	}
	sum := haircut(loan, info.LoanRatios)
	sum.Add(sum, haircut(margin, info.MarginRatios))
	sum.Add(sum, haircut(pm, info.PortfolioMarginRatios))
	return sum
}

// haircutValue applies a piecewise-linear haircut to value using
// boundary cutoffs and precomputed cumulative haircuts. Mirrors legacy
// CalculateAssetValueViaTiersRatio.
//
//	If value falls in tier i (value <= boundary[i]):
//	  out = (value - boundary[i-1]) * ratio[i] / 100  +  precomputed[i-1]
//	with boundary[-1] = precomputed[-1] = 0.
//	If value exceeds the last boundary, out = precomputed[last].
func haircutValue(value *big.Int, tiers []modelspec.TierRatio) *big.Int {
	if len(tiers) == 0 {
		return new(big.Int)
	}
	v := new(big.Int).Set(value)
	for i, t := range tiers {
		if v.Cmp(t.BoundaryValue) <= 0 {
			if i > 0 {
				v.Sub(v, tiers[i-1].BoundaryValue)
			}
			v.Mul(v, new(big.Int).SetUint64(uint64(t.Ratio)))
			v.Quo(v, percentageDivisor)
			if i > 0 {
				v.Add(v, tiers[i-1].PrecomputedValue)
			}
			return v
		}
	}
	return new(big.Int).Set(tiers[len(tiers)-1].PrecomputedValue)
}

// CexAssets returns the per-asset CEX totals slice indexed by
// AssetCatalog index. Symbol order is derived from the first user-shard
// CSV's header — this is the same convention legacy
// src/utils/utils.go:ParseAssetIndexFromUserFile uses, so the resulting
// commitment matches byte-for-byte. The returned slice is always length
// corespec.AssetCounts, with unused slots filled by reservedSymbol
// entries whose ratios are MaxTierBoundary placeholders.
//
// The slice is loaded lazily on the first call and cached; subsequent
// calls return a defensive copy so callers cannot mutate cached state.
func (c *csvSnapshot) CexAssets(ctx context.Context) ([]modelspec.CexAssetInfo, error) {
	c.once.Do(func() {
		c.assets, c.err = loadCSVSnapshot(c.cfg.UserDataDir)
	})
	if c.err != nil {
		return nil, c.err
	}
	out := make([]modelspec.CexAssetInfo, len(c.assets))
	copy(out, c.assets)
	return out, nil
}

func (c *csvSnapshot) SnapshotID() string { return c.cfg.SnapshotID }

func (c *csvSnapshot) InvalidCount() uint64 { return c.invalidCount.Load() }

// loadCSVSnapshot is the one-shot CSV → []CexAssetInfo absorber. It
// fuses legacy ParseAssetIndexFromUserFile + ParseCexAssetInfoFromFile
// into a single deterministic pass.
//
// Steps:
//  1. Pick the first .csv file in dir other than cex_assets_info.csv
//     and read its header. Asset names live at column i*6+4 of the
//     header row (after a 2-column rn,id prelude); the count comes
//     from (headerLen - 3) / 6, matching legacy semantics.
//  2. Parse cex_assets_info.csv into a per-symbol bundle. Per-symbol
//     price multipliers are read from pricing.PriceMultiplier so
//     two-digit assets keep their shifted 1e14 / 1e2 split.
//  3. Compose: the symbol order from (1) drives the Index of each
//     CexAssetInfo; the bundle from (2) supplies BasePrice and three
//     TierRatio slices padded to corespec.TierCount with
//     PrecomputedValue filled.
//  4. Pad the slice up to corespec.AssetCounts with reservedSymbol
//     entries so the witness shape stays constant across deployments
//     with fewer assets than the engine cap.
func loadCSVSnapshot(dir string) ([]modelspec.CexAssetInfo, error) {
	if dir == "" {
		return nil, errors.New("binance snapshot: UserDataDir is empty")
	}
	order, err := readUserAssetOrder(dir)
	if err != nil {
		return nil, err
	}
	if len(order) > corespec.AssetCounts {
		return nil, fmt.Errorf(
			"binance snapshot: user CSV has %d assets, exceeds engine cap %d",
			len(order), corespec.AssetCounts,
		)
	}
	bySymbol, err := readCexAssetsCSV(filepath.Join(dir, cexAssetsCSVName))
	if err != nil {
		return nil, err
	}
	if len(order) != len(bySymbol) {
		return nil, fmt.Errorf(
			"binance snapshot: user CSV header has %d assets but %s has %d",
			len(order), cexAssetsCSVName, len(bySymbol),
		)
	}

	assets := make([]modelspec.CexAssetInfo, corespec.AssetCounts)
	for i, sym := range order {
		info, ok := bySymbol[sym]
		if !ok {
			return nil, fmt.Errorf(
				"binance snapshot: user CSV references asset %q absent from %s",
				sym, cexAssetsCSVName,
			)
		}
		info.Index = uint32(i)
		assets[i] = info
	}
	for i := len(order); i < corespec.AssetCounts; i++ {
		assets[i] = reservedCexAsset(uint32(i))
	}
	return assets, nil
}

// listUserShards returns the absolute paths of all per-shard user CSV
// files in dir, sorted lexically for determinism. The cex_assets_info
// file is excluded. Returns an error if dir is empty / unreadable / has
// no user shards.
func listUserShards(dir string) ([]string, error) {
	if dir == "" {
		return nil, errors.New("binance snapshot: UserDataDir is empty")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("binance snapshot: read user data dir %q: %w", dir, err)
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
			"binance snapshot: no user CSV files found in %q (expected at least one .csv besides %s)",
			dir, cexAssetsCSVName,
		)
	}
	// os.ReadDir already returns entries lexically sorted but be
	// explicit so test fixtures with deliberate ordering remain stable.
	sort.Strings(paths)
	return paths, nil
}

// readUserAssetOrder mirrors legacy
// src/utils/utils.go:ParseAssetIndexFromUserFile. It picks the first
// non-cex .csv file in dir (deterministic via filename sort), reads
// only its header row, and decodes the column-i*6+4 positions into
// lower-cased symbols.
func readUserAssetOrder(dir string) ([]string, error) {
	shards, err := listUserShards(dir)
	if err != nil {
		return nil, err
	}
	path := shards[0]

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("binance snapshot: open user CSV %q: %w", path, err)
	}
	defer f.Close()

	header, err := csv.NewReader(f).Read()
	if err != nil {
		return nil, fmt.Errorf("binance snapshot: read user CSV header %q: %w", path, err)
	}
	// Legacy layout: prelude of 2 columns (rn, id), then 6 columns per
	// asset (equity, debt, NAME, loan, margin, portfolio_margin), plus
	// one trailing column. (headerLen - 3) / 6 yields the asset count.
	if len(header) < 3 || (len(header)-3)%6 != 0 {
		return nil, fmt.Errorf(
			"binance snapshot: user CSV %q has malformed header column count %d",
			path, len(header),
		)
	}
	assetCount := (len(header) - 3) / 6
	out := make([]string, assetCount)
	for i := range assetCount {
		sym := strings.ToLower(strings.TrimSpace(header[i*6+4]))
		if sym == "" {
			return nil, fmt.Errorf(
				"binance snapshot: user CSV %q has empty asset name at column %d",
				path, i*6+4,
			)
		}
		out[i] = sym
	}
	return out, nil
}

// readCexAssetsCSV parses cex_assets_info.csv into a per-symbol bundle.
// Mirrors legacy ParseCexAssetInfoFromFile but stops before assigning
// Index — index assignment is the caller's concern, since it depends
// on the user CSV header order, not the cex_assets_info.csv row order.
//
// Per-row layout (legacy, byte-locked):
//
//	col 0: token (lower-cased on entry, used as map key)
//	col 1: asset_usdt_price (float; multiplier from pricing)
//	col 2: collateral_vip_loan_ratio_tiers
//	col 3: collateral_margin_ratio_tiers
//	col 4: collateral_portfolio_margin_ratio_tiers
func readCexAssetsCSV(path string) (map[string]modelspec.CexAssetInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("binance snapshot: open %q: %w", path, err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("binance snapshot: read %q: %w", path, err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("binance snapshot: %q has no data rows", path)
	}
	rows = rows[1:] // drop header

	prov := pricing{}
	out := make(map[string]modelspec.CexAssetInfo, len(rows))
	for i, row := range rows {
		if len(row) != 5 {
			return nil, fmt.Errorf(
				"binance snapshot: %q row %d has %d columns, expected 5",
				path, i+2, len(row),
			)
		}
		symbol := strings.ToLower(strings.TrimSpace(row[0]))
		if symbol == "" {
			return nil, fmt.Errorf("binance snapshot: %q row %d: empty symbol", path, i+2)
		}
		if _, dup := out[symbol]; dup {
			return nil, fmt.Errorf("binance snapshot: %q duplicate symbol %q", path, symbol)
		}
		basePrice, err := convertFloatStrToUint64(row[1], prov.PriceMultiplier(symbol))
		if err != nil {
			return nil, fmt.Errorf(
				"binance snapshot: %q row %d (symbol %q): parse price: %w",
				path, i+2, symbol, err,
			)
		}
		loan, err := parseTierRatios(row[2])
		if err != nil {
			return nil, fmt.Errorf(
				"binance snapshot: %q row %d (symbol %q): parse loan tiers: %w",
				path, i+2, symbol, err,
			)
		}
		margin, err := parseTierRatios(row[3])
		if err != nil {
			return nil, fmt.Errorf(
				"binance snapshot: %q row %d (symbol %q): parse margin tiers: %w",
				path, i+2, symbol, err,
			)
		}
		pm, err := parseTierRatios(row[4])
		if err != nil {
			return nil, fmt.Errorf(
				"binance snapshot: %q row %d (symbol %q): parse portfolio margin tiers: %w",
				path, i+2, symbol, err,
			)
		}
		out[symbol] = modelspec.CexAssetInfo{
			Symbol:                symbol,
			BasePrice:             basePrice,
			LoanRatios:            loan,
			MarginRatios:          margin,
			PortfolioMarginRatios: pm,
		}
	}
	return out, nil
}

// maxTierBoundary is the largest BoundaryValue a TierRatio may carry —
// the 2^118 cap baked into the tier_3bucket circuit. Mirrors legacy
// MaxTierBoundaryValue verbatim.
var maxTierBoundary, _ = new(big.Int).SetString("332306998946228968225951765070086144", 10)

// tierBoundaryScale is the multiplier applied to the integer boundary
// values written in cex_assets_info.csv. Equals corespec.DefaultValueScale
// (1e16) — boundary values are quoted in standard-scale value units.
var tierBoundaryScale = big.NewInt(corespec.DefaultValueScale)

// hundred is the divisor used by the haircut math (Ratio is a /100
// fraction).
var hundred = big.NewInt(100)

// parseTierRatios parses one tier-ratio cell of cex_assets_info.csv
// into a corespec.TierCount-padded TierRatio slice with
// PrecomputedValue filled. Mirrors legacy ParseTiersRatioFromStr +
// CalculatePrecomputedValue + PaddingTierRatios fused.
//
// Cell grammar (legacy, byte-locked):
//
//	"[lo-hi:ratio, lo-hi:ratio, ...]"  or  ""  for empty.
//
// Each lo / hi / ratio is an integer-valued float. Boundary values are
// scaled by tierBoundaryScale; ratios are kept as uint8 percentages.
// Returns an error on malformed grammar, non-monotonic boundaries, or
// boundaries exceeding maxTierBoundary.
func parseTierRatios(raw string) ([]modelspec.TierRatio, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, "[]")
	if raw == "" {
		return paddingTierRatios(nil), nil
	}
	entries := strings.Split(raw, ",")
	parsed := make([]modelspec.TierRatio, 0, len(entries))
	for i, entry := range entries {
		entry = strings.TrimSpace(entry)
		parts := strings.Split(entry, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("tier entry %d: expected 'lo-hi:ratio', got %q", i, entry)
		}
		rangeParts := strings.Split(strings.TrimSpace(parts[0]), "-")
		if len(rangeParts) != 2 {
			return nil, fmt.Errorf("tier entry %d: expected 'lo-hi', got %q", i, parts[0])
		}
		lo, err := convertFloatStrToUint64(strings.TrimSpace(rangeParts[0]), 1)
		if err != nil {
			return nil, fmt.Errorf("tier entry %d: lo: %w", i, err)
		}
		hi, err := convertFloatStrToUint64(strings.TrimSpace(rangeParts[1]), 1)
		if err != nil {
			return nil, fmt.Errorf("tier entry %d: hi: %w", i, err)
		}
		ratio, err := convertFloatStrToUint64(strings.TrimSpace(parts[1]), 1)
		if err != nil {
			return nil, fmt.Errorf("tier entry %d: ratio: %w", i, err)
		}
		loBig := new(big.Int).Mul(new(big.Int).SetUint64(lo), tierBoundaryScale)
		hiBig := new(big.Int).Mul(new(big.Int).SetUint64(hi), tierBoundaryScale)
		if hiBig.Cmp(loBig) < 0 {
			return nil, fmt.Errorf("tier entry %d: boundary hi < lo (%s < %s)", i, hiBig, loBig)
		}
		if hiBig.Cmp(maxTierBoundary) > 0 {
			return nil, fmt.Errorf(
				"tier entry %d: boundary %s exceeds MaxTierBoundaryValue", i, hiBig,
			)
		}
		if len(parsed) > 0 && hiBig.Cmp(parsed[len(parsed)-1].BoundaryValue) <= 0 {
			return nil, fmt.Errorf(
				"tier entry %d: boundary %s not strictly greater than previous %s",
				i, hiBig, parsed[len(parsed)-1].BoundaryValue,
			)
		}
		parsed = append(parsed, modelspec.TierRatio{
			BoundaryValue: hiBig,
			Ratio:         uint8(ratio),
		})
	}
	if len(parsed) > corespec.TierCount {
		return nil, fmt.Errorf(
			"tier cell has %d tiers, exceeds TierCount cap %d",
			len(parsed), corespec.TierCount,
		)
	}
	fillPrecomputedValues(parsed)
	return paddingTierRatios(parsed), nil
}

// fillPrecomputedValues writes the cumulative haircut at each tier
// boundary into t[i].PrecomputedValue:
//
//	cum_i = sum_{j<=i} (boundary[j] - boundary[j-1]) * ratio[j] / 100
//	       where boundary[-1] := 0
//
// Mirrors legacy CalculatePrecomputedValue. The in-circuit haircut
// lookup uses this value to apply the piecewise-linear haircut in O(1)
// per query.
func fillPrecomputedValues(t []modelspec.TierRatio) {
	cum := new(big.Int)
	for i := range t {
		lo := new(big.Int)
		if i > 0 {
			lo.Set(t[i-1].BoundaryValue)
		}
		diff := new(big.Int).Sub(t[i].BoundaryValue, lo)
		diff.Mul(diff, new(big.Int).SetUint64(uint64(t[i].Ratio)))
		diff.Quo(diff, hundred)
		cum.Add(cum, diff)
		t[i].PrecomputedValue = new(big.Int).Set(cum)
	}
}

// paddingTierRatios pads in up to corespec.TierCount entries. Trailing
// pad entries inherit the final PrecomputedValue (so the in-circuit
// query for an out-of-range collateral still resolves to the correct
// cap), with BoundaryValue = maxTierBoundary and Ratio = 0. Mirrors
// legacy PaddingTierRatios.
func paddingTierRatios(in []modelspec.TierRatio) []modelspec.TierRatio {
	out := make([]modelspec.TierRatio, corespec.TierCount)
	var lastPrecomputed *big.Int
	if len(in) > 0 {
		lastPrecomputed = in[len(in)-1].PrecomputedValue
	}
	for i := range corespec.TierCount {
		if i < len(in) {
			out[i] = in[i]
			continue
		}
		precomp := new(big.Int)
		if lastPrecomputed != nil {
			precomp.Set(lastPrecomputed)
		}
		out[i] = modelspec.TierRatio{
			BoundaryValue:    new(big.Int).Set(maxTierBoundary),
			Ratio:            0,
			PrecomputedValue: precomp,
		}
	}
	return out
}

// reservedCexAsset returns a placeholder CexAssetInfo for unused asset
// slots — Symbol set to reservedSymbol, all numeric fields zero, all
// three TierRatio slices padded with maxTierBoundary entries.
func reservedCexAsset(index uint32) modelspec.CexAssetInfo {
	return modelspec.CexAssetInfo{
		Symbol:                reservedSymbol,
		Index:                 index,
		LoanRatios:            paddingTierRatios(nil),
		MarginRatios:          paddingTierRatios(nil),
		PortfolioMarginRatios: paddingTierRatios(nil),
	}
}

// convertFloatStrToUint64 mirrors legacy ConvertFloatStrToUint64. The
// "0.0" early return preserves byte-for-byte semantics for cells the
// CSV publisher writes as a literal "0.0".
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

