# Profile descriptors

`profile/` contains customer-facing deployment descriptors for the zkpor
engine. Post-R10, profile packages are descriptor-only: the engine does not
link customer raw CSV parsers. A customer or operator must normalize exchange
exports into the model-standard CSV files before starting zkpor services.

## Directory layout

```text
profile/
├── README.md
├── binance/
│   └── binance.toml
├── sea_reference/
│   └── sea_reference.toml
└── declarative/
    ├── profile.go      # TOML schema + Load/Validate
    └── builders.go     # profile values -> engine helpers
```

Add a new customer by copying the closest existing TOML file into
`profile/<customer>/<customer>.toml`. Do not add `snapshot.go` or other raw
ETL code under `profile/<customer>`; raw export conversion belongs in an
external preprocessor that writes standard CSV.

## Input contract

`[snapshot].source_type` must select one of the standard snapshot connectors:

| Model | `profile.model` | `snapshot.source_type` | Required files in `snapshot.user_data_dir` |
|---|---|---|---|
| T1 simple margin | `t1_simple_margin` | `t1_standard_csv.v1` | `accounts.csv`, `cex_assets.csv` |
| T2 static haircut margin | `t2_static_haircut_margin` | `t2_standard_csv.v1` | `accounts.csv`, `cex_assets.csv` |
| T3 tiered haircut margin, 1 pool | `t3_tiered_haircut_margin_1pool` | `t3_standard_csv.v1` | `accounts.csv`, `cex_assets.csv`, `tier_ratios.csv` |
| T4 tiered haircut margin, 3 pool | `t4_tiered_haircut_margin_3pool` | `t4_standard_csv.v1` | `accounts.csv`, `cex_assets.csv`, `tier_ratios.csv` |

Standard CSV amounts are already scaled non-negative integers. The engine does
not parse exchange decimal strings, wide raw exports, or customer-specific
column names at runtime.

## Required profile fields

`[profile]`

- `name`: human-readable descriptor name.
- `model`: solvency model id. Must match the selected standard connector.
- `asset_capacity`: trusted-setup asset slot capacity. Must be positive and at
  least `len([catalog].symbols)`.

`[identity]`

- `scheme`: account-id derivation scheme. V1 uses
  `passthrough_hex_bn254_reduced.v0`.

`[insolvent]`

- `action`: invalid account policy. V1 uses `drop_and_log.v0`.

`[constraint]`

- `module`: constraint module id. Empty string means the model default noop.

`[snapshot]`

- `source_type`: one of the `t*_standard_csv.v1` connector ids above.
- `user_data_dir`: directory containing the canonical standard CSV files for
  the snapshot.
- `snapshot_id`: operator-supplied snapshot label or timestamp.

`[snapshot.format]`

- Optional CSV dialect settings shared with `core/snapshot/csv`.
- Common default: `null_values = ["", "NA", "null"]`.

`[[batch_shapes]]`

- `asset_count_tier`: max non-empty assets per account for this proving tier.
- `users_per_batch`: number of users per circuit batch at this tier.
- Provide at least one shape. Tiers are sorted by the builder before use.

`[pricing]`

- `default_price_scale` and `default_balance_scale` must be positive.
- `two_digit_assets`, `two_digit_price_scale`, and
  `two_digit_balance_scale` are optional. If `two_digit_assets` is non-empty,
  both two-digit scales must be positive.
- `BuildPricing` enforces the G6 invariant:
  `default_price_scale * default_balance_scale ==
  two_digit_price_scale * two_digit_balance_scale`.

`[catalog]`

- `symbols`: publishable reference list for auditors and verifier-side users.
- The committed per-snapshot asset order still comes from canonical
  `cex_assets.csv`.

## Minimal T4 example

```toml
[profile]
name = "example_exchange"
model = "t4_tiered_haircut_margin_3pool"
asset_capacity = 500

[identity]
scheme = "passthrough_hex_bn254_reduced.v0"

[insolvent]
action = "drop_and_log.v0"

[constraint]
module = ""

[snapshot]
source_type = "t4_standard_csv.v1"
user_data_dir = "/standard-data/<snapshot_id>"
snapshot_id = "<set per snapshot>"

[snapshot.format]
null_values = ["", "NA", "null"]

[[batch_shapes]]
asset_count_tier = 50
users_per_batch = 700

[[batch_shapes]]
asset_count_tier = 500
users_per_batch = 92

[pricing]
default_price_scale = 100000000
default_balance_scale = 100000000

[catalog]
symbols = []
```

## Authoring checklist

1. Choose the solvency model first, then choose the matching
   `t*_standard_csv.v1` connector.
2. Set `asset_capacity` to the capacity used for trusted setup and services.
3. Define batch shapes that cover the expected max non-empty assets per user.
4. Keep `identity.scheme` and `insolvent.action` on the frozen V1 ids unless a
   new registry entry has been added intentionally.
5. Produce canonical standard CSV files in `snapshot.user_data_dir`.
6. Run validation/tests:

```bash
go test ./profile/declarative
go build ./zkpor/...
go vet ./zkpor/...
```

When running inside a restricted sandbox, set `GOCACHE` to a writable path, for
example `GOCACHE=/private/tmp/zkpor-gocache`.
