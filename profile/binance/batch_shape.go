package binance

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// Two shapes historically used by Binance in production:
//   - 50-asset / 700-user   (long-tail retail accounts)
//   - 500-asset / 92-user   (whale accounts)
type shapeProvider struct {
	shapes []spec.BatchShape
}

// shapeOverrideEnv is the env-var name the smoke harness sets to swap
// the production shapes for a smaller set (e.g. "5_10" for end-to-end
// sample-data smoke). Format: comma-separated "<assetTier>_<usersPerBatch>"
// pairs, e.g. "5_10" or "5_10,500_92". When unset, NewBatchShape
// returns the production shapes verbatim.
const shapeOverrideEnv = "ZKPOR_BATCH_SHAPE_OVERRIDE"

// NewBatchShape returns Binance's BatchShapeProvider. When the
// ZKPOR_BATCH_SHAPE_OVERRIDE env var is set, it replaces the production
// shapes with the parsed override — intended for the end-to-end smoke
// harness, NOT for production deployment.
func NewBatchShape() spec.BatchShapeProvider {
	ss := []spec.BatchShape{
		{AssetCountTier: 50, UsersPerBatch: 700},
		{AssetCountTier: 500, UsersPerBatch: 92},
	}
	if override := os.Getenv(shapeOverrideEnv); override != "" {
		parsed, err := parseShapeOverride(override)
		if err != nil {
			panic(fmt.Sprintf("%s: %v", shapeOverrideEnv, err))
		}
		ss = parsed
	}
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].AssetCountTier < ss[j].AssetCountTier
	})
	return &shapeProvider{shapes: ss}
}

// parseShapeOverride decodes a "<tier>_<users>[,<tier>_<users>...]"
// string into BatchShape values. Rejects empty fields, non-positive
// numbers, and duplicate AssetCountTier values (the interface requires
// unique tiers).
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

func (p *shapeProvider) Model() spec.SolvencyModelID { return SolvencyModel }

func (p *shapeProvider) Shapes() []spec.BatchShape {
	out := make([]spec.BatchShape, len(p.shapes))
	copy(out, p.shapes)
	return out
}

func (p *shapeProvider) SelectFor(nonEmptyAssetCount int) (spec.BatchShape, error) {
	for _, s := range p.shapes {
		if nonEmptyAssetCount <= s.AssetCountTier {
			return s, nil
		}
	}
	last := p.shapes[len(p.shapes)-1]
	return spec.BatchShape{}, fmt.Errorf(
		"no batch shape fits %d non-empty assets (max tier is %d)",
		nonEmptyAssetCount, last.AssetCountTier,
	)
}

func (p *shapeProvider) KeyName(s spec.BatchShape, module string) string {
	return s.StandardKeyName(SolvencyModel, module)
}
