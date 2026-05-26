package main

import (
	"context"
	"testing"

	t4spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/binance"
)

// TestTiersFromShapes locks the binance deployment's tier view at
// {50, 500} so the witness's bucketing assignment can't drift away
// from the SetBatchCreateUserCircuitWitness padding the prover does
// downstream.
func TestTiersFromShapes(t *testing.T) {
	got := tiersFromShapes(binance.NewBatchShape().Shapes())
	want := []int{50, 500}
	if len(got) != len(want) {
		t.Fatalf("tiers length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tiers[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

// TestPricingValueScaleInvariant_DefaultPath confirms the G6 assert
// the witness runs on startup holds for the binance pricing default
// path (no-symbol → 1e8 × 1e8 == 1e16).
func TestPricingValueScaleInvariant_DefaultPath(t *testing.T) {
	p := binance.NewPricing()
	if got := p.PriceMultiplier("") * p.BalanceMultiplier(""); got != p.ValueScale() {
		t.Fatalf("invariant violated: %d × %d = %d, want ValueScale=%d",
			p.PriceMultiplier(""), p.BalanceMultiplier(""), got, p.ValueScale())
	}
}

// TestStreamAndBucket_RoutesAccountsByNonEmptyCount confirms accounts
// are bucketed by their non-empty asset count, not their slice length:
// an account with 50 slots but only 3 non-empty assets lands in the
// 50-tier bucket (smallest tier ≥ 3).
func TestStreamAndBucket_RoutesAccountsByNonEmptyCount(t *testing.T) {
	tiers := []int{50, 500}
	src := &fakeSnapshot{accounts: []t4spec.AccountInfo{
		// 3 non-empty assets → tier 50
		{AccountIndex: 0, Assets: padAssets([]t4spec.AccountAsset{
			{Index: 0, Equity: 10},
			{Index: 5, Loan: 1},
			{Index: 8, Margin: 2},
		}, 50)},
		// 51 non-empty assets → tier 500
		{AccountIndex: 1, Assets: nonEmpty(51, 500)},
	}}

	got := streamAndBucket(context.Background(), src, tiers)
	if len(got[50]) != 1 {
		t.Fatalf("tier 50 bucket length = %d, want 1", len(got[50]))
	}
	if len(got[500]) != 1 {
		t.Fatalf("tier 500 bucket length = %d, want 1", len(got[500]))
	}
}

type fakeSnapshot struct {
	accounts []t4spec.AccountInfo
}

func (f *fakeSnapshot) CexAssets(context.Context) ([]t4spec.CexAssetInfo, error) {
	return nil, nil
}
func (f *fakeSnapshot) AccountStream(context.Context) (<-chan t4spec.AccountInfo, error) {
	ch := make(chan t4spec.AccountInfo, len(f.accounts))
	for _, a := range f.accounts {
		ch <- a
	}
	close(ch)
	return ch, nil
}
func (f *fakeSnapshot) InvalidCount() uint64 { return 0 }
func (f *fakeSnapshot) SnapshotID() string   { return "fake" }

// padAssets returns a tier-sized slice with non-zero entries at the
// indices in `nonEmpty` and zero entries at every other slot up to
// totalLen.
func padAssets(nonEmpty []t4spec.AccountAsset, totalLen int) []t4spec.AccountAsset {
	out := make([]t4spec.AccountAsset, totalLen)
	for i := range totalLen {
		out[i] = t4spec.AccountAsset{Index: uint16(i)}
	}
	for _, a := range nonEmpty {
		out[a.Index] = a
	}
	return out
}

// nonEmpty returns a totalLen-sized slice with the first `count`
// entries carrying a non-zero balance.
func nonEmpty(count, totalLen int) []t4spec.AccountAsset {
	out := make([]t4spec.AccountAsset, totalLen)
	for i := range totalLen {
		out[i] = t4spec.AccountAsset{Index: uint16(i)}
	}
	for i := range count {
		out[i].Equity = 1
	}
	return out
}
