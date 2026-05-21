# Project Context — zkpor Engine

이 문서는 Part 1 (컨셉 & 컨텍스트) 이며, 이후 모든 설계 결정의 기준선이다.
"이 작업이 scope 안인가"의 판정 기준 역할을 한다.

## Project In One Paragraph

zkpor은 Binance OSS zk-PoR v2를 표준화·일반화한 **다거래소 PoR 엔진 제품**
이다. 5-tier 솔밴시 모델 카탈로그와 N개의 고객사 프로파일을 직교 축으로
운영한다. 한 엔진 코드 베이스로 여러 거래소의 PoR 운영을 지원하며, 각
거래소는 자기 사업모델에 맞는 모델을 카탈로그에서 선택해 자기 프로파일에서
어댑터만 구현한다. SaaS 형태로 엔진 통합을 판매하는 것이 운영 모델이다.

## Strong Guarantee

이 시스템이 강하게 말할 수 있는 보장:

```text
주어진 user balance dataset이 published CEX totals과 산술적으로 일치하며,
선택된 솔밴시 모델의 각 사용자 조건을 통과함을 zk로 증명한다.
```

이 보장은 두 부분으로 나뉜다.

1. **Sum equality** — 모든 모델이 enforce. `published_total[asset] ==
   Σ_users user[asset]`. 거래소가 발표한 자산 총량이 실제 user 잔고 합과 같다.
2. **Per-user solvency** — 모델별로 정의됨. `tier_3bucket`은 tier-haircut
   기반 사용자별 over-collateral; `spot_simple`은 자동 만족 (부채 없음).
   각 모델의 회로 정의가 정확한 부등식을 명시한다.

## Not Guaranteed

zk 단독으로는 보장하지 않는 것 — 명시적으로 적는다.

| 보장하지 않는 것 | 보완책 |
|---|---|
| 거래소가 dataset에 포함시키지 않은 사용자/계정의 존재 (dummy account 공격이 아니라 *누락* 공격) | 사용자 self-inclusion verifier로 자기 잔고가 dataset에 들어갔는지 검증 + AccountIDProvider derivation scheme 공개 |
| 거래소 ETL이 내부 데이터를 정확히 추출했는지 | snapshot 시점 commit + 외부 audit. 엔진은 입력 dataset을 신뢰함 |
| 솔밴시 정의 자체의 정합성 (예: tier 값 조작) | 솔밴시 모델은 audit 대상이며 catalog governance(rule of three + trusted-setup ceremony)로 통제 |
| user balance의 *시점*. snapshot 이후 잔고 변동 | snapshot ID + 발행 시점을 published artifact에 포함, 사용자는 snapshot 시점 잔고만 검증 |
| ConstraintModule이 추가한 제약식의 의미 | module 소스 공개 의무 + `.vk` 파일명에 module ID 노출 |

## V1 Scope

- **우선 대상**:
  - `tier_3bucket` 모델 회로 이식 완료 (legacy `circuit/` → `zkpor/core/
    solvency/tier_3bucket/circuit/`).
  - `binance` 고객사 프로파일 end-to-end 동작 (snapshot CSV loader 포함).
  - 4개 서비스 (witness / prover / userproof / verifier) 가 신규 어댑터 기반
    으로 작동.
- **scope 밖 / 나중 module**:
  - 나머지 4개 모델(`spot_simple`, `merkle_classic`, `over_collateral_simple`,
    `tier_1bucket`) 회로 구현 — 카탈로그 등재만 유지, 회로는 고객 신호 시 구현.
  - 호스팅 인프라 / SaaS 운영 컨트롤 플레인.
  - verifier 웹 UI / 사용자-facing 검증 도구.
  - 컴플라이언스 인증 (SOC2, MiCA 적합 등).
  - 다중 model composition (한 회로 instance가 두 model 합성).
- **확장이 깨지지 않도록 지금 지켜야 할 것**:
  - `SolvencyModelID` · `BatchShape` · `ConstraintModuleID` 명명 규약.
  - Key file naming `zkpor.<model>.<shape>[.<module>].{pk,vk,r1cs}`.
  - `core/spec/` 인터페이스 호환성 (추가 OK, 제거/변경은 versioned change).
  - Model = math, Profile = deployment 분리. 한 쪽이 다른 쪽 이름을 흡수
    하지 않는다.

## Customer And Operating Model

| 항목 | 값 |
|---|---|
| 목표 고객 (1차) | 마진/론 사업 거래소 (Binance-class, OKX-class). 이미 PoR을 운영하거나 도입 계획. |
| 목표 고객 (2차) | 한국·EU·일본 규제 spot 거래소 — spot_simple 모델로. |
| 운영 형태 | managed SaaS — 엔진 통합 + 운영 컨설팅. 고객 인프라에 엔진 배포하되 운영은 우리. |
| 핵심 SLA (잠정) | snapshot 입수 ~ proof publish 24h. customer onboarding 1~4개월. |
| 통합 표면 | 고객은 `profile/<customer>/` 패키지를 구현 (어댑터 N개). 코어는 우리가 관리. |
| Verifier 분배 | `.vk` + 명시적 model/shape/module ID. 검증은 고객사 또는 third-party가 자체 도구로 수행. |

## Product / Protocol Decisions To Preserve

처음 내린, 이후 흔들면 안 되는 결정.

| 영역 | 결정 |
|---|---|
| Hash | Poseidon over BN254 |
| Tree | Sparse Merkle Tree, depth = 28 |
| Account capacity | 2^28 ≈ 268M users per snapshot |
| Asset capacity | `AssetCounts = 500` per snapshot (padding으로 통일) |
| Batch commitment | `Poseidon(beforeRoot, afterRoot, beforeCexCommit, afterCexCommit)` |
| Account leaf | model-specific. tier_3bucket: `Poseidon(AccountID, TotalEquity, TotalDebt, TotalCollateral, AssetsCommitment)`. spot_simple은 reduce. |
| ValueScale | `PriceMultiplier × BalanceMultiplier == ValueScale`. 기본 1e16. |
| Catalog | 5-tier, **single selection** per circuit instance, composition 미지원 (v1). |
| Catalog governance | **Rule of three** — 패턴은 3번째 사용 사례 후 코어 승격. |
| ConstraintModule | **add only** — base-circuit 제약 weaken/remove 금지. |
| Sum equality | **모든 모델 mandatory** — irreducible PoR claim. |
| 명명 분리 | `SolvencyModelID` (math) ≠ `ProfileID` (deployment). `BatchShape` (dimensions) ≠ `BatchProfile`. |
| Key file naming | `zkpor.<model>.<assetTier>_<usersPerBatch>[.<module>].{pk,vk,r1cs}` |
| Legacy 호환 | `BatchShape.LegacyKeyName()` 가 기존 `zkpor50_700` 명명 유지. |
| Identity 공개 | `AccountIDProvider.Scheme()` 으로 derivation 알고리즘 ID 공개. 사용자가 자기 ID 재현 가능해야. |
| Adapter 패키지 | `profile/<customer>/` 는 **단일 Go 패키지**. 한 customer = 한 import. |
| 거래소명 분리 | 거래소 이름을 model id에 박지 않는다 (예: `tier_3bucket`, not `binance_v2`). |
| Engine 스코프 | 엔진 프로그램만. 호스팅/UI/컴플라이언스 인증은 별도 product. |

## Open Questions

처음 시점에 아직 못 정한 것. 정해지면 여기서 제거하고 적절한 문서로 옮긴다.

| ID | 질문 | 정해야 하는 시점 |
|---|---|---|
| Q1 | `tier_3bucket` 회로 이식 후 `.pk`/`.vk` byte-equivalence를 어떤 방법으로 검증? (해시 비교? deterministic re-run?) | R1 진입 전 |
| Q2 | `AccountIDProvider.Scheme()` 의 v1 freeze 형태 — 현재 `passthrough_hex.v0` 임시. HMAC/salt 정식 derivation으로 갈지, customer-side 책임으로 둘지. | R3 전 |
| Q3 | 두 번째 customer 온보딩 시 적용할 model — spot_simple (한국 spot)? merkle_classic (Bybit-class)? 시장 신호 따라. | R4 |
| Q4 | `ConstraintModule` 공개 API 의 v1 freeze 시점 — 첫 번째 module 등장 후 surface 최소화로 좁히기. | R3 후 (첫 module 등장 시) |
| Q5 | RiskPolicy 데이터를 CSV로 계속 받을지, JSON/YAML schema로 옮길지 — schema validator 도입 시점. | R2 |
| Q6 | LegacyKeyName 폐기 일정 — catalog freeze (R7) 이후 한 release에 deprecate? | R7 |
| Q7 | 다중 customer 가 같은 model을 쓸 때 `.vk` 공유 정책 — customer마다 독립 발행 vs 공유? | R4 |
