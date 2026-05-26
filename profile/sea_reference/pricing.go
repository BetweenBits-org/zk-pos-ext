package sea_reference

import (
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

type pricing struct{}

// NewPricing returns sea_reference's PriceScaleProvider. Default
// scales for every symbol — no per-symbol two-digit split. SEA
// reference profile assumes the customer's asset universe is bounded
// to mainstream majors and stablecoins where 1e8 / 1e8 scaling is
// sufficient (vs binance's broad altcoin coverage that required the
// 1e14 / 1e2 split for low-unit-price tokens).
//
// If a real SEA customer requires the shifted split for specific
// symbols (e.g. IDRT, SHIB), copy binance/pricing.go's twoDigitAssets
// pattern and amend the map here.
func NewPricing() spec.PriceScaleProvider { return pricing{} }

func (pricing) PriceMultiplier(symbol string) int64 {
	_ = symbol
	return spec.DefaultPriceScale
}

func (pricing) BalanceMultiplier(symbol string) int64 {
	_ = symbol
	return spec.DefaultBalanceScale
}

func (pricing) ValueScale() int64 { return spec.DefaultValueScale }
