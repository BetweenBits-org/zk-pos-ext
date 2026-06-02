// Package host contains off-circuit (native) helpers specific to the
// t3_tiered_haircut_margin_1pool solvency model. Single-bucket
// simplification of t4_tiered_haircut_margin_3pool/host — the 3-bucket
// collateral split (Loan / Margin / PortfolioMargin) is collapsed into
// one Collateral pool with one tier curve per asset.
package host

import (
	"fmt"
	"math/big"

	t3spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t3_tiered_haircut_margin_1pool/spec"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// Native byte-packing constants (same shapes as T4's; values reused
// across the tier-curve family of models).
var (
	twoToThe64      = new(big.Int).Lsh(big.NewInt(1), 64)
	twoToThe128     = new(big.Int).Lsh(big.NewInt(1), 128)
	twoToThe8       = new(big.Int).Lsh(big.NewInt(1), 8)
	twoToThe126     = new(big.Int).Lsh(big.NewInt(1), 126)
	twoToThe134     = new(big.Int).Lsh(big.NewInt(1), 134)
	maxTierBoundary = new(big.Int).Lsh(big.NewInt(1), 118)
)

// ComputeUserAssetsCommitment returns the Poseidon commitment of one
// user's per-asset 4-tuple sequence (Index, Equity, Debt, Collateral),
// packed and padded as the circuit expects. assetCountTiers MUST be
// sorted ascending and contain at least one tier >= len(assets).
//
// Byte-equivalent to in-circuit ComputeFlatUint64Commitment over the
// same flat sequence (3 uint64s per field element).
func ComputeUserAssetsCommitment(assets []t3spec.AccountAsset, assetCountTiers []int) []byte {
	hasher := poseidon.NewPoseidon()
	flat := paddingAccountAssets(assets, assetCountTiers)
	const fieldsPerAsset = 4
	const valsPerField = 3
	targetCounts := getAssetsCountOfUser(assets, assetCountTiers)
	totalUint64s := targetCounts * fieldsPerAsset
	nEles := (totalUint64s + valsPerField - 1) / valsPerField

	a := new(big.Int)
	b := new(big.Int)
	c := new(big.Int)
	tmp := new(big.Int)
	for i := range nEles {
		a.SetUint64(0)
		if i*valsPerField < len(flat) {
			a.SetUint64(flat[i*valsPerField])
		}
		b.SetUint64(0)
		if i*valsPerField+1 < len(flat) {
			b.SetUint64(flat[i*valsPerField+1])
		}
		c.SetUint64(0)
		if i*valsPerField+2 < len(flat) {
			c.SetUint64(flat[i*valsPerField+2])
		}
		// sum = a * 2^128 + b * 2^64 + c
		tmp.Mul(a, twoToThe128)
		tmp.Add(tmp, new(big.Int).Mul(b, twoToThe64))
		tmp.Add(tmp, c)
		hasher.Write(tmp.Bytes())
	}
	return hasher.Sum(nil)
}

// ComputeCexAssetsCommitment returns the Poseidon commitment over the
// global per-asset state, padded to `capacity` slots. Single-bucket
// version of t4's helper (one CollateralRatios slice per asset).
//
// Each entry's CollateralRatios slice MUST already have length
// corespec.TierCount — the snapshot adapter is responsible for
// padding. Panics if len(cexAssetsInfo) > capacity.
func ComputeCexAssetsCommitment(cexAssetsInfo []t3spec.CexAssetInfo, capacity int) []byte {
	if len(cexAssetsInfo) > capacity {
		panic(fmt.Sprintf("ComputeCexAssetsCommitment: %d entries exceeds capacity %d",
			len(cexAssetsInfo), capacity))
	}
	hasher := poseidon.NewPoseidon()
	padded := make([]t3spec.CexAssetInfo, len(cexAssetsInfo), capacity)
	copy(padded, cexAssetsInfo)
	for i := len(cexAssetsInfo); i < capacity; i++ {
		padded = append(padded, t3spec.CexAssetInfo{
			Symbol:           "reserved",
			Index:            uint32(i),
			CollateralRatios: reservedTierRatios(),
		})
	}
	for i := range padded {
		for _, chunk := range convertAssetInfoToBytes(padded[i]) {
			hasher.Write(chunk)
		}
	}
	return hasher.Sum(nil)
}

// getAssetsCountOfUser returns the smallest tier in assetCountTiers
// that is >= len(assets).
func getAssetsCountOfUser(assets []t3spec.AccountAsset, assetCountTiers []int) int {
	count := len(assets)
	for _, v := range assetCountTiers {
		if count <= v {
			return v
		}
	}
	return 0
}

// paddingAccountAssets converts a user's per-asset records into the
// flat uint64 sequence: (Index, Equity, Debt, Collateral) per asset,
// with padding entries inserted at synthetic indices in the gaps.
func paddingAccountAssets(assets []t3spec.AccountAsset, assetCountTiers []int) []uint64 {
	targetCounts := getAssetsCountOfUser(assets, assetCountTiers)
	if targetCounts < len(assets) {
		panic("the target counts is less than the length of assets")
	}
	const fieldsPerAsset = 4
	out := make([]uint64, targetCounts*fieldsPerAsset)
	paddingCounts := targetCounts - len(assets)
	currentPaddingCounts := 0
	currentAssetIndex := 0
	index := 0
	for i := range assets {
		if currentPaddingCounts < paddingCounts {
			for j := currentAssetIndex; j < int(assets[i].Index); j++ {
				currentPaddingCounts++
				out[index*fieldsPerAsset] = uint64(j)
				index++
				if currentPaddingCounts >= paddingCounts {
					break
				}
			}
		}
		out[index*fieldsPerAsset] = uint64(assets[i].Index)
		out[index*fieldsPerAsset+1] = assets[i].Equity
		out[index*fieldsPerAsset+2] = assets[i].Debt
		out[index*fieldsPerAsset+3] = assets[i].Collateral
		index++
		currentAssetIndex = int(assets[i].Index) + 1
	}
	for i := index; i < targetCounts; i++ {
		out[i*fieldsPerAsset] = uint64(currentAssetIndex)
		currentAssetIndex++
	}
	return out
}

// convertAssetInfoToBytes packs one CexAssetInfo into the byte-chunk
// sequence the Poseidon hasher absorbs. T3 layout: 1 packing
// (Equity / Debt / BasePrice), then Collateral alone, then
// TierCount/2 packings of CollateralRatios (1 bucket).
func convertAssetInfoToBytes(a t3spec.CexAssetInfo) [][]byte {
	res := make([][]byte, 0, 2+corespec.TierCount/2)
	// Pack 1: Equity * 2^128 + Debt * 2^64 + BasePrice
	res = append(res, packThreeUint64(a.TotalEquity, a.TotalDebt, a.BasePrice))
	// Pack 2: Collateral alone
	res = append(res, new(big.Int).SetUint64(a.Collateral).Bytes())
	res = append(res, convertTierRatiosToBytes(a.CollateralRatios)...)
	return res
}

// packThreeUint64 returns the big-endian unpadded byte representation
// of {high * 2^128 + mid * 2^64 + low}.
func packThreeUint64(high, mid, low uint64) []byte {
	res := new(big.Int).Mul(new(big.Int).SetUint64(high), twoToThe128)
	res.Add(res, new(big.Int).Mul(new(big.Int).SetUint64(mid), twoToThe64))
	res.Add(res, new(big.Int).SetUint64(low))
	return res.Bytes()
}

// convertTierRatiosToBytes pairs adjacent tier entries and packs them
// into one ~252-bit chunk per pair:
//
//	chunk = ratio[i] + boundary[i]*2^8 + ratio[i+1]*2^126 + boundary[i+1]*2^134
//
// Requires len(ratios) to be even (corespec.TierCount enforces this).
func convertTierRatiosToBytes(ratios []t3spec.TierRatio) [][]byte {
	res := make([][]byte, 0, len(ratios)/2)
	for i := 0; i < len(ratios); i += 2 {
		a := new(big.Int).SetUint64(uint64(ratios[i].Ratio))
		a.Add(a, new(big.Int).Mul(ratios[i].BoundaryValue, twoToThe8))

		b := new(big.Int).Mul(new(big.Int).SetUint64(uint64(ratios[i+1].Ratio)), twoToThe126)
		b.Add(b, new(big.Int).Mul(ratios[i+1].BoundaryValue, twoToThe134))

		res = append(res, new(big.Int).Add(a, b).Bytes())
	}
	return res
}

// reservedTierRatios builds the TierCount-length padding slice for
// reserved CEX-asset slots: every entry has BoundaryValue = 2^118 (the
// maxTierBoundary sentinel), Ratio = 0, PrecomputedValue = 0.
func reservedTierRatios() []t3spec.TierRatio {
	out := make([]t3spec.TierRatio, corespec.TierCount)
	for i := range corespec.TierCount {
		out[i] = t3spec.TierRatio{
			BoundaryValue:    new(big.Int).Set(maxTierBoundary),
			Ratio:            0,
			PrecomputedValue: new(big.Int).SetUint64(0),
		}
	}
	return out
}
