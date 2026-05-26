// Package declarative defines the on-disk TOML schema for a customer
// profile, plus a Load helper that deserialises a file into the
// schema struct.
//
// v1 FROZEN (R7) — the schema below is frozen as the canonical
// `profile.toml` shape for the v1 catalog. Any structural change after
// this freeze is a *versioned change* requiring:
//
//   - new TOML field (additive): allowed with a default in Load so
//     existing files continue to parse — minor schema bump.
//   - removed field: deprecate-then-remove across two version cycles
//     minimum, with a parser warning during the deprecation window.
//   - renamed field: disallowed in v1 (breaks every existing file).
//   - new TOML table: same rules as new field.
//
// Two reference instantiations:
//
//   - profile/binance/binance.toml          — t4_tiered_haircut_margin_3pool model
//   - profile/sea_reference/sea_reference.toml — t1_simple_margin model
//
// At R7 these are *documentation-grade* artefacts (the loader and
// schema are exercised by tests), not the actual service input. The
// procedural Go adapters in profile/<customer>/ remain authoritative
// for now. A future slice (R7+1 / V1 production deployment) will swap
// service startup from "construct adapters in Go" to "Load(profile.toml)
// → assemble adapters from values". That transition is intentionally
// NOT part of R7 — flipping it requires every adapter constructor to
// accept its values via arguments, a separate large refactor.
//
// The schema is intentionally over-permissive — fields only some
// models need (e.g. pricing.two_digit_assets) are present at the top
// level so one struct covers all v1 catalog models. Per-model required-
// field validation lives in Validate(), not the struct shape.
package declarative

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Profile is the root schema. Fields map one-to-one to TOML tables /
// keys. Optional fields use zero-value defaults; per-model
// requirements are validated by Validate() rather than the schema
// itself (since both models share one struct).
type Profile struct {
	Profile     ProfileMeta    `toml:"profile"`
	Identity    Identity       `toml:"identity"`
	Insolvent   Insolvent      `toml:"insolvent"`
	Constraint  Constraint     `toml:"constraint"`
	Snapshot    Snapshot       `toml:"snapshot"`
	BatchShapes []BatchShape   `toml:"batch_shapes"`
	Pricing     Pricing        `toml:"pricing"`
	Catalog     CatalogConfig  `toml:"catalog"`
}

// ProfileMeta carries the profile-identifying fields.
type ProfileMeta struct {
	Name          string `toml:"name"`           // human-readable profile id, e.g. "binance"
	Model         string `toml:"model"`          // SolvencyModelID, e.g. "t4_tiered_haircut_margin_3pool"
	AssetCapacity int    `toml:"asset_capacity"` // trusted-setup asset slot count
}

// Identity selects the AccountIDProvider's derivation scheme.
type Identity struct {
	// Scheme is the engine-frozen identifier
	// (e.g. "passthrough_hex_bn254_reduced.v0"). G2 universal contract.
	Scheme string `toml:"scheme"`
}

// Insolvent selects the InvalidAccountPolicy action. At R5 only
// "drop_and_log" is implemented across profiles.
type Insolvent struct {
	Action string `toml:"action"`
}

// Constraint selects the ConstraintModule. Empty Module == noop.
type Constraint struct {
	Module string `toml:"module"` // "" → NoExtensionID; else module ID
}

// Snapshot describes the source-type and per-source parameters.
type Snapshot struct {
	SourceType  string `toml:"source_type"`   // e.g. "binance_csv", "sea_csv"
	UserDataDir string `toml:"user_data_dir"` // directory holding the CSV inputs
	SnapshotID  string `toml:"snapshot_id"`   // human-readable timestamp / sequence
}

// BatchShape mirrors core/spec.BatchShape.
type BatchShape struct {
	AssetCountTier int `toml:"asset_count_tier"`
	UsersPerBatch  int `toml:"users_per_batch"`
}

// Pricing carries the per-symbol multiplier configuration. The
// two_digit_* fields are t4_tiered_haircut_margin_3pool-style operator knobs — spot
// profiles may leave them empty / zero.
type Pricing struct {
	DefaultPriceScale    int64    `toml:"default_price_scale"`
	DefaultBalanceScale  int64    `toml:"default_balance_scale"`
	TwoDigitAssets       []string `toml:"two_digit_assets"`
	TwoDigitPriceScale   int64    `toml:"two_digit_price_scale"`
	TwoDigitBalanceScale int64    `toml:"two_digit_balance_scale"`
}

// CatalogConfig holds the catalog's static symbol list. In production
// the order is derived from the user-CSV header at snapshot time; the
// list here is a publishable reference for verifiers.
type CatalogConfig struct {
	Symbols []string `toml:"symbols"`
}

// Load reads the file at path and unmarshals it into a Profile.
// Returns a descriptive error if the file is missing, malformed, or
// fails schema validation.
func Load(path string) (*Profile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("declarative profile: read %q: %w", path, err)
	}
	p := &Profile{}
	if err := toml.Unmarshal(raw, p); err != nil {
		return nil, fmt.Errorf("declarative profile: parse %q: %w", path, err)
	}
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("declarative profile %q: %w", path, err)
	}
	return p, nil
}

// Validate enforces the cross-field invariants the schema's optional-
// field layout cannot express alone:
//   - profile.{name,model} must be non-empty
//   - asset_capacity > 0 and >= len(catalog.symbols)
//   - identity.scheme non-empty
//   - at least one batch_shape
//   - each batch_shape.{asset_count_tier,users_per_batch} > 0
//   - pricing.default_* multipliers > 0
//   - t4_tiered_haircut_margin_3pool-specific: if two_digit_assets is non-empty, the
//     two_digit_* scales must be > 0
func (p *Profile) Validate() error {
	if p.Profile.Name == "" {
		return fmt.Errorf("profile.name is empty")
	}
	if p.Profile.Model == "" {
		return fmt.Errorf("profile.model is empty")
	}
	if p.Profile.AssetCapacity <= 0 {
		return fmt.Errorf("profile.asset_capacity must be > 0, got %d", p.Profile.AssetCapacity)
	}
	if len(p.Catalog.Symbols) > p.Profile.AssetCapacity {
		return fmt.Errorf("catalog has %d symbols but asset_capacity = %d",
			len(p.Catalog.Symbols), p.Profile.AssetCapacity)
	}
	if p.Identity.Scheme == "" {
		return fmt.Errorf("identity.scheme is empty")
	}
	if len(p.BatchShapes) == 0 {
		return fmt.Errorf("batch_shapes is empty")
	}
	for i, s := range p.BatchShapes {
		if s.AssetCountTier <= 0 || s.UsersPerBatch <= 0 {
			return fmt.Errorf("batch_shapes[%d] has non-positive field: %+v", i, s)
		}
	}
	if p.Pricing.DefaultPriceScale <= 0 || p.Pricing.DefaultBalanceScale <= 0 {
		return fmt.Errorf("pricing default_* multipliers must be > 0")
	}
	if len(p.Pricing.TwoDigitAssets) > 0 {
		if p.Pricing.TwoDigitPriceScale <= 0 || p.Pricing.TwoDigitBalanceScale <= 0 {
			return fmt.Errorf("two_digit_assets requires two_digit_* multipliers > 0")
		}
	}
	return nil
}
