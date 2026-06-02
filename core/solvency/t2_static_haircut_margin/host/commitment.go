// Package host contains off-circuit (native) helpers specific to the
// t2_static_haircut_margin solvency model. Structural simplification
// of t3_tiered_haircut_margin_1pool/host — the piecewise-linear tier
// curve is collapsed to a single Haircut basis-points constant per
// asset (no tier table).
package host

import (
	"fmt"
	"math/big"

	t2spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t2_static_haircut_margin/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// Native byte-packing constants.
var (
	twoToThe16  = new(big.Int).Lsh(big.NewInt(1), 16)
	twoToThe64  = new(big.Int).Lsh(big.NewInt(1), 64)
	twoToThe128 = new(big.Int).Lsh(big.NewInt(1), 128)
)

// ComputeUserAssetsCommitment returns the Poseidon commitment of one
// user's per-asset 4-tuple sequence (Index, Equity, Debt, Collateral),
// packed 3 uint64s per field element. Same layout as T3.
func ComputeUserAssetsCommitment(assets []t2spec.AccountAsset, assetCountTiers []int) []byte {
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
		tmp.Mul(a, twoToThe128)
		tmp.Add(tmp, new(big.Int).Mul(b, twoToThe64))
		tmp.Add(tmp, c)
		hasher.Write(tmp.Bytes())
	}
	return hasher.Sum(nil)
}

// ComputeCexAssetsCommitment returns the Poseidon commitment over the
// global per-asset state, padded to `capacity` slots. T2 layout: 2
// field elements per asset (Equity/Debt/BasePrice packing +
// Collateral/Haircut packing).
//
// Each entry's Haircut MAY be zero (collateral not accepted). Panics
// if len(cexAssetsInfo) > capacity.
func ComputeCexAssetsCommitment(cexAssetsInfo []t2spec.CexAssetInfo, capacity int) []byte {
	if len(cexAssetsInfo) > capacity {
		panic(fmt.Sprintf("ComputeCexAssetsCommitment: %d entries exceeds capacity %d",
			len(cexAssetsInfo), capacity))
	}
	hasher := poseidon.NewPoseidon()
	padded := make([]t2spec.CexAssetInfo, len(cexAssetsInfo), capacity)
	copy(padded, cexAssetsInfo)
	for i := len(cexAssetsInfo); i < capacity; i++ {
		padded = append(padded, t2spec.CexAssetInfo{
			Symbol: "reserved",
			Index:  uint32(i),
			// Haircut = 0 (collateral not accepted in reserved slots)
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
func getAssetsCountOfUser(assets []t2spec.AccountAsset, assetCountTiers []int) int {
	count := len(assets)
	for _, v := range assetCountTiers {
		if count <= v {
			return v
		}
	}
	return 0
}

// paddingAccountAssets converts a user's per-asset records into the
// flat uint64 sequence: (Index, Equity, Debt, Collateral) per asset.
func paddingAccountAssets(assets []t2spec.AccountAsset, assetCountTiers []int) []uint64 {
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
// sequence the Poseidon hasher absorbs. T2 layout: 1 packing
// (Equity / Debt / BasePrice), then 1 packing (Collateral * 2^16 + Haircut).
func convertAssetInfoToBytes(a t2spec.CexAssetInfo) [][]byte {
	res := make([][]byte, 0, 2)
	// Pack 1: Equity * 2^128 + Debt * 2^64 + BasePrice
	res = append(res, packThreeUint64(a.TotalEquity, a.TotalDebt, a.BasePrice))
	// Pack 2: Collateral * 2^16 + Haircut
	v := new(big.Int).Mul(new(big.Int).SetUint64(a.Collateral), twoToThe16)
	v.Add(v, new(big.Int).SetUint64(uint64(a.Haircut)))
	res = append(res, v.Bytes())
	return res
}

func packThreeUint64(high, mid, low uint64) []byte {
	res := new(big.Int).Mul(new(big.Int).SetUint64(high), twoToThe128)
	res.Add(res, new(big.Int).Mul(new(big.Int).SetUint64(mid), twoToThe64))
	res.Add(res, new(big.Int).SetUint64(low))
	return res.Bytes()
}
