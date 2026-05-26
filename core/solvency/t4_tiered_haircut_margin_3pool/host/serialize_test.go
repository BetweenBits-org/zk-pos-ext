package host_test

import (
	"bytes"
	"math/big"
	"testing"

	t4host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host"
	t4spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// roundTripCapacity locks the witness's per-deployment capacity for
// the round-trip tests. The decoder self-infers capacity from
// len(BeforeCexAssets), so the test must pad BeforeCexAssets up to
// this value to exercise the dense Assets expansion the prover sees.
const roundTripCapacity = 500

// TestEncodeDecodeBatchWitness_RoundTrip exercises the witness on-wire
// format end-to-end: encode, decode, verify the round-trip preserves
// every header field and that CreateUserOps[*].Assets is expanded back
// to a dense capacity-length slice (the prover's expectation, with
// capacity inferred from len(w.BeforeCexAssets)).
func TestEncodeDecodeBatchWitness_RoundTrip(t *testing.T) {
	beforeCex := make([]t4spec.CexAssetInfo, roundTripCapacity)
	beforeCex[0] = t4spec.CexAssetInfo{
		TotalEquity: 100, TotalDebt: 50, BasePrice: 7_000_000,
		Symbol: "BTC", Index: 0,
		LoanCollateral: 30, MarginCollateral: 10, PortfolioMarginCollateral: 5,
		LoanRatios:            tierRatiosOf(11),
		MarginRatios:          tierRatiosOf(22),
		PortfolioMarginRatios: tierRatiosOf(33),
	}
	src := &t4spec.BatchCreateUserWitness{
		BatchCommitment:           bytesPattern(0x01, 32),
		BeforeAccountTreeRoot:     bytesPattern(0x02, 32),
		AfterAccountTreeRoot:      bytesPattern(0x03, 32),
		BeforeCEXAssetsCommitment: bytesPattern(0x04, 32),
		AfterCEXAssetsCommitment:  bytesPattern(0x05, 32),
		BeforeCexAssets:           beforeCex,
		CreateUserOps: []t4spec.CreateUserOperation{
			{
				BeforeAccountTreeRoot: bytesPattern(0x10, 32),
				AfterAccountTreeRoot:  bytesPattern(0x11, 32),
				Assets: []t4spec.AccountAsset{
					{Index: 0, Equity: 100, Debt: 50},
					{Index: 7, Loan: 5, PortfolioMargin: 1},
				},
				AccountIndex:  42,
				AccountIDHash: bytesPattern(0x20, 32),
			},
		},
	}

	encoded, err := t4host.EncodeBatchWitness(src)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	dst, err := t4host.DecodeBatchWitness(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Header round-trip
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
	// Assets must be expanded to dense capacity length (inferred from
	// len(BeforeCexAssets) by the decoder).
	if len(dst.CreateUserOps[0].Assets) != roundTripCapacity {
		t.Fatalf("dense Assets length = %d, want %d", len(dst.CreateUserOps[0].Assets), roundTripCapacity)
	}
	if dst.CreateUserOps[0].Assets[0].Equity != 100 || dst.CreateUserOps[0].Assets[0].Debt != 50 {
		t.Fatalf("dense Assets[0] = %+v, want (Equity=100, Debt=50)", dst.CreateUserOps[0].Assets[0])
	}
	if dst.CreateUserOps[0].Assets[7].Loan != 5 || dst.CreateUserOps[0].Assets[7].PortfolioMargin != 1 {
		t.Fatalf("dense Assets[7] = %+v, want (Loan=5, PortfolioMargin=1)", dst.CreateUserOps[0].Assets[7])
	}
	// Padding slot stays zero-balance at its synthetic index.
	if dst.CreateUserOps[0].Assets[3].Index != 3 || dst.CreateUserOps[0].Assets[3].Equity != 0 {
		t.Fatalf("padding Assets[3] = %+v, want index=3 zero balance", dst.CreateUserOps[0].Assets[3])
	}
}

func bytesPattern(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func tierRatiosOf(seed uint8) []t4spec.TierRatio {
	out := make([]t4spec.TierRatio, corespec.TierCount)
	for i := range corespec.TierCount {
		out[i] = t4spec.TierRatio{
			BoundaryValue:    new(big.Int).Mul(big.NewInt(int64(i+1)), big.NewInt(corespec.DefaultValueScale)),
			Ratio:            uint8(int(seed) + 80 - i),
			PrecomputedValue: new(big.Int).SetUint64(0),
		}
	}
	return out
}
