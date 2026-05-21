package binance

import (
	"fmt"
	"sort"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// Two shapes historically used by Binance in production:
//   - 50-asset / 700-user   (long-tail retail accounts)
//   - 500-asset / 92-user   (whale accounts)
type shapeProvider struct {
	shapes []spec.BatchShape
}

// NewBatchShape returns Binance's BatchShapeProvider.
func NewBatchShape() spec.BatchShapeProvider {
	ss := []spec.BatchShape{
		{AssetCountTier: 50, UsersPerBatch: 700},
		{AssetCountTier: 500, UsersPerBatch: 92},
	}
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].AssetCountTier < ss[j].AssetCountTier
	})
	return &shapeProvider{shapes: ss}
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
