# Profile descriptor 작성 가이드

`profile/` 은 zkpor 엔진의 고객별 deployment descriptor 를 담는다. R10 이후
profile package 는 descriptor-only 이다. 엔진은 고객 raw CSV parser 를 link
하지 않는다. 고객 또는 operator 는 zkpor service 를 시작하기 전에 거래소
export 를 model-standard CSV 로 정규화해야 한다.

## 디렉터리 구조

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

새 고객을 추가할 때는 가장 가까운 기존 TOML 을
`profile/<customer>/<customer>.toml` 로 복사해서 시작한다.
`profile/<customer>` 아래에 `snapshot.go` 나 raw ETL 코드를 추가하지 않는다.
raw export 변환은 standard CSV 를 쓰는 외부 preprocessor 책임이다.

## 입력 계약

`[snapshot].source_type` 은 standard snapshot connector 중 하나여야 한다.

| Model | `profile.model` | `snapshot.source_type` | `snapshot.user_data_dir` 필수 파일 |
|---|---|---|---|
| T1 simple margin | `t1_simple_margin` | `t1_standard_csv.v1` | `accounts.csv`, `cex_assets.csv` |
| T2 static haircut margin | `t2_static_haircut_margin` | `t2_standard_csv.v1` | `accounts.csv`, `cex_assets.csv` |
| T3 tiered haircut margin, 1 pool | `t3_tiered_haircut_margin_1pool` | `t3_standard_csv.v1` | `accounts.csv`, `cex_assets.csv`, `tier_ratios.csv` |
| T4 tiered haircut margin, 3 pool | `t4_tiered_haircut_margin_3pool` | `t4_standard_csv.v1` | `accounts.csv`, `cex_assets.csv`, `tier_ratios.csv` |

Standard CSV 의 금액 필드는 이미 scale 된 non-negative integer 여야 한다.
엔진 runtime 은 거래소 decimal 문자열, wide raw export, 고객별 컬럼명을
파싱하지 않는다.

## 필수 profile 필드

`[profile]`

- `name`: 사람이 읽을 수 있는 descriptor 이름.
- `model`: solvency model id. 선택한 standard connector 와 일치해야 한다.
- `asset_capacity`: trusted setup 에 사용한 asset slot capacity. 양수여야 하며
  최소 `len([catalog].symbols)` 이상이어야 한다.

`[identity]`

- `scheme`: account-id derivation scheme. V1 은
  `passthrough_hex_bn254_reduced.v0`.

`[insolvent]`

- `action`: invalid account policy. V1 은 `drop_and_log.v0`.

`[constraint]`

- `module`: constraint module id. 빈 문자열은 model default noop 을 뜻한다.

`[snapshot]`

- `source_type`: 위 표의 `t*_standard_csv.v1` connector id 중 하나.
- `user_data_dir`: 해당 snapshot 의 canonical standard CSV 파일들이 있는
  디렉터리.
- `snapshot_id`: operator 가 부여하는 snapshot label 또는 timestamp.

`[snapshot.format]`

- `core/snapshot/csv` 와 공유하는 optional CSV dialect 설정.
- 일반 기본값: `null_values = ["", "NA", "null"]`.

`[[batch_shapes]]`

- `asset_count_tier`: 해당 proving tier 에서 account 당 허용하는 최대
  non-empty asset 수.
- `users_per_batch`: 해당 tier 의 circuit batch 당 user 수.
- 최소 하나 이상의 shape 를 제공해야 한다. builder 는 tier 를 정렬해서
  사용한다.

`[pricing]`

- `default_price_scale`, `default_balance_scale` 은 양수여야 한다.
- `two_digit_assets`, `two_digit_price_scale`, and
  `two_digit_balance_scale` 은 optional 이다. `two_digit_assets` 가 비어 있지
  않으면 두 two-digit scale 모두 양수여야 한다.
- `BuildPricing` 은 G6 invariant 를 검증한다.
  `default_price_scale * default_balance_scale ==
  two_digit_price_scale * two_digit_balance_scale`.

`[catalog]`

- `symbols`: auditor 와 verifier-side user 에게 공개 가능한 reference symbol
  list.
- per-snapshot committed asset order 는 canonical `cex_assets.csv` 에서 온다.

## 최소 T4 예시

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

## 작성 체크리스트

1. 먼저 solvency model 을 고르고, 그 model 과 일치하는
   `t*_standard_csv.v1` connector 를 선택한다.
2. `asset_capacity` 를 trusted setup 과 service 에서 사용할 capacity 로 맞춘다.
3. 예상되는 user 당 최대 non-empty asset 수를 덮는 batch shape 를 정의한다.
4. 새 registry entry 를 의도적으로 추가한 경우가 아니라면
   `identity.scheme`, `insolvent.action` 은 frozen V1 id 를 유지한다.
5. `snapshot.user_data_dir` 에 canonical standard CSV 파일을 생성한다.
6. 검증 명령을 실행한다.

```bash
go test ./profile/declarative
go build ./zkpor/...
go vet ./zkpor/...
```

제한된 sandbox 안에서 실행할 때는 `GOCACHE` 를 writable path 로 지정한다.
예: `GOCACHE=/private/tmp/zkpor-gocache`.
