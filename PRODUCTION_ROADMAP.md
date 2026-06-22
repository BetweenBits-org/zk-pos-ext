# Production Roadmap — zkpor Engine (go-forward)

이 문서는 **go-forward 로드맵**이다 (재편 2026-06-22). 1차 로드맵의 stage·gate
**이력**(Stage R0~MS, closed gates G1·G2·G4~G13·G17~G19 등)은
`PRODUCTION_ROADMAP_v1_FROZEN.md` 로 **migration**되었고 그 문서는 frozen이다.
본 문서는 그 위에서 **두 트랙(v1 freeze / v2 sovereign fork)** 으로 재편한
현재 계획만 담는다.

## Source-of-truth Priority

문서가 충돌하면 아래 우선순위로 해소한다.

| 우선순위 | 문서 | 역할 |
|---:|---|---|
| 1 | `zkpor/core/spec/*` 코드 | frozen 계약 (인터페이스·카탈로그 상수·명명 규약). 코드가 source. |
| 2 | `zkpor/docs/01-project-context.md` | 컨셉·scope·strong guarantee·preserve 결정. |
| 3 | `zkpor/docs/02-module-architecture.md` | ConstraintModule + Profile architecture lock. |
| 4 | **`zkpor/PRODUCTION_ROADMAP.md` (이 문서)** | **go-forward** stage·gate·deferred work 의 source-of-truth. |
| 4b | `zkpor/PRODUCTION_ROADMAP_v1_FROZEN.md` | 1차 로드맵 **이력**(frozen). closed gate 의 결정 트레일은 여기. |
| 5 | `zkpor/AGENTS.md`, `zkpor/CLAUDE.md` | agent contract / 자동 로드 메모리. |
| 6 | `zkpor/HANDOFF.md` | 현재 시점 인수인계 (휘발성). |
| 7 | `docs/06-units-and-scaling.md` 등 docs | 단위·방법론 reference. |

## 0. 재편 배경 — 1차 로드맵 완료

frozen 문서가 기록하는 1차 로드맵이 확보한 것:

- **제품 정체성** — 4-tier 솔밴시 카탈로그(T1~T4) ⟂ profile 직교, R7 freeze.
- **대형 플랫폼 audit 차용** — Binance OSS zk-PoR v2 lineage. *독자 audit 비용의 의도적 연기*.
- **SaaS 이식성** — port inversion(R12-E/F/G), 자립 Go 모듈(`zk-pos-ext`), declarative profile.
- **동작 검증** — 4모델 E2E smoke, 1M-scale 측정(R11-D), GPU 검증(R13, L4 2.65×).

→ **"빌린 신뢰 위에서, 이식 가능하고, 실제 동작하는 도구"** 완성. 이것이 곧 **v1**이다.

## 1. 두 트랙 프레임

```
v1 (frozen) ── 빌린 신뢰로 지금 판다 ───────────────┐  현 repo (zk-pos-ext)
   · 아키텍처/기능 freeze. 유지보수 + first-customer pull 만.   │  borrowed Binance audit
   · 영업·onboarding(BAU).                                      │
                                                                │
v2 (sovereign) ── 최종 목표로 공격적 R&D (병렬, 지금 시작) ──┘  분리 fork
   · 독자 audit + upstream gnark + 독자 Poseidon2 + GKR + GPU.
   · 출시 전에도 영업 자산. cutover 만 트리거-게이트.
```

| | v1 | v2 |
|---|---|---|
| 정체 | frozen 제품 | sovereign 엔진 |
| 신뢰(거래소 audit) | **빌린**(Binance lineage) | **독자** |
| repo | 현 `zk-pos-ext` | **fork = 분리노선** |
| 스택 | 현 bnb-chain gnark fork (v0.10) | upstream gnark v0.15 + 독자 Poseidon2 |
| 비용/시점 | 낮음 / 지금 판다 | 높음 / 지금 R&D·트리거 시 cutover |

**프레임의 핵심**: freeze가 "주권 vs GTM 속도" 긴장을 해소한다. v1이 팔리는 동안
v2가 연구된다 — 충돌 없음. v2의 **R&D는 지금**(리드타임 + 영업자산), **cutover만
트리거-게이트**.

## 2. Track 1 — v1 Freeze & Sell

### 상태
**1차 영업 준비 완료.** 4모델 회로 + reference/declarative profile + 5 서비스
4-model dispatch + standard CSV + 자립 모듈 + 측정 baseline. (상세 = frozen 문서.)

### Freeze 정책
- 아키텍처/기능 **freeze**. v1에 신규 feature·신규 model·신규 module 안 올린다.
- 허용: 유지보수 fix + **first-customer pull-based closer**(아래)만.
- **원칙: 고객(deal)이 당기지 않는 readiness는 만들지 않는다.** (T4 과투자 역전 차단.)

### v1 closers (pull-based, "A0")
| closer | 상태 | trigger |
|---|---|---|
| **G20 operator policy pin (B-wire)** | B-core landed (`core/tierpolicy` + `SnapshotSource.PolicyCommitment` + `VerifyCommitment`). **잔여**: (a) `profile.toml [risk_policy] commitment` additive 필드, (b) snapshot-load digest 대조 reject(fail-closed). | 첫 operator-enforced 정책 고객 |
| **예금자-검증 신뢰** (self-inclusion verifier, G14) | 제품 표면. zk가 보장 못 하는 *누락 공격* 보완책(`docs/01`). | 첫 customer SLA 협상 |

### 이번 재편 결정 — v1에서 제거/이관 (2026-06-22)
| 항목 | 처리 | 근거 / 결과 |
|---|---|---|
| **T2 trusted setup 재실행** | **소멸 (ELIMINATED, 연기 아님)** | v1 GTM(SEA-spot)에 T2 고객 없음 + v2가 전 모델 재-ceremony → v1 재실행은 낭비. 회로의 `haircut_bp≤10000` cap(soundness)은 **유지**. **결과: T2 production key가 capped 회로와 미정합 → v1에서 T2는 dormant**(재-ceremony 시점 = v2). |
| **GPU 영구 통합 (gpu wire)** | **→ v2** | build-tag seam(`core/host/prove_*.go`)은 이미 v1 코드에 dormant(CPU 빌드 no-op)로 유지. 영구 통합(fork vendor/icicle v3)은 v2 재플랫폼과 묶임. |
| **gnark 보안권고 remediation** | **→ v2** | upstream v0.11이 픽스한 GHSA 2건 미적용. 보안 부채 해소 = upstream 이전 = v2 재플랫폼과 불가분(§3 참조). |

### Scope Boundary
v1 출하 단위 = **backend + CLI + file artifacts**. UI/web/사용자-facing 페이지는
engine 밖. (상세 = frozen 문서 §Scope Boundary. v1에서 불변.)

## 3. Track 2 — v2 Sovereign Engine (분리 fork)

### Repo 모델
`zk-pos-ext`를 **fork → 분리노선**으로 진행한다. v1(현 repo) frozen, v2(fork)에서
공격적 병렬 R&D. v1 오염 0 + 1회 재플랫폼으로 모든 변경을 묶는다.

### North star (definition of done)
> **독자 audit** + **upstream gnark v0.15** + **독자 Poseidon2 stack(host+circuit)** +
> **Poseidon2-GKR 회로 최적화** + **GPU 영구 통합** = **최저 prove-COGS의 독자 신뢰 PoR 엔진.**

### 왜 fork / 왜 분해 불가 (구조적 필연)
upstream gnark/gnark-crypto에는 **원조 Poseidon이 없다**(Poseidon2만; gnark v0.15
`std/hash` = mimc·poseidon2, gnark-crypto v0.20 `fr` = poseidon2). zkpor의 account
leaf·SMT 노드·batch commitment는 전부 원조 Poseidon. → **stale fork를 벗어나는
순간 = 해시 정체성 변경 = 재-ceremony = 재-audit.** "보안만 bump"하는 값싼 경로가
존재하지 않는다. 따라서 보안·독자성·최적화는 **한 motion**이고, 분리 fork에서
1회 재플랫폼으로 처리하는 것이 맞다.

### Milestone 0 — de-risk spikes (진입점, "공격적"을 측정으로 시작)
- **(a) T4 해시 비중 측정** — Poseidon 해싱이 ~64M constraint의 몇 %인가 → GKR ROI 상한 확정.
- **(b) BN254 Poseidon2-GKR production-readiness** — gnark PR #1407이 *"params only for BLS12-377"* 였음 → BN254 준비도 확인.

### Milestone chain
```
M0 de-risk(상기)
  → M1 upstream gnark v0.15 base + NoOp 비교자 재구현(std/rangecheck·std/math/cmp 위)
  → M2 독자 Poseidon2 stack (host + in-circuit gadget)
  → M3 Poseidon2-GKR PoC (Merkle 해싱 collapse)
  → M4 GPU 영구 통합 (upstream icicle v3.2.2, gpu wire)
  → M5 ceremony 리허설 (4모델 재-setup)
  → M6 독자 audit
```

### 가드레일
1. **de-risk-first** — M0 전에 GKR 본작업 금지. 측정이 ROI를 부정하면 방향 재고.
2. **capacity-cap** — v2가 v1 first-customer 지원(B-wire/onboarding)을 굶기지 않도록 % 상한.
3. **마일스톤 규율** — 분기 단위 검증 게이트. 출시 deadline 없는 research 표류 방지.

### Cutover 트리거 (v2 → production / 고객 migration)
`min(보안부채 강제, prove-COGS가 deal 차단)`. "고객이 독자성 요구"는 약한 보조
트리거. **R&D는 트리거 무관 = 지금 시작**; cutover(재-ceremony·마이그레이션·v1 EOL)만
트리거-게이트.

### 영업 자산
v2 로드맵 자체가 **출시 전 세일즈 차별화** — "독자 audit 엔진 + 최저 proving 비용으로
가는 길"이 *"Binance fork 아니냐"* 반론을 선제 차단한다.

## 4. Decision Log — 2026-06-22 재편

| # | 결정 | 비고 |
|---|---|---|
| D1 | **v1 freeze** (아키텍처/기능), 빌린 신뢰로 영업 지속 | 1차 영업 준비 완료 기반 |
| D2 | **v2 = `zk-pos-ext` fork, 분리노선** 공격적 병렬 R&D | freeze가 긴장 해소 |
| D3 | **T2 trusted setup 재실행 = 소멸** (연기 아님) | v2 재-ceremony가 흡수; T2 v1 dormant |
| D4 | **GPU 영구 통합 → v2** | seam은 v1 dormant 유지 |
| D5 | **gnark 보안권고 remediation → v2** | upstream 이전과 불가분 |
| D6 | **PRODUCTION_ROADMAP.md 전면 재작성**, 1차 이력 → frozen 문서 | 본 재편 |

## 5. Gate Register (go-forward)

closed gates(G1·G2·G4~G13·G17~G19)의 결정 트레일은 **frozen 문서**에. 아래는
forward-relevant open gate만.

| Gate | Track | Status | 결정 / 잔여 |
|---|---|---|---|
| **G3** ConstraintModule 공개 API freeze | v1/v2 | deferred | 첫 비-noop module 등장 시 surface 확정. |
| **G14** 사용자-facing verification 분배 | v1 | deferred | self-inclusion verifier. 첫 customer SLA pull. |
| **G16** Module composition compatibility | v1/v2 | deferred | 첫 multi-module composition 시 process 확정. |
| **G20** RiskPolicy operator pin (B-wire) | **v1** | deferred (pull) | B-core landed. 잔여 = profile.toml `[risk_policy] commitment` 필드 + snapshot-load reject. **(구) item 'T2 setup 재실행'은 D3로 소멸.** 첫 operator-enforced 정책 고객 전 close. |
| **G15** Prove-path GPU 가속 | → v2 | **superseded by G21** | GPU 영구 통합이 v2 재플랫폼에 흡수(§3 M4). |
| **G21** **Engine Sovereignty Pivot (v2)** | **v2** | **open (R&D 진행)** | upstream gnark v0.15 + 독자 Poseidon2 + Poseidon2-GKR + GPU + 독자 audit. 분해 불가(Poseidon 갭). 진입 = M0 de-risk spike 2개. cutover = `min(보안부채, prove-COGS deal-block)`. |

### Gate → Track
```text
G3, G16  --> 첫 module / composition 등장 시 (v1·v2 공통)
G14      --> 첫 customer SLA (v1 Track 1)
G20      --> 첫 operator-enforced 정책 고객 (v1 Track 1, B-wire 2-items)
G15      --> superseded → G21 (v2)
G21      --> v2 분리 fork. R&D 지금 시작, cutover 트리거-게이트.
```
