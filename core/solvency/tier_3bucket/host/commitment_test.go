package host_test

import (
	"bytes"
	"hash"
	"math/big"
	"testing"

	legacyutils "github.com/binance/zkmerkle-proof-of-solvency/src/utils"
	tier3host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/host"
	tier3spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// sampleTierRatios returns a TierCount-length tier table for tests:
// strictly-increasing boundaries 1e16, 2e16, ..., TierCount*1e16 and
// strictly-decreasing ratios. PrecomputedValue is unused by the
// commitment math but the type carries it.
func sampleTierRatios(seed uint8) []tier3spec.TierRatio {
	out := make([]tier3spec.TierRatio, corespec.TierCount)
	for i := range corespec.TierCount {
		out[i] = tier3spec.TierRatio{
			BoundaryValue:    new(big.Int).Mul(big.NewInt(int64(i+1)), big.NewInt(corespec.DefaultValueScale)),
			Ratio:            uint8(int(seed) + 80 - i),
			PrecomputedValue: new(big.Int).SetUint64(0),
		}
	}
	return out
}

func tierToLegacy(in []tier3spec.TierRatio) [legacyutils.TierCount]legacyutils.TierRatio {
	var out [legacyutils.TierCount]legacyutils.TierRatio
	for i := range legacyutils.TierCount {
		out[i] = legacyutils.TierRatio{
			BoundaryValue:    new(big.Int).Set(in[i].BoundaryValue),
			Ratio:            in[i].Ratio,
			PrecomputedValue: new(big.Int).Set(in[i].PrecomputedValue),
		}
	}
	return out
}

// TestComputeUserAssetsCommitment_LegacyParity confirms the host
// commitment of a sample user's assets is byte-equal to legacy
// ComputeUserAssetsCommitment for the same input. Locks in
// trusted-setup byte compatibility of the user-asset commitment
// emitter path.
func TestComputeUserAssetsCommitment_LegacyParity(t *testing.T) {
	specAssets := []tier3spec.AccountAsset{
		{Index: 0, Equity: 100, Debt: 30, Loan: 20, Margin: 0, PortfolioMargin: 10},
		{Index: 3, Equity: 5_000_000, Debt: 1_234_567, Loan: 800_000, Margin: 100, PortfolioMargin: 50},
		{Index: 17, Equity: 999_999_999, Debt: 0, Loan: 999_999_999, Margin: 0, PortfolioMargin: 0},
	}
	legacyAssets := make([]legacyutils.AccountAsset, len(specAssets))
	for i, a := range specAssets {
		legacyAssets[i] = legacyutils.AccountAsset{
			Index: a.Index, Equity: a.Equity, Debt: a.Debt, Loan: a.Loan,
			Margin: a.Margin, PortfolioMargin: a.PortfolioMargin,
		}
	}

	// Snapshot the legacy module-global AssetCountsTiers so the host
	// helper receives the same tier set the legacy uses internally.
	assetCountTiers := append([]int(nil), legacyutils.AssetCountsTiers...)

	got := tier3host.ComputeUserAssetsCommitment(specAssets, assetCountTiers)
	var legacyHasher hash.Hash = poseidon.NewPoseidon()
	want := legacyutils.ComputeUserAssetsCommitment(&legacyHasher, legacyAssets)

	if !bytes.Equal(got, want) {
		t.Fatalf("user assets commitment byte mismatch:\n  host   = %x\n  legacy = %x", got, want)
	}
}

// TestComputeCexAssetsCommitment_LegacyParity confirms the host
// commitment over the global per-asset state is byte-equal to legacy
// ComputeCexAssetsCommitment for the same input (a single non-reserved
// entry; the helper pads the remaining slots up to AssetCounts).
func TestComputeCexAssetsCommitment_LegacyParity(t *testing.T) {
	spec0 := tier3spec.CexAssetInfo{
		TotalEquity:               1_000_000_000_000,
		TotalDebt:                 250_000_000_000,
		BasePrice:                 6_500_000_000_000,
		Symbol:                    "BTC",
		Index:                     0,
		LoanCollateral:            500_000_000_000,
		MarginCollateral:          200_000_000_000,
		PortfolioMarginCollateral: 50_000_000_000,
		LoanRatios:                sampleTierRatios(1),
		MarginRatios:              sampleTierRatios(2),
		PortfolioMarginRatios:     sampleTierRatios(3),
	}
	legacy0 := legacyutils.CexAssetInfo{
		TotalEquity:               spec0.TotalEquity,
		TotalDebt:                 spec0.TotalDebt,
		BasePrice:                 spec0.BasePrice,
		Symbol:                    spec0.Symbol,
		Index:                     spec0.Index,
		LoanCollateral:            spec0.LoanCollateral,
		MarginCollateral:          spec0.MarginCollateral,
		PortfolioMarginCollateral: spec0.PortfolioMarginCollateral,
		LoanRatios:                tierToLegacy(spec0.LoanRatios),
		MarginRatios:              tierToLegacy(spec0.MarginRatios),
		PortfolioMarginRatios:     tierToLegacy(spec0.PortfolioMarginRatios),
	}

	got := tier3host.ComputeCexAssetsCommitment([]tier3spec.CexAssetInfo{spec0})
	want := legacyutils.ComputeCexAssetsCommitment([]legacyutils.CexAssetInfo{legacy0})
	if !bytes.Equal(got, want) {
		t.Fatalf("cex assets commitment byte mismatch:\n  host   = %x\n  legacy = %x", got, want)
	}
}
