package binance

import (
	"strings"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// Two-digit assets shift the multiplier split from 1e8 / 1e8 to
// 1e14 / 1e2 so very small unit prices remain representable while
// preserving the same 1e16 ValueScale.
const (
	twoDigitPriceScale   int64 = 100_000_000_000_000 // 1e14
	twoDigitBalanceScale int64 = 100                 // 1e2
)

// twoDigitAssets is the curated set requiring the shifted split.
// Sourced from legacy src/utils/constants.go AssetTypeForTwoDigits.
var twoDigitAssets = map[string]struct{}{
	"bttc":       {},
	"shib":       {},
	"lunc":       {},
	"xec":        {},
	"win":        {},
	"bidr":       {},
	"spell":      {},
	"hot":        {},
	"doge":       {},
	"pepe":       {},
	"floki":      {},
	"idrt":       {},
	"dogs":       {},
	"bonk":       {},
	"1000sats":   {},
	"neiro":      {},
	"1000pepper": {},
	"not":        {},
	"nft":        {},
	"bome":       {},
	"1mbabydoge": {},
	"hmstr":      {},
	"wlfi":       {},
	"pump":       {},
	"monky":      {},
	"1000cheems": {},
	"idr":        {},
}

type pricing struct{}

// NewPricing returns Binance's PriceScaleProvider.
func NewPricing() spec.PriceScaleProvider { return pricing{} }

func (pricing) PriceMultiplier(symbol string) int64 {
	if _, ok := twoDigitAssets[strings.ToLower(symbol)]; ok {
		return twoDigitPriceScale
	}
	return spec.DefaultPriceScale
}

func (pricing) BalanceMultiplier(symbol string) int64 {
	if _, ok := twoDigitAssets[strings.ToLower(symbol)]; ok {
		return twoDigitBalanceScale
	}
	return spec.DefaultBalanceScale
}

func (pricing) ValueScale() int64 { return spec.DefaultValueScale }
