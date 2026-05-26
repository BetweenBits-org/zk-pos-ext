package binance

import (
	"testing"

	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
)

// testPricing returns the engine-default PriceScaleProvider tuned to
// match binance.toml — same default scales, same two-digit asset list,
// same shifted multipliers. Constructed via declarative.BuildPricing
// so the G6 invariant assert covers the test path too.
//
// The two-digit list is hand-mirrored from binance.toml. A drift
// surfaces as a test failure inside BuildPricing (G6 invariant
// violation) rather than silent miscount.
func testPricing(t *testing.T) corespec.PriceScaleProvider {
	t.Helper()
	p, err := declarative.BuildPricing(declarative.Pricing{
		DefaultPriceScale:    100_000_000,         // 1e8
		DefaultBalanceScale:  100_000_000,         // 1e8
		TwoDigitPriceScale:   100_000_000_000_000, // 1e14
		TwoDigitBalanceScale: 100,                 // 1e2
		TwoDigitAssets: []string{
			"bttc", "shib", "lunc", "xec", "win", "bidr", "spell", "hot",
			"doge", "pepe", "floki", "idrt", "dogs", "bonk", "1000sats",
			"neiro", "1000pepper", "not", "nft", "bome", "1mbabydoge",
			"hmstr", "wlfi", "pump", "monky", "1000cheems", "idr",
		},
	})
	if err != nil {
		t.Fatalf("declarative.BuildPricing for tests: %v", err)
	}
	return p
}
