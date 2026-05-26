package sea_reference

import (
	"testing"

	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
)

// testPricing returns the engine-default PriceScaleProvider tuned to
// match sea_reference.toml — uniform 1e8 × 1e8 scaling, no two-digit
// asset list. Built via declarative.BuildPricing so the G6 invariant
// assert covers the test path.
func testPricing(t *testing.T) corespec.PriceScaleProvider {
	t.Helper()
	p, err := declarative.BuildPricing(declarative.Pricing{
		DefaultPriceScale:   100_000_000,
		DefaultBalanceScale: 100_000_000,
	})
	if err != nil {
		t.Fatalf("declarative.BuildPricing for tests: %v", err)
	}
	return p
}
