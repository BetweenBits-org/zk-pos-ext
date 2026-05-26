// Package host contains off-circuit (native) helpers specific to the
// spot_simple solvency model. Pairs with the sibling spec/ (data
// shapes + interfaces) and circuit/ (in-circuit constraints)
// packages: spec is the contract, circuit is the in-circuit emitter,
// host is the off-circuit emitter that produces byte-equivalent
// commitments for verifier / userproof / witness builders.
//
// Universal off-circuit helpers (e.g. Merkle proof verification) live
// at zkpor/core/host. Anything spot_simple-specific in layout
// (the 1-tuple per-asset record, the 1-field-per-asset CEX packing)
// belongs here.
package host

import (
	"fmt"
	"math/big"

	spotspec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/spot_simple/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// twoToThe64 == 2^64. Positional weight of TotalEquity in the
// {TotalEquity * 2^64 + BasePrice} packing used by the per-asset CEX
// commitment.
var twoToThe64 = new(big.Int).Lsh(big.NewInt(1), 64)

// ComputeUserAssetsCommitment returns the Poseidon commitment of one
// user's per-asset 1-tuple sequence (Index, Equity), padded to the
// smallest assetCountTiers entry that fits len(assets). Missing slots
// are filled with zero-Equity entries at synthetic indices, so the
// asset-index sequence is strictly increasing and dense up to the tier.
//
// assetCountTiers MUST be sorted ascending and contain at least one
// tier >= len(assets); the smallest such tier is selected.
//
// Byte-equivalent to the in-circuit
// `corecircuit.ComputeFlatUint64Commitment` over the same flat
// sequence — circuit and host paths produce identical commitment
// bytes for identical inputs.
func ComputeUserAssetsCommitment(assets []spotspec.AccountAsset, assetCountTiers []int) []byte {
	flat := paddingAccountAssets(assets, assetCountTiers)
	const fieldsPerAsset = 2
	const valsPerField = 3

	targetCounts := getAssetsCountOfUser(assets, assetCountTiers)
	totalUint64s := targetCounts * fieldsPerAsset
	nEles := (totalUint64s + valsPerField - 1) / valsPerField

	hasher := poseidon.NewPoseidon()
	tmp := new(big.Int)
	a, b, c := new(big.Int), new(big.Int), new(big.Int)
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
		// sum = a * 2^128 + b * 2^64 + c (matches circuit's packing)
		tmp.Lsh(a, 128)
		tmp.Add(tmp, new(big.Int).Lsh(b, 64))
		tmp.Add(tmp, c)
		hasher.Write(tmp.Bytes())
	}
	return hasher.Sum(nil)
}

// ComputeCexAssetsCommitment returns the Poseidon commitment over the
// global per-asset state, padded to `capacity` slots. Each slot packs
// {TotalEquity * 2^64 + BasePrice} into one 128-bit field element —
// matches the in-circuit fillCexAssetCommitment in the spot_simple
// circuit package.
//
// `capacity` is the per-deployment asset capacity (the value the
// trusted setup ceremony baked in). Caller MAY supply cexAssetsInfo
// shorter than capacity; helper pads with reserved entries
// (TotalEquity=0, BasePrice=0). Panics if len(cexAssetsInfo) > capacity.
func ComputeCexAssetsCommitment(cexAssetsInfo []spotspec.CexAssetInfo, capacity int) []byte {
	if len(cexAssetsInfo) > capacity {
		panic(fmt.Sprintf("ComputeCexAssetsCommitment: %d entries exceeds capacity %d",
			len(cexAssetsInfo), capacity))
	}
	hasher := poseidon.NewPoseidon()
	tmp := new(big.Int)
	eq := new(big.Int)
	for i := range capacity {
		if i < len(cexAssetsInfo) {
			eq.SetUint64(cexAssetsInfo[i].TotalEquity)
			tmp.Mul(eq, twoToThe64)
			tmp.Add(tmp, new(big.Int).SetUint64(cexAssetsInfo[i].BasePrice))
		} else {
			tmp.SetInt64(0)
		}
		hasher.Write(tmp.Bytes())
	}
	return hasher.Sum(nil)
}

// getAssetsCountOfUser returns the smallest tier in assetCountTiers
// that is >= len(assets). Mirrors the tier_3bucket helper.
func getAssetsCountOfUser(assets []spotspec.AccountAsset, assetCountTiers []int) int {
	count := len(assets)
	for _, v := range assetCountTiers {
		if count <= v {
			return v
		}
	}
	return 0
}

// paddingAccountAssets converts a user's per-asset records into the
// flat uint64 sequence consumed by ComputeUserAssetsCommitment:
// {AssetIndex, Equity} per asset, with padding entries inserted at
// synthetic indices in the gaps between real entries so the asset
// index sequence is strictly increasing and dense up to targetCounts.
// Panics if len(assets) > targetCounts.
func paddingAccountAssets(assets []spotspec.AccountAsset, assetCountTiers []int) []uint64 {
	targetCounts := getAssetsCountOfUser(assets, assetCountTiers)
	if targetCounts < len(assets) {
		panic("the target counts is less than the length of assets")
	}
	const fieldsPerAsset = 2
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
				// Equity stays zero
				index++
				if currentPaddingCounts >= paddingCounts {
					break
				}
			}
		}
		out[index*fieldsPerAsset] = uint64(assets[i].Index)
		out[index*fieldsPerAsset+1] = assets[i].Equity
		index++
		currentAssetIndex = int(assets[i].Index) + 1
	}
	for i := index; i < targetCounts; i++ {
		out[i*fieldsPerAsset] = uint64(currentAssetIndex)
		currentAssetIndex++
	}
	return out
}
