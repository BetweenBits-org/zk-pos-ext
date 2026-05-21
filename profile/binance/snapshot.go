package binance

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	modelspec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/spec"
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
}

// NewSnapshotCSV constructs a SnapshotSource backed by the legacy CSV
// directory layout. CexAssets reads cex_assets_info.csv and the first
// user-shard CSV's header on first call; the result is cached for the
// lifetime of the returned source. AccountStream is still stubbed and
// will be implemented in the R2 second sub-slice.
func NewSnapshotCSV(cfg SnapshotConfig) modelspec.SnapshotSource {
	return &csvSnapshot{cfg: cfg}
}

// errAccountStreamPending marks the user-shard streaming path that has
// not yet been migrated from legacy src/utils/utils.go.
var errAccountStreamPending = errors.New(
	"binance snapshot CSV AccountStream not yet implemented (R2 step 2)",
)

func (c *csvSnapshot) AccountStream(ctx context.Context) (<-chan modelspec.AccountInfo, error) {
	return nil, errAccountStreamPending
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

// readUserAssetOrder mirrors legacy
// src/utils/utils.go:ParseAssetIndexFromUserFile. It picks the first
// non-cex .csv file in dir (deterministic via filename sort), reads
// only its header row, and decodes the column-i*6+4 positions into
// lower-cased symbols.
func readUserAssetOrder(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("binance snapshot: read user data dir %q: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".csv") || name == cexAssetsCSVName {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf(
			"binance snapshot: no user CSV files found in %q (expected at least one .csv besides %s)",
			dir, cexAssetsCSVName,
		)
	}
	// os.ReadDir already returns entries lexically sorted but be
	// explicit so test fixtures with deliberate ordering remain stable.
	sort.Strings(names)
	path := filepath.Join(dir, names[0])

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

