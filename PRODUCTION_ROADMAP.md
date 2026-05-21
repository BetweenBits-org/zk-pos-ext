# Production Roadmap — zkpor Engine

이 문서는 production 구현 순서와 결정 게이트의 source-of-truth다. Part 3
(운영 로드맵 & 게이트 & 스테이지).

## Source-of-truth Priority

문서가 충돌하면 아래 우선순위로 해소한다.

| 우선순위 | 문서 | 역할 |
|---:|---|---|
| 1 | `core/spec/solvency_models.go`, `core/spec/batch_shape.go` 등 코드 | frozen 계약 (인터페이스 시그니처, 카탈로그 상수, 명명 규약). 코드와 문서가 어긋나면 코드가 source. |
| 2 | `docs/01-project-context.md` | 컨셉·scope·strong guarantee·preserve 결정. 계약 변경의 정합성 기준. |
| 3 | `PRODUCTION_ROADMAP.md` (이 문서) | stage·게이트·deferred work 의 source-of-truth. |
| 4 | `AGENTS.md`, `CLAUDE.md` | agent contract 및 자동 로드 메모리. |
| 5 | `HANDOFF.md` | 현재 시점 인수인계. 휘발성 — 다른 source와 충돌 시 후순위. |
| 6 | `../docs/*.md` (legacy historical notes) | 참고. source 아님. |

## Stages

production 구현 순서. 각 stage는 `목표 / 산출물 / exit criteria`를 가진다.

### Stage R0 — Decision gate triage

목표: 모든 결정을 닫는 게 아니라, 어떤 게이트가 어떤 stage를 막는지 분리한다.

Exit criteria:

- 각 게이트가 `closed` / `deferred` / `experimental` 중 하나로 표기된다.
- 다음 stage(R1)를 막는 게이트는 반드시 닫는다.
- 본 문서의 Gate Register 가 R0 완료 시점의 ground truth가 된다.

### Stage R1 — tier_3bucket 회로 이식

목표: legacy `../circuit/batch_create_user_circuit.go` 와 `../circuit/utils.go`
를 `core/solvency/tier_3bucket/circuit/` 로 이식한다.

산출물:

- `core/solvency/tier_3bucket/circuit/` 안에 BatchCreateUserCircuit + 회로
  유틸리티가 자리잡고 `core/circuit/` 의 universal 헬퍼를 호출하는 형태.
- legacy `../circuit/` 는 그대로 — 비교 reference로 보존.

Exit criteria:

- `go build ./zkpor/... && go vet ./zkpor/...` 통과.
- 같은 입력에 대해 신구 회로의 R1CS constraint 수 일치 확인 (구조 동일).
- `.pk`/`.vk` byte-equivalence 검증 (G1 closed).

Blocking gates: G1.

### Stage R2 — CSV ETL absorb

목표: `../src/utils/utils.go` 의 `ParseUserDataSet` 패밀리(자산 카탈로그
파싱, 사용자 CSV 파싱, RiskPolicy CSV 파싱)를 `profile/binance/snapshot.go`
+ `profile/binance/risk.go` + `profile/binance/catalog.go` 로 흡수한다.

산출물:

- `profile/binance/snapshot.go` 의 `errStubSnapshot` 제거. `AccountStream`,
  `CexAssets` 가 실제 데이터 yield.
- `RiskPolicy`, `AssetCatalog` 가 같은 CSV 출처에서 일관 구축.
- sample data (`../src/sampledata/`) 로 end-to-end snapshot 로드 작동.

Exit criteria:

- sample data 로드 → `AccountStream` 채널이 정상 채워짐.
- legacy ETL 출력과 신규 ETL 출력의 deterministic byte 비교 통과.
- G5 closed (RiskPolicy schema 결정).

Blocking gates: G5.

### Stage R3 — Service rewiring

목표: 4개 서비스(`../src/witness`, `../src/prover`, `../src/userproof`,
`../src/verifier`) 의 `main.go` 가 `zkpor/profile/binance` 어댑터를 사용하도록
재배선한다. legacy `../src/utils` import 제거.

산출물:

- 4개 서비스가 `zkpor/profile/binance` import 만으로 동작.
- 서비스 startup에서 `ValueScale` invariant assert (G6 closed).
- `AccountIDProvider.Scheme()` 가 customer 자체 derivation 정식으로 freeze
  (G2 closed).

Exit criteria:

- sample data 기준 end-to-end PoR 생성·검증 통과 (witness → proof →
  verifier).
- `.vk` 파일이 새 명명 규약(`zkpor.tier_3bucket.<shape>.vk`) 으로 생성.
- legacy `.pk`/`.vk` 와 byte-equivalent (G1 재확인).

Blocking gates: G2, G6.

### Stage R4 — Second customer profile (deferred, awaits signal)

목표: 첫 비-Binance 고객사 프로파일 도입. `profile/<customer>/` 추가.

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

- 새 model 의 `core/solvency/<id>/spec/` + `circuit/` + 도구.
- 새 trusted setup ceremony 완료, `.pk`/`.vk` publish.
- 두 model이 같은 `core/circuit/` substrate를 공유 — 추상화 검증.

Exit criteria:

- 새 model 회로 audit 완료.
- substrate 가 두 model 모두 지원하는지 확인 (rule-of-three 두 번째 event).
- 첫 substrate refactor 후보 식별 → R6 로 carry.

### Stage R6 — Third model + core/circuit 보강 (rule-of-three trigger)

목표: 세 번째 model 구현 시점. 이때 처음으로 `core/circuit/` 에 추가 헬퍼
승격 검토 (G11). 후보: RLC-based sum equality helper, account leaf
composition helper.

산출물:

- 세 번째 model 의 회로 구현.
- 세 model 모두에 공통으로 적용 가능한 패턴이 `core/circuit/` 으로 승격됨.
- substrate API v1 잠정 정착.

Exit criteria:

- 세 model 모두 `core/circuit/` 새 헬퍼 호출 형태로 정리.
- G11 closed.

Blocking gates: G11.

### Stage R7 — v1 catalog freeze

목표: 5-tier 카탈로그 모두 (또는 우선 정해진 subset) 구현 완료. v1 카탈로그
stable 선언. 추가 model은 v2 카탈로그로 미룬다.

산출물:

- 5개 model 모두 회로·spec·trusted setup 완료 (혹은 일부 deprecated 처리).
- `core/spec/solvency_models.go` 가 v1 카탈로그로 freeze.
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
| **G1** trusted-setup byte-equivalence 검증 방법 | deferred | R1 | 미정. 후보: legacy R1CS 출력 hash 비교 또는 .pk SHA256 비교. | R1 진입 전 검증 절차 합의. |
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

## Gate → Stage Dependency

어떤 게이트가 어떤 stage를 막는지.

```text
G1  --> R1 (trusted-setup byte-equivalence)
G2  --> R3 (identity scheme freeze)
G3  --> R3+ (first non-noop module 등장 시)
G4  --> R7 (catalog freeze)
G5  --> R2 (RiskPolicy schema)
G6  --> R3 (ValueScale assert)
G10 --> R7 (LegacyKeyName deprecate)
G11 --> R6 (core/circuit promotion)
G12 --> R4 (multi-customer .vk policy)

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
