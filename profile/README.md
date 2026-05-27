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

## raw export 매핑 위치

Profile TOML 은 "거래소 raw 컬럼 A 를 engine 필드 B 로 매핑한다"는 ETL
설정 파일이 아니다. Profile 은 이미 정규화된 standard snapshot 을 어떤
model 로 읽을지 고르는 deployment descriptor 이다.

운영 흐름은 다음과 같다.

```text
거래소 raw export
  -> customer/operator preprocessor
  -> model-standard CSV directory
  -> zkpor profile.toml + standard snapshot connector
  -> witness/proof
```

따라서 특정 거래소를 붙일 때 작성해야 하는 매핑은 `profile/<customer>/`
Go package 가 아니라 외부 preprocessor 에 둔다. 그 preprocessor 의 책임은
다음 값들을 model-standard CSV 로 만드는 것이다.

- 고객 raw account/user identifier 를 공개된 scheme 에 따라 64-hex
  `account_id` 로 변환한다. 현행 V1 profile 의
  `passthrough_hex_bn254_reduced.v0` 은 standard CSV 의 `account_id` 가 이미
  32-byte hex 라는 뜻이다.
- 거래소별 symbol, product account, wallet bucket, collateral bucket 을
  선택한 solvency model 의 field 로 접는다.
- decimal balance/price 를 profile pricing multiplier 로 scale 해서
  base-10 integer 문자열로 쓴다.
- wide export 는 `(account_id, asset_index)` grain 의 long-form
  `accounts.csv` 로 펼친다.
- auditor/public statement 와 맞는 per-asset totals 를 `cex_assets.csv` 로
  쓴다.
- tier haircut model(T3/T4)은 tier curve 를 `tier_ratios.csv` 로 쓴다.

CSV header 는 아래 schema 와 정확히 일치해야 한다. 기본 reader 는 unknown
column, duplicate header, duplicate primary key 를 reject 한다.

## model-standard CSV schema

공통 규칙:

- 모든 금액과 가격은 non-negative integer 이며 decimal 문자열은 허용하지
  않는다.
- `equity`, `debt`, collateral 계열, `total_equity`, `total_debt` 는
  balance-scaled amount 이고, `base_price` 는 price-scaled quote price 이다.
- `account_id` 는 64-hex 문자열이다. 엔진은 이를 BN254 field element 로
  reduce/canonicalize 한 뒤 account leaf hash 에 사용한다.
- `account_index` 는 optional 이다. header 에 넣었으면 한 account 안에서
  고정되어야 하고, 완전히 생략하면 parser 가 first-seen valid account order 로
  dense Merkle leaf order 를 파생한다.
- `asset_index` 는 `[0, profile.asset_capacity)` 범위의 catalog slot 이다.
- `cex_assets.csv` 는 real asset row 만 넣는다. parser 가 나머지 capacity 를
  `reserved` zero slot 으로 padding 한다.

### T1 `t1_simple_margin`

`accounts.csv`

```csv
account_index,account_id,asset_index,equity,debt
```

`cex_assets.csv`

```csv
asset_index,symbol,total_equity,total_debt,base_price
```

Spot 거래소는 `debt = 0`, `total_debt = 0` 으로 T1 을 사용할 수 있다.
parser 는 account rows 로 per-user `TotalEquity`, `TotalDebt` 를 계산하고
회로는 `TotalEquity >= TotalDebt` 를 검증한다.

### T2 `t2_static_haircut_margin`

`accounts.csv`

```csv
account_index,account_id,asset_index,equity,debt,collateral
```

`cex_assets.csv`

```csv
asset_index,symbol,total_equity,total_debt,base_price,collateral,haircut_bp
```

`collateral <= equity` 여야 하고, `haircut_bp` 는 `[0, 10000]` 범위의 static
haircut basis points 이다. parser 는
`sum(collateral * base_price * haircut_bp / 10000)` 으로 per-user
`TotalCollateral` 을 계산한다.

### T3 `t3_tiered_haircut_margin_1pool`

`accounts.csv`

```csv
account_index,account_id,asset_index,equity,debt,collateral
```

`cex_assets.csv`

```csv
asset_index,symbol,total_equity,total_debt,base_price,collateral
```

`tier_ratios.csv`

```csv
asset_index,tier_index,boundary_value,ratio,precomputed_value
```

T3 는 single collateral pool 에 asset별 tier curve 를 적용한다.
`tier_index` 는 asset별 dense sequence 이고, `boundary_value` 는 strictly
increasing 이어야 한다. `precomputed_value` 는 audited circuit 의 cumulative
tier function 과 일치해야 한다.

### T4 `t4_tiered_haircut_margin_3pool`

`accounts.csv`

```csv
account_index,account_id,asset_index,equity,debt,loan_collateral,margin_collateral,portfolio_margin_collateral
```

`cex_assets.csv`

```csv
asset_index,symbol,total_equity,total_debt,base_price,loan_collateral,margin_collateral,portfolio_margin_collateral
```

`tier_ratios.csv`

```csv
asset_index,collateral_pool,tier_index,boundary_value,ratio,precomputed_value
```

`collateral_pool` 은 `loan`, `margin`, `portfolio_margin` 중 하나다.
각 `(asset_index, collateral_pool)` 별로 독립적인 dense tier curve 를 제공한다.
row 단위로
`loan_collateral + margin_collateral + portfolio_margin_collateral <= equity`
여야 한다. parser 는 pool별 tier curve 를 적용한 뒤 합산해서 per-user
`TotalCollateral` 을 계산한다.

## scaling 예시

raw 값이 `BTC balance = 1.23456789`, `BTC price = 65000.12` 이고 기본 scale
`1e8 x 1e8` 을 사용한다면 preprocessor 는 다음 integer 를 CSV 에 쓴다.

```text
equity     = 123456789
base_price = 6500012000000
```

`TotalEquity`, `TotalDebt`, `TotalCollateral` 의 value unit 은
`price_scale * balance_scale` 이다. 모든 symbol 에 대해 이 product 가 같아야
서로 다른 asset 을 같은 quote-currency 단위로 합산할 수 있다.

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
- R10 product runtime 은 canonical standard CSV 를 기본 dialect 로 읽는다.
  고객 raw export 의 dialect/column mapping 은 preprocessor 단계에서
  처리하는 것이 정상 경로다.

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

## 추가회로가 추가 입력을 요구할 때

추가회로(alpha module)는 열려 있지만, 입력 확장은 아무 CSV 에나 임의 열을
붙이는 방식으로 열려 있지 않다. 현재 `t*_standard_csv.v1` connector 는
model-standard schema 를 기준으로 동작한다. 기본 reader 는 schema 에 없는
column 과 file 을 회로 입력으로 해석하지 않는다.

추가회로를 붙일 때는 필요한 입력 수준에 따라 경로가 갈린다.

1. **기존 witness 로 검증 가능한 rule**
   - 예: account total, CEX total, collateral total, 공개된 tier curve 로
     표현 가능한 추가 제약.
   - 새 `ConstraintModule` 을 model host registry 에 등록하고
     `[constraint].module` 에 그 id 를 쓴다.
   - CSV schema 는 그대로 둔다.
2. **새 per-account/per-asset private input 이 필요한 rule**
   - 예: 거래소별 위험등급, product bucket, account group, 특수 collateral
     carve-out 이 회로 안에서 직접 검증되어야 하는 경우.
   - `t4_standard_csv.v1` 에 column 을 몰래 추가하지 않는다.
   - 가능하면 아래 일반 alpha sidecar schema 로 입력을 운반한다.
   - 그 sidecar 를 실제 witness/`ConstraintContext` 로 투영하는 새
     versioned snapshot connector 를 만든다. 예:
     `t4_<rule>_standard_csv.v1`.
   - 필요한 값이 현재 `ConstraintContext` 에 없으면 회로 surface 를 함께
     확장해야 한다.
   - 이 변경은 `(model, batch_shape, asset_capacity, module)` 뿐 아니라
     witness/schema 계약까지 바꾸므로 별도 audit 과 trusted setup 이 필요하다.
3. **public parameter 만 필요한 rule**
   - 예: 한도값, jurisdiction code, 정책 threshold.
   - 가능하면 CSV private input 이 아니라 public input 으로 둔다.
   - parameter 값을 in-circuit constant 로 박으면 값이 바뀔 때마다 ceremony 가
     갈라지므로 피한다.
4. **회로와 무관한 audit/preprocessor metadata**
   - raw export 단계에서는 어떤 부가 파일이나 column 을 써도 된다.
   - 단, zkpor `snapshot.user_data_dir` 로 들어오는 standard CSV 에는 회로와
     public statement 에 필요한 canonical field 만 남긴다.

즉, 추가 입력까지 필요한 customer-specific 회로는
`[constraint].module` 하나만으로 끝나지 않는다. 보통 다음을 한 묶음으로
버전 관리한다.

- 새 `constraint.module` id
- 새 `snapshot.source_type` id
- 추가 schema/parser 또는 `SnapshotSource`
- 필요 시 `ConstraintContext` / witness struct 확장
- 새 `.pk/.vk` ceremony

### 일반 alpha sidecar schema

임의의 module field 를 표현하기 위한 공통 sidecar 는 EAV 형태다. CSV header
자체를 customer 별로 열어두지 않고, `field_name` 과 `value` row 로 임의
입력을 운반한다. 이 계약은 `core/snapshot/schema.StandardAlphaSchema`
(`alpha_sidecar.v1`) 에 고정되어 있다.

`alpha_manifest.csv`

```csv
module_id,scope,field_name,field_type,required,description
```

`alpha_values.csv`

```csv
module_id,scope,subject,field_name,value
```

`scope` 와 `subject` 규칙:

| `scope` | `subject` |
|---|---|
| `snapshot` | literal `snapshot` |
| `asset` | decimal `asset_index` |
| `account` | 64-hex `account_id` |
| `account_asset` | `<account_id>:<asset_index>` |

예시:

```csv
# alpha_manifest.csv
module_id,scope,field_name,field_type,required,description
regulator.kr.user_limit_v1,account,daily_limit,uint64,1,per-account limit
```

```csv
# alpha_values.csv
module_id,scope,subject,field_name,value
regulator.kr.user_limit_v1,account,0000000000000000000000000000000000000000000000000000000000000001,daily_limit,100000000
```

주의: sidecar schema 는 transport 표준이다. 이 파일이 존재한다고 해서 값이
자동으로 회로에 들어가지는 않는다. 해당 `constraint.module` 과
`snapshot.source_type` 이 그 sidecar 를 읽고, typed witness 또는
`ConstraintContext` 로 값을 전달하도록 명시적으로 구현되어야 한다.

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
2. 거래소 raw export 를 위 model-standard CSV schema 로 변환하는
   preprocessor 를 준비한다.
3. `asset_capacity` 를 trusted setup 과 service 에서 사용할 capacity 로 맞춘다.
4. 예상되는 user 당 최대 non-empty asset 수를 덮는 batch shape 를 정의한다.
5. 새 registry entry 를 의도적으로 추가한 경우가 아니라면
   `identity.scheme`, `insolvent.action` 은 frozen V1 id 를 유지한다.
6. `snapshot.user_data_dir` 에 canonical standard CSV 파일을 생성한다.
7. 검증 명령을 실행한다.

```bash
go test ./profile/declarative
go build ./zkpor/...
go vet ./zkpor/...
```

제한된 sandbox 안에서 실행할 때는 `GOCACHE` 를 writable path 로 지정한다.
예: `GOCACHE=/private/tmp/zkpor-gocache`.
