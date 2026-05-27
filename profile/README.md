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

- `module`: alpha-layer constraint module id. 빈 문자열은 model default noop 을
  뜻한다. 자세한 매핑 기준은 아래 "추가회로(alpha) 매핑"을 따른다.

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

## 추가회로(alpha) 매핑

Profile 에서 회로를 고르는 설정은 세 단계다.

1. **default**: `[constraint].module = ""`
   - `corespec.NoExtensionID` 이며, registry lookup 을 하지 않는다.
   - 선택한 model host 가 `NewNoopConstraint()` 를 반환한다.
   - base solvency circuit 외 추가 제약은 없다.
   - key stem 은 module suffix 가 없는 `zkpor.<model>.<tier>_<users>` 형식이다.
2. **solvency model**: `[profile].model`
   - base circuit, public statement 의미, standard snapshot connector 를 고른다.
   - alpha module 타입도 model 별로 고정된다. 예를 들어 T1 module 은 T4
     `ConstraintContext` 에 연결할 수 없다.
   - `snapshot.source_type` 은 선택한 model 의 `t*_standard_csv.v1` 과
     일치해야 한다.
3. **alpha module**: non-empty `[constraint].module`
   - 선택한 model 의 host registry 에서 같은 id 의 `ConstraintModule` 을
     찾는다.
   - module 은 add-only 이다. base solvency model 의 제약을 제거하거나
     약화할 수 없다.
   - key stem 은 `zkpor.<model>.<tier>_<users>.<module>` 형식이므로 같은
     `(model, asset_capacity, batch_shape, module)` tuple 에서만 `.pk/.vk`
     호환을 기대할 수 있다.
   - engine binary 에 module 이 link/register 되어 있어야 하며, 비-noop
     module 은 별도 trusted setup 과 audit/source 공개가 필요하다.

현재 product profile path 에서 활성화된 alpha registry 는 다음과 같다.

| `profile.model` | default module | non-empty `constraint.module` 상태 |
|---|---|---|
| `t1_simple_margin` | `t1host.NewNoopConstraint()` | T1 host registry 에 등록된 T1-typed module 만 사용 가능 |
| `t4_tiered_haircut_margin_3pool` | `t4host.NewNoopConstraint()` | T4 host registry 에 등록된 T4-typed module 만 사용 가능 |
| `t2_static_haircut_margin` | noop only | 회로 hook 은 있으나 product profile registry path 는 아직 열지 않는다 |
| `t3_tiered_haircut_margin_1pool` | noop only | 회로 hook 은 있으나 product profile registry path 는 아직 열지 않는다 |

V1 catalog 에 등록된 비-noop alpha module 은 아직 없다. 따라서 현행 profile
TOML 은 새 module 을 의도적으로 추가하고 ceremony 를 분기하기 전까지
`module = ""` 를 유지한다.

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
