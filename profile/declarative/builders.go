// builders.go — declarative → spec adapter constructors (R8-B/1).
//
// The model-blind half of the wiring: identity, insolvent, batch
// shape, pricing, catalog. All five build engine-universal spec
// implementations from the Profile struct, with no knowledge of the
// solvency model or the customer profile name.
//
// The model-typed half (snapshot connector, constraint module) lives
// under core/solvency/<model>/host/builder.go because both interfaces
// are model-parameterised — see R8-B/2.
//
// Why builders live here rather than in core/spec or core/host:
//   - core/spec is the frozen interface layer; constructors that bind
//     declarative TOML keys to interface implementations would couple
//     spec to a particular configuration shape.
//   - core/host owns the universal *implementations* (identity scheme,
//     drop-and-log policy) registered via init(); the *declarative*
//     mapping from TOML → host lookup is one layer up.
//
// All builders panic on programmer errors (referring to an
// unregistered scheme/action, schema-rule violations Validate()
// already covers). Errors returned from builders signal value-level
// problems the caller may handle (e.g. catalog symbol exceeds
// capacity at a different layer than Validate).
package declarative

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// BuildIdentity returns the AccountIDProvider registered in core/host
// under cfg.Scheme. Validate has already asserted Scheme is non-empty;
// an unknown scheme panics inside host.NewIdentity (build-time linking
// failure — see G17).
func BuildIdentity(cfg Identity) spec.AccountIDProvider {
	return host.NewIdentity(cfg.Scheme)
}

// BuildInsolvent returns the InvalidAccountPolicy registered in
// core/host under cfg.Action. Validate has already asserted Action is
// non-empty; an unknown action panics inside host.NewInsolventPolicy.
func BuildInsolvent(cfg Insolvent) spec.InvalidAccountPolicy {
	return host.NewInsolventPolicy(cfg.Action)
}

// shapeOverrideEnv preserves the smoke-harness override path that
// profile/binance/batch_shape.go originally hosted. Setting this var
// replaces the declarative shapes with the parsed comma-separated
// "<tier>_<users>" list — used end-to-end smoke ONLY, never in
// production.
const shapeOverrideEnv = "ZKPOR_BATCH_SHAPE_OVERRIDE"

// BuildBatchShape converts the declarative shape list into a sorted
// []spec.BatchShape. Honors ZKPOR_BATCH_SHAPE_OVERRIDE when set, so
// the smoke harness keeps working without per-profile code.
//
// Note: this builder returns just the []BatchShape values; the
// model-typed BatchShapeProvider wrapper (which also carries the
// SolvencyModelID) is constructed by the per-model builder in R8-B/2.
//
// Returns error when:
//   - the list is empty (Validate already catches this for the
//     non-override path; the override path can still produce empty).
//   - any (tier, users) is non-positive or two entries share a tier.
//   - the override env var has a malformed value.
func BuildBatchShape(cfgShapes []BatchShape) ([]spec.BatchShape, error) {
	var out []spec.BatchShape
	if override := os.Getenv(shapeOverrideEnv); override != "" {
		parsed, err := parseShapeOverride(override)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", shapeOverrideEnv, err)
		}
		out = parsed
	} else {
		out = make([]spec.BatchShape, 0, len(cfgShapes))
		seen := make(map[int]struct{}, len(cfgShapes))
		for i, s := range cfgShapes {
			if s.AssetCountTier <= 0 || s.UsersPerBatch <= 0 {
				return nil, fmt.Errorf("batch_shapes[%d] has non-positive field: %+v", i, s)
			}
			if _, dup := seen[s.AssetCountTier]; dup {
				return nil, fmt.Errorf("batch_shapes[%d] duplicate AssetCountTier %d", i, s.AssetCountTier)
			}
			seen[s.AssetCountTier] = struct{}{}
			out = append(out, spec.BatchShape{
				AssetCountTier: s.AssetCountTier,
				UsersPerBatch:  s.UsersPerBatch,
			})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("batch_shapes resolved to empty list")
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].AssetCountTier < out[j].AssetCountTier
	})
	return out, nil
}

// parseShapeOverride decodes "<tier>_<users>[,<tier>_<users>...]" into
// BatchShape values. Same syntax + checks as the legacy
// profile/binance.parseShapeOverride (R8-B/1 promotion).
func parseShapeOverride(raw string) ([]spec.BatchShape, error) {
	entries := strings.Split(raw, ",")
	out := make([]spec.BatchShape, 0, len(entries))
	seen := make(map[int]struct{}, len(entries))
	for i, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return nil, fmt.Errorf("entry %d: empty", i)
		}
		parts := strings.Split(entry, "_")
		if len(parts) != 2 {
			return nil, fmt.Errorf("entry %d: expected 'tier_users', got %q", i, entry)
		}
		tier, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || tier <= 0 {
			return nil, fmt.Errorf("entry %d: tier must be a positive integer, got %q", i, parts[0])
		}
		users, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || users <= 0 {
			return nil, fmt.Errorf("entry %d: users must be a positive integer, got %q", i, parts[1])
		}
		if _, dup := seen[tier]; dup {
			return nil, fmt.Errorf("entry %d: duplicate AssetCountTier %d", i, tier)
		}
		seen[tier] = struct{}{}
		out = append(out, spec.BatchShape{AssetCountTier: tier, UsersPerBatch: users})
	}
	return out, nil
}

// BuildPricing constructs a PriceScaleProvider from the declarative
// Pricing config. Symbols listed in cfg.TwoDigitAssets get the
// (TwoDigitPriceScale, TwoDigitBalanceScale) pair; everything else
// gets (DefaultPriceScale, DefaultBalanceScale).
//
// Enforces the G6 invariant (PriceMultiplier * BalanceMultiplier ==
// ValueScale) for both the default and the two-digit paths at builder
// time. Services that previously asserted this on startup now inherit
// the check transparently.
//
// Returns error when:
//   - any multiplier is non-positive (Validate covers the default
//     pair; the two-digit pair is covered here when the list is
//     non-empty).
//   - default × default != two_digit_price × two_digit_balance (the
//     two paths MUST share one ValueScale).
func BuildPricing(cfg Pricing) (spec.PriceScaleProvider, error) {
	if cfg.DefaultPriceScale <= 0 || cfg.DefaultBalanceScale <= 0 {
		return nil, fmt.Errorf("pricing default multipliers must be > 0, got %d × %d",
			cfg.DefaultPriceScale, cfg.DefaultBalanceScale)
	}
	defaultScale := cfg.DefaultPriceScale * cfg.DefaultBalanceScale
	if len(cfg.TwoDigitAssets) > 0 {
		if cfg.TwoDigitPriceScale <= 0 || cfg.TwoDigitBalanceScale <= 0 {
			return nil, fmt.Errorf("two_digit_assets requires two_digit_* multipliers > 0")
		}
		shifted := cfg.TwoDigitPriceScale * cfg.TwoDigitBalanceScale
		if shifted != defaultScale {
			return nil, fmt.Errorf(
				"G6 invariant: default %d × %d = %d ≠ two_digit %d × %d = %d",
				cfg.DefaultPriceScale, cfg.DefaultBalanceScale, defaultScale,
				cfg.TwoDigitPriceScale, cfg.TwoDigitBalanceScale, shifted)
		}
	}
	twoDigit := make(map[string]struct{}, len(cfg.TwoDigitAssets))
	for _, s := range cfg.TwoDigitAssets {
		twoDigit[strings.ToLower(s)] = struct{}{}
	}
	return &declarativePricing{
		defaultPrice:    cfg.DefaultPriceScale,
		defaultBalance:  cfg.DefaultBalanceScale,
		twoDigitPrice:   cfg.TwoDigitPriceScale,
		twoDigitBalance: cfg.TwoDigitBalanceScale,
		twoDigitSymbols: twoDigit,
		valueScale:      defaultScale,
	}, nil
}

// declarativePricing is the universal PriceScaleProvider promoted in
// R8-B/1 from profile/{binance,sea_reference}/pricing.go. Symbols are
// matched case-insensitively against twoDigitSymbols; matches use the
// shifted multiplier pair, misses fall through to default.
type declarativePricing struct {
	defaultPrice    int64
	defaultBalance  int64
	twoDigitPrice   int64
	twoDigitBalance int64
	twoDigitSymbols map[string]struct{}
	valueScale      int64
}

func (p *declarativePricing) PriceMultiplier(symbol string) int64 {
	if _, ok := p.twoDigitSymbols[strings.ToLower(symbol)]; ok {
		return p.twoDigitPrice
	}
	return p.defaultPrice
}

func (p *declarativePricing) BalanceMultiplier(symbol string) int64 {
	if _, ok := p.twoDigitSymbols[strings.ToLower(symbol)]; ok {
		return p.twoDigitBalance
	}
	return p.defaultBalance
}

func (p *declarativePricing) ValueScale() int64 { return p.valueScale }

// BuildCatalog constructs an AssetCatalog from cfg.Catalog.Symbols
// (case-folded to lower) and the given capacity. When the symbol list
// is empty, the catalog is constructed with zero entries — callers
// that need a header-derived order (the legacy binance path) MUST
// build their catalog directly from the snapshot CSV header instead.
//
// Capacity is the trusted-setup contract; symbols list MUST fit
// inside it.
//
// Returns error when:
//   - capacity <= 0.
//   - len(symbols) > capacity.
//   - duplicate symbol in the list (lower-cased).
func BuildCatalog(cfg CatalogConfig, capacity int) (spec.AssetCatalog, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("catalog capacity must be > 0, got %d", capacity)
	}
	if len(cfg.Symbols) > capacity {
		return nil, fmt.Errorf("catalog has %d symbols but capacity = %d", len(cfg.Symbols), capacity)
	}
	syms := make([]string, len(cfg.Symbols))
	idx := make(map[string]uint16, len(cfg.Symbols))
	for i, s := range cfg.Symbols {
		ls := strings.ToLower(s)
		if _, dup := idx[ls]; dup {
			return nil, fmt.Errorf("catalog symbols[%d] is a duplicate of an earlier entry: %q", i, ls)
		}
		syms[i] = ls
		idx[ls] = uint16(i)
	}
	return &declarativeCatalog{symbols: syms, index: idx, capacity: capacity}, nil
}

// declarativeCatalog is the universal AssetCatalog promoted in R8-B/1
// from profile/{binance,sea_reference}/catalog.go (byte-equivalent
// implementations).
type declarativeCatalog struct {
	symbols  []string
	index    map[string]uint16
	capacity int
}

func (c *declarativeCatalog) Capacity() int { return c.capacity }

func (c *declarativeCatalog) Symbols() []string {
	out := make([]string, len(c.symbols))
	copy(out, c.symbols)
	return out
}

func (c *declarativeCatalog) IndexOf(symbol string) (uint16, bool) {
	i, ok := c.index[strings.ToLower(symbol)]
	return i, ok
}
