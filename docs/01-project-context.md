# Project Context — zkpor Engine

이 문서는 Part 1 (컨셉 & 컨텍스트) 이며, 이후 모든 설계 결정의 기준선이다.
"이 작업이 scope 안인가"의 판정 기준 역할을 한다.

## Project In One Paragraph

zkpor은 Binance OSS zk-PoR v2를 표준화·일반화한 **다거래소 PoR 엔진 제품**
이다. 5-tier 솔밴시 모델 카탈로그와 N개의 고객사 프로파일을 직교 축으로
운영한다. 한 엔진 코드 베이스로 여러 거래소의 PoR 운영을 지원하며, 각
거래소는 자기 사업모델에 맞는 모델을 카탈로그에서 선택해 자기 프로파일에서
어댑터만 구현한다. SaaS 형태로 엔진 통합을 판매하는 것이 운영 모델이다.

methodology는 Part 1 (컨셉) + Part 3 (로드맵) + 횡단 규율 (AGENTS / HANDOFF)
을 적용한다. **Part 2 (POC)는 의도적으로 생략** — 검증된 Binance OSS 위에서의
productization이라 prototype 단계가 존재하지 않는다.

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

## Constraint Architecture

엔진의 한 회로 instance가 강제하는 제약은 다음 세 층의 합이다 — 이 분리가
PoR 엔진의 멘탈모델이다.

```text
setup = c1 ⊕ c2 ⊕ c3 ⊕ c4       // universal mandatory (모든 model 공통)
      ⊕ L[k]                     // single selection, k ∈ catalog ∪ {None}
      ⊕ alpha(profile)           // customer-specific ConstraintModule
```

### Layer 1 — Universal mandatory (`c1..c4`)

모든 솔밴시 모델이 enforce해야 하는 PoR substrate. 이 층은 model에 따라
바뀌지 않으며 `zkpor/core/circuit/` + `zkpor/core/spec/` 의 frozen contract
가 곧 c1..c4 의 정의다.

| 항목 | 책임 |
|---|---|
| c1 — account tree 무결성 | Merkle proof verify/update (`zkpor/core/circuit/merkle.go`) |
| c2 — commitment 무결성 | uint64 packing + batch commitment (`zkpor/core/circuit/commitment.go`) |
| c3 — 배치 연속성 | before/after root chaining + batch-commitment public input |
| c4 — 합계 등식 | `published_total[asset] == Σ_users user[asset]` (irreducible PoR claim) |

c4 (sum equality)는 PoR의 본질 — 어떤 모델·고객·모듈도 c4를 약화할 수 없다.

### Layer 2 — Catalog selection (`L[k]`)

5-tier 솔밴시 모델 카탈로그(`zkpor/core/spec/solvency_models.go`)에서 **정확히
하나를 선택**한다. 각 모델 `L[k]`는 자기 회로 정의로 c1..c4 위에 모델-특화
제약을 얹는다 (예: tier_3bucket의 tier-haircut, 3-bucket collateral). `k = None`
일 수도 있지만 v1 카탈로그의 5개 모델은 모두 명시적 제약을 추가한다.

- 단일 선택 (composition 미지원 v1). 결정 근거는 `## Decision Rationale` 참조.
- 카탈로그 entry 추가는 rule of three + audit + trusted setup ceremony 필요.

### Layer 3 — Customer extension (`alpha(profile)`)

고객사가 선택한 모델 위에 추가 제약을 얹는 ConstraintModule. **add only**
— c1..c4 또는 L[k]의 제약을 weaken/remove 할 수 없다. 모듈을 얹으면 trusted
setup이 (model, module) pair별로 분기되며 `.vk` 파일명에 module ID가 노출된다.

### 두 가지 확장 축

이 아키텍처는 두 직교 확장 축을 갖는다.

| 축 | 의미 | 비용 |
|---|---|---|
| **Profile choice** | 솔밴시 *모델*을 통째로 교체 (math가 바뀜) | 회로·.pk·.vk 별개. 거래소 사업 구조 변경 시 |
| **ConstraintModule** | 한 모델 안에서 *strictness만* 추가 | 같은 모델 + 모듈 ID 조합으로 .pk·.vk 분기. 규제·법규 대응 시 |

거래소가 "tier 모델 안에서 추가 규제 강화"를 원하면 ConstraintModule, "tier
모델 자체를 안 쓴다"면 Profile 교체. 두 자유도가 분리되어 있다.

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
  - `zkpor/core/spec/` 인터페이스 호환성 (추가 OK, 제거/변경은 versioned change).
  - Model = math, Profile = deployment 분리. 한 쪽이 다른 쪽 이름을 흡수
    하지 않는다.

## Customer And Operating Model

| 항목 | 값 |
|---|---|
| 목표 고객 (1차) | 마진/론 사업 거래소 (Binance-class, OKX-class). 이미 PoR을 운영하거나 도입 계획. |
| 목표 고객 (2차) | 한국·EU·일본 규제 spot 거래소 — spot_simple 모델로. |
| 운영 형태 | managed SaaS — 엔진 통합 + 운영 컨설팅. 고객 인프라에 엔진 배포하되 운영은 우리. |
| 핵심 SLA (잠정) | snapshot 입수 ~ proof publish 24h. customer onboarding 1~4개월. |
| 통합 표면 | 고객은 `zkpor/profile/<customer>/` 패키지를 구현 (어댑터 N개). 코어는 우리가 관리. |
| Verifier 분배 | `.vk` + 명시적 model/shape/module ID. 검증은 고객사 또는 third-party가 자체 도구로 수행. |
| 차별화 우위 | (1) customer 통합 비용 (2) audit trust. tech/UX는 후순위. |

### 카탈로그 tier → 타겟 고객

| Tier | Model ID | 타겟 고객 | 통합 난이도 |
|---|---|---|---|
| Basic | `spot_simple` | 한국/EU/일본 regulated spot 거래소, 스테이블코인 발행처, 커스터디 | 가장 낮음 |
| Standard | `merkle_classic` | Bybit / KuCoin / HTX 등 mid-tier 마진 거래소 — 기존 Merkle-PoR 의 zk 격상 | 중간 |
| Pro-A | `over_collateral_simple` | 단순 마진 거래소 (단일 collateral pool, 자산-level 고정 haircut) | 중상 |
| Pro-B | `tier_1bucket` | 파생-heavy 거래소 (size-tiered haircut, 비즈니스 라인 미분리) | 중상 |
| Enterprise | `tier_3bucket` | Binance / OKX class (VIP loan + cross margin + portfolio margin) | 가장 높음 |

이 매핑이 product line 의 정체성이며 GTM 순서의 근거다 (`## Market Context`
참조).

## Market Context

엔진의 카탈로그 우선순위는 산업 조사 결과를 반영한다. 다음 통찰이 5-tier
구성과 GTM 순서를 정당화한다.

### 두 질문이 갈리는 답

| 질문 | 답 |
|---|---|
| Q. 글로벌 거래소가 마진/론/선물 사업을 하는가? | **압도적 다수 yes** (~70~80%, 규제 spot-only 거래소 제외) |
| Q. 그 거래소들이 *zk PoR로* 위험-솔밴시까지 보장하는가? | **Binance·OKX 정도만 yes**. 나머지는 off-chain Merkle + sum equality. |

→ "**마진 사업이 있다 ≠ zk solvency 채택한다**" 는 점이 엔진 전략의 핵심.

### 거래소별 채택 현황 (snapshot)

| 거래소 | 사업 | 현 PoR 형태 | zk 솔밴시? |
|---|---|---|---|
| Binance | 마진·론·선물 | zk + Merkle (이 코드의 출처) | ✅ tier haircut |
| OKX | 마진·론·선물 | zk + Merkle | ✅ tier (Binance 모방) |
| Bybit, Crypto.com, BingX, MEXC, Gate.io, Bitget, Bitfinex, KuCoin, HTX | 마진·선물 | Merkle + sum | ❌ |
| Kraken | 일부 마진 | Merkle + 감사 | ❌ |
| Coinbase | 일부 마진 | 감사 보고서 | ❌ (PoR 자체 없음) |
| Upbit, Bithumb, Coinone (한국) | spot only (규제) | 미공개 / 감사 | ❌ |
| bitFlyer (일본) | 일부 derivatives | 감사 | ❌ |
| Bitstamp, Bitvavo (EU) | spot 중심 | 감사 | ❌ |

### 시사점 → GTM 정렬

1. **tier_3bucket 시장은 작다** — 채택 가능 거래소 수 기준 2~3개. 단가는
   높지만 deal cycle이 길다.
2. **merkle_classic / over_collateral_simple 잠재 시장이 가장 크다** — 마진
   사업을 하지만 아직 zk를 안 쓴 거래소 5~10개. zk 격상 욕구가 있는 mid-tier.
3. **spot_simple 도 시장이 넓다** — 한국·EU·일본 regulated spot 거래소들.
   통합 난이도 가장 낮음. 빠른 PMF 검증에 유리.

→ GTM 순서: **Basic 또는 Standard 부터 진입 → Pro로 확장 → Enterprise로
upsell**. tier_3bucket을 "주력"으로 보지 않는다. tier_3bucket은 reference
implementation이며 시장 진입의 시작점이 아니다.

이 통찰이 없으면 "Binance가 쓰는 거니까 이게 표준이고 우선 구현하자" 같은
잘못된 우선순위가 자연스럽게 잡힌다. 회로는 Binance에서 출발하지만 GTM은
거기서 출발하지 않는다.

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
| Profile-based 선택 이유 | 단일 솔밴시 회로 대신 model 카탈로그를 택한 이유 — 사업모델별 솔밴시 식이 다르며(Bybit-style over-collateral ≠ Binance tier-haircut ≠ dYdX maintenance margin), 단일 c5 toggle은 단순 모델 거래소에 불필요 비용을 강제한다. `## Constraint Architecture` 참조. |
| 두 확장 축 | **Profile choice** (모델 교체, math 바뀜) ≠ **ConstraintModule** (한 모델 안 strictness 추가). v1은 두 축 모두 지원, 모델 composition은 지원 안 함. |
| ConstraintModule | **add only** — base-circuit 제약 weaken/remove 금지. |
| Sum equality | **모든 모델 mandatory** — irreducible PoR claim (= c4). |
| 명명 분리 | `SolvencyModelID` (math) ≠ `ProfileID` (deployment). `BatchShape` (dimensions) ≠ `BatchProfile`. |
| Key file naming | `zkpor.<model>.<assetTier>_<usersPerBatch>[.<module>].{pk,vk,r1cs}` |
| Legacy 호환 | `BatchShape.LegacyKeyName()` 가 기존 `zkpor50_700` 명명 유지. |
| Identity 공개 | `AccountIDProvider.Scheme()` 으로 derivation 알고리즘 ID 공개. 사용자가 자기 ID 재현 가능해야. |
| Adapter 패키지 | `zkpor/profile/<customer>/` 는 **단일 Go 패키지**. 한 customer = 한 import. |
| 거래소명 분리 | 거래소 이름을 model id에 박지 않는다 (예: `tier_3bucket`, not `binance_v2`). |
| Engine 스코프 | 엔진 프로그램만. 호스팅/UI/컴플라이언스 인증은 별도 product. |
| POC 단계 | methodology Part 2 (POC) 적용 안 함. 검증된 Binance OSS 출발점이라 prototype 단계 부재. |

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
