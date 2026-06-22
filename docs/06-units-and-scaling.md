# Units & Scaling — 단위와 스케일 체계

이 문서는 zkpor 엔진의 **모든 수치 입력이 어떤 단위(스케일)로 표현되는지**,
그리고 그 단위들이 왜 한눈에 통일돼 보이지 않는지를 설명하는 횡단 reference
다. CSV snapshot 입력 → 파서 → 회로로 이어지는 값의 단위 흐름을 한곳에
모은다. 모델별 spec(`04-solvency-models.md`)이나 회로 정의보다 우선하지
않으며, 그 문서들이 전제하는 단위 규약을 명문화한 것이다.

이 문서의 출발점: snapshot CSV를 처음 보면 같은 파일 안에 "수량"과 "가격"이
섞여 있고, 모델마다 haircut 비율 단위가 달라 **단위가 통일돼 있지 않은 것처럼
보인다**. 대부분은 의도된 3단 구조이고, 한 군데(ratio family 분기)만 모델
구조 차이를 반영한 분기다. 이 문서가 그 전부를 정리한다.

## Source-of-truth 위치

| 우선순위 | 위치 | 관계 |
|---:|---|---|
| 1 | `core/spec/price.go`, `core/spec/constants.go` (frozen 계약) | `ValueScale` 불변식과 기본 배율의 정의. 이 문서보다 우선. |
| 2 | `core/snapshot/<model>/parser.go` (`addScaled`, `collateralValue`) | 수량×가격→가치 변환의 실제 코드. |
| 3 | `core/solvency/<model>/circuit/` | 회로가 같은 변환을 enforce. host와 byte-equivalent. |
| 4 | **이 문서** | 위 코드가 전제하는 단위 규약의 서술. |
| 5 | `04-solvency-models.md` §12 (CSV 스키마) | 모델별 컬럼 목록. 단위 해석은 이 문서가 owner. |

## 1. 한눈에 — 3 scale family + 2 ratio family

엔진의 수치는 의미가 다른 5종류로 나뉜다. "단위가 안 맞아 보이는" 이유는
이들이 한 CSV/한 줄에 공존하기 때문이며, 역할이 다른 것이지 오류가 아니다.

| Family | 정의 | 기본값 | 등장 위치 |
|---|---|---|---|
| **balance-scale** (수량) | 실수량 × `BalanceMultiplier` | 1e8 | CSV 입력: `equity`, `debt`, `*collateral`, `total_*` |
| **price-scale** (가격) | quote 가격 × `PriceMultiplier` | 1e8 | CSV 입력: `base_price` 만 |
| **value-scale** (가치) | balance × price | **1e16** (`ValueScale`) | **파서가 계산** — leaf의 `TotalEquity/Debt/Collateral`, tier `boundary_value`/`precomputed_value` |
| **basis points** (만분율) | 0~10000, ÷10000 | — | `haircut_bp` (**T2 전용**) |
| **percent** (백분율) | 0~100, ÷100, 정수만 | — | `ratio` (**T3/T4**) |

핵심 한 줄: **CSV에는 value-scale가 직접 들어오지 않는다.** 수량(balance-scale)과
가격(price-scale)이 따로 들어오고, 파서가 둘을 곱해 비로소 가치(value-scale)를
만든다. 솔밴시 부등식은 오직 value-scale에서만 비교된다.

## 2. 핵심 불변식 — ValueScale (G6)

```text
PriceMultiplier(s) × BalanceMultiplier(s) == ValueScale()   (모든 symbol s 공통)
기본:  1e8 × 1e8 == 1e16
```

서로 다른 자산을 같은 단위로 더하려면, 모든 자산의 (가격배율 × 수량배율)이
같아야 한다. 그렇지 않으면 자산 간 합이 서로 다른 단위를 섞게 되어 솔밴시
제약이 무의미해진다.

- enforce 지점: `profile/declarative/builders.go::BuildPricing` (G6 invariant,
  builder time). verifier는 동일 매핑을 받아야 totals를 해석할 수 있다 —
  배율 매핑은 published proof artifact의 일부다.
- **비대칭 분할 허용**: 64-bit head-room을 자산별로 다르게 쓸 수 있다. 예:
  SHIB는 `1e14 (price) × 1e2 (balance)`, BTC는 `1e8 × 1e8` — *곱은 동일하게
  1e16*. 저단가·고수량 자산이 정밀도를 가격 쪽에 몰 수 있게 한다
  (`core/spec/price.go` 참조).

## 3. 3단 변환 흐름 — 수량 × 가격 = 가치

```text
[CSV 입력]                         [파서: addScaled]                 [leaf / 회로]
equity (balance-scale, ×1e8) ──┐
                               ├── equity × base_price ──────────▶ TotalEquity (value-scale, ×1e16)
base_price (price-scale, ×1e8)─┘
```

- CSV의 `equity`/`debt`/`*collateral`은 **수량**, `base_price`는 **가격**.
- 파서 `addScaled(dst, amount, price)` = `dst += amount × price` →
  `TotalEquity`/`TotalDebt`는 **value-scale(1e16)**
  (`core/snapshot/t1.../parser.go`).
- 회로도 동일: `api.Mul(equity, basePrice)` 로 USD-scaled 누산
  (`core/solvency/t1.../circuit/batch_create_user_circuit.go`). host와
  byte-equivalent.
- verifier UX는 totals를 `ValueScale`로 나눠 실제 quote 가치를 복원한다.

## 4. 두 검증은 서로 다른 단위에서 돈다

엔진은 "모든 토큰을 quote 가치로 환산"하기만 하는 게 아니다. **무엇을
검증하느냐에 따라 단위가 다르다** — 이 분리가 PoR 건전성의 핵심이다.

| 검증 | 단위 | 가격 의존? | 근거 |
|---|---|---|---|
| **Sum equality (c4)** — `published_total[asset] == Σ user[asset]` | **balance-scale (per-asset 수량)** | ❌ 산술만 | CEX assets commitment은 자산별 raw 수량을 누적. 회로의 `afterCexAssets[j]` per-asset 누산 |
| **Per-user solvency** — `TotalEquity ≥ TotalDebt` 등 | **value-scale** | ✅ base_price 신뢰 | leaf의 value-scale totals 비교 |

왜 sum-equality를 환산하지 않는가:
1. **자산 간 상쇄 방지** — USD로 합치면 BTC 부족을 ETH 초과로 가릴 수 있다.
   자산별로 봐야 부족이 드러난다.
2. **가격 비신뢰** — sum-equality는 가격과 무관하게 산술적으로 참이어야 한다.
   환산하면 PoR의 가장 근본적 주장이 `base_price` 정직성에 의존하게 된다.

→ per-user 솔밴시는 환산이 **수학적 필수**(이종 자산 바스켓 비교), sum-equality는
환산이 **금기**(가격 주입·상쇄). 엔진은 이 둘을 정확히 분리한다. spot-only
(debt=0)면 per-user 솔밴시가 자명 만족이라 사실상 수량 비교만으로 충분하다.

## 5. CSV별·컬럼별 단위 지도

### `accounts.csv`
| 컬럼 | 단위 family | 모델 |
|---|---|---|
| `account_index` | 무차원 (Merkle 순번) | all (optional) |
| `account_id` | 식별자 (64-hex → BN254) | all |
| `asset_index` | 무차원 (catalog slot) | all |
| `equity` | balance-scale | all |
| `debt` | balance-scale (spot=0) | all |
| `collateral` | balance-scale | T2, T3 |
| `loan_/margin_/portfolio_margin_collateral` | balance-scale | T4 |

### `cex_assets.csv`
| 컬럼 | 단위 family | 모델 |
|---|---|---|
| `asset_index`, `symbol` | 무차원 / 문자열 | all |
| `total_equity`, `total_debt` | balance-scale (per-asset 수량 합) | all |
| `base_price` | **price-scale** | all |
| `collateral` | balance-scale | T2, T3 |
| `loan_/margin_/portfolio_margin_collateral` | balance-scale | T4 |
| `haircut_bp` | **basis points (÷10000)** | T2 전용 |

### `tier_ratios.csv` (T3/T4)
| 컬럼 | 단위 family | 모델 |
|---|---|---|
| `asset_index`, `tier_index` | 무차원 | T3, T4 |
| `collateral_pool` | enum (`loan`/`margin`/`portfolio_margin`) | T4 전용 |
| `ratio` | **percent (÷100), 정수 % 만** | T3, T4 |
| `boundary_value` | **value-scale** (collateral × base_price) | T3, T4 |
| `precomputed_value` | **value-scale** (누적 haircut 값) | T3, T4 |

## 6. Ratio family 분기 — 왜 T2만 bp이고 T3/T4는 percent인가

같은 "haircut 비율"인데 T2는 basis point(÷10000), T3/T4는 percent(÷100)다.
**이는 출신(신규 vs Binance 이식) 차이일 뿐 아니라, 모델 구조 차이를 반영한
원칙적 분기다.** 정밀도를 어느 축에 두느냐가 다르기 때문이다.

| | haircut 구조 | 정밀도를 둘 축 | ratio 단위 |
|---|---|---|---|
| **T2 (flat)** | 자산당 단일 평면 배율 | **boundary 축이 없음** → 정밀도를 ratio에 둘 수밖에 | **bp (0.01%)** |
| **T3/T4 (tiered)** | piecewise-linear 곡선 | **boundary (value-scale, 118-bit 고정밀)** | percent (정수%) |

- T3/T4는 곡선을 더 세밀하게 만들고 싶으면 **tier를 추가**(boundary 추가)하는
  게 정공법이다. 한 구간의 기울기(ratio)를 소수점으로 만들 필요가 없어
  byte-clean한 percent로 충분하다. `ratio`가 `uint8`(8-bit)인 것은 boundary와
  함께 2개를 한 field element에 packing하기 위한 최적값이기도 하다
  (`core/solvency/t4.../circuit/utils.go::convertTierRatiosToVariables`).
- T2는 평면이라 boundary 축이 없다. 87.5% 같은 값을 담을 곳이 ratio 필드뿐이라
  bp(0.01%) 해상도가 정당하다. bp는 haircut/rate의 업계 표준 단위이기도 하다.
- ratio는 **모델 경계를 넘어 비교되지 않는다** — 각 모델은 독립 회로·독립
  trusted setup·독립 audit. divisor도 회로-local 상수. 따라서 전역 단위
  통일이 soundness상 요구되지 않는다.

설계 평가: **단위 차이가 도메인 차이(flat vs tiered)를 추적하므로 타당한
분기다.** 양쪽을 억지로 통일하면 (T2→percent: 정밀도 손실 / T3/T4→bp: packing
붕괴 + trusted setup 재-ceremony) 오히려 나쁜 설계가 된다. 통일해야 할 결함이
아니라, **분기를 의도적으로 명시하는 것**이 올바른 조치다 (이 절이 그 명문화).

## 7. tier_ratios 단위 함정 — boundary/precomputed는 value-scale다

`boundary_value`와 `precomputed_value`는 **balance-scale가 아니라 value-scale**
다. host 파서와 회로 모두 `collateral × base_price`(= value-scale)를 만든 뒤
boundary와 비교한다:

- host: `core/snapshot/t4.../parser.go::collateralValue` — `v.Mul(v, price)`
  후 `haircutValue(v, tiers)` 에서 `value.Cmp(tier.BoundaryValue)`.
- 회로: `core/solvency/t4.../circuit/utils.go::getAndCheckTierRatiosQueryResults`
  — `collateralValue := api.Mul(userCollateral, assetPrice)` 후 boundary 비교.

따라서 tier boundary를 **수량(balance-scale)으로 채우면 1e8배 어긋나** haircut이
완전히 틀어진다. 예 (기본 1e8/1e8): "loan collateral $1,000,000까지 80%"
tier라면

```text
boundary_value = 1,000,000 × ValueScale(1e16) = 1e22   ✅ (value-scale)
boundary_value = 1,000,000 × BalanceScale(1e8) = 1e14  ❌ (틀림)
```

테스트 fixture(`profile/t4_reference/testdata/happy/tier_ratios.csv`)의
`boundary_value = 1e20` 도 `$10,000 × 1e16`인 value-scale 값이다.

## 8. 흔한 혼란 / FAQ

**Q. `cex_assets.csv` 한 줄에 단위가 섞여 보인다.**
`total_equity`(수량)와 `base_price`(가격)는 역할이 다른 입력이다. 곱해야
가치가 된다(§3). 의도된 구조다.

**Q. 같은 80% haircut인데 왜 T2는 `8000`, T3/T4는 `80`인가?**
T2=basis point(÷10000), T3/T4=percent(÷100). 같은 개념이지만 단위가 다르다
(§6). 고객 대면 입력은 adapter 레이어에서 단일 표현으로 받아 모델별 native
단위로 변환하는 것을 권장한다.

**Q. tier `boundary_value`를 자산 수량으로 채워도 되나?**
안 된다. value-scale(`collateral × base_price`)여야 한다(§7).

**Q. T3/T4는 소수점 haircut이 안 되나?**
금액(collateral/equity/가치)은 balance-scale·value-scale로 소수점이 표현된다.
다만 tier별 `ratio`는 정수 %만 가능(소수점 불가). 더 세밀한 곡선이 필요하면
ratio가 아니라 **tier를 추가**해 흡수한다(§6).

**Q. 정밀도는 어디에 있나?**
tiered 모델에서 정밀도는 `boundary_value`(value-scale, 118-bit)에 있다. ratio는
정책 해상도(1%)다. flat 모델(T2)에는 boundary가 없으므로 정밀도가 ratio(bp)에
있다.
