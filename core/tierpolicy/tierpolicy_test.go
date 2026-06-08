package tierpolicy_test

import (
	"bytes"
	"math/big"
	"testing"

	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/BetweenBits-org/zk-pos-ext/core/tierpolicy"
)

func tier(b int64, r uint8) tierpolicy.Tier {
	return tierpolicy.Tier{Boundary: big.NewInt(b), Ratio: r}
}

// haircutAt replicates the parser's haircutValue piecewise evaluation
// (core/snapshot/t3_*/parser.go) so the test can assert that a curve
// built by BuildTierCurve agrees with how the engine consumes
// precomputed_value: evaluating at boundary[i] must equal precomputed[i].
func haircutAt(value *big.Int, curve []tierpolicy.TierRatio) *big.Int {
	prevBoundary := big.NewInt(0)
	prevPrecomp := big.NewInt(0)
	for _, t := range curve {
		if value.Cmp(t.Boundary) <= 0 {
			delta := new(big.Int).Sub(value, prevBoundary)
			delta.Mul(delta, big.NewInt(int64(t.Ratio)))
			delta.Div(delta, big.NewInt(100))
			return delta.Add(delta, prevPrecomp)
		}
		prevBoundary = t.Boundary
		prevPrecomp = t.Precomputed
	}
	return prevPrecomp
}

func TestBuildTierCurve_Recipe(t *testing.T) {
	cases := []struct {
		name string
		in   []tierpolicy.Tier
		want []int64
	}{
		{"two-tier", []tierpolicy.Tier{tier(100, 50), tier(300, 10)}, []int64{50, 70}},
		{"floor-truncates", []tierpolicy.Tier{tier(3, 7)}, []int64{0}},                     // floor(21/100)=0
		{"floor-cumulative", []tierpolicy.Tier{tier(10, 33), tier(25, 33)}, []int64{3, 7}}, // 3, 3+floor(495/100)=7
		// The audited recipe gives precomputed[0] = floor(1e8*100/100) = 1e8.
		// The reference fixture core/snapshot/t3_*/parser_test.go ships
		// precomputed_value=0 for this very curve — recipe-inconsistent, yet
		// accepted because no test account exercises the field. This asserts
		// the *correct* value, documenting that gap.
		{"single-full", []tierpolicy.Tier{tier(100_000_000, 100)}, []int64{100_000_000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tierpolicy.BuildTierCurve(tc.in)
			if err != nil {
				t.Fatalf("BuildTierCurve: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tc.want))
			}
			for i, w := range tc.want {
				if got[i].Precomputed.Cmp(big.NewInt(w)) != 0 {
					t.Errorf("precomputed[%d] = %s, want %d", i, got[i].Precomputed, w)
				}
				if got[i].Boundary.Cmp(tc.in[i].Boundary) != 0 || got[i].Ratio != tc.in[i].Ratio {
					t.Errorf("tier[%d] authoritative fields mutated", i)
				}
			}
		})
	}
}

func TestBuildTierCurve_DoesNotMutateInput(t *testing.T) {
	in := []tierpolicy.Tier{tier(100, 50), tier(300, 10)}
	b0 := new(big.Int).Set(in[0].Boundary)
	if _, err := tierpolicy.BuildTierCurve(in); err != nil {
		t.Fatalf("BuildTierCurve: %v", err)
	}
	if in[0].Boundary.Cmp(b0) != 0 {
		t.Fatalf("input boundary mutated: %s", in[0].Boundary)
	}
}

func TestBuildTierCurve_EvaluationInvariant(t *testing.T) {
	curve, err := tierpolicy.BuildTierCurve([]tierpolicy.Tier{tier(100, 50), tier(300, 10), tier(1000, 5)})
	if err != nil {
		t.Fatalf("BuildTierCurve: %v", err)
	}
	for i, tr := range curve {
		got := haircutAt(tr.Boundary, curve)
		if got.Cmp(tr.Precomputed) != 0 {
			t.Errorf("haircutAt(boundary[%d]) = %s, want precomputed %s", i, got, tr.Precomputed)
		}
	}
}

func TestBuildTierCurve_Validation(t *testing.T) {
	tooMany := make([]tierpolicy.Tier, tierpolicy.MaxTiers+1)
	for i := range tooMany {
		tooMany[i] = tier(int64(i+1)*10, 1)
	}
	cases := []struct {
		name string
		in   []tierpolicy.Tier
	}{
		{"empty", nil},
		{"too-many", tooMany},
		{"ratio-over-100", []tierpolicy.Tier{tier(100, 101)}},
		{"equal-boundaries", []tierpolicy.Tier{tier(100, 10), tier(100, 10)}},
		{"decreasing-boundaries", []tierpolicy.Tier{tier(100, 10), tier(50, 10)}},
		{"nil-boundary", []tierpolicy.Tier{{Boundary: nil, Ratio: 10}}},
		{"negative-boundary", []tierpolicy.Tier{{Boundary: big.NewInt(-1), Ratio: 10}}},
		{"boundary-over-max", []tierpolicy.Tier{{Boundary: new(big.Int).Add(tierpolicy.MaxBoundaryValue, big.NewInt(1)), Ratio: 10}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tierpolicy.BuildTierCurve(tc.in); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestBuildTierCurve_BoundaryAtMaxAccepted(t *testing.T) {
	if _, err := tierpolicy.BuildTierCurve([]tierpolicy.Tier{{Boundary: new(big.Int).Set(tierpolicy.MaxBoundaryValue), Ratio: 100}}); err != nil {
		t.Fatalf("boundary == MaxBoundaryValue should be accepted: %v", err)
	}
}

// --- PolicyCommitment ---

func t2Policy() tierpolicy.Policy {
	return tierpolicy.Policy{
		Model: corespec.T2StaticHaircutMargin,
		Assets: []tierpolicy.AssetPolicy{
			{AssetIndex: 0, Haircut: 9000},
			{AssetIndex: 1, Haircut: 8500},
		},
	}
}

func t3Policy() tierpolicy.Policy {
	return tierpolicy.Policy{
		Model: corespec.T3TieredHaircutMargin1Pool,
		Assets: []tierpolicy.AssetPolicy{
			{AssetIndex: 0, Pools: [][]tierpolicy.Tier{{tier(100, 50), tier(300, 10)}}},
		},
	}
}

func t4Policy() tierpolicy.Policy {
	return tierpolicy.Policy{
		Model: corespec.T4TieredHaircutMargin3Pool,
		Assets: []tierpolicy.AssetPolicy{
			{AssetIndex: 0, Pools: [][]tierpolicy.Tier{
				{tier(100, 50)},               // loan
				{tier(200, 40)},               // margin
				{tier(300, 30), tier(900, 5)}, // portfolio_margin
			}},
		},
	}
}

func mustCommit(t *testing.T, p tierpolicy.Policy) []byte {
	t.Helper()
	d, err := tierpolicy.PolicyCommitment(p)
	if err != nil {
		t.Fatalf("PolicyCommitment: %v", err)
	}
	if len(d) == 0 {
		t.Fatalf("empty digest")
	}
	return d
}

func TestPolicyCommitment_Deterministic(t *testing.T) {
	for _, p := range []tierpolicy.Policy{t2Policy(), t3Policy(), t4Policy()} {
		if !bytes.Equal(mustCommit(t, p), mustCommit(t, p)) {
			t.Fatalf("model %q: digest not deterministic", p.Model)
		}
	}
}

func TestPolicyCommitment_AssetOrderInsensitive(t *testing.T) {
	a := t2Policy()
	b := t2Policy()
	b.Assets[0], b.Assets[1] = b.Assets[1], b.Assets[0] // shuffle input order
	if !bytes.Equal(mustCommit(t, a), mustCommit(t, b)) {
		t.Fatalf("digest changed when only input asset order changed")
	}
}

func TestPolicyCommitment_TamperSensitive(t *testing.T) {
	base := mustCommit(t, t3Policy())

	// changed ratio
	r := t3Policy()
	r.Assets[0].Pools[0][1].Ratio = 11
	if bytes.Equal(base, mustCommit(t, r)) {
		t.Errorf("digest unchanged after ratio tamper")
	}
	// changed boundary
	b := t3Policy()
	b.Assets[0].Pools[0][1].Boundary = big.NewInt(301)
	if bytes.Equal(base, mustCommit(t, b)) {
		t.Errorf("digest unchanged after boundary tamper")
	}
	// added asset
	add := t3Policy()
	add.Assets = append(add.Assets, tierpolicy.AssetPolicy{AssetIndex: 1, Pools: [][]tierpolicy.Tier{{tier(100, 50)}}})
	if bytes.Equal(base, mustCommit(t, add)) {
		t.Errorf("digest unchanged after adding an asset")
	}

	// T2 haircut tamper
	h0 := mustCommit(t, t2Policy())
	h := t2Policy()
	h.Assets[0].Haircut = 10000
	if bytes.Equal(h0, mustCommit(t, h)) {
		t.Errorf("digest unchanged after haircut tamper")
	}
}

func TestPolicyCommitment_CrossModelDistinct(t *testing.T) {
	// A T2 and a T3 policy that both reduce to "asset 0, one number" must
	// still differ thanks to the model domain tag + structural prefixes.
	d2 := mustCommit(t, tierpolicy.Policy{
		Model:  corespec.T2StaticHaircutMargin,
		Assets: []tierpolicy.AssetPolicy{{AssetIndex: 0, Haircut: 50}},
	})
	d3 := mustCommit(t, tierpolicy.Policy{
		Model:  corespec.T3TieredHaircutMargin1Pool,
		Assets: []tierpolicy.AssetPolicy{{AssetIndex: 0, Pools: [][]tierpolicy.Tier{{tier(50, 50)}}}},
	})
	if bytes.Equal(d2, d3) {
		t.Fatalf("T2 and T3 digests collided")
	}
}

func TestPolicyCommitment_Validation(t *testing.T) {
	cases := []struct {
		name string
		p    tierpolicy.Policy
	}{
		{"unsupported-model-t1", tierpolicy.Policy{Model: corespec.T1SimpleMargin, Assets: []tierpolicy.AssetPolicy{{AssetIndex: 0}}}},
		{"empty-assets", tierpolicy.Policy{Model: corespec.T2StaticHaircutMargin}},
		{"t2-with-pools", tierpolicy.Policy{Model: corespec.T2StaticHaircutMargin, Assets: []tierpolicy.AssetPolicy{{AssetIndex: 0, Pools: [][]tierpolicy.Tier{{tier(100, 50)}}}}}},
		{"t2-haircut-over-10000", tierpolicy.Policy{Model: corespec.T2StaticHaircutMargin, Assets: []tierpolicy.AssetPolicy{{AssetIndex: 0, Haircut: 10001}}}},
		{"t3-zero-pools", tierpolicy.Policy{Model: corespec.T3TieredHaircutMargin1Pool, Assets: []tierpolicy.AssetPolicy{{AssetIndex: 0}}}},
		{"t4-two-pools", tierpolicy.Policy{Model: corespec.T4TieredHaircutMargin3Pool, Assets: []tierpolicy.AssetPolicy{{AssetIndex: 0, Pools: [][]tierpolicy.Tier{{tier(100, 50)}, {tier(200, 40)}}}}}},
		{"duplicate-asset", tierpolicy.Policy{Model: corespec.T2StaticHaircutMargin, Assets: []tierpolicy.AssetPolicy{{AssetIndex: 0, Haircut: 1}, {AssetIndex: 0, Haircut: 2}}}},
		{"bad-tier-in-pool", tierpolicy.Policy{Model: corespec.T3TieredHaircutMargin1Pool, Assets: []tierpolicy.AssetPolicy{{AssetIndex: 0, Pools: [][]tierpolicy.Tier{{tier(100, 101)}}}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tierpolicy.PolicyCommitment(tc.p); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}
