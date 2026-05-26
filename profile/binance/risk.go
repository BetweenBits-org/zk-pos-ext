package binance

import (
	"strings"

	modelspec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec"
)

// Ratios is the per-symbol tier-ratio bundle loaded from a snapshot's
// cex_assets_info.csv. Each slice represents one collateral bucket
// (Loan / Margin / PortfolioMargin); empty means the bucket is closed
// for that symbol.
type Ratios struct {
	Loan            []modelspec.TierRatio
	Margin          []modelspec.TierRatio
	PortfolioMargin []modelspec.TierRatio
}

type riskPolicy struct {
	ratios map[string]Ratios // lowercase symbol -> bundle
}

// NewRisk returns Binance's RiskPolicy over the given per-symbol
// ratios. Symbols are lower-cased on entry. Unknown symbols return
// empty slices (= no collateral allowed in any bucket).
func NewRisk(ratios map[string]Ratios) modelspec.RiskPolicy {
	lc := make(map[string]Ratios, len(ratios))
	for k, v := range ratios {
		lc[strings.ToLower(k)] = v
	}
	return &riskPolicy{ratios: lc}
}

func (r *riskPolicy) LoanRatios(symbol string) []modelspec.TierRatio {
	return r.ratios[strings.ToLower(symbol)].Loan
}

func (r *riskPolicy) MarginRatios(symbol string) []modelspec.TierRatio {
	return r.ratios[strings.ToLower(symbol)].Margin
}

func (r *riskPolicy) PortfolioMarginRatios(symbol string) []modelspec.TierRatio {
	return r.ratios[strings.ToLower(symbol)].PortfolioMargin
}
