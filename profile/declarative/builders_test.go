package declarative_test

import (
	"os"
	"testing"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	snapshotmapping "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/mapping"
	snapshotschema "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/snapshot/schema"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
)

// TestBuildIdentity_KnownScheme confirms the builder wires through to
// the host registry — same provider Scheme() round-trips.
func TestBuildIdentity_KnownScheme(t *testing.T) {
	p := declarative.BuildIdentity(declarative.Identity{
		Scheme: host.IdentitySchemePassthroughHexBN254ReducedV0,
	})
	if got := p.Scheme(); got != host.IdentitySchemePassthroughHexBN254ReducedV0 {
		t.Fatalf("Scheme() = %q, want %q",
			got, host.IdentitySchemePassthroughHexBN254ReducedV0)
	}
}

// TestBuildIdentity_UnknownPanics asserts the build-time-omission
// contract (G17 c): a scheme not linked into the binary panics.
func TestBuildIdentity_UnknownPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on unknown scheme")
		}
	}()
	declarative.BuildIdentity(declarative.Identity{Scheme: "missing.v0"})
}

// TestBuildInsolvent_KnownAction confirms the builder dispatches to
// host.NewInsolventPolicy.
func TestBuildInsolvent_KnownAction(t *testing.T) {
	policy := declarative.BuildInsolvent(declarative.Insolvent{
		Action: host.InsolventActionDropAndLogV0,
	})
	if got := policy.OnInsolventAccount("u", "r"); got != spec.InvalidActionDrop {
		t.Fatalf("OnInsolventAccount = %v, want InvalidActionDrop", got)
	}
}

// TestBuildInsolvent_UnknownPanics asserts the build-time contract.
func TestBuildInsolvent_UnknownPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on unknown action")
		}
	}()
	declarative.BuildInsolvent(declarative.Insolvent{Action: "missing.v0"})
}

// TestBuildSnapshotMapping_ValidatesAgainstModelSchema verifies the
// profile-level builder binds R9-C mappings to the selected model's
// standard schema.
func TestBuildSnapshotMapping_ValidatesAgainstModelSchema(t *testing.T) {
	cfg, err := declarative.BuildSnapshotMapping(spec.T1SimpleMargin, declarative.Snapshot{
		Files: []snapshotmapping.File{{
			Name:   "accounts.csv",
			Source: "user_balances.csv",
			Columns: map[string]snapshotmapping.Column{
				"account_id":  {Source: "id", Type: snapshotschema.FieldAccountID},
				"asset_index": {Source: "asset_index", Type: snapshotschema.FieldUint16},
				"equity":      {Source: "balance", Type: snapshotschema.FieldUint64, DecimalScale: 100_000_000},
				"debt":        {Constant: "0", Type: snapshotschema.FieldUint64},
			},
		}},
	})
	if err != nil {
		t.Fatalf("BuildSnapshotMapping: %v", err)
	}
	if len(cfg.Files) != 1 || cfg.Files[0].Name != "accounts.csv" {
		t.Fatalf("cfg.Files = %+v", cfg.Files)
	}
}

// TestBuildSnapshotMapping_RejectsWrongModelShape confirms validation
// is tied to profile.model, not just arbitrary column names.
func TestBuildSnapshotMapping_RejectsWrongModelShape(t *testing.T) {
	_, err := declarative.BuildSnapshotMapping(spec.T4TieredHaircutMargin3Pool, declarative.Snapshot{
		Files: []snapshotmapping.File{{
			Name:   "accounts.csv",
			Source: "user_balances.csv",
			Columns: map[string]snapshotmapping.Column{
				"account_id":  {Source: "id"},
				"asset_index": {Source: "asset_index"},
				"equity":      {Source: "balance"},
				"debt":        {Constant: "0"},
			},
		}},
	})
	if err == nil {
		t.Fatal("BuildSnapshotMapping accepted T1-shaped mapping for T4")
	}
}

// TestBuildBatchShape_HappyDefault verifies non-override path: shapes
// are returned in ascending AssetCountTier order.
func TestBuildBatchShape_HappyDefault(t *testing.T) {
	os.Unsetenv("ZKPOR_BATCH_SHAPE_OVERRIDE")
	shapes, err := declarative.BuildBatchShape([]declarative.BatchShape{
		{AssetCountTier: 500, UsersPerBatch: 92},
		{AssetCountTier: 50, UsersPerBatch: 700},
	})
	if err != nil {
		t.Fatalf("BuildBatchShape: %v", err)
	}
	if len(shapes) != 2 {
		t.Fatalf("len = %d, want 2", len(shapes))
	}
	if shapes[0].AssetCountTier != 50 || shapes[1].AssetCountTier != 500 {
		t.Fatalf("not ascending: %+v", shapes)
	}
}

// TestBuildBatchShape_DuplicateTierRejected enforces the unique-tier
// invariant the BatchShapeProvider interface requires.
func TestBuildBatchShape_DuplicateTierRejected(t *testing.T) {
	os.Unsetenv("ZKPOR_BATCH_SHAPE_OVERRIDE")
	_, err := declarative.BuildBatchShape([]declarative.BatchShape{
		{AssetCountTier: 50, UsersPerBatch: 700},
		{AssetCountTier: 50, UsersPerBatch: 92},
	})
	if err == nil {
		t.Fatal("expected duplicate-tier error")
	}
}

// TestBuildBatchShape_OverrideWins asserts the smoke-harness env var
// replaces the declared shapes.
func TestBuildBatchShape_OverrideWins(t *testing.T) {
	t.Setenv("ZKPOR_BATCH_SHAPE_OVERRIDE", "5_10")
	shapes, err := declarative.BuildBatchShape([]declarative.BatchShape{
		{AssetCountTier: 50, UsersPerBatch: 700},
		{AssetCountTier: 500, UsersPerBatch: 92},
	})
	if err != nil {
		t.Fatalf("BuildBatchShape: %v", err)
	}
	if len(shapes) != 1 || shapes[0].AssetCountTier != 5 || shapes[0].UsersPerBatch != 10 {
		t.Fatalf("override didn't apply: %+v", shapes)
	}
}

// TestBuildBatchShape_OverrideParseError surfaces malformed overrides
// as a builder error (not a silent fall-through to the declared list).
func TestBuildBatchShape_OverrideParseError(t *testing.T) {
	t.Setenv("ZKPOR_BATCH_SHAPE_OVERRIDE", "garbage")
	_, err := declarative.BuildBatchShape([]declarative.BatchShape{
		{AssetCountTier: 50, UsersPerBatch: 700},
	})
	if err == nil {
		t.Fatal("expected parse error from malformed override")
	}
}

// TestBuildBatchShapeProvider_Wires verifies the wrapper exposes
// Model/SelectFor/KeyName off the underlying []BatchShape from
// BuildBatchShape.
func TestBuildBatchShapeProvider_Wires(t *testing.T) {
	os.Unsetenv("ZKPOR_BATCH_SHAPE_OVERRIDE")
	p, err := declarative.BuildBatchShapeProvider("t4_tiered_haircut_margin_3pool",
		[]declarative.BatchShape{
			{AssetCountTier: 500, UsersPerBatch: 92},
			{AssetCountTier: 50, UsersPerBatch: 700},
		})
	if err != nil {
		t.Fatalf("BuildBatchShapeProvider: %v", err)
	}
	if p.Model() != "t4_tiered_haircut_margin_3pool" {
		t.Fatalf("Model() = %q", p.Model())
	}
	got, err := p.SelectFor(20)
	if err != nil || got.AssetCountTier != 50 {
		t.Fatalf("SelectFor(20) = %+v, %v; want tier=50", got, err)
	}
	if name := p.KeyName(got, ""); name != "zkpor.t4_tiered_haircut_margin_3pool.50_700" {
		t.Fatalf("KeyName = %q", name)
	}
}

// TestBuildBatchShapeProvider_EmptyModelRejected guards against
// direct callers that bypass declarative.Load (which Validates the
// model field).
func TestBuildBatchShapeProvider_EmptyModelRejected(t *testing.T) {
	_, err := declarative.BuildBatchShapeProvider("",
		[]declarative.BatchShape{{AssetCountTier: 50, UsersPerBatch: 700}})
	if err == nil {
		t.Fatal("expected error on empty model")
	}
}

// TestBuildPricing_Default verifies the no-two-digit-list path — every
// symbol returns the default scales and ValueScale is their product.
func TestBuildPricing_Default(t *testing.T) {
	p, err := declarative.BuildPricing(declarative.Pricing{
		DefaultPriceScale:   1e8,
		DefaultBalanceScale: 1e8,
	})
	if err != nil {
		t.Fatalf("BuildPricing: %v", err)
	}
	if p.PriceMultiplier("btc") != 1e8 || p.BalanceMultiplier("btc") != 1e8 {
		t.Fatalf("default symbol multipliers wrong")
	}
	if p.ValueScale() != 1e16 {
		t.Fatalf("ValueScale = %d, want 1e16", p.ValueScale())
	}
}

// TestBuildPricing_TwoDigitSwitch confirms the case-insensitive
// shifted-multiplier path matches Binance's historical SHIB scale.
func TestBuildPricing_TwoDigitSwitch(t *testing.T) {
	p, err := declarative.BuildPricing(declarative.Pricing{
		DefaultPriceScale:    1e8,
		DefaultBalanceScale:  1e8,
		TwoDigitAssets:       []string{"SHIB"},
		TwoDigitPriceScale:   1e14,
		TwoDigitBalanceScale: 1e2,
	})
	if err != nil {
		t.Fatalf("BuildPricing: %v", err)
	}
	if p.PriceMultiplier("shib") != 1e14 || p.BalanceMultiplier("shib") != 1e2 {
		t.Fatalf("two-digit miss: price=%d balance=%d", p.PriceMultiplier("shib"), p.BalanceMultiplier("shib"))
	}
	if p.PriceMultiplier("btc") != 1e8 {
		t.Fatalf("default symbol picked up two-digit scale")
	}
}

// TestBuildPricing_G6InvariantViolation locks the G6 assert: the
// default and two-digit ValueScales MUST be equal.
func TestBuildPricing_G6InvariantViolation(t *testing.T) {
	_, err := declarative.BuildPricing(declarative.Pricing{
		DefaultPriceScale:    1e8,
		DefaultBalanceScale:  1e8,
		TwoDigitAssets:       []string{"shib"},
		TwoDigitPriceScale:   1e14,
		TwoDigitBalanceScale: 1e3, // 1e14 * 1e3 = 1e17 ≠ 1e16
	})
	if err == nil {
		t.Fatal("expected G6 invariant violation")
	}
}

// TestBuildCatalog_Happy verifies the symbol list round-trips and
// case-insensitive lookup works.
func TestBuildCatalog_Happy(t *testing.T) {
	c, err := declarative.BuildCatalog(declarative.CatalogConfig{
		Symbols: []string{"BTC", "ETH", "USDT"},
	}, 10)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	if c.Capacity() != 10 {
		t.Fatalf("Capacity = %d, want 10", c.Capacity())
	}
	syms := c.Symbols()
	if len(syms) != 3 || syms[0] != "btc" {
		t.Fatalf("Symbols = %v", syms)
	}
	if idx, ok := c.IndexOf("Eth"); !ok || idx != 1 {
		t.Fatalf("IndexOf(Eth) = %d, %v; want 1, true", idx, ok)
	}
}

// TestBuildCatalog_EmptySymbolsOK confirms an empty symbol list is
// accepted — this is the binance path where catalog identity comes
// from the snapshot CSV header at runtime.
func TestBuildCatalog_EmptySymbolsOK(t *testing.T) {
	c, err := declarative.BuildCatalog(declarative.CatalogConfig{}, 500)
	if err != nil {
		t.Fatalf("BuildCatalog with empty symbols: %v", err)
	}
	if c.Capacity() != 500 {
		t.Fatalf("Capacity = %d, want 500", c.Capacity())
	}
	if len(c.Symbols()) != 0 {
		t.Fatalf("expected zero symbols, got %v", c.Symbols())
	}
}

// TestBuildCatalog_OverCapacity locks the trusted-setup invariant.
func TestBuildCatalog_OverCapacity(t *testing.T) {
	_, err := declarative.BuildCatalog(declarative.CatalogConfig{
		Symbols: []string{"a", "b", "c", "d", "e"},
	}, 3)
	if err == nil {
		t.Fatal("expected capacity overflow error")
	}
}

// TestBuildCatalog_Duplicate locks the unique-symbol invariant.
func TestBuildCatalog_Duplicate(t *testing.T) {
	_, err := declarative.BuildCatalog(declarative.CatalogConfig{
		Symbols: []string{"btc", "BTC"},
	}, 10)
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}
