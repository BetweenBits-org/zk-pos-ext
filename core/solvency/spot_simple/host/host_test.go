package host_test

import (
	"bytes"
	"math/big"
	"testing"

	spothost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/spot_simple/host"
	spotspec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/spot_simple/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// TestEncodeDecodeBatchWitness_RoundTrip exercises the witness on-wire
// format end-to-end: encode, decode, verify the round-trip preserves
// every header field and that CreateUserOps[*].Assets is expanded back
// to a dense capacity-length slice (capacity inferred from
// len(BeforeCexAssets) per the spot decoder).
func TestEncodeDecodeBatchWitness_RoundTrip(t *testing.T) {
	const capacity = 8
	beforeCex := make([]spotspec.CexAssetInfo, capacity)
	beforeCex[0] = spotspec.CexAssetInfo{
		TotalEquity: 100, BasePrice: 7_000_000, Symbol: "BTC", Index: 0,
	}
	src := &spotspec.BatchCreateUserWitness{
		BatchCommitment:           bytesPattern(0x01, 32),
		BeforeAccountTreeRoot:     bytesPattern(0x02, 32),
		AfterAccountTreeRoot:      bytesPattern(0x03, 32),
		BeforeCEXAssetsCommitment: bytesPattern(0x04, 32),
		AfterCEXAssetsCommitment:  bytesPattern(0x05, 32),
		BeforeCexAssets:           beforeCex,
		CreateUserOps: []spotspec.CreateUserOperation{
			{
				BeforeAccountTreeRoot: bytesPattern(0x10, 32),
				AfterAccountTreeRoot:  bytesPattern(0x11, 32),
				Assets: []spotspec.AccountAsset{
					{Index: 0, Equity: 100},
					{Index: 5, Equity: 7},
				},
				AccountIndex:  42,
				AccountIDHash: bytesPattern(0x20, 32),
			},
		},
	}

	encoded, err := spothost.EncodeBatchWitness(src)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	dst, err := spothost.DecodeBatchWitness(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !bytes.Equal(dst.BatchCommitment, src.BatchCommitment) ||
		!bytes.Equal(dst.BeforeAccountTreeRoot, src.BeforeAccountTreeRoot) ||
		!bytes.Equal(dst.AfterAccountTreeRoot, src.AfterAccountTreeRoot) ||
		!bytes.Equal(dst.BeforeCEXAssetsCommitment, src.BeforeCEXAssetsCommitment) ||
		!bytes.Equal(dst.AfterCEXAssetsCommitment, src.AfterCEXAssetsCommitment) {
		t.Fatalf("header field mismatch after round-trip")
	}
	if len(dst.CreateUserOps) != 1 {
		t.Fatalf("CreateUserOps length = %d, want 1", len(dst.CreateUserOps))
	}
	if len(dst.CreateUserOps[0].Assets) != capacity {
		t.Fatalf("dense Assets length = %d, want %d", len(dst.CreateUserOps[0].Assets), capacity)
	}
	if dst.CreateUserOps[0].Assets[0].Equity != 100 {
		t.Fatalf("dense Assets[0].Equity = %d, want 100", dst.CreateUserOps[0].Assets[0].Equity)
	}
	if dst.CreateUserOps[0].Assets[5].Equity != 7 {
		t.Fatalf("dense Assets[5].Equity = %d, want 7", dst.CreateUserOps[0].Assets[5].Equity)
	}
	if dst.CreateUserOps[0].Assets[3].Index != 3 || dst.CreateUserOps[0].Assets[3].Equity != 0 {
		t.Fatalf("padding Assets[3] = %+v, want index=3 zero equity", dst.CreateUserOps[0].Assets[3])
	}
}

// TestComputeCexAssetsCommitment_Deterministic asserts the helper
// produces stable, non-empty output and is order-sensitive in inputs.
// Without a legacy reference, byte-equivalence is checked against
// re-computation on the same input.
func TestComputeCexAssetsCommitment_Deterministic(t *testing.T) {
	assets := []spotspec.CexAssetInfo{
		{TotalEquity: 1_000_000_000_000, BasePrice: 6_500_000_000_000, Symbol: "BTC", Index: 0},
		{TotalEquity: 5_000_000_000_000, BasePrice: 350_000_000_000, Symbol: "ETH", Index: 1},
	}
	const capacity = 5
	h1 := spothost.ComputeCexAssetsCommitment(assets, capacity)
	h2 := spothost.ComputeCexAssetsCommitment(assets, capacity)
	if !bytes.Equal(h1, h2) {
		t.Fatalf("not deterministic: %x vs %x", h1, h2)
	}
	if len(h1) == 0 {
		t.Fatal("empty commitment output")
	}

	// Swap order → different commitment.
	swapped := []spotspec.CexAssetInfo{assets[1], assets[0]}
	hSwap := spothost.ComputeCexAssetsCommitment(swapped, capacity)
	if bytes.Equal(h1, hSwap) {
		t.Fatal("commitment order-invariant — should be order-sensitive")
	}
}

// TestComputeCexAssetsCommitment_PadsToCapacity locks in the contract
// that the helper extends shorter inputs with zero entries up to
// capacity. Two inputs that are equal modulo trailing zeros up to
// capacity must hash to the same value.
func TestComputeCexAssetsCommitment_PadsToCapacity(t *testing.T) {
	a := []spotspec.CexAssetInfo{{TotalEquity: 7, BasePrice: 11, Symbol: "X", Index: 0}}
	b := []spotspec.CexAssetInfo{
		{TotalEquity: 7, BasePrice: 11, Symbol: "X", Index: 0},
		{TotalEquity: 0, BasePrice: 0, Symbol: "reserved", Index: 1},
		{TotalEquity: 0, BasePrice: 0, Symbol: "reserved", Index: 2},
	}
	const capacity = 3
	ha := spothost.ComputeCexAssetsCommitment(a, capacity)
	hb := spothost.ComputeCexAssetsCommitment(b, capacity)
	if !bytes.Equal(ha, hb) {
		t.Fatalf("padding-equivalent inputs hashed differently: %x vs %x", ha, hb)
	}
}

// TestComputeCexAssetsCommitment_RejectsOverCapacity exercises the
// guard against inputs longer than capacity — should panic.
func TestComputeCexAssetsCommitment_RejectsOverCapacity(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when len(input) > capacity")
		}
	}()
	too_long := make([]spotspec.CexAssetInfo, 5)
	spothost.ComputeCexAssetsCommitment(too_long, 3)
}

// TestAccountLeafHash_Deterministic asserts the leaf hash is stable
// for the same input and changes when any tracked field changes.
func TestAccountLeafHash_Deterministic(t *testing.T) {
	// Production AccountIDs are always fr.Element-reduced (snapshot
	// adapter does the round-trip). Test fixtures must satisfy the
	// same invariant — PoseidonBytes panics on ≥ modulus inputs.
	account := &spotspec.AccountInfo{
		AccountID:   modSafe(bytesPattern(0x55, 32)),
		TotalEquity: big.NewInt(123_456),
		Assets: []spotspec.AccountAsset{
			{Index: 0, Equity: 100},
			{Index: 3, Equity: 50},
		},
	}
	tiers := []int{5}

	h1 := spothost.AccountLeafHash(account, tiers)
	h2 := spothost.AccountLeafHash(account, tiers)
	if !bytes.Equal(h1, h2) {
		t.Fatalf("not deterministic: %x vs %x", h1, h2)
	}

	// Change TotalEquity → different hash.
	account2 := *account
	account2.TotalEquity = big.NewInt(999_999)
	h3 := spothost.AccountLeafHash(&account2, tiers)
	if bytes.Equal(h1, h3) {
		t.Fatal("hash invariant under TotalEquity change — should depend on it")
	}

	// Change one Asset equity → different hash.
	account3 := *account
	assets := make([]spotspec.AccountAsset, len(account.Assets))
	copy(assets, account.Assets)
	assets[0].Equity = 200
	account3.Assets = assets
	h4 := spothost.AccountLeafHash(&account3, tiers)
	if bytes.Equal(h1, h4) {
		t.Fatal("hash invariant under per-asset Equity change")
	}
}

// TestPaddingAccounts_RoundsUpToBatch verifies the padding extends an
// odd-length account slice to a whole-number-of-batches multiple, with
// padding rows carrying zero balances and assetKey synthetic Assets.
func TestPaddingAccounts_RoundsUpToBatch(t *testing.T) {
	original := []spotspec.AccountInfo{
		{AccountIndex: 0, TotalEquity: big.NewInt(10)},
		{AccountIndex: 1, TotalEquity: big.NewInt(20)},
		{AccountIndex: 2, TotalEquity: big.NewInt(30)},
	}
	const (
		assetKey          = 5
		paddingStartIndex = 100
		usersPerBatch     = 4
	)
	newStart, padded := spothost.PaddingAccounts(original, assetKey, paddingStartIndex, usersPerBatch)
	if len(padded) != 4 {
		t.Fatalf("padded length = %d, want 4 (one full batch)", len(padded))
	}
	if newStart != 101 {
		t.Fatalf("paddingStartIndex returned = %d, want 101 (advanced by 1)", newStart)
	}
	pad := padded[3]
	if pad.AccountIndex != 100 {
		t.Fatalf("padding row AccountIndex = %d, want 100", pad.AccountIndex)
	}
	if pad.TotalEquity.Sign() != 0 {
		t.Fatalf("padding row TotalEquity = %s, want 0", pad.TotalEquity)
	}
	if len(pad.Assets) != assetKey {
		t.Fatalf("padding row Assets length = %d, want %d", len(pad.Assets), assetKey)
	}
	for i, a := range pad.Assets {
		if a.Index != uint16(i) || a.Equity != 0 {
			t.Fatalf("padding row Assets[%d] = %+v, want index=%d zero equity", i, a, i)
		}
	}
}

func bytesPattern(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

// modSafe reduces a 32-byte slice modulo the bn254 fr modulus —
// matches the snapshot adapter's AccountID normalisation so test
// fixtures stay below the PoseidonBytes panic threshold.
func modSafe(b []byte) []byte {
	return new(fr.Element).SetBytes(b).Marshal()
}
