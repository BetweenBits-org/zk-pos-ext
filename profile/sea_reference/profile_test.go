package sea_reference

import (
	"reflect"
	"testing"

	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

func TestCatalogCapacityAndLookup(t *testing.T) {
	c := NewCatalog([]string{"BTC", "ETH", "USDT"}, 5)
	if c.Capacity() != 5 {
		t.Fatalf("Capacity = %d, want 5", c.Capacity())
	}
	if got := c.Symbols(); !reflect.DeepEqual(got, []string{"btc", "eth", "usdt"}) {
		t.Fatalf("Symbols = %v, want lowercased", got)
	}
	if i, ok := c.IndexOf("Eth"); !ok || i != 1 {
		t.Fatalf("IndexOf(Eth) = %d,%v, want 1,true (case-insensitive)", i, ok)
	}
	if _, ok := c.IndexOf("xrp"); ok {
		t.Fatal("IndexOf(xrp) should miss")
	}
}

func TestCatalogRejectsCapacityBelowSymbols(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when capacity < len(symbols)")
		}
	}()
	NewCatalog([]string{"BTC", "ETH", "USDT"}, 2)
}

func TestIdentitySchemeFrozen(t *testing.T) {
	// G2 universal scheme — value must match exactly across profiles.
	if got := NewIdentity().Scheme(); got != "passthrough_hex_bn254_reduced.v0" {
		t.Fatalf("Scheme() = %q, want passthrough_hex_bn254_reduced.v0", got)
	}
}

func TestIdentityRejectsBadHexInput(t *testing.T) {
	id := NewIdentity()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on len != 64")
		}
	}()
	id.DeriveAccountID("abc") // not 64 hex chars
}

func TestPricingDefaultScalesAcrossSymbols(t *testing.T) {
	p := NewPricing()
	if p.PriceMultiplier("btc") != corespec.DefaultPriceScale {
		t.Errorf("BTC PriceMultiplier != DefaultPriceScale")
	}
	if p.PriceMultiplier("idr") != corespec.DefaultPriceScale {
		t.Errorf("IDR PriceMultiplier != DefaultPriceScale (sea_reference is uniform-scale)")
	}
	if p.ValueScale() != corespec.DefaultValueScale {
		t.Errorf("ValueScale != DefaultValueScale")
	}
}

func TestBatchShapeDefault(t *testing.T) {
	t.Setenv(shapeOverrideEnv, "")
	shapes := NewBatchShape().Shapes()
	want := []corespec.BatchShape{{AssetCountTier: 50, UsersPerBatch: 1000}}
	if !reflect.DeepEqual(shapes, want) {
		t.Fatalf("default shapes = %#v, want %#v", shapes, want)
	}
}

func TestBatchShapeOverride(t *testing.T) {
	t.Setenv(shapeOverrideEnv, "5_10")
	shapes := NewBatchShape().Shapes()
	want := []corespec.BatchShape{{AssetCountTier: 5, UsersPerBatch: 10}}
	if !reflect.DeepEqual(shapes, want) {
		t.Fatalf("override shapes = %#v, want %#v", shapes, want)
	}
}

func TestInsolventPolicyAlwaysDrops(t *testing.T) {
	action := NewInsolventPolicy().OnInsolventAccount("user1", "test reason")
	if action != corespec.InvalidActionDrop {
		t.Fatalf("expected InvalidActionDrop, got %v", action)
	}
}

func TestNoopConstraintIdIsNoExtension(t *testing.T) {
	m := NewNoopConstraint()
	if string(m.ID()) != corespec.NoExtensionID {
		t.Fatalf("ID = %q, want NoExtensionID %q", m.ID(), corespec.NoExtensionID)
	}
}
