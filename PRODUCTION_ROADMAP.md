# Production Roadmap — zkpor Engine

이 문서는 zkpor **v1 엔진**의 go-forward 로드맵이다. v1 의 stage·gate **이력**
(Stage R0~MS, closed gates)은 `PRODUCTION_ROADMAP_v1_FROZEN.md` 로 migration
되었다(frozen). 차세대 엔진의 R&D 는 **별도 내부 라인**에서 진행하며 그 상세는
본 문서 범위 밖이다.

## Source-of-truth Priority

문서가 충돌하면 아래 우선순위로 해소한다.

| 우선순위 | 문서 | 역할 |
|---:|---|---|
| 1 | `core/spec/*` 코드 | frozen 계약 (인터페이스·카탈로그 상수·명명 규약). 코드가 source. |
| 2 | `docs/01-project-context.md` | 컨셉·scope·strong guarantee·preserve 결정. |
| 3 | `docs/02-module-architecture.md` | ConstraintModule + Profile architecture lock. |
| 4 | **`PRODUCTION_ROADMAP.md` (이 문서)** | go-forward stage·gate·deferred work 의 source-of-truth. |
| 4b | `PRODUCTION_ROADMAP_v1_FROZEN.md` | v1 stage·gate **이력**(frozen). closed gate 트레일. |
| 5 | `AGENTS.md`, `CLAUDE.md` | agent contract / 자동 로드 메모리. |
| 6 | `HANDOFF.md` | 현재 시점 인수인계. |

## 0. v1 상태 — frozen

v1 이 확보한 것:

- **제품 정체성** — 4-tier 솔밴시 카탈로그(T1~T4) ⟂ profile 직교, R7 freeze.
- **SaaS 이식성** — port inversion(R12-E/F/G), 자립 Go 모듈(`zk-pos-ext`), declarative profile.
- **동작 검증** — 4모델 E2E smoke, 1M-scale 측정(R11-D), GPU 검증(R13).

v1 은 **아키텍처/기능 freeze**. 신규 feature·model·module 을 올리지 않으며,
유지보수 fix + **first-customer pull-based closer** 만 진행한다.

## 1. v1 closers (pull-based)

| closer | 상태 | trigger |
|---|---|---|
| **G20 operator policy pin (B-wire)** | B-core landed (`core/tierpolicy` + `SnapshotSource.PolicyCommitment` + `VerifyCommitment`). **잔여**: profile.toml `[risk_policy] commitment` additive 필드 + snapshot-load digest 대조 reject(fail-closed). | 첫 operator-enforced 정책 고객 |
| **예금자-검증 신뢰** (self-inclusion verifier, G14) | 제품 표면. zk 가 보장 못 하는 *누락 공격* 보완책(`docs/01`). | 첫 customer SLA 협상 |

**원칙**: 고객(deal)이 당기지 않는 readiness 는 만들지 않는다.

T2 모델 메모: 회로에 `haircut_bp≤10000` cap(soundness)이 반영되어 있으나 production
key 는 재생성하지 않았다 — 재-ceremony 는 차세대 엔진 라인에서 일괄 처리하며,
**현재 v1 에서 T2 는 dormant** 다.

## 2. Scope Boundary

v1 출하 단위 = **backend + CLI + file artifacts**. UI/web/사용자-facing 페이지는
engine 밖, V1 scope 미포함. 이 boundary 가 모든 stage 의 exit criteria 해석 기준.

| Engine 안 (V1 scope) | Engine 밖 (external client / post-V1) |
|---|---|
| `core/spec/` 인터페이스 + 카탈로그 | 웹/모바일/임베드 위젯 |
| `core/solvency/<model>/circuit/` + `.pk`/`.vk`/`.r1cs` | self-verifier UI |
| `profile/<customer>/` 어댑터 set | customer 운영 인프라 (k8s, cron, S3, KMS) |
| witness / prover / userproof / verifier CLI | proof 시각화 / dashboard / 결과 UX |

의도: (1) audit boundary 단순화(CLI 입출력 + artifact format 만 감사), (2) customer
UX 자유, (3) `docs/01` 의 차별화 우위와 정합.

## 3. Gate Register (go-forward)

closed gates(G1·G2·G4~G13·G17~G19)의 결정 트레일은 **frozen 문서**에. 아래는
forward-relevant open gate 만.

| Gate | Status | 결정 / 잔여 |
|---|---|---|
| **G3** ConstraintModule 공개 API freeze | deferred | 첫 비-noop module 등장 시 surface 확정. |
| **G14** 사용자-facing verification 분배 | deferred | self-inclusion verifier. 첫 customer SLA pull. |
| **G16** Module composition compatibility | deferred | 첫 multi-module composition 시 process 확정. |
| **G20** RiskPolicy operator pin (B-wire) | deferred (pull) | B-core landed. 잔여 = profile.toml `[risk_policy] commitment` 필드 + snapshot-load reject. 첫 operator-enforced 정책 고객 전 close. |

### Gate → Trigger
```text
G3, G16  --> 첫 module / composition 등장 시
G14      --> 첫 customer SLA
G20      --> 첫 operator-enforced 정책 고객 (B-wire 2-items)
```
