package host_test

import (
	"bytes"
	"hash"
	"math/big"
	"testing"

	legacyutils "github.com/binance/zkmerkle-proof-of-solvency/src/utils"
	tier3host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/host"
	tier3spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// TestAccountLeafHash_LegacyParity confirms the per-account SMT leaf
// hash produced by the host helper is byte-equal to legacy
// utils.AccountInfoToHash for the same input. Locks in trusted-setup
// byte compatibility of the witness/userproof leaf path.
func TestAccountLeafHash_LegacyParity(t *testing.T) {
	specAccount := &tier3spec.AccountInfo{
		AccountIndex:    7,
		AccountID:       []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10},
		TotalEquity:     new(big.Int).SetUint64(1_000_000_000),
		TotalDebt:       new(big.Int).SetUint64(250_000_000),
		TotalCollateral: new(big.Int).SetUint64(500_000_000),
		Assets: []tier3spec.AccountAsset{
			{Index: 0, Equity: 100, Debt: 30, Loan: 20, Margin: 0, PortfolioMargin: 10},
			{Index: 5, Equity: 5_000_000, Debt: 1_234_567, Loan: 800_000, Margin: 100, PortfolioMargin: 50},
		},
	}
	legacyAccount := &legacyutils.AccountInfo{
		AccountIndex:    specAccount.AccountIndex,
		AccountId:       specAccount.AccountID,
		TotalEquity:     specAccount.TotalEquity,
		TotalDebt:       specAccount.TotalDebt,
		TotalCollateral: specAccount.TotalCollateral,
		Assets:          asLegacyAssets(specAccount.Assets),
	}
	assetCountTiers := append([]int(nil), legacyutils.AssetCountsTiers...)

	got := tier3host.AccountLeafHash(specAccount, assetCountTiers)
	var legacyHasher hash.Hash = poseidon.NewPoseidon()
	want := legacyutils.AccountInfoToHash(legacyAccount, &legacyHasher)

	if !bytes.Equal(got, want) {
		t.Fatalf("AccountLeafHash byte mismatch:\n  host   = %x\n  legacy = %x", got, want)
	}
}

// TestPaddingAccounts_LegacyParity confirms the witness builder's
// per-tier padding (assetKey + paddingStartIndex semantics) matches
// legacy utils.PaddingAccounts exactly: same final length, same
// AccountIndex sequence, same zero-balance shape.
func TestPaddingAccounts_LegacyParity(t *testing.T) {
	const (
		assetKey         = 50
		usersPerBatch    = 700
		paddingStartFrom = 1234
	)
	specAccounts := []tier3spec.AccountInfo{
		{
			AccountIndex:    1,
			AccountID:       []byte{0xa},
			TotalEquity:     new(big.Int).SetUint64(100),
			TotalDebt:       new(big.Int).SetUint64(50),
			TotalCollateral: new(big.Int).SetUint64(60),
			Assets:          []tier3spec.AccountAsset{{Index: 0, Equity: 10}},
		},
		{
			AccountIndex:    2,
			AccountID:       []byte{0xb},
			TotalEquity:     new(big.Int).SetUint64(200),
			TotalDebt:       new(big.Int).SetUint64(100),
			TotalCollateral: new(big.Int).SetUint64(110),
			Assets:          []tier3spec.AccountAsset{{Index: 3, Equity: 20, Loan: 5}},
		},
	}
	legacyAccounts := []legacyutils.AccountInfo{
		{
			AccountIndex:    specAccounts[0].AccountIndex,
			AccountId:       specAccounts[0].AccountID,
			TotalEquity:     specAccounts[0].TotalEquity,
			TotalDebt:       specAccounts[0].TotalDebt,
			TotalCollateral: specAccounts[0].TotalCollateral,
			Assets:          asLegacyAssets(specAccounts[0].Assets),
		},
		{
			AccountIndex:    specAccounts[1].AccountIndex,
			AccountId:       specAccounts[1].AccountID,
			TotalEquity:     specAccounts[1].TotalEquity,
			TotalDebt:       specAccounts[1].TotalDebt,
			TotalCollateral: specAccounts[1].TotalCollateral,
			Assets:          asLegacyAssets(specAccounts[1].Assets),
		},
	}

	hostNext, hostPadded := tier3host.PaddingAccounts(specAccounts, assetKey, paddingStartFrom, usersPerBatch)
	legacyNext, legacyPadded := legacyutils.PaddingAccounts(legacyAccounts, assetKey, paddingStartFrom)

	if hostNext != legacyNext {
		t.Fatalf("next index host=%d legacy=%d", hostNext, legacyNext)
	}
	if len(hostPadded) != len(legacyPadded) {
		t.Fatalf("padded length host=%d legacy=%d", len(hostPadded), len(legacyPadded))
	}
	for i := range hostPadded {
		if hostPadded[i].AccountIndex != legacyPadded[i].AccountIndex {
			t.Fatalf("padded[%d].AccountIndex host=%d legacy=%d", i, hostPadded[i].AccountIndex, legacyPadded[i].AccountIndex)
		}
		if len(hostPadded[i].Assets) != len(legacyPadded[i].Assets) {
			t.Fatalf("padded[%d].Assets length host=%d legacy=%d", i, len(hostPadded[i].Assets), len(legacyPadded[i].Assets))
		}
	}
}

func asLegacyAssets(in []tier3spec.AccountAsset) []legacyutils.AccountAsset {
	out := make([]legacyutils.AccountAsset, len(in))
	for i, a := range in {
		out[i] = legacyutils.AccountAsset{
			Index: a.Index, Equity: a.Equity, Debt: a.Debt, Loan: a.Loan,
			Margin: a.Margin, PortfolioMargin: a.PortfolioMargin,
		}
	}
	return out
}
