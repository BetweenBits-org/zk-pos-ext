# Solvency Model Reference

이 문서는 zkpor 카탈로그의 4 model 각각이 *어떤 산업 reference 를 일반화한 결과인지*, 그리고 회로 spec (leaf shape, 솔밴시 식, per-asset shape) 이 **왜 지금 형태인지** 결정 트레일을 기록한다. R6 진입 (`t1_simple_margin` 구현 + `spot_simple` 흡수) 전 freeze 된 reference 다.

긴 산업 조사 자료가 아니라 *spec 결정의 근거* 다. 자료 원문은 출처 링크로 보존.

## Source-of-truth 위치

| 우선순위 | 문서 | 이 문서와의 관계 |
|---:|---|---|
| 1 | `zkpor/core/spec/` 코드 (frozen 계약) | model ID + leaf shape + 인터페이스가 우선. |
| 2 | `01-project-context.md` | 컨셉·strong guarantee. 이 문서는 그 컨셉을 model 단위로 구체화. |
| 3 | `02-module-architecture.md` | module 라이브러리. 이 문서는 model 카탈로그. 두 축은 직교. |
| 4 | **이 문서** | model spec 의 industry reference + 일반화 트레일. |
| 5 | `PRODUCTION_ROADMAP.md` | stage·timing. 이 문서의 spec 결정과 충돌해선 안 됨. |

# 1. 카탈로그 (Tn naming, R6 freeze)

```
T1 Simple Margin                       (spot 흡수 — debt=0 case)
T2 Static-Haircut Margin
T3 Tiered-Haircut Margin · 1 Pool
T4 Tiered-Haircut Margin · 3 Pool
```

5-tier 의 product line marketing 은 `01-project-context.md` 의 "Basic / Standard / Pro-A / Pro-B / Enterprise" 표가 그대로 유효 — display map 으로 매핑 (`core/spec/solvency_models.go` 의 `ModelDisplay`).

`spot_simple` 은 T1 의 `debt=0` 자명 case 로 흡수됨 — 카탈로그 entry 폐기, 회로/spec/host 한 벌 사용. `merkle_classic` 도 T1 의 *일반 case* 로 흡수. 통합 결정 트레일은 §6 참조.

# 2. 차원별 spectrum

모든 model 은 다음 공통 baseline 위에 있다:

```
∀ user :  거래소 wallet 잔액 ≥ user 의 청구 가능 잔액 합
```

각 tier 는 한 차원씩 검증을 풍부화한다.

| 차원 | T1 | T2 | T3 | T4 |
|---|:---:|:---:|:---:|:---:|
| 부채 허용 (per-user `TotalDebt` 검증) | ✅ | ✅ | ✅ | ✅ |
| collateral risk-weight (haircut) | ❌ | ✅ asset-level 고정 | ✅ tier-기반 곡선 | ✅ tier-기반 곡선 |
| haircut size-dependent | — | ❌ | ✅ | ✅ |
| collateral 비즈니스 라인 분리 | — | — | ❌ | ✅ 3 bucket |
| 회로 비용 (상대) | 1× | ~1.3× | ~2× | ~3× |

`debt=0` 으로 supply 시 T1 은 spot 거래소도 처리 (자명 만족).

# 3. 통일된 AccountLeaf shape

4 model 모두 동일 5-input Poseidon 사용:

```
AccountLeaf = Poseidon(
    AccountID,
    TotalEquity,
    TotalDebt,
    TotalCollateral,
    AssetsCommitment,
)
```

| Slot | T1 | T2 | T3 | T4 |
|---|---|---|---|---|
| `AccountID` | bn254 fr.Element | 동일 | 동일 | 동일 |
| `TotalEquity` | user asset 총합 | 동일 | 동일 | 동일 |
| `TotalDebt` | user 부채 총합 (spot=0) | 동일 | 동일 | 동일 |
| `TotalCollateral` | 0 (haircut 부재) | sum(asset_i × haircut_i) | sum(haircut_curve(asset_i)) | sum_buckets(...) |
| `AssetsCommitment` | per-asset (Equity) flat commitment | per-asset (Equity, Debt) | per-asset (Equity, Debt, Collateral) | per-asset (Equity, Debt, Loan, Margin, PM) |

→ **leaf signature universal**, slot 의미만 model 별. R6 host helper `AccountLeafHash` 가 4 model 공유 가능 (G11 promotion 후보).

`AssetsCommitment` 은 `core/circuit/commitment.go::ComputeFlatUint64Commitment` 의 결과. per-asset shape 만 model 별로 다름.

# 4. T1 — Simple Margin

## Industry reference

| 출처 | 형태 | T1 매핑 |
|---|---|---|
| **Binance OSS PoR v2** ([zk-SNARKs blog](https://www.binance.com/en/blog/tech/how-zksnarks-improve-binances-proof-of-reserves-system-6654580406550811626)) | 5-input Poseidon leaf + per-user net-balance≥0 | T1 의 `TotalEquity ≥ TotalDebt` assertion 의 시발점. 단, T4 가 *full* 형태 (3-bucket collateral) — T1 은 collateral 측 검증 생략한 축소. |
| **Bybit Merkle PoR** ([learn.bybit.com](https://learn.bybit.com/en/blockchain/what-is-merkle-tree)) | 24-level Merkle, leaf = user balance hash, no zk | off-chain → in-circuit hoist. T1 은 동일 user balance leaf 를 zk 로 묶음. |
| **KuCoin Merkle PoR** ([kucoin.com/proof-of-reserves](https://www.kucoin.com/proof-of-reserves)) | 단순 Merkle, hash(user_balance_list) | 동일. T1 = 산업 표준 Merkle PoR 의 zk 격상. |
| **HTX Merkle Sum Tree v2** ([htx.com/support/84949005158246](https://www.htx.com/support/84949005158246)) | Merkle Sum Tree, snapshot + proof path | T1 의 sum equality (`ΣTotalEquity == published_total`) 와 호환. |
| **OKX zk-STARK PoR** ([okx.com/help/zero-knowledge-proofs](https://www.okx.com/en-us/help/zero-knowledge-proofs-what-are-zk-starks-and-how-do-they-work-v2)) | Plonky2 Merkle Sum Tree, 3 constraints: Total / Non-neg / Inclusion | T1 의 3 constraint 직접 대응. |
| **Provisions (Bünz et al. 2015)** ([eprint.iacr.org/2015/1008](https://eprint.iacr.org/2015/1008.pdf)) | Schnorr-기반 privacy-preserving proof of solvency | 학술 기원. zk-SNARK 로의 generalization 을 future work 로 명시. |

## 솔밴시 식

```
∀ user :  user.TotalEquity ≥ user.TotalDebt           (per-user, account-level only)
sum_users (user.TotalEquity - user.TotalDebt)[asset]
                                       == published_total[asset]   (per-asset sum equality)
```

- `TotalDebt = 0` 인 spot 사용자는 첫 식이 trivially 만족 → spot 흡수 자연.
- 자산별 차용 (per-asset `Debt > Equity`) 허용 — *account-level* net 만 검증. Binance OSS 의 동일 의미.

## per-asset shape

```go
type AccountAsset struct {
    Index  uint16
    Equity uint64
    Debt   uint64   // spot = 0
}
```

- spot 거래소는 항상 `Debt = 0` supply.
- mid-tier margin 거래소는 두 필드 모두 active.

## AssetsCommitment

`ComputeFlatUint64Commitment([]uint64{equity_0, debt_0, equity_1, debt_1, ...})` — 2-field per asset flat packing.

## 회로 핵심 constraint

1. Account leaf hash = above 5-input Poseidon (Merkle 위치 검증).
2. Merkle root before / after 일관성 (witness 가 leaf 갱신 시 root 변화 valid).
3. per-user `TotalEquity ≥ TotalDebt` (gnark `AssertIsLessOrEqual`).
4. per-asset sum equality (CexAssetsCommitment).

## 차이 — 다른 model 대비

| | T2/T3/T4 |
|---|---|
| `TotalCollateral` slot | T1 = 0 ; T2~T4 = haircut 합 |
| per-asset `Collateral_*` 필드 | T1 부재 ; T2~T4 active |
| RiskPolicy interface | T1 부재 ; T2~T4 active |

# 5. T2 — Static-Haircut Margin

## Industry reference

| 출처 | 형태 | T2 매핑 |
|---|---|---|
| **Aave V3 LTV / Liquidation Threshold** ([governance.aave.com](https://governance.aave.com/), [risk-v3 repo](https://github.com/aave/risk-v3/blob/main/asset-risk/risk-parameters.md)) | per-asset 고정 LTV (예: ETH 75%, stablecoin 90%, 알트 35~65%) | T2 의 `haircut_i` 직접 차용. asset 별 *고정* 위험 가중치. |

거래소 1:1 매칭 없음 — T2 는 zkpor 가 카탈로그 spectrum 을 메우기 위해 정의한 model. Aave V3 의 *over-collateralized* 모델이 in-circuit reference.

## 솔밴시 식

```
∀ user :  Σ_i (user.collateral_i × haircut_i) ≥ user.TotalDebt
```

`haircut_i` 는 자산 `i` 의 *상수* (예: 0.90 = BTC, 0.85 = ETH, …). global RiskPolicy 가 자산 → haircut 매핑 공급.

## per-asset shape

```go
type AccountAsset struct {
    Index      uint16
    Equity     uint64
    Debt       uint64
    Collateral uint64
}
```

## 차이 — T1 대비

- `Collateral` 필드 추가.
- `TotalCollateral = Σ (collateral_i × haircut_i)` — haircut 이 *상수* 라 회로 비용 = per-asset multiply 1번.
- RiskPolicy interface 신설 (`asset_index → haircut_basis_points`).

# 6. T3 — Tiered-Haircut Margin · 1 Pool

## Industry reference

| 출처 | 형태 | T3 매핑 |
|---|---|---|
| **dYdX IMF Curve** ([docs.dydx.xyz](https://docs.dydx.xyz/concepts/trading/margin)) | piecewise-linear Initial Margin Fraction: lower_cap < open_notional → linear ↑ → 100% at upper_cap | T3 의 `haircut_curve(collateral_i)` 직접 차용. *size-tiered* — 큰 포지션일수록 큰 haircut. |
| **Bitget / Gate.com tiered MMR** ([gate.com 문서](https://www.gate.com/help/futures/futures/38042/maintenance-margin-calculation)) | risk tier 별 maintenance margin rate | T3 의 tier table 의미. |

거래소 1:1 매칭 없음 — perp DEX 의 *initial margin* 곡선이 *collateral haircut* 으로 mapping. tier_3bucket 의 단순화 형태 (3 bucket → 1).

## 솔밴시 식

```
∀ user :  Σ_i  haircut_curve_i(user.collateral_i)  ≥  user.TotalDebt

haircut_curve_i :  uint64 → uint64   (piecewise-linear, asset-별)
```

curve 는 RiskPolicy 가 (asset, tier boundaries, tier ratios) 형태로 공급. T4 와 동일 curve shape, *1 pool* 만 차이.

## per-asset shape

T2 와 동일 (Equity, Debt, Collateral).

## 차이 — T2 대비

- haircut 이 *상수* → *함수*. 회로 비용 = piecewise-linear lookup (gnark hint + tier comparison).

## 차이 — T4 대비

- collateral pool 1개 (= bucket 1개) ; T4 는 3 bucket 의 sum.

# 7. T4 — Tiered-Haircut Margin · 3 Pool

## Industry reference

| 출처 | 형태 | T4 매핑 |
|---|---|---|
| **Binance OSS PoR v2** ([github.com/binance/zkmerkle-proof-of-solvency](https://github.com/binance/zkmerkle-proof-of-solvency)) | tier-based haircut + 3-bucket collateral (Loan / Margin / PortfolioMargin), zk-SNARK in-circuit | T4 의 직접 출처. zkpor R1-R3 이 이 회로의 productization. |
| **OKX zk-STARK PoR v2** ([okx GitHub](https://github.com/okx/proof-of-reserves-v2)) | Plonky2 Merkle Sum Tree, 3 claims (Total / Non-neg / Inclusion) | T4 의 동일 의미 (Binance 모방). |

VIP 론 / cross margin / portfolio margin 을 *별도 회계* 로 운영하는 거래소 — 한 bucket 의 collateral 을 다른 bucket 의 debt 에 못 씀.

## 솔밴시 식

```
∀ user :  Σ_{b ∈ {Loan, Margin, PortfolioMargin}}
            Σ_i  haircut_curve_b_i(user.collateral_b_i)  ≥  user.TotalDebt
```

각 bucket 마다 별도 curve table.

## per-asset shape

```go
type AccountAsset struct {
    Index             uint16
    Equity            uint64
    Debt              uint64
    Loan              uint64   // bucket 0
    Margin            uint64   // bucket 1
    PortfolioMargin   uint64   // bucket 2
}
```

(현 `tier_3bucket/spec/types.go::AccountAsset` 와 동일)

## 차이 — T3 대비

- 3 bucket 평가 → 회로 비용 ~3x.
- bucket 별 curve table 별도.

# 8. 통합 결정 트레일

## §8.1 spot 흡수 (5 → 4)

| 후보 | 결정 |
|---|---|
| spot_simple 별도 entry 유지 | ❌ |
| spot_simple + merkle_classic 통합 → T1 | ✅ |

근거:
1. **math superset**: spot_simple = T1 (`debt=0` case).
2. **회로 비용 추가 ≈ 0**: per-user `Equity ≥ Debt` constraint 1개 추가, debt=0 이면 trivially 만족.
3. **ceremony 공유**: 같은 `.vk` 가 두 customer (spot + simple margin) 동시 처리 — G12 closure 와 정합.
4. **leaf shape 자연**: 현 `spot_simple/host/account.go` 의 slot 3, 4 가 이미 zero-padded — T1 의 `TotalDebt` slot 활성화만 필요.
5. **R5 patrón 보존**: `profile/sea_reference` 는 T1 standard CSV 에서 `debt=0` 을 명시 supply 하는 spot reference descriptor 로 유지 (이전엔 spot_simple).

## §8.2 T1/T2 통합 안 함

per-asset haircut multiply 가 spot/simple-margin 거래소에 *불필요* 비용. T1 = "haircut 부재", T2 = "고정 haircut" 의 *math 적 거리* 가 spot↔simple-margin 거리보다 큼.

## §8.3 T3/T4 통합 안 함

tier_3bucket 회로 = `tier_1bucket × 3 bucket`. 통합 시 `tier_1bucket` 거래소가 항상 3 bucket 평가 → ~3x 회로 비용. `01-project-context.md` Line 259 *"단일 c5 toggle은 단순 모델 거래소에 불필요 비용을 강제한다"* 와 충돌.

## §8.4 5-tier marketing 유지

product line "Basic / Standard / Pro-A / Pro-B / Enterprise" 은 회로 ID 와 무관한 *marketing* 차원. T1 의 *use case* 가 두 가지 (spot only / simple margin) — display map 에서 분리 표현 가능. `01-project-context.md` 의 customer-tier 표는 그대로 유효.

# 9. R6 implementation 영향

| 작업 | 위치 |
|---|---|
| spec types — `TotalDebt` 추가 | `core/solvency/t1_simple_margin/spec/types.go` |
| circuit — per-user `Equity ≥ Debt` constraint 추가 | `core/solvency/t1_simple_margin/circuit/batch_create_user_circuit.go` |
| AccountLeafHash — slot 3 (`TotalDebt`) 활성화 | `core/solvency/t1_simple_margin/host/account.go` |
| per-asset shape — `Debt` 필드 추가 | `core/solvency/t1_simple_margin/spec/types.go::AccountAsset` |
| AssetsCommitment — 2-field per asset (Equity, Debt) | `host/commitment.go::ComputeUserAssetsCommitment` |
| sea_reference standard CSV — `Debt = 0` 명시 supply | `profile/sea_reference/sea_reference.toml` + `core/snapshot/t1_simple_margin` |
| `merkle_classic/` 디렉터리 폐기 (T1 흡수) | `core/solvency/merkle_classic/` 삭제 |
| `spot_simple/` 디렉터리 rename | `core/solvency/spot_simple/` → `t1_simple_margin/` |

R5-0 의 host helpers (commit e8eabed) 가 generalize 의 base. NbConstraints baseline 은 T1 generalize 후 재기록 — 33,306 → ~34k (per-user 1 LessOrEqual × users_per_batch 추가) 예상.

# 10. Implementation status (R6 + T2/T3 + R6.5 env fix)

| Model | spec | circuit | host | NbConstraints (tiny shape 5_10_2) | Setup time |
|---|:---:|:---:|:---:|---:|---:|
| T1 `t1_simple_margin` | ✅ | ✅ | ✅ | **38,149** | 2.79s |
| T2 `t2_static_haircut_margin` | ✅ | ✅ | ✅ | **48,886** | 6.11s |
| T3 `t3_tiered_haircut_margin_1pool` | ✅ | ✅ | ✅ | **274,650** | 22.90s |
| T4 `t4_tiered_haircut_margin_3pool` | ✅ | ✅ | ✅ | **723,790** | 58.15s |

T1 의 R1CS sha256 baseline (audit lock 후보):

```
R1CS sha256         = d2df98c8969280900ac36424358454af5a223b331839b9f2080cbc548aebe0b0
coefficients sha256 = a0f392899e054ad792c9f96775be7f13ca2b13d4c92e878e3d846f1a14745aab
```

회로 비용 chain 확인 (T1 < T2 < T3 < T4 — 예상 ordering 맞음):

- T1 → T2: ~28% 증가 (per-asset haircut multiply + division 추가)
- T2 → T3: ~5.6× (tier-curve lookup table 비용)
- T3 → T4: ~2.6× (3 bucket × tier-curve)

모든 4 model 의 alpha-layer noop-module zero-cost 검증 통과 (nil-module
NbConstraints == noop-module NbConstraints).

T2 / T3 의 spec 결정 트레일:

- **T3 implementation (R6/T3)**: T4 의 3-bucket loop 를 single Collateral pool 로 collapse. per-asset 4-tuple (Index, Equity, Debt, Collateral). 같은 tier-curve evaluation 알고리즘 (T4 의 universal piecewise-linear cumulative-sum 헬퍼 그대로). RLC sumB 3 powers per slot (T4: 5).
- **T2 implementation (R6/T2)**: T3 의 tier 곡선을 single Haircut basis-points 상수로 collapse. per-asset 1 multiply + 1 division. assetHaircutTable lookup 추가. T3 의 tier-table lookup 제거 → 회로 ~30-40% 절감 (실측 ~5.6× 절감 — tier-table evaluation cost 가 예상보다 큼).

# 11. Open work — R6 follow-up

- ~~**bw6 env fix (R6.5)**~~ — ✅ closed. 원인: Go module cache 의 stale package list (build cache). fix: `go clean -cache` 후 fresh download — fork module 의 zip 안에는 bw6-633/761 + poseidon 패키지 모두 *존재*했으나 extracted cache 의 일부 sub-package 가 stale (외부 trigger 로 invalidated 됐을 가능성). zkpor 코드 정합 입증 + 4 model setup smoke baseline 기록.
- **T2 / T3 의 실제 첫 customer signal** — 현 시점 reference 가 DeFi 인접 분야 (Aave / dYdX) 차용이라 거래소 1:1 매칭 부재. 첫 customer narrowing 시 spec 재검토 (over-collateral 비율 범위, tier curve 구체화) 후보.
- **R7 freeze 직전** — Tn naming 의 `_1pool` / `_3pool` 약자 vs full (`single_pool` / `triple_pool`) 재검토 가능.
- **R6 promotion candidates carry** — `PowersOfSixteenBits` (4 model 모두 동일 정의), R1CS hash test helpers, identity DeriveAccountID body 등 — 3-model evidence 확보된 항목은 G11 second batch promotion 가능 (R7 freeze 직전 또는 별 슬라이스).
- **T2/T3/T4 setup_test 에 R1CS sha256 baseline 기록 추가** — T1 의 패턴 (R1CS + coefficients sha256 hashR1Cs 헬퍼 호출) 을 다른 model 도 동등 적용. 현재 T2/T3/T4 setup_test 는 NbConstraints 만 log. T1 패턴 promote 시 4 model 모두 audit-lockable.

# 12. R9 standard raw snapshot schema v1

R9 의 raw data layer 는 customer 원본 CSV 를 직접 표준화하지 않는다.
표준은 **mapping 이후 canonical row** 다. 즉 customer 는 자기 컬럼명,
decimal 문자열, wide-format CSV 를 유지할 수 있고, R9-C mapping layer 가
이를 아래 schema 로 변환한다. 그 다음 model parser 가 `SnapshotSource`
값을 만든다.

Code source-of-truth:

- shared metadata: `core/snapshot/schema`
- model schema: `core/snapshot/<model>/standard_schema.go`

공통 규칙:

1. 금액은 모두 scaling 완료된 non-negative integer (`uint64` 또는
   `bigint`) 다. 원본 decimal scaling 은 mapping layer 책임.
2. `account_id` 는 64-hex 문자열이며 parser 가 BN254 fr.Element
   SetBytes→Marshal 로 canonical bytes 를 만든다.
3. `account_index` 는 optional 이지만 있으면 dense Merkle leaf order 로
   검증한다. 없으면 deterministic file order 에서 first-seen valid
   account 순서로 dense index 를 파생한다.
4. `(account_id, asset_index)` 는 account file 안에서 unique 해야 한다.
   누락된 account-asset pair 는 zero balance 로 해석한다.
5. `cex_assets.csv` 는 real asset row 만 포함할 수 있고, parser 가
   `AssetCatalog.Capacity()` 까지 `reserved` zero slot 으로 pad 한다.

## §12.1 T1 `t1_simple_margin`

`accounts.csv`

| field | type | required | 의미 |
|---|---|:---:|---|
| `account_index` | `uint32` | no | Merkle leaf order. 없으면 파생. |
| `account_id` | `account_id_hex_bn254` | yes | user/account canonical identifier |
| `asset_index` | `uint16` | yes | catalog slot |
| `equity` | `uint64` | yes | user claim |
| `debt` | `uint64` | yes | user debt. spot 은 0 |

`cex_assets.csv`

| field | type | required | 의미 |
|---|---|:---:|---|
| `asset_index` | `uint16` | yes | catalog slot |
| `symbol` | `string` | yes | lower-case symbol |
| `total_equity` | `uint64` | yes | published total equity |
| `total_debt` | `uint64` | yes | published total debt |
| `base_price` | `uint64` | yes | price-scaled reporting value |

Derived invariant: parser 는 account rows 로 `TotalEquity`,
`TotalDebt` 를 계산하고, 회로가 `TotalEquity >= TotalDebt` 를 검증한다.

## §12.2 T2 `t2_static_haircut_margin`

T2 는 T1 account row 에 single-pool `collateral` 을 추가한다.
`cex_assets.csv` 는 `collateral`, `haircut_bp` 를 추가로 가진다.

`accounts.csv`: `account_index`, `account_id`, `asset_index`, `equity`,
`debt`, `collateral`

`cex_assets.csv`: `asset_index`, `symbol`, `total_equity`, `total_debt`,
`base_price`, `collateral`, `haircut_bp`

Invariants:

- `haircut_bp ∈ [0, 10000]`.
- `collateral <= equity` 는 invalid-account policy 이전의 row-level
  check 다.
- parser 는 `Σ(collateral × haircut_bp / 10000)` 로
  `TotalCollateral` 을 계산한다.

## §12.3 T3 `t3_tiered_haircut_margin_1pool`

T3 account / cex asset shape 는 T2 와 같지만 static `haircut_bp` 대신
별도 tier curve file 을 사용한다.

`accounts.csv`: `account_index`, `account_id`, `asset_index`, `equity`,
`debt`, `collateral`

`cex_assets.csv`: `asset_index`, `symbol`, `total_equity`, `total_debt`,
`base_price`, `collateral`

`tier_ratios.csv`

| field | type | required | 의미 |
|---|---|:---:|---|
| `asset_index` | `uint16` | yes | catalog slot |
| `tier_index` | `uint16` | yes | asset 별 dense tier order |
| `boundary_value` | `bigint` | yes | tier upper boundary |
| `ratio` | `uint8` | yes | audited circuit 의 tier ratio |
| `precomputed_value` | `bigint` | yes | boundary 누적값 |

Invariants:

- `tier_index` 는 asset 별 dense sequence.
- `boundary_value` 는 asset 별 strictly increasing.
- `precomputed_value` 는 회로의 tier cumulative function 과 일치해야 한다.

## §12.4 T4 `t4_tiered_haircut_margin_3pool`

T4 는 Binance-class 3-pool account shape 를 사용한다.

`accounts.csv`: `account_index`, `account_id`, `asset_index`, `equity`,
`debt`, `loan_collateral`, `margin_collateral`,
`portfolio_margin_collateral`

`cex_assets.csv`: `asset_index`, `symbol`, `total_equity`, `total_debt`,
`base_price`, `loan_collateral`, `margin_collateral`,
`portfolio_margin_collateral`

`tier_ratios.csv`

| field | type | required | 의미 |
|---|---|:---:|---|
| `asset_index` | `uint16` | yes | catalog slot |
| `collateral_pool` | `enum` | yes | `loan`, `margin`, `portfolio_margin` |
| `tier_index` | `uint16` | yes | asset+pool 별 dense tier order |
| `boundary_value` | `bigint` | yes | tier upper boundary |
| `ratio` | `uint8` | yes | audited circuit 의 tier ratio |
| `precomputed_value` | `bigint` | yes | boundary 누적값 |

Invariants:

- `collateral_pool` 은 closed enum 이며 pool 별 tier curve 는 독립이다.
- `loan_collateral + margin_collateral + portfolio_margin_collateral <= equity`
  는 invalid-account policy 이전의 row-level check 다.
- parser 는 pool 별 tier curve 를 적용한 뒤 합산해 `TotalCollateral` 을
  계산한다.
