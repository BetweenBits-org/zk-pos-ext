# Production Roadmap — zkpor Engine

이 문서는 production 구현 순서와 결정 게이트의 source-of-truth다. Part 3
(운영 로드맵 & 게이트 & 스테이지).

## Source-of-truth Priority

문서가 충돌하면 아래 우선순위로 해소한다.

| 우선순위 | 문서 | 역할 |
|---:|---|---|
| 1 | `zkpor/core/spec/solvency_models.go`, `zkpor/core/spec/batch_shape.go` 등 코드 | frozen 계약 (인터페이스 시그니처, 카탈로그 상수, 명명 규약). 코드와 문서가 어긋나면 코드가 source. |
| 2 | `zkpor/docs/01-project-context.md` | 컨셉·scope·strong guarantee·preserve 결정. 계약 변경의 정합성 기준. |
| 3 | `zkpor/PRODUCTION_ROADMAP.md` (이 문서) | stage·게이트·deferred work 의 source-of-truth. |
| 4 | `zkpor/AGENTS.md`, `zkpor/CLAUDE.md` | agent contract 및 자동 로드 메모리. |
| 5 | `zkpor/HANDOFF.md` | 현재 시점 인수인계. 휘발성 — 다른 source와 충돌 시 후순위. |
| 6 | `docs/*.md` (legacy historical notes) | 참고. source 아님. |

## Scope Boundary

zkpor engine 의 출하 단위는 **backend + CLI + file artifacts** 다. UI
/ web frontend / 사용자-facing 검증 페이지는 engine 밖, V1 scope 에
포함하지 않는다. 이 boundary 는 모든 stage 의 exit criteria 를 해석하는
기준이며, 위에서 surface 되지 않은 작업은 engine 책임이 아니다.

| Engine 안 (V1 scope) | Engine 밖 (external client / post-V1) |
|---|---|
| `zkpor/core/spec/` 인터페이스 + 카탈로그 | 웹 페이지 / 모바일 앱 / 임베드 위젯 |
| `zkpor/core/solvency/<model>/circuit/` + `.pk`/`.vk`/`.r1cs` artifact | 사용자가 자기 inclusion 을 확인하는 self-verifier UI |
| `zkpor/profile/<customer>/` 어댑터 set | customer 의 운영 인프라 (k8s, cron, S3, KMS) |
| `src/witness` CLI — witness 생성 | proof 시각화 / dashboard |
| `src/prover` CLI — groth16.Prove → proof 파일 | inclusion proof 결과의 UX (성공/실패 페이지) |
| `src/userproof` CLI — 사용자별 Merkle inclusion proof → DB 행 | 사용자에게 inclusion proof 를 노출하는 customer-facing 페이지 |
| `src/verifier` CLI — `groth16.Verify` → exit code | PoR 검증 결과의 일반인용 분배 (호스팅 페이지 등) |

이 분할의 의도:

1. **Audit boundary 단순화** — CLI 입출력 + artifact format 만 감사
   대상. UI 갈아끼워도 audit 무효 안 됨.
2. **Customer flexibility** — 각 customer 가 자기 UX (web / mobile /
   internal dashboard) 를 자유롭게 wrap. engine 은 UI dep 을 강제하지
   않는다.
3. **차별화 우위와 정합** — `docs/01-project-context.md` 의 "차별화
   우위 = customer 통합 비용 + audit trust; tech/UX 는 후순위" 와 같은
   결.
4. **R3 / R4 acceptance 명확화** — "사용자가 자기 잔고를 확인할 도구
   없음" 은 engine 결함이 아니라 customer / partner 영역. SLA 협상
   대상 (G14).

reference CLI / sample 검증 도구 정도는 engine 옆에 둘 수 있으나, 그
범위는 V1 이후 별도 결정 (G14 참조).

## Stages

production 구현 순서. 각 stage는 `목표 / 산출물 / exit criteria`를 가진다.

### Stage R0 — Decision gate triage

목표: 모든 결정을 닫는 게 아니라, 어떤 게이트가 어떤 stage를 막는지 분리한다.

Exit criteria:

- 각 게이트가 `closed` / `deferred` / `experimental` 중 하나로 표기된다.
- 다음 stage(R1)를 막는 게이트는 반드시 닫는다.
- 본 문서의 Gate Register 가 R0 완료 시점의 ground truth가 된다.

### Stage R1 — tier_3bucket 회로 이식

목표: legacy `circuit/batch_create_user_circuit.go` 와 `circuit/utils.go`
를 `zkpor/core/solvency/tier_3bucket/circuit/` 로 이식한다.

산출물:

- `zkpor/core/solvency/tier_3bucket/circuit/` 안에 BatchCreateUserCircuit + 회로
  유틸리티가 자리잡고 `zkpor/core/circuit/` 의 universal 헬퍼를 호출하는 형태.
- legacy `circuit/` 는 그대로 — 비교 reference로 보존.

Exit criteria:

- `go build ./zkpor/... && go vet ./zkpor/...` 통과.
- legacy `circuit/` 와 zkpor `core/solvency/tier_3bucket/circuit/` 가
  line-by-line port 형태로 대응 (구조 동일).
- 회로 IR 컴파일 (`frontend.Compile` 성공) 검증과 `.pk`/`.vk`
  byte-equivalence (G1 closure) 는 **R3 step 0 / step 3 으로 carry**.
  R1 자체는 코드 port 완료 시점에서 마감.

Blocking gates: (없음 — G1 은 R3 step 3 에서 closure).

### Stage R2 — CSV ETL absorb

목표: `src/utils/utils.go` 의 `ParseUserDataSet` 패밀리(자산 카탈로그
파싱, 사용자 CSV 파싱, RiskPolicy CSV 파싱)를 `zkpor/profile/binance/snapshot.go`
+ `zkpor/profile/binance/risk.go` + `zkpor/profile/binance/catalog.go` 로 흡수한다.

산출물:

- `zkpor/profile/binance/snapshot.go` 의 `errStubSnapshot` 제거. `AccountStream`,
  `CexAssets` 가 실제 데이터 yield.
- `RiskPolicy`, `AssetCatalog` 가 같은 CSV 출처에서 일관 구축.
- sample data (`src/sampledata/`) 로 end-to-end snapshot 로드 작동.

Exit criteria:

- sample data 로드 → `AccountStream` 채널이 정상 채워짐.
- legacy ETL 출력과 신규 ETL 출력의 deterministic byte 비교 통과.
- G5 closed (RiskPolicy schema 결정).

Blocking gates: G5.

### Stage R3 — 회로/Setup 검증 + Service rewiring

목표: R1 에서 carry 된 G1 (byte-equivalence) 을 닫고, Constraint
Architecture (`c1⊕c2⊕c3⊕c4 ⊕ L[k] ⊕ alpha(profile)`) 의 **alpha
layer wiring** 을 회로 코드에 반영한 뒤, 4개 서비스 (`src/witness`,
`src/prover`, `src/userproof`, `src/verifier`) 의 `main.go` 를
`zkpor/profile/binance` 어댑터로 재배선한다.

R3 는 5 sub-slice 로 나뉜다. 각 step 는 자체 commit 단위.

#### R3 step 0 — Setup smoke (수준 A)

목표: `frontend.Compile + groth16.Setup` 이 zkpor tier_3bucket 회로에서
에러 없이 끝나는지 확인. 회로 IR 결함을 alpha wiring / byte-equivalence
작업 전에 잡는다.

산출물:

- `zkpor/core/solvency/tier_3bucket/circuit/setup_test.go` — BatchShape
  (예: `{50, 700}`) 인스턴스로 Compile + Setup 호출.
- `oR1cs.GetNbConstraints()` 출력 (legacy 와 fuzzy 비교; 정확 일치는
  step 3 의 G1 closure 단계).

Exit criteria:

- Setup smoke test 통과.
- 회로 코드 외 변경 0.

#### R3 step 1 — AccountID fr.Element 정규화 위치 결정 (G13 closure)

목표: legacy 는 `new(fr.Element).SetBytes(id).Marshal()` round-trip 으로
ID 를 commitment-ready 형태로 normalize 한다. zkpor 는 현재 passthrough.
SHA256-derived ID 약 절반이 modulus 이상이라 byte-equivalence 가 깨진다.
정규화를 (a) snapshot 어댑터 / (b) identity provider / (c) witness builder
중 어디에 둘지 결정한다.

산출물:

- 결정 노트 (commit 메시지 또는 short ADR) 에 채택 layer + 근거 기록.
- Decision Gate Register 의 G13 status: `deferred → closed`.

Exit criteria:

- G13 closed.
- impl 은 step 2 로 carry.

#### R3 step 2 — alpha wiring + fr.Element 적용

목표: Constraint Architecture 의 alpha layer 를 회로 코드에 반영.
`BatchCreateUserCircuit.Define(api)` 가 `ConstraintModule.Define(api, ctx)`
를 호출하는 hook 을 추가한다. R3 step 1 결정대로 fr.Element 정규화를
선택된 layer 에 적용.

산출물:

- `Define()` 가 module hook 호출 (또는 wrapper circuit 이 module 합성).
- `NewBatchCreateUserCircuit` (또는 외부 builder) 가 module 을 받는 형태.
- 선택된 layer 에 fr.Element 정규화 코드 추가.
- noopModule 로 회귀 없음 확인 (step 0 의 `NbConstraints` 와 동일).

Exit criteria:

- Setup smoke (step 0) 가 alpha wiring 적용 후에도 통과.
- noopModule 인 경우 constraint 수 변동 0.

#### R3 step 3 — G1 검증 절차 합의 + 실행

목표: G1 (trusted-setup byte-equivalence) 의 검증 방법을 합의하고
실행한다. 두 후보:

- (a) legacy `circuit/` 의 R1CS hash 와 zkpor `core/solvency/tier_3bucket/
  circuit/` 의 R1CS hash 비교.
- (b) legacy `.pk` SHA256 과 zkpor `.pk` SHA256 비교.

산출물:

- Decision Gate Register G1 entry: `deferred → closed`.
- 합의된 절차에 따라 zkpor 회로의 R1CS / `.pk` / `.vk` 가 legacy 와
  byte-equivalent 임을 입증하는 테스트 또는 ad-hoc 스크립트.

Exit criteria:

- G1 closed.
- 합의된 비교 산출물이 byte-equivalent.

#### R3 step 4 — 4 service main.go rewiring (R3 본체)

목표: step 0..3 까지 닫힌 회로 + 새 `.vk` 명명으로 4개 서비스 wire-up.
`src/witness`, `src/prover`, `src/userproof`, `src/verifier` 의 `main.go`
가 `zkpor/profile/binance` 어댑터를 사용하도록 재배선. legacy `src/utils`
import 제거. `ValueScale` invariant assert (G6). `AccountIDProvider.
Scheme()` v1 freeze (G2).

**Slice 분해는 agent 자율**. step 4 는 한 commit 이 아니라 4 서비스
별 commit (witness → prover → userproof → verifier, 또는 의존도 따라).
agent 는 진입 시 분해를 HANDOFF Resume Actions 에 자기 슬라이스로
박는다. 같은 commit 에 묶는 것은 import 경로 동시 교체 같은 사소한
변경에 한정한다. 4 서비스 사이의 결합도 (DB 스키마 공유, file
hand-off, witness→prover artifact 의존 등) 는 코드를 만져봐야 드러나
므로 사전 분해를 박지 않는다.

산출물:

- 4개 서비스가 `zkpor/profile/binance` import 만으로 동작.
- 서비스 startup `ValueScale` assert (G6 closed).
- `AccountIDProvider.Scheme()` v1 freeze (G2 closed).
- `.vk` 파일이 새 명명 규약(`zkpor.tier_3bucket.<shape>.vk`) 으로 생성.

Exit criteria:

- sample data 기준 **CLI** end-to-end PoR 생성·검증 통과 (witness →
  proof → verifier). userproof 서비스가 사용자별 inclusion proof
  데이터를 DB 행으로 적재.
- G2, G6 closed.
- **Engine boundary**: 사용자-facing UI / web frontend / inclusion 검증
  페이지는 engine 밖, V1 scope 미포함 (`## Scope Boundary` 참조).

Blocking gates: G1 (step 3), G2 (step 4), G6 (step 4), G13 (step 1).

### Stage R4 — Second customer profile (deferred, awaits signal)

목표: 첫 비-Binance 고객사 프로파일 도입. `zkpor/profile/<customer>/` 추가.

산출물:

- 새 고객사 어댑터 8개 (catalog, pricing, identity, insolvent, batch_shape,
  risk, snapshot, constraint_noop or custom).
- 해당 고객사가 채택할 model 결정 (Q3 답).
- 두 고객사가 같은 model을 쓸 때 `.vk` 공유 정책 (G12 closed, Q7 답).

Exit criteria:

- 새 고객사 sample data로 end-to-end PoR 통과.
- multi-customer 운영 시 `.vk` 공유/분리 정책 문서화.

Blocking gates: G12.

### Stage R5 — Second model implementation (rule-of-three first event)

목표: 카탈로그 두 번째 model 회로 구현. 후보: `spot_simple` (한국 spot
거래소) 또는 `merkle_classic` (Bybit-class). R4 의 customer 선택과 연결.

산출물:

- 새 model 의 `zkpor/core/solvency/<id>/spec/` + `circuit/` + 도구.
- 새 trusted setup ceremony 완료, `.pk`/`.vk` publish.
- 두 model이 같은 `zkpor/core/circuit/` substrate를 공유 — 추상화 검증.

Exit criteria:

- 새 model 회로 audit 완료.
- substrate 가 두 model 모두 지원하는지 확인 (rule-of-three 두 번째 event).
- 첫 substrate refactor 후보 식별 → R6 로 carry.

### Stage R6 — Third model + core/circuit 보강 (rule-of-three trigger)

목표: 세 번째 model 구현 시점. 이때 처음으로 `zkpor/core/circuit/` 에 추가 헬퍼
승격 검토 (G11). 후보: RLC-based sum equality helper, account leaf
composition helper.

산출물:

- 세 번째 model 의 회로 구현.
- 세 model 모두에 공통으로 적용 가능한 패턴이 `zkpor/core/circuit/` 으로 승격됨.
- substrate API v1 잠정 정착.

Exit criteria:

- 세 model 모두 `zkpor/core/circuit/` 새 헬퍼 호출 형태로 정리.
- G11 closed.

Blocking gates: G11.

### Stage R7 — v1 catalog freeze

목표: 5-tier 카탈로그 모두 (또는 우선 정해진 subset) 구현 완료. v1 카탈로그
stable 선언. 추가 model은 v2 카탈로그로 미룬다.

산출물:

- 5개 model 모두 회로·spec·trusted setup 완료 (혹은 일부 deprecated 처리).
- `zkpor/core/spec/solvency_models.go` 가 v1 카탈로그로 freeze.
- LegacyKeyName deprecate 일정 결정 (G10 closed).

Exit criteria:

- 카탈로그 stability 선언 문서화.
- 신규 model 제안은 v2 정의서로 격상.

Blocking gates: G4, G10.

## Decision Gate Register

닫아야 할 설계 결정. 상태 의미:

| 상태 | 의미 |
|---|---|
| `closed` | 구현이 의존해도 되는 결정. 변경은 versioned change. |
| `deferred` | 지금은 막지 않지만 지정된 blocker stage 전에는 닫아야 함. |
| `experimental` | fixture/test 편의용 임시값. 공개 계약에 노출 금지. |

| Gate | Status | Blocker stage | 결정 / 현재 marker | Next action |
|---|---|---|---|---|
| **G1** trusted-setup byte-equivalence 검증 방법 + 실행 | deferred | R3 step 3 | 미정. 후보: (a) legacy 와 zkpor 의 R1CS hash 비교, (b) legacy 와 zkpor 의 `.pk` SHA256 비교. R3 step 0 (Setup smoke), step 1 (G13), step 2 (alpha wiring) 가 G1 의 전제. | R3 step 3 진입 시 (a)/(b) 중 합의 → 실행 → closed. |
| **G2** AccountIDProvider scheme v1 freeze | deferred | R3 | `passthrough_hex.v0` 임시. customer-side derivation 가정. | R3 전 HMAC/salt 정식 derivation 채택 여부 결정. |
| **G3** ConstraintModule 공개 API freeze | deferred | R3 후 | 현재 `ConstraintContext` 가 minimal surface. 두 번째 module 등장 시 확정. | 첫 비-noop module 등장 시 API surface 검토. |
| **G4** catalog stability 선언 | deferred | R7 | 5-tier 잠정 확정. 회로 구현은 1/5. | 모든 model 구현 후 freeze. |
| **G5** RiskPolicy 데이터 schema | deferred | R2 | 현재 `cex_assets_info.csv` 형식 (legacy). | CSV 유지 vs JSON/YAML schema 도입 결정. |
| **G6** ValueScale invariant assert 위치 | experimental | R3 | spec 명시되어 있으나 service 코드에서 assert 없음. | R3 wiring 시 startup assert 추가. |
| **G7** InvalidAccountPolicy 운영 정책 | closed | R0 | drop + log (legacy 동등). | customer 요구 시 별도 정책. 변경 시 customer review. |
| **G8** BatchShape v1 정착 (binance) | closed | R0 | `{50,700}` + `{500,92}` (Binance reference). | 다른 customer 시 별도 shape 정의. |
| **G9** module ID 명명 규약 | closed | R0 | `<exchange>.<rule>_v<version>` 형식. filename-safe (lowercase, digits, dots, underscores). | — |
| **G10** LegacyKeyName 폐기 일정 | deferred | R7 | 현재 호환 유지 (`BatchShape.LegacyKeyName()`). | catalog freeze 후 한 release에서 deprecate. |
| **G11** core/circuit 추가 헬퍼 승격 규약 | deferred | R6 | rule-of-three — 3번째 model 등장 시 검토. | 세 번째 model 구현 시점. |
| **G12** multi-customer profile 충돌 정책 | deferred | R4 | profile/<customer>/ 단일 패키지 가정. shape/.vk 공유 정책 미정. | 두 번째 customer 등장 시. |
| **G13** AccountID fr.Element 정규화 위치 | closed | R3 step 1 | **(a) snapshot 어댑터** 채택. legacy `src/utils/utils.go:553` 와 동일 layer 에서 `new(fr.Element).SetBytes(id).Marshal()` round-trip. 근거: G1 byte-equivalence 비용 최저 (snapshot 출력 hex 직접 비교 가능), `AccountInfo.AccountID == userproof.AccountID == field input` 단일 형태 유지, R3 step 4 service rewire 시 호출 누락 위험 없음. 트레이드오프: `profile/binance/snapshot.go` 가 bn254 에 직접 결합 — 현재 카탈로그 5 model 전부 bn254 라 실질 충돌 없음, 두 번째 customer profile (R4) 등장 시 R6 helper 승격 후보로 carry. (b)/(c) 는 layering 더 깔끔하나 user-facing inconsistency / interface 확장 / 회귀 위험으로 기각. | impl: R3 step 2 (alpha wiring 과 동반). `AccountIDProvider.Scheme()` 명칭 갱신은 R3 step 4 (G2 closure) 동반. |
| **G14** 사용자-facing verification 분배 책임 | deferred | post-V1 / customer SLA | V1 engine 은 CLI + file artifact + userproof DB 행만 출하. 사용자가 자기 inclusion 을 확인하는 UI / 페이지는 engine 밖 (`## Scope Boundary` 참조). 후보 owner: (a) customer 가 자체 UI 구축, (b) partner / SI 가 reference UI 제공, (c) zkpor 가 reference open-source CLI/static page 부속 제공. | 첫 customer 통합 (R4 진입) 시 SLA 협상 항목으로 surface. V1 안에서는 결정 보류. |

## Gate → Stage Dependency

어떤 게이트가 어떤 stage를 막는지.

```text
G1  --> R3 step 3 (trusted-setup byte-equivalence)
G2  --> R3 step 4 (identity scheme freeze)
G3  --> R3+ (first non-noop module 등장 시)
G4  --> R7 (catalog freeze)
G5  --> R2 (RiskPolicy schema)
G6  --> R3 step 4 (ValueScale assert)
G10 --> R7 (LegacyKeyName deprecate)
G11 --> R6 (core/circuit promotion)
G12 --> R4 (multi-customer .vk policy)
G13 --> R3 step 1 (AccountID fr.Element normalization)
G14 --> post-V1 / customer SLA (user-facing verification distribution)

(G7, G8, G9 는 R0 시점에 closed)
```

## Parallel Workstreams

병렬 진행 가능한 작업 줄기와 의존.

```text
Foundation                      Customer Path                  Catalog Maturity
───────────                     ─────────────                  ─────────────────
R1 (circuit port)
  │
  v
R2 (CSV absorb)
  │
  v
R3 (service rewire)  ─────────> R4 (second customer)
                                  │
                                  v
                                R5 (second model)  ──────────> R6 (third + promotion)
                                                                  │
                                                                  v
                                                                R7 (catalog freeze)
```

Foundation (R1+R2+R3) 은 직렬. R4 는 R3 완료 후 진입. R5 는 R4 의 model
선택에 의존. R6·R7 은 시장 신호 따라.
