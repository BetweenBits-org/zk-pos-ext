// Package host contains off-circuit (native) helpers specific to the
// t4_tiered_haircut_margin_3pool solvency model. Pairs with the sibling spec/ (data
// shapes + interfaces) and circuit/ (in-circuit constraints)
// packages: spec is the contract, circuit is the in-circuit emitter,
// host is the off-circuit emitter that produces byte-equivalent
// commitments for verifier / userproof / witness builders.
//
// Universal off-circuit helpers (e.g. Merkle proof verification) live
// at zkpor/core/host. Anything model-specific in layout (the
// t4_tiered_haircut_margin_3pool 6-tuple, the 3-bucket commitment packing, the tier
// table padding) belongs here.
package host

import (
	"fmt"
	"math/big"

	t4spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// Native byte-packing constants. Pinned by the trusted-setup contract:
// changing any of these forks .vk and invalidates published proofs.
var (
	// twoToThe64 == 2^64. Positional weight of the middle uint64 in the
	// 192-bit packing {high * 2^128 + mid * 2^64 + low}.
	twoToThe64 = new(big.Int).Lsh(big.NewInt(1), 64)
	// twoToThe128 == 2^128. Positional weight of the high uint64.
	twoToThe128 = new(big.Int).Lsh(big.NewInt(1), 128)
	// twoToThe8 == 2^8. Positional weight of BoundaryValue in the tier
	// packing {ratio + boundaryValue * 2^8}.
	twoToThe8 = new(big.Int).Lsh(big.NewInt(1), 8)
	// twoToThe126 == 2^126. Positional weight of the second tier's ratio.
	twoToThe126 = new(big.Int).Lsh(big.NewInt(1), 126)
	// twoToThe134 == 2^134. Positional weight of the second tier's
	// boundary value (8 bits of ratio + 126 bits of boundary = 134).
	twoToThe134 = new(big.Int).Lsh(big.NewInt(1), 134)
	// maxTierBoundary == 2^118. Reserved-tier and padding-tier boundary
	// value (mirrors legacy MaxTierBoundaryValue).
	maxTierBoundary = new(big.Int).Lsh(big.NewInt(1), 118)
)

// ComputeUserAssetsCommitment returns the Poseidon commitment of one
// user's per-asset 5-tuple sequence, packed and padded as the circuit
// expects. assetCountTiers MUST be sorted ascending and contain at
// least one tier >= len(assets); the smallest such tier is selected
// and missing asset slots are filled with zero entries at synthetic
// indices.
//
// Byte-equivalent to legacy src/utils/utils.go ComputeUserAssetsCommitment.
// The layout (6 fields per asset: Index, Equity, Debt, Loan, Margin,
// PortfolioMargin; pack 3 uint64s per 192-bit field element) is part
// of the t4_tiered_haircut_margin_3pool trusted-setup contract — changing it forks .vk.
func ComputeUserAssetsCommitment(assets []t4spec.AccountAsset, assetCountTiers []int) []byte {
	hasher := poseidon.NewPoseidon()
	flat := paddingAccountAssets(assets, assetCountTiers)
	const fieldsPerAsset = 6
	const valsPerField = 3
	targetCounts := getAssetsCountOfUser(assets, assetCountTiers)
	nEles := (targetCounts*fieldsPerAsset + 2) / valsPerField

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
// global per-asset state, padded to `capacity` slots. cexAssetsInfo
// MAY be shorter than capacity; the helper appends "reserved" entries
// (BasePrice=0, all collateral 0, max-boundary tier ratios) so the
// commitment shape is constant for a given trusted setup.
//
// `capacity` is the per-deployment asset capacity — typically the
// profile's AssetCatalog.Capacity() value. It is part of the trusted
// setup contract: keygen and witness MUST use the same capacity, and
// changing it forks .vk. Panics if len(cexAssetsInfo) > capacity.
//
// Each entry's LoanRatios / MarginRatios / PortfolioMarginRatios slice
// MUST already have length corespec.TierCount — the snapshot adapter
// is responsible for that padding. Byte-equivalent to legacy
// src/utils/utils.go ComputeCexAssetsCommitment at capacity ==
// legacyutils.AssetCounts (500).
func ComputeCexAssetsCommitment(cexAssetsInfo []t4spec.CexAssetInfo, capacity int) []byte {
	if len(cexAssetsInfo) > capacity {
		panic(fmt.Sprintf("ComputeCexAssetsCommitment: %d entries exceeds capacity %d",
			len(cexAssetsInfo), capacity))
	}
	hasher := poseidon.NewPoseidon()
	padded := make([]t4spec.CexAssetInfo, len(cexAssetsInfo), capacity)
	copy(padded, cexAssetsInfo)
	for i := len(cexAssetsInfo); i < capacity; i++ {
		padded = append(padded, t4spec.CexAssetInfo{
			Symbol:                "reserved",
			Index:                 uint32(i),
			LoanRatios:            reservedTierRatios(),
			MarginRatios:          reservedTierRatios(),
			PortfolioMarginRatios: reservedTierRatios(),
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
// that is >= len(assets). Mirrors legacy GetAssetsCountOfUser; assumes
// the caller has guaranteed assetCountTiers is sorted ascending and
// has at least one element large enough.
func getAssetsCountOfUser(assets []t4spec.AccountAsset, assetCountTiers []int) int {
	count := len(assets)
	for _, v := range assetCountTiers {
		if count <= v {
			return v
		}
	}
	return 0
}

// paddingAccountAssets converts a user's per-asset records into the
// flat uint64 sequence consumed by ComputeUserAssetsCommitment.
// Padding entries are inserted at synthetic asset indices in the gaps
// between real entries (and after the last real entry) so the asset
// index sequence is strictly increasing and dense up to targetCounts.
// Panics if len(assets) > targetCounts.
func paddingAccountAssets(assets []t4spec.AccountAsset, assetCountTiers []int) []uint64 {
	targetCounts := getAssetsCountOfUser(assets, assetCountTiers)
	if targetCounts < len(assets) {
		panic("the target counts is less than the length of assets")
	}
	const fieldsPerAsset = 6
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
		out[index*fieldsPerAsset+3] = assets[i].Loan
		out[index*fieldsPerAsset+4] = assets[i].Margin
		out[index*fieldsPerAsset+5] = assets[i].PortfolioMargin
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
// sequence the Poseidon hasher absorbs. Two 192-bit packings
// (Equity/Debt/BasePrice and the three collateral fields) plus
// TierCount/2 packings per bucket (3 buckets). Mirrors legacy
// ConvertAssetInfoToBytes for CexAssetInfo.
func convertAssetInfoToBytes(a t4spec.CexAssetInfo) [][]byte {
	res := make([][]byte, 0, 2+3*(corespec.TierCount/2))

	// Pack 1: Equity * 2^128 + Debt * 2^64 + BasePrice
	res = append(res, packThreeUint64(a.TotalEquity, a.TotalDebt, a.BasePrice))
	// Pack 2: LoanCollateral * 2^128 + MarginCollateral * 2^64 + PMCollateral
	res = append(res, packThreeUint64(a.LoanCollateral, a.MarginCollateral, a.PortfolioMarginCollateral))

	res = append(res, convertTierRatiosToBytes(a.LoanRatios)...)
	res = append(res, convertTierRatiosToBytes(a.MarginRatios)...)
	res = append(res, convertTierRatiosToBytes(a.PortfolioMarginRatios)...)
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
// into one 252-bit field-element-sized chunk per pair:
//
//	chunk = ratio[i] + boundary[i]*2^8 + ratio[i+1]*2^126 + boundary[i+1]*2^134
//
// Requires len(ratios) to be even (corespec.TierCount enforces this).
func convertTierRatiosToBytes(ratios []t4spec.TierRatio) [][]byte {
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
func reservedTierRatios() []t4spec.TierRatio {
	out := make([]t4spec.TierRatio, corespec.TierCount)
	for i := range corespec.TierCount {
		out[i] = t4spec.TierRatio{
			BoundaryValue:    new(big.Int).Set(maxTierBoundary),
			Ratio:            0,
			PrecomputedValue: new(big.Int).SetUint64(0),
		}
	}
	return out
}
