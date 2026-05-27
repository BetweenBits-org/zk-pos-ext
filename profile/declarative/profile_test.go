package declarative_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
)

// TestLoadBinance verifies the published binance.toml parses with the
// expected key fields. Asserts shape and a few canonical values
// rather than locking in the entire two_digit_assets list (which is
// allowed to evolve under operator control without forcing a schema
// version bump).
func TestLoadBinance(t *testing.T) {
	p, err := declarative.Load("../binance/binance.toml")
	if err != nil {
		t.Fatalf("Load binance.toml: %v", err)
	}
	if p.Profile.Name != "binance" {
		t.Errorf("Name = %q, want binance", p.Profile.Name)
	}
	if p.Profile.Model != "t4_tiered_haircut_margin_3pool" {
		t.Errorf("Model = %q, want t4_tiered_haircut_margin_3pool", p.Profile.Model)
	}
	if p.Profile.AssetCapacity != 500 {
		t.Errorf("AssetCapacity = %d, want 500", p.Profile.AssetCapacity)
	}
	if p.Identity.Scheme != "passthrough_hex_bn254_reduced.v0" {
		t.Errorf("Identity.Scheme = %q", p.Identity.Scheme)
	}
	if len(p.BatchShapes) != 2 {
		t.Errorf("BatchShapes length = %d, want 2", len(p.BatchShapes))
	}
	if p.BatchShapes[0].AssetCountTier != 50 || p.BatchShapes[0].UsersPerBatch != 700 {
		t.Errorf("BatchShapes[0] = %+v", p.BatchShapes[0])
	}
	if p.BatchShapes[1].AssetCountTier != 500 || p.BatchShapes[1].UsersPerBatch != 92 {
		t.Errorf("BatchShapes[1] = %+v", p.BatchShapes[1])
	}
	if p.Pricing.DefaultPriceScale != 100_000_000 {
		t.Errorf("DefaultPriceScale = %d", p.Pricing.DefaultPriceScale)
	}
	if p.Pricing.TwoDigitPriceScale != 100_000_000_000_000 {
		t.Errorf("TwoDigitPriceScale = %d", p.Pricing.TwoDigitPriceScale)
	}
	if len(p.Pricing.TwoDigitAssets) < 5 {
		t.Errorf("TwoDigitAssets length = %d, want >= 5", len(p.Pricing.TwoDigitAssets))
	}
	if got := p.Snapshot.Format.NullValues; len(got) != 3 || got[0] != "" {
		t.Errorf("Snapshot.Format.NullValues = %#v", got)
	}
}

// TestLoadSeaReference verifies the published sea_reference.toml
// parses + represents the t1_simple_margin model correctly.
func TestLoadSeaReference(t *testing.T) {
	p, err := declarative.Load("../sea_reference/sea_reference.toml")
	if err != nil {
		t.Fatalf("Load sea_reference.toml: %v", err)
	}
	if p.Profile.Name != "sea_reference" {
		t.Errorf("Name = %q", p.Profile.Name)
	}
	if p.Profile.Model != "t1_simple_margin" {
		t.Errorf("Model = %q, want t1_simple_margin", p.Profile.Model)
	}
	if p.Profile.AssetCapacity != 50 {
		t.Errorf("AssetCapacity = %d, want 50", p.Profile.AssetCapacity)
	}
	if p.Identity.Scheme != "passthrough_hex_bn254_reduced.v0" {
		t.Errorf("Identity.Scheme = %q", p.Identity.Scheme)
	}
	if len(p.BatchShapes) != 1 {
		t.Errorf("BatchShapes length = %d, want 1 (spot has a single-tier default)", len(p.BatchShapes))
	}
	if len(p.Pricing.TwoDigitAssets) != 0 {
		t.Errorf("TwoDigitAssets non-empty = %v (sea_reference is uniform-scale)", p.Pricing.TwoDigitAssets)
	}
	if got := p.Snapshot.Format.NullValues; len(got) != 3 || got[0] != "" {
		t.Errorf("Snapshot.Format.NullValues = %#v", got)
	}
	wantCatalog := []string{"btc", "eth", "usdt", "usdc", "bnb"}
	if len(p.Catalog.Symbols) != len(wantCatalog) {
		t.Errorf("Catalog.Symbols length = %d, want %d", len(p.Catalog.Symbols), len(wantCatalog))
	}
}

// TestLoadWithSnapshotMapping verifies the additive R9-C
// [snapshot.format] and [[snapshot.files]] schema can be parsed and
// validated without changing existing profile files.
func TestLoadWithSnapshotMapping(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mapped.toml")
	raw := `
[profile]
name = "mapped"
model = "t1_simple_margin"
asset_capacity = 50

[identity]
scheme = "passthrough_hex_bn254_reduced.v0"

[insolvent]
action = "drop_and_log.v0"

[constraint]
module = ""

[snapshot]
source_type = "t1_standard_csv.v1"
user_data_dir = "/data/<snapshot_id>"
snapshot_id = "<set per snapshot>"

[snapshot.format]
delimiter = ","
null_values = ["", "NA"]

[[snapshot.files]]
name = "accounts.csv"
source = "user_balances.csv"
mode = "direct"

[snapshot.files.columns.account_id]
source = "id"
type = "account_id_hex_bn254"

[snapshot.files.columns.asset_index]
source = "asset_index"
type = "uint16"

[snapshot.files.columns.equity]
source = "balance"
type = "uint64"
decimal_scale = 100000000

[snapshot.files.columns.debt]
constant = "0"
type = "uint64"

[[batch_shapes]]
asset_count_tier = 50
users_per_batch = 1000

[pricing]
default_price_scale = 100000000
default_balance_scale = 100000000

[catalog]
symbols = ["btc"]
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	p, err := declarative.Load(path)
	if err != nil {
		t.Fatalf("Load mapped profile: %v", err)
	}
	if len(p.Snapshot.Files) != 1 {
		t.Fatalf("Snapshot.Files length = %d, want 1", len(p.Snapshot.Files))
	}
	if p.Snapshot.Files[0].Columns["equity"].DecimalScale != 100_000_000 {
		t.Fatalf("equity DecimalScale = %d", p.Snapshot.Files[0].Columns["equity"].DecimalScale)
	}
}

// TestValidateRejectsEmpty asserts the schema validation catches
// obviously malformed inputs.
func TestValidateRejectsEmpty(t *testing.T) {
	p := &declarative.Profile{}
	if err := p.Validate(); err == nil {
		t.Fatal("Validate accepted empty profile")
	}
}

// TestValidateRejectsCapacityBelowSymbols asserts the
// catalog/capacity invariant is enforced.
func TestValidateRejectsCapacityBelowSymbols(t *testing.T) {
	p := &declarative.Profile{
		Profile:   declarative.ProfileMeta{Name: "x", Model: "t4_tiered_haircut_margin_3pool", AssetCapacity: 2},
		Identity:  declarative.Identity{Scheme: "passthrough_hex_bn254_reduced.v0"},
		Insolvent: declarative.Insolvent{Action: "drop_and_log.v0"},
		BatchShapes: []declarative.BatchShape{
			{AssetCountTier: 50, UsersPerBatch: 700},
		},
		Pricing: declarative.Pricing{DefaultPriceScale: 1e8, DefaultBalanceScale: 1e8},
		Catalog: declarative.CatalogConfig{Symbols: []string{"a", "b", "c", "d"}},
	}
	if err := p.Validate(); err == nil {
		t.Fatal("Validate accepted symbols > capacity")
	}
}

// TestValidateRejectsEmptyInsolventAction asserts the new R8-A guard
// catches the easy mistake of leaving insolvent.action blank — host
// registry lookup would also panic at service startup, but profile
// load is the earlier, cheaper failure surface.
func TestValidateRejectsEmptyInsolventAction(t *testing.T) {
	p := &declarative.Profile{
		Profile:  declarative.ProfileMeta{Name: "x", Model: "t1_simple_margin", AssetCapacity: 50},
		Identity: declarative.Identity{Scheme: "passthrough_hex_bn254_reduced.v0"},
		// Insolvent.Action intentionally empty.
		BatchShapes: []declarative.BatchShape{{AssetCountTier: 50, UsersPerBatch: 1000}},
		Pricing:     declarative.Pricing{DefaultPriceScale: 1e8, DefaultBalanceScale: 1e8},
	}
	if err := p.Validate(); err == nil {
		t.Fatal("Validate accepted empty insolvent.action")
	}
}

// TestValidateRejectsTwoDigitWithoutScales asserts the cross-field
// dependency: if two_digit_assets is non-empty, the two_digit_*
// multiplier fields must be positive.
func TestValidateRejectsTwoDigitWithoutScales(t *testing.T) {
	p := &declarative.Profile{
		Profile:   declarative.ProfileMeta{Name: "x", Model: "t4_tiered_haircut_margin_3pool", AssetCapacity: 500},
		Identity:  declarative.Identity{Scheme: "passthrough_hex_bn254_reduced.v0"},
		Insolvent: declarative.Insolvent{Action: "drop_and_log.v0"},
		BatchShapes: []declarative.BatchShape{
			{AssetCountTier: 50, UsersPerBatch: 700},
		},
		Pricing: declarative.Pricing{
			DefaultPriceScale:   1e8,
			DefaultBalanceScale: 1e8,
			TwoDigitAssets:      []string{"shib"},
			// TwoDigitPriceScale + TwoDigitBalanceScale intentionally 0
		},
	}
	if err := p.Validate(); err == nil {
		t.Fatal("Validate accepted two_digit_assets without scales")
	}
}
