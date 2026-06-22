# 운영자 가이드 — 솔벤시 모델 T1–T4 와 테넌트 프로파일 작성

이 문서는 **운영자(operator)** 가 테넌트 거래소를 zkpor 엔진에 온보딩할 때,
그 거래소의 사업 모델에 맞는 **솔벤시 모델(T1–T4)** 을 고르고 **프로파일과
표준 CSV 를 올바르게 채우도록** 돕는 실무 가이드다. 마지막 [§12 용어 풀이](#12-용어-풀이-glossary)
에 모든 용어를 모았다.

이 문서는 *운영자 관점의 설명서* 다. 계약(스펙)의 권위 있는 출처는 아니며,
충돌 시 아래 순서가 우선한다.

| 우선순위 | 출처 | 다루는 것 |
|---:|---|---|
| 1 | `core/spec/`, `core/solvency/<model>/` 코드 (frozen 계약) | model ID, leaf shape, 회로 제약 |
| 2 | `docs/04-solvency-models.md` | model spec 의 *결정 근거* 와 산업 reference |
| 3 | `profile/README.md` | profile descriptor 필드의 정밀 reference + CSV schema |
| 4 | **이 문서** | "테넌트 사업모델 → model 선택 → 필드 매핑" 운영 가이드 |

---

## 1. 한눈에 보기

거래소가 **무엇을 하는가**로 model 이 정해진다. 핵심 질문 하나: *사용자가
돈을 빌릴 수 있는가(부채/마진/론), 빌릴 수 있다면 담보를 어떻게 위험가중하는가.*

| 테넌트 사업 모델 | 모델 | 마케팅 tier | model ID |
|---|---|---|---|
| **현물(spot) 전용** — 대출/마진 없음. 한국·EU·일본·SEA 규제 거래소, 스테이블코인 발행처, 커스터디 | **T1** | Basic | `t1_simple_margin` |
| **단순 마진** — 빌릴 수 있지만 담보 위험가중은 안 함(계정 단위 net 만) / 또는 기존 Merkle-PoR 의 zk 격상 | **T1** | Standard | `t1_simple_margin` |
| **마진 + 자산별 고정 haircut** — 담보 1 pool, 자산마다 *상수* 위험가중치 (BTC 90%, 알트 50% …) | **T2** | Pro-A | `t2_static_haircut_margin` |
| **마진 + 규모별 tier haircut** — 담보 1 pool, 포지션이 클수록 위험가중치↑ (piecewise-linear 곡선) | **T3** | Pro-B | `t3_tiered_haircut_margin_1pool` |
| **VIP 론 + 크로스 마진 + 포트폴리오 마진을 분리 회계** — 담보 3 pool, 각 pool 별 tier 곡선 | **T4** | Enterprise | `t4_tiered_haircut_margin_3pool` |

> **가장 흔한 두 경우**
> - 규제 spot 거래소 → **T1, `debt = 0`**. 가장 단순, 통합 난이도 최저.
> - Binance/OKX 급 풀서비스 거래소 → **T4** (3-pool). 이 엔진 회로의 원형.

상승 순서 `T1 → T2 → T3 → T4` 는 *검증 풍부도(verification richness)* 와
*회로 비용* 이 함께 오르는 방향이다. 회로 비용 대략 `1× → 1.3× → 7× → 19×`
(NbConstraints 기준, [docs/04 §10](04-solvency-models.md)).

**과대 선택을 피하라.** 담보 위험가중이 필요 없는 거래소에 T4 를 쓰면 불필요한
회로 비용·증명 시간을 강제한다. 테넌트의 *실제* 사업 라인에 가장 *작은* 모델을
고르는 것이 원칙이다.

---

## 2. 운영 데이터 흐름 (operator 가 책임지는 경계)

```
거래소 raw export (각사 컬럼명/소수점 문자열/wide CSV)
   │   ← 운영자/전처리기(preprocessor) 책임
   ▼
model-standard 정규화 CSV  (accounts.csv · cex_assets.csv [· tier_ratios.csv])
   │   ← zkpor profile.toml + standard snapshot connector
   ▼
witness → proof (.pk/.vk) → verifier
```

R10 이후 **zkpor 엔진은 거래소 raw export 를 직접 파싱하지 않는다.** 엔진이
읽는 것은 *이미 정규화된* model-standard CSV 다. 따라서 거래소별 매핑(컬럼명,
소수점 스케일링, 담보 버킷 접기)은 `profile/<customer>/` Go 코드가 아니라
**외부 전처리기**에 둔다. profile TOML 은 "이 정규화된 스냅샷을 어떤 model 로
읽을지" 고르는 *deployment descriptor* 일 뿐이다.

운영자가 책임지는 3가지:
1. 거래소 사업모델 → **올바른 model 선택** (이 문서 §1, §11).
2. raw export → **model-standard CSV 변환** 전처리기 (§4 각 모델의 필드 매핑).
3. **profile.toml 작성** + asset_capacity / batch_shape / pricing 설정 (§6–7).

---

## 3. 모든 모델의 공통 보장 (baseline)

### 3.1 엔진이 강하게 보장하는 것

> **"주어진 user balance dataset 이 published CEX totals 와 산술적으로
> 일치하며, 선택한 솔벤시 모델의 각 사용자 조건을 통과한다."**

구체적으로 4개 model 모두 다음을 회로 안에서 강제한다.

1. **계정 leaf 무결성** — 각 사용자가 5-input Poseidon leaf 로 Merkle tree 에
   올바른 위치에 들어있다 (`AccountLeaf = Poseidon(AccountID, TotalEquity,
   TotalDebt, TotalCollateral, AssetsCommitment)`).
2. **per-asset 합계 일치(sum equality)** — 사용자별 잔액을 자산마다 더하면
   `cex_assets.csv` 의 published total 과 정확히 같다.
3. **per-user 솔벤시 조건** — model 마다 다름 (§4).
4. **Merkle root before/after 일관성** — 사용자 op 가 tree 를 올바르게 갱신.

### 3.2 zk 가 보장하지 *않는* 것

- **거래소가 dataset 에 포함시키지 않은 사용자/부채의 존재.** zk 는 *제출된*
  데이터의 내부 일관성만 증명한다. 누락된 부채는 보이지 않는다.
  - 보완책: 사용자 self-inclusion verifier + AccountIDProvider scheme 공개 +
    외부 audit. 이건 zk 밖의 운영/거버넌스 책임이다.

### 3.3 통일된 account leaf — slot 의미만 model 별

4개 model 모두 동일한 5-input leaf 를 쓴다. 슬롯에 들어가는 *값의 의미*만 다르다.

| slot | T1 | T2 | T3 | T4 |
|---|---|---|---|---|
| `AccountID` | 사용자 canonical id (BN254 field element) | 동일 | 동일 | 동일 |
| `TotalEquity` | Σ(`equity` × `base_price`) — **quote 통화로 환산한** 자산 합 | 동일 | 동일 | 동일 |
| `TotalDebt` | Σ(`debt` × `base_price`) — 환산한 부채 합 | 동일 | 동일 | 동일 |
| `TotalCollateral` | **0** (위험가중 담보 없음) | Σ 정적-haircut 담보가치 | Σ tier곡선 담보가치 | Σ 3-pool tier곡선 담보가치 |
| `AssetsCommitment` | per-asset (Equity, Debt) flat commitment | +Collateral | +Collateral | +Loan, Margin, PM |

> **중요 — 모든 금액은 quote 통화 가치로 환산된다.** `TotalEquity`/`TotalDebt`
> 는 raw 수량이 아니라 `수량 × base_price` 다. 서로 다른 자산을 같은 단위로
> 더하려면 모든 자산의 `price_scale × balance_scale` 곱(= **ValueScale**)이
> 같아야 한다 (§7).

---

## 4. 모델별 상세

각 모델을 같은 틀로 설명한다: **정의 → 어떤 테넌트 → 솔벤시 식(회로가 검증) →
입력 필드 → 테넌트 데이터 매핑 → 주의점 → 미니 예시**.

### 4.1 T1 — Simple Margin (현물 친화)

**정의.** 위험가중 담보가 *없는* 가장 단순한 모델. 계정 단위로 "자산 ≥ 부채"만
검증한다. `debt = 0` 으로 공급하면 현물 전용 거래소를 그대로 처리한다(조건이
자명하게 만족됨 → spot 흡수).

**어떤 테넌트.**
- **Basic**: 한국/EU/일본/SEA 규제 spot 거래소, 스테이블코인 발행처, 커스터디.
  → `debt = 0`, `total_debt = 0`.
- **Standard**: Bybit/KuCoin/HTX 급 mid-tier 마진 거래소가 기존 off-chain
  Merkle-PoR 를 zk 로 격상. → `debt` 활성.

**솔벤시 식 (회로 검증).**
```
∀ user :  TotalDebt ≤ TotalEquity          (둘 다 quote 가치 환산)
∀ asset :  Σ_user(equity) == total_equity,  Σ_user(debt) == total_debt
```
자산별 차용(per-asset `debt > equity`)은 허용 — *계정 단위* net 만 본다.

**입력 파일.** `accounts.csv`, `cex_assets.csv`

```csv
# accounts.csv
account_index,account_id,asset_index,equity,debt
# cex_assets.csv
asset_index,symbol,total_equity,total_debt,base_price
```

**테넌트 데이터 매핑.**
| engine 필드 | 테넌트 데이터에서 채우는 값 |
|---|---|
| `account_id` | 사용자/계정 식별자를 공개 scheme 으로 64-hex 화 (`passthrough_hex_bn254_reduced.v0` → 이미 32-byte hex 라는 뜻) |
| `asset_index` | catalog slot 번호 `[0, asset_capacity)` |
| `equity` | 그 자산에 대한 사용자 청구가능 수량 (balance-scaled 정수) |
| `debt` | 그 자산에 대한 사용자 부채 수량. **현물이면 0** |
| `base_price` | 그 자산의 quote 통화 가격 (price-scaled 정수) |

**주의점.** 현물 거래소는 `debt`/`total_debt` 컬럼을 전부 0 으로 채운다.
컬럼 자체는 schema 에 존재해야 한다(생략 불가).

**미니 예시 (현물).** 사용자가 1.5 BTC, 1000 USDT 보유, 부채 없음:
```csv
account_index,account_id,asset_index,equity,debt
0,1111...1111,0,150000000,0          # 1.5 BTC (balance scale 1e8)
0,1111...1111,2,100000000000,0       # 1000 USDT
```

---

### 4.2 T2 — Static-Haircut Margin (자산별 고정 haircut)

**정의.** 담보 1 pool + 자산마다 *상수* haircut(basis points). 사용자가 빌린
부채를 *위험가중한 담보가치* 가 덮는지 검증한다. T1 에 `collateral` 필드와
`RiskPolicy.Haircut(symbol)` 이 추가된 형태.

**어떤 테넌트.** 마진 사업을 하지만 위험모델이 단순한 거래소 — 담보 pool 이
하나이고, haircut 이 포지션 크기와 무관한 자산-level 상수. (Aave V3 의 자산별
고정 LTV 가 in-circuit reference.)

**솔벤시 식 (회로 검증).**
```
∀ user :  TotalDebt  ≤  Σ_i ( collateral_i × base_price_i × haircut_bp_i / 10000 )
∀ asset, row :  collateral ≤ equity              (row-level pre-check)
```
> ⚠ **마진 모델의 핵심:** 부채는 `equity` 가 아니라 **위험가중 담보**로 검증된다.
> `collateral` 을 0 으로 두고 `debt > 0` 이면 그 사용자는 솔벤시 조건에서 탈락한다.

**입력 파일.** `accounts.csv`, `cex_assets.csv` (haircut 은 cex_assets 에)

```csv
# accounts.csv
account_index,account_id,asset_index,equity,debt,collateral
# cex_assets.csv
asset_index,symbol,total_equity,total_debt,base_price,collateral,haircut_bp
```

**테넌트 데이터 매핑.** T1 매핑 + 다음 2개.
| engine 필드 | 테넌트 데이터 |
|---|---|
| `collateral` (accounts) | 그 자산 중 담보로 인정되는 수량. **`collateral ≤ equity`** 필수 |
| `haircut_bp` (cex_assets) | 자산별 위험가중치, basis points `[0, 10000]`. **10000 = 100%(haircut 없음), 0 = 담보 불인정** |

**haircut_bp 감각.** `9000` = 담보 가치의 90% 만 인정(=10% haircut).
스테이블코인 ~9500–10000, BTC/ETH ~8000–9000, 변동성 큰 알트 ~3000–6000 식으로
테넌트의 risk team 이 정한 값을 그대로 옮긴다.

**주의점.**
- `haircut_bp` 는 자산마다 하나의 상수다. 규모별로 달라야 하면 T3 로 올라가야 한다.
- haircut 정책은 audit/authorization 대상이다. [§6.3 policy commitment](#63-위험정책-policy-commitment-감사인가) 참조.

---

### 4.3 T3 — Tiered-Haircut Margin · 1 Pool (규모별 tier 곡선)

**정의.** 담보 1 pool + 자산마다 *piecewise-linear tier 곡선*. 같은 자산이라도
담보 규모가 클수록 한계 위험가중치가 달라진다(보통 큰 포지션일수록 낮은 인정률).
haircut 이 *상수* → *함수* 로 바뀐 것이 T2 와의 유일한 차이.

**어떤 테넌트.** 파생상품 비중이 높아 규모별 위험관리(maintenance/initial margin
tier)를 쓰지만, 담보를 사업 라인별로 분리회계하지는 *않는* 거래소. (dYdX IMF
곡선, Bitget/Gate tiered MMR 이 reference.)

**솔벤시 식 (회로 검증).**
```
∀ user :  TotalDebt  ≤  Σ_i  haircut_curve_i( collateral_i 의 가치 )
∀ asset, row :  collateral ≤ equity
```
`haircut_curve_i` 는 자산별 tier 표(boundary/ratio)로 정의되는 단조 piecewise-linear
함수다.

**입력 파일.** `accounts.csv`, `cex_assets.csv`, **`tier_ratios.csv`**

```csv
# accounts.csv   (T2 와 동일)
account_index,account_id,asset_index,equity,debt,collateral
# cex_assets.csv  (haircut_bp 없음)
asset_index,symbol,total_equity,total_debt,base_price,collateral
# tier_ratios.csv
asset_index,tier_index,boundary_value,ratio,precomputed_value
```

**tier_ratios.csv 필드 의미.**
| 필드 | 의미 |
|---|---|
| `tier_index` | 자산별 dense 순번 (0,1,2,…) |
| `boundary_value` | 그 tier 의 *상한* (담보 가치, value-scale 단위). 자산 안에서 **strictly increasing** |
| `ratio` | 그 tier 구간의 한계 인정률, 퍼센트 `[0, 100]` (예: `100` = 100% 인정) |
| `precomputed_value` | boundary 까지의 *누적* 인정 담보가치. **반드시 audited 회로 공식과 일치** |

> **`precomputed_value` 를 손으로 계산하지 마라.** 한 비트만 어긋나도 증명이
> 늦게 실패하거나 audited 회로 의미에서 벗어난다. 엔진의 단일 진리원천
> **`core/tierpolicy/tierpolicy.go::BuildTierCurve`** 가 `(boundary, ratio)` →
> `precomputed` 를 회로와 byte 단위로 동일하게 생성한다. 전처리기에서 이 함수를
> 호출해 `tier_ratios.csv` 를 만든다.
>
> 누적 공식(참고용, 검증은 BuildTierCurve 가):
> ```
> precomputed[0] = floor(boundary[0] × ratio[0] / 100)
> precomputed[i] = precomputed[i-1] + floor((boundary[i]-boundary[i-1]) × ratio[i] / 100)
> ```

**제약(회로 range-check).** tier 수 ≤ `TierCount`(=12), `0 ≤ ratio ≤ 100`,
boundary `≤ 2^118`, boundary strictly increasing.

---

### 4.4 T4 — Tiered-Haircut Margin · 3 Pool (Binance 급)

**정의.** 담보를 **3개 독립 pool — `loan` / `margin` / `portfolio_margin`** 으로
나누고 각 pool 마다 별도 tier 곡선을 적용한다. 한 pool 의 담보를 다른 pool 의
부채에 쓸 수 없다(분리회계). 이 엔진 회로의 원형(Binance OSS PoR v2 이식).

**어떤 테넌트.** VIP 론 + 크로스 마진 + 포트폴리오 마진을 *별도 회계* 로
운영하는 Binance/OKX 급 거래소.

**솔벤시 식 (회로 검증).**
```
∀ user :  TotalDebt  ≤  Σ_{pool∈{loan,margin,pm}}  Σ_i  haircut_curve_{pool,i}( collateral_{pool,i} 의 가치 )
∀ asset, row :  loan_collateral + margin_collateral + portfolio_margin_collateral ≤ equity
```

**입력 파일.** `accounts.csv`, `cex_assets.csv`, **`tier_ratios.csv`** (pool 컬럼 추가)

```csv
# accounts.csv
account_index,account_id,asset_index,equity,debt,loan_collateral,margin_collateral,portfolio_margin_collateral
# cex_assets.csv
asset_index,symbol,total_equity,total_debt,base_price,loan_collateral,margin_collateral,portfolio_margin_collateral
# tier_ratios.csv
asset_index,collateral_pool,tier_index,boundary_value,ratio,precomputed_value
```

**T3 와의 차이.**
- `collateral` 한 컬럼 → `loan_collateral` / `margin_collateral` /
  `portfolio_margin_collateral` 세 컬럼.
- `tier_ratios.csv` 에 **`collateral_pool`** 컬럼 추가. 값은 closed enum
  `loan` · `margin` · `portfolio_margin`. `(asset_index, collateral_pool)` 마다
  독립적인 dense tier 곡선을 둔다.
- 행 단위 합계 제약이 세 pool 합 `≤ equity`.

**테넌트 데이터 매핑.** 테넌트의 각 담보 버킷(VIP 론 담보 / 크로스마진 담보 /
포트폴리오마진 담보)을 정확히 해당 pool 컬럼으로 접는다. 버킷이 한두 개뿐이면
나머지 pool 은 0 으로 두면 된다(그 pool 곡선은 빈 곡선이어도 됨).

**미니 예시 (loan pool 만 사용, ratio 100% 단일 tier).**
```csv
# accounts.csv
account_id,asset_index,equity,debt,loan_collateral,margin_collateral,portfolio_margin_collateral
1111...1111,0,10,0,10,0,0
# tier_ratios.csv
asset_index,collateral_pool,tier_index,boundary_value,ratio,precomputed_value
0,loan,0,100000000000000000000,100,100000000000000000000
0,margin,0,100000000000000000000,100,100000000000000000000
0,portfolio_margin,0,100000000000000000000,100,100000000000000000000
```

---

## 5. 모델 간 차이 요약

| 차원 | T1 | T2 | T3 | T4 |
|---|:---:|:---:|:---:|:---:|
| per-user 부채 검증 | `debt ≤ equity` | `debt ≤ 담보가치` | `debt ≤ 담보가치` | `debt ≤ 담보가치` |
| 담보 위험가중 | 없음 | 자산별 *상수* haircut | 자산별 *tier 곡선* | pool별 tier 곡선 |
| haircut 규모 의존 | — | ❌ | ✅ | ✅ |
| 담보 pool 수 | 0 | 1 | 1 | **3** (loan/margin/pm) |
| `accounts.csv` 추가 컬럼 | — | `collateral` | `collateral` | `loan/margin/pm_collateral` |
| `tier_ratios.csv` 필요 | ❌ | ❌ | ✅ | ✅ (+`collateral_pool`) |
| 위험정책 공급원 | 없음 | `haircut_bp` (cex_assets) | tier 곡선 | pool별 tier 곡선 |
| 상대 회로 비용 | 1× | ~1.3× | ~7× | ~19× |

---

## 6. 프로파일 TOML 작성

새 테넌트는 가장 가까운 `profile/t{1,2,3,4}_reference/*.toml` 을
`profile/<customer>/<customer>.toml` 로 복사해 시작한다.
`profile/<customer>/` 아래에 `snapshot.go` 나 raw ETL 코드를 두지 않는다.

### 6.1 필드별 의미

```toml
[profile]
name           = "example_exchange"                 # 사람이 읽는 descriptor 이름
model          = "t4_tiered_haircut_margin_3pool"   # §1 의 model ID — connector 와 일치 필수
asset_capacity = 500                                # trusted setup 의 asset slot 수. > 0, ≥ len(catalog.symbols)

[identity]
scheme = "passthrough_hex_bn254_reduced.v0"         # V1 frozen. account_id 가 이미 32-byte hex

[insolvent]
action = "drop_and_log.v0"                          # V1 frozen. invalid 계정은 drop 후 로깅

[constraint]
module = ""                                         # 빈 문자열 = 추가회로 없음(noop). §6.4

[snapshot]
source_type   = "t4_standard_csv.v1"                # model 의 t*_standard_csv.v1 connector
user_data_dir = "/standard-data/<snapshot_id>"      # 정규화 CSV 들이 있는 디렉터리
snapshot_id   = "<set per snapshot>"                # 운영자가 부여하는 라벨/타임스탬프

[snapshot.format]
null_values = ["", "NA", "null"]

[[batch_shapes]]                                    # 최소 1개. §6.2
asset_count_tier = 50                               # 그 tier 에서 계정당 허용하는 non-empty 자산 최대 수
users_per_batch  = 700                              # 그 tier 의 batch 당 user 수

[[batch_shapes]]
asset_count_tier = 500
users_per_batch  = 92

[pricing]                                           # §7
default_price_scale   = 100000000                   # 1e8
default_balance_scale = 100000000                   # 1e8

[catalog]
symbols = []                                        # auditor 공개용 reference. 실제 순서는 cex_assets.csv
```

| 블록 | model 별 차이 |
|---|---|
| `[profile].model` + `[snapshot].source_type` | 반드시 같은 model 의 짝 (`t1_simple_margin` ↔ `t1_standard_csv.v1` …) |
| `[[batch_shapes]]` | model 무관 구조. 단 T3/T4 는 tier 비용이 커서 같은 shape 라도 keygen 시간이 길다 |
| `[pricing]` | model 무관. value-scale 불변식은 §7 |
| `tier_ratios.csv` 존재 | T3/T4 만. profile TOML 이 아니라 `user_data_dir` 의 파일 |

### 6.2 batch_shapes 정하기

- `asset_count_tier`: 한 계정이 가질 수 있는 *non-empty* 자산 수의 상한을 덮어야
  한다. 실제 사용자 중 가장 많은 자산을 보유한 계정을 기준으로 잡는다.
- `users_per_batch`: 한 회로 batch 가 처리하는 사용자 수. 크면 batch 수는 줄지만
  회로/메모리 비용이 커진다.
- 여러 shape 를 두면 작은 계정은 작은 tier 로 싸게 처리된다(builder 가 tier 정렬).

### 6.3 위험정책(policy commitment) — 감사/인가

T2/T3/T4 의 haircut/tier 정책은 단순 데이터가 아니라 **감사·인가 대상**이다.
엔진은 `core/tierpolicy` 로 정책의 canonical digest 를 만든다.

- `tierpolicy.PolicyCommitment(policy)` — 운영자 위험정책(T2 자산별 `haircut_bp`;
  T3/T4 자산·pool별 tier 곡선)의 capacity-독립 Poseidon digest. **authoritative
  입력(boundary, ratio, haircut_bp)만** 흡수하고, 파생값 `precomputed_value` 나
  변동성 큰 per-snapshot totals 는 흡수하지 않는다 → 자산 capacity 를 몰라도
  정책만으로 계산 가능.
- `tierpolicy.VerifyCommitment(got, expectedHex)` — fail-closed 인가의 비교 절반.
  운영자가 pin 한 값과 스냅샷에서 재계산한 digest 가 다르면 거부한다.

운영자 실무: **tier 곡선/haircut 은 `tierpolicy` 를 거쳐 만들고**, 정책이 바뀌면
digest 가 바뀐다는 점을 인지한다(자산 1개의 haircut·boundary·ratio 만 바꿔도
digest 변경). precomputed 손계산 금지 원칙(§4.3)과 같은 이유다.

### 6.4 추가회로(alpha module)

`[constraint].module = ""` 가 기본(noop, 추가 제약 없음). 거래소별/규제별 추가
제약(집중도 한도, KYC tier, jurisdiction 한도 등)은 add-only `ConstraintModule`
로만 붙인다 — base 솔벤시 제약을 약화/제거할 수 없다. 비-noop module 은 별도
trusted setup + audit 가 필요하며, 새 입력이 필요하면 CSV 에 임의 컬럼을 붙이지
말고 versioned connector + alpha sidecar schema 로 운반한다. 상세는
[`profile/README.md`](../profile/README.md) "추가회로(alpha) 매핑" 참조. V1
카탈로그에는 아직 비-noop module 이 없다 → 현행 profile 은 `module = ""` 유지.

---

## 7. 스케일링 (value scale) 작성법

모든 금액·가격은 **이미 정수로 스케일된** 값이어야 한다 (소수점 문자열 불가).
`equity`/`debt`/`collateral`/`total_*` 는 balance-scaled, `base_price` 는
price-scaled.

**예.** raw `BTC balance = 1.23456789`, `BTC price = 65000.12`, 기본 scale `1e8 × 1e8`:
```
equity     = 123456789          # 1.23456789 × 1e8
base_price = 6500012000000      # 65000.12   × 1e8
```

**ValueScale 불변식 (G6).** `TotalEquity`/`TotalDebt`/`TotalCollateral` 의 단위는
`price_scale × balance_scale`(= **ValueScale**, 기본 `1e16`)다. 서로 다른 자산을
같은 quote 단위로 더하려면 **모든 자산에서 이 곱이 같아야** 한다.

소수 자릿수가 작은 자산(밈코인 등)을 다른 scale 로 쓰려면 `two_digit_assets` +
`two_digit_price_scale` + `two_digit_balance_scale` 을 쓰되, 반드시:
```
default_price_scale × default_balance_scale == two_digit_price_scale × two_digit_balance_scale
```
(`BuildPricing` 이 startup 에서 assert. 예: t4_reference 는 `1e8×1e8 == 1e14×1e2`.)

---

## 8. 운영자 워크플로 & 체크리스트

1. **모델 선택** — §1 / §11 의사결정 트리로 테넌트 사업모델 → T1–T4 결정.
2. **connector 짝 맞추기** — `[profile].model` 과 `[snapshot].source_type` 을
   같은 model 로.
3. **전처리기 작성** — raw export → §4 의 model-standard CSV. CSV header 는
   schema 와 *정확히* 일치해야 한다(unknown column / 중복 header / 중복 primary
   key 는 reject). `(account_id, asset_index)` 는 계정 안에서 unique.
4. **asset_capacity** 를 trusted setup 과 service 에서 쓸 capacity 로 맞춘다.
5. **batch_shapes** 로 사용자당 최대 non-empty 자산 수를 덮는다.
6. **pricing** — ValueScale 불변식 확인 (§7).
7. **T3/T4 tier 곡선** — `tierpolicy.BuildTierCurve` 로 `precomputed_value` 생성,
   `tier_ratios.csv` 작성. 정책 digest 인지 (§6.3).
8. **frozen id 유지** — 의도적 신규 등록이 아니면 `identity.scheme`,
   `insolvent.action`, `constraint.module=""` 그대로.
9. **검증 실행** (sandbox 면 `GOCACHE` 를 writable path 로):
   ```bash
   cd zkpor
   go test ./profile/declarative
   go build ./...
   go vet ./...
   ```

---

## 9. 자주 하는 실수 (failure modes)

| 증상 | 원인 / 교정 |
|---|---|
| 마진 사용자가 대량 drop | T2–T4 인데 `collateral` 을 안 채움. 부채는 **담보**로 검증된다 — `debt > 0` 이면 위험가중 담보가 그 부채를 덮어야 한다 |
| sum equality 실패 | `accounts.csv` 합과 `cex_assets.csv` 의 `total_*` 불일치. 전처리기 집계 오류 |
| 증명이 늦게 실패 | `precomputed_value` 손계산. `tierpolicy.BuildTierCurve` 로 재생성 |
| 자산 합산이 말이 안 됨 | 자산별 `price_scale × balance_scale` 불일치. ValueScale 불변식 위반 (§7) |
| CSV reject | header 불일치 / unknown column / 중복 primary key. schema 와 정확히 일치시킬 것 |
| `collateral ≤ equity` 위반으로 row reject | 담보가 보유 자산을 초과. 전처리 매핑 오류 |
| 과대 비용 | spot/단순마진인데 T3/T4 선택. 가장 작은 적합 모델로 내릴 것 |
| policy 인가 거부 | pin 한 digest 와 스냅샷 정책 digest 불일치. 정책 변경 시 digest 갱신 |

---

## 10. 모델별 참조 프로파일

| model | 참조 TOML | testdata |
|---|---|---|
| T1 | [`profile/t1_reference/t1_reference.toml`](../profile/t1_reference/t1_reference.toml) | `testdata/happy/{accounts,cex_assets}.csv` |
| T2 | [`profile/t2_reference/t2_reference.toml`](../profile/t2_reference/t2_reference.toml) | `+ haircut_bp` in cex_assets |
| T3 | [`profile/t3_reference/t3_reference.toml`](../profile/t3_reference/t3_reference.toml) | `+ tier_ratios.csv` |
| T4 | [`profile/t4_reference/t4_reference.toml`](../profile/t4_reference/t4_reference.toml) | `+ tier_ratios.csv (collateral_pool)` |

---

## 11. 모델 선택 의사결정 트리

```
Q1. 사용자가 부채(대출/마진/론)를 가질 수 있는가?
  ├─ 아니오 (현물 전용) ───────────────────────► T1  (debt = 0)
  └─ 예
       Q2. 담보를 위험가중(haircut)하는가?
         ├─ 아니오 (계정 net 만 검증, off-chain Merkle 격상) ─► T1  (debt 활성)
         └─ 예
              Q3. haircut 이 포지션 크기에 따라 달라지는가?
                ├─ 아니오 (자산별 고정 상수) ───────────────► T2
                └─ 예 (규모별 tier 곡선)
                     Q4. 담보를 사업 라인(론/크로스/포트폴리오)별로 분리회계?
                       ├─ 아니오 (단일 pool) ───────────────► T3
                       └─ 예 (3 pool 분리) ─────────────────► T4
```

---

## 12. 용어 풀이 (glossary)

### 12.1 솔벤시·회계 용어

- **솔벤시(solvency)**: 거래소가 사용자에게 갚아야 할 잔액·부채를 충분한 자산/
  담보로 덮고 있다는 상태. 이 엔진은 이를 *zk 증명* 으로 보인다.
- **PoR (Proof of Reserves)**: 거래소가 보유 자산이 사용자 잔액을 덮음을
  증명하는 것. zk-PoR 은 개별 잔액을 공개하지 않고 증명한다.
- **equity (지분/자산)**: 사용자가 그 자산에 대해 *청구 가능한* 수량. leaf 의
  `TotalEquity` 는 `Σ(equity × base_price)` 로 quote 통화 가치 환산된 합.
- **debt (부채)**: 사용자가 그 자산에 대해 *빚진* 수량(마진/론). 현물은 0.
  `TotalDebt = Σ(debt × base_price)`.
- **collateral (담보)**: 부채를 뒷받침하려 잡아둔 자산. T2–T4 에서만 사용.
  솔벤시 검증은 부채를 *위험가중 담보가치* 와 비교한다(equity 가 아님).
- **net / 계정 단위 검증**: 자산별로 보지 않고 계정 전체에서 `자산 ≥ 부채` 를
  보는 것. T1 의 솔벤시 방식.

### 12.2 위험가중·정책 용어

- **haircut (헤어컷)**: 담보 가치를 *깎아서* 인정하는 비율. "haircut 10%" = 담보의
  90% 만 인정. 변동성·유동성 위험을 반영.
- **basis point (bp, 베이시스 포인트)**: 1/10000. `haircut_bp = 9000` → 90%.
  T2 는 `[0, 10000]`, `10000 = 100%(haircut 없음)`, `0 = 담보 불인정`.
- **ratio (T3/T4 tier 인정률)**: 퍼센트 `[0, 100]`. tier 구간의 한계 인정률.
  분모는 100 (`PercentMultiplier`).
- **LTV (Loan-to-Value)**: 담보 대비 대출 비율. Aave V3 의 자산별 고정 LTV 가
  T2 haircut 의 reference.
- **RiskPolicy**: model 의 자산별 위험가중치 *값* 을 공급하는 인터페이스
  (T2: `Haircut(symbol)`; T3: `CollateralRatios`; T4: `Loan/Margin/PortfolioMarginRatios`).
  위험가중 *모델 자체*(곡선 모양)는 회로가 고정하며 협상 불가.
- **policy commitment**: 운영자 위험정책의 canonical Poseidon digest
  (`tierpolicy.PolicyCommitment`). 인가 게이트에서 pin 값과 비교(fail-closed).

### 12.3 tier 곡선 용어

- **tier (등급/구간)**: 담보 규모 구간. 큰 포지션일수록 다른 인정률을 적용하기
  위한 분할.
- **piecewise-linear (조각별 선형)**: 구간마다 다른 기울기(ratio)를 갖는 선형
  함수. tier 곡선의 형태.
- **boundary_value**: 한 tier 의 *상한* (담보 가치, value-scale 단위). 자산 안에서
  strictly increasing. 회로 range-check 상한 `2^118`.
- **precomputed_value**: boundary 까지의 *누적* 인정 담보가치. 회로가 in-circuit
  재계산을 피하려고 미리 받는 값. **`tierpolicy.BuildTierCurve` 로만 생성**.
- **TierCount**: pool 당 허용 tier slot 수 = **12**. 곡선의 tier 수는 이 이하.
- **collateral pool (담보 풀)**: 담보를 분리회계하는 버킷. T4 는 3개 —
  `loan`(VIP 론) / `margin`(크로스 마진) / `portfolio_margin`(포트폴리오 마진).
  한 pool 담보를 다른 pool 부채에 못 쓴다.

### 12.4 데이터·스케일 용어

- **account_id**: 사용자/계정 canonical 식별자. 64-hex 문자열 → 엔진이 BN254
  field element 로 reduce. V1 scheme `passthrough_hex_bn254_reduced.v0` 는 입력이
  이미 32-byte hex 라는 뜻.
- **account_index**: Merkle leaf 순서. optional — 생략하면 parser 가 first-seen
  valid 순서로 dense 파생.
- **asset_index**: catalog slot 번호 `[0, asset_capacity)`.
- **asset_capacity**: trusted setup 이 잡아둔 자산 slot 수. `cex_assets.csv` 의
  실제 자산보다 크면 parser 가 `reserved` zero slot 으로 padding.
- **base_price**: 자산의 quote 통화 가격(price-scaled 정수).
- **price_scale / balance_scale**: 소수점을 정수로 옮기는 배율(기본 각 `1e8`).
- **ValueScale**: `price_scale × balance_scale`(기본 `1e16`). `Total*` 값의 단위.
  모든 자산에서 같아야 cross-asset 합산이 성립(불변식 G6).
- **two_digit_assets**: 소수 자릿수가 작은 자산군에 다른 scale 을 쓰기 위한 목록.
  단 ValueScale 곱은 default 와 같아야 함.

### 12.5 회로·증명 용어

- **zk-SNARK**: 데이터를 공개하지 않고 "계산이 옳다"를 증명하는 영지식 증명.
  이 엔진은 Poseidon over BN254, SMT depth 28.
- **Merkle tree / root**: 모든 사용자 leaf 를 한 해시(root)로 묶는 트리. root 가
  스냅샷 전체를 대표.
- **account leaf**: 사용자 1명의 5-input Poseidon 해시
  `Poseidon(AccountID, TotalEquity, TotalDebt, TotalCollateral, AssetsCommitment)`.
  4개 model 공통 시그니처.
- **AssetsCommitment**: 사용자 per-asset 값들을 flat-packing 한 commitment.
  per-asset shape 만 model 별(§3.3).
- **sum equality (합계 일치)**: 사용자 잔액을 자산마다 더한 값이 published total 과
  같음을 회로가 강제하는 제약.
- **batch / batch_shape**: 사용자들을 나눠 증명하는 단위와 그 크기 설정
  (`asset_count_tier`, `users_per_batch`).
- **trusted setup / .pk / .vk**: 회로별 증명키(.pk)·검증키(.vk)를 만드는 의식.
  `(model, asset_capacity, batch_shape, module)` 마다 별도. model/회로 변경은
  새 ceremony + audit 가 필요.
- **ConstraintModule (alpha module)**: base 솔벤시 회로에 *추가만* 하는 제약 훅.
  base 제약을 약화/제거 불가. 비-noop 은 별도 setup/audit.
- **drop_and_log**: invalid 계정 처리 정책(V1 기본). 조건 위반 계정을 제외하고
  로깅. profile `[insolvent].action = "drop_and_log.v0"`.

---

### 부록 — model ↔ connector ↔ 파일 매핑

| model | `source_type` | `accounts.csv` 추가 컬럼 | `cex_assets.csv` 추가 컬럼 | `tier_ratios.csv` |
|---|---|---|---|---|
| `t1_simple_margin` | `t1_standard_csv.v1` | — | — | ❌ |
| `t2_static_haircut_margin` | `t2_standard_csv.v1` | `collateral` | `collateral`, `haircut_bp` | ❌ |
| `t3_tiered_haircut_margin_1pool` | `t3_standard_csv.v1` | `collateral` | `collateral` | ✅ (`asset_index,tier_index,boundary_value,ratio,precomputed_value`) |
| `t4_tiered_haircut_margin_3pool` | `t4_standard_csv.v1` | `loan_collateral,margin_collateral,portfolio_margin_collateral` | 동일 3 컬럼 | ✅ (+`collateral_pool`) |
