# Production Roadmap — zkpor Engine

이 문서는 production 구현 순서와 결정 게이트의 source-of-truth다. Part 3
(운영 로드맵 & 게이트 & 스테이지).

## Source-of-truth Priority

문서가 충돌하면 아래 우선순위로 해소한다.

| 우선순위 | 문서 | 역할 |
|---:|---|---|
| 1 | `zkpor/core/spec/solvency_models.go`, `zkpor/core/spec/batch_shape.go` 등 코드 | frozen 계약 (인터페이스 시그니처, 카탈로그 상수, 명명 규약). 코드와 문서가 어긋나면 코드가 source. |
| 2 | `zkpor/docs/01-project-context.md` | 컨셉·scope·strong guarantee·preserve 결정. 계약 변경의 정합성 기준. |
| 3 | `zkpor/docs/02-module-architecture.md` | ConstraintModule + Profile descriptor 의 architecture lock (composition / catalog / param 규약). |
| 4 | `zkpor/PRODUCTION_ROADMAP.md` (이 문서) | stage·게이트·deferred work 의 source-of-truth. |
| 5 | `zkpor/AGENTS.md`, `zkpor/CLAUDE.md` | agent contract 및 자동 로드 메모리. |
| 6 | `zkpor/HANDOFF.md` | 현재 시점 인수인계. 휘발성 — 다른 source와 충돌 시 후순위. |
| 7 | `docs/*.md` (legacy historical notes) | 참고. source 아님. |

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

#### R3 step 2 — alpha wiring + fr.Element 적용 ✅ closed

목표: Constraint Architecture 의 alpha layer 를 회로 코드에 반영.
`BatchCreateUserCircuit.Define(api)` 가 `ConstraintModule.Define(api, ctx)`
를 호출하는 hook 을 추가한다. R3 step 1 결정대로 fr.Element 정규화를
선택된 layer 에 적용.

산출물 (commit ccc3fe4):

- `BatchCreateUserCircuit` 에 unexported `module
  tier3spec.ConstraintModule` 필드 + pointer-receiver
  `SetConstraintModule` setter. gnark frontend 가 exported
  Variable-bearing 필드만 reflect 하므로 module 필드는 Compile 에
  비가시. 외부 wrapper circuit 합성 패턴은 채택하지 않음 — 같은 hook
  계약을 단일 Circuit 타입 안에 둠으로써 service 가 `c.SetConstraintModule(m)`
  한 줄로 alpha 를 갈아 끼울 수 있음.
- Define() 끝 (모든 base 제약 emit 후) 에서 `module.Define(api, ctx)`
  호출. ConstraintContext 는 per-user 총액 (Equity/Debt/CollateralReal)
  을 user 루프 안에서 capture 한 `moduleUserOps` 슬라이스 + Before/After
  CexAssets view + 공유 Rangechecker 로 구성. flat-copy helper 2개
  (`toCircuitCexAssetView`, `toCircuitTierRatioView`) 가 in-circuit
  타입 → spec view 타입 변환을 in-circuit 비용 0 으로 처리.
- `profile/binance/snapshot.go::parseAccountRow` 에 bn254 fr.Element
  SetBytes→Marshal round-trip 1줄. legacy `src/utils/utils.go:553` 와
  동일 layer (G13 (a) 채택의 impl).
- 회귀 가드 — `TestSetupSmoke_AlphaNoopBaseline` 가 nil-module +
  noop-module 두 경로에서 `NbConstraints == 723790` 동일함을 assert.
  `TestParseAccountRow_NormalizesAccountID` 가 all-FF 32-byte 입력 →
  reduced output positive guard.

Exit criteria:

- ✅ Setup smoke (step 0 + alpha-baseline 둘 다) 통과 — NbConstraints
  변동 0 (723790).
- ✅ noopModule 인 경우 constraint 수 변동 0.

#### R3 step 3 — G1 검증 절차 합의 + 실행 ✅ closed

목표: G1 (trusted-setup byte-equivalence) 의 검증 방법을 합의하고
실행한다. 채택 후보:

- (a) legacy 와 zkpor R1CS 의 L·R==O 행렬 비교 — `bn254.R1CS.GetR1Cs()`
  로 추출한 후 SHA256. **채택**.
- (b) legacy `.pk` SHA256 과 zkpor `.pk` SHA256 비교 — `groth16.Setup`
  이 `sampleToxicWaste()` 로 randomness 를 뽑으므로 deterministic
  하지 않다. 동일한 toxic waste 를 재사용해야만 .pk 가 byte-equal —
  과거 production ceremony 의 waste 는 (의도적으로) 파기되어 있어
  불가능. 기각.

산출물 (commit 1398e04):

- `zkpor/core/solvency/tier_3bucket/circuit/legacy_compare_test.go`
  `TestLegacyCompare_R1CSStructure` — tiny shape (5, 50, 2) 에서
  legacy + zkpor 각각 `frontend.Compile` → `*bn254.R1CS.GetR1Cs()` →
  L/R/O term 직렬화 SHA256. 두 hash 동일.
    legacy R1CS sha256 = 678eb23f62a9932bb93a8f0811db3b64a4bfd8eadb5e743791d93b27c0b95b32
    zkpor  R1CS sha256 = 678eb23f62a9932bb93a8f0811db3b64a4bfd8eadb5e743791d93b27c0b95b32
  Coefficient table SHA256 도 동일 — fail-fast 보조 signal. tiny
  shape 에서의 일치는 production shape 일치를 내포 — Define 이
  shape-invariant (loop-driven) 하기 때문.
- `zkpor/profile/binance/legacy_compare_test.go`
  `TestLegacyCompare_SampleDataAccountIDs` — sample_users0.csv (100
  rows) 를 legacy `ReadUserDataFromCsvFile` 와 zkpor
  `csvSnapshot.AccountStream` 양쪽에 흘려 AccountID 90개 valid +
  10개 invalid 분류 까지 모두 일치.

의도적 제외 (G1 의 정의 안에서 무관):

- Hint identifier (`solver.HintID`) — Go reflect path 로 derive
  되므로 legacy `circuit.IntegerDivision` 과 zkpor
  `corecircuit.IntegerDivision` 이 다른 ID 를 가진다. 단, hint 는
  solver-side metadata 이며 A·s ∘ B·s = C·s 매트릭스에 기여하지 않음.
  각 service 가 zkpor IntegerDivision 을 `solver.RegisterHint` 로
  등록해야 함 — R3 step 4 wiring.
- gnark debug metadata (SymbolTable, DebugInfo, MDebug, Logs) —
  source path / line number 를 담아 byte 비교를 잘못 깨뜨림. .pk/.vk
  에 영향 0 이므로 비교에서 제외.

Production-shape 일회성 검증 절차 (필요 시 후속 agent 실행 가능):

```
shape constants in TestLegacyCompare_R1CSStructure
  userAssetCounts=5  → 50
  allAssetCounts=50  → 500
  batchCounts=2      → 700
go test -run TestLegacyCompare_R1CSStructure \
  ./core/solvency/tier_3bucket/circuit/... \
  -timeout 60m -v
예상 비용: 분 단위 compile + 수 GB peak memory (양쪽 합산).
```

Exit criteria:

- ✅ G1 closed.
- ✅ Tiny-shape R1CS L·R==O byte-equivalence 통과 (commit 1398e04).
- ✅ Sample-corpus AccountID byte-equivalence 90/90 pass + 10/10
  invalid 분류 parity (commit 1398e04).

#### R3 step 4 — service rewiring via zkpor/cmd/* (R3 본체)

목표: step 0..3 까지 닫힌 회로 + 새 `.vk` 명명으로 4개 서비스 wire-up.
**legacy `src/witness`, `src/prover`, `src/userproof`, `src/verifier` 는
직접 수정하지 않는다 (AGENTS.md "legacy 수정 금지" 정합)**. 대신
`zkpor/cmd/{witness,prover,userproof,verifier}` 에 신규 entry 를 두고
`zkpor/profile/binance` + `zkpor/core/solvency/tier_3bucket/...` 어댑터
로 service 를 합성한다. legacy `src/` 는 untouched reference (trusted
setup 그대로 유효) 로 보존되며, zkpor binary 가 점진적으로 대체한다.
`ValueScale` invariant assert (G6). `AccountIDProvider.Scheme()` v1
freeze (G2). 4 service 의 service-logic (witness/prover/model package)
도 legacy 패키지를 import 하지 않고 zkpor 측에서 새로 합성한다 — legacy
`src/utils` 에 있던 host-side 헬퍼 (native Poseidon 패킹, Merkle proof
verify, CexCommitment 등) 은 zkpor 내 적절한 layer 로 추출·이식한다.

**Slice 분해는 agent 자율**. step 4 는 한 commit 이 아니라 service 별
+ host-helper 추출 commit (예: verifier → witness → prover → userproof,
또는 의존도 따라). agent 는 진입 시 분해를 HANDOFF Resume Actions 에
자기 슬라이스로 박는다. 같은 commit 에 묶는 것은 import 경로 동시
교체 같은 사소한 변경에 한정한다. 4 서비스 사이의 결합도 (DB 스키마
공유, file hand-off, witness→prover artifact 의존 등) 는 코드를
만져봐야 드러나므로 사전 분해를 박지 않는다.

산출물:

- `zkpor/cmd/{witness,prover,userproof,verifier}` 4 개 신규 binary —
  legacy `src/utils` 또는 `src/{witness,prover,userproof,verifier}`
  import 0.
- 서비스 startup `ValueScale` assert (G6 closed).
- `AccountIDProvider.Scheme()` v1 freeze (G2 closed).
- `.vk` 파일이 새 명명 규약(`zkpor.tier_3bucket.<shape>.vk`) 으로 생성.

Exit criteria:

- sample data 기준 **CLI** end-to-end PoR 생성·검증 통과 (witness →
  proof → verifier). userproof 서비스가 사용자별 inclusion proof
  데이터를 DB 행으로 적재. **CLOSED** — `scripts/smoke.sh` 가 docker
  compose mysql → keygen → witness → prover → verifier(batch, DB
  direct) → userproof → verifier(-user) 풀 파이프라인을 통과 (commit
  d7c23f3, A5). Sample data (170 valid accounts, 17 batches) tiny
  shape (5, 5, 10), 모든 stage 가 account tree root `142c03677f6f…`
  일치 + account #0 inclusion `verify pass!!!`.
- G2, G6 closed.
- **Engine boundary**: 사용자-facing UI / web frontend / inclusion 검증
  페이지는 engine 밖, V1 scope 미포함 (`## Scope Boundary` 참조).

R3 step 4 안에서 fold 된 부수 작업 (이번 closure 에서 동반 완료):

- **AssetCounts 재배치 (slice E, commit 1d5571b)** — `corespec.AssetCounts
  = 500` 이 core/spec 에 박혀 있었으나 실제로는 deployment cap.
  `AssetCatalog.Capacity()` 가 단일 진실원으로 격상, `SnapshotConfig.
  AssetCapacity` + 4 service config 의 `AssetCapacity` 로 흐름. R5 의
  declarative `profile.toml` (G12) 슬롯에 자연 매핑. Smoke 가 tiny
  capacity 로 가능해진 부수 효과 (286k constraints, keygen 21s).
- **Shape override env var (slice A1, commit 11f2d0a)** —
  `ZKPOR_BATCH_SHAPE_OVERRIDE` 가 binance production shape 을 smoke
  shape 으로 swap. 서비스/테스트/config 무변경.
- **Verifier DB direct proof read (slice A4, commit f1ba54a)** —
  proof.csv 중간 hop 제거, `ProofStore.ListAllInOrder` 로 verifier 가
  직접 ingest. legacy CSV 경로도 backward-compat.
- **3 버그 fix (A5 commit d7c23f3)** — bsmt `Commit(&v)` 가 pruning
  version 임을 발견, `Commit(nil)` 로 통일. `SetBatchCreateUserCircuit
  Witness` padding entries 의 6개 collateral 필드 zero-init 누락
  (gnark `can't set fr.Element with <nil>`) → `paddingAsset` 헬퍼로
  통일. verifier 의 per-asset equity≥debt panic → warning (tier_3bucket
  모델은 자산별 차용 허용).

R3 step 4 미잔존 follow-ups (post-smoke, 별도 슬라이스):

- witness multi-worker 병렬 + DB resume + tree rollback
- prover Redis BLPOP 큐 + -rerun 모드 + multi-worker
- userproof multi-worker + resume + -memory_tree 플래그
- snapshot multi-shard concurrency
- `core/constraint_modules/noop/` promotion (universal layer 로 이동)
- Store driver 인터페이스 + PG adapter (slice D, deferred)
- EC2 원격 sync 스크립트 (slice F, deferred)

Blocking gates: G1 (step 3), G2 (step 4), G6 (step 4), G13 (step 1).
**모두 closed.**

### Stage R4 — Second model implementation: `spot_simple` (SEA GTM driver)

목표: SEA 시장의 80~100% 가 spot-dominant 라는 조사 결과
(`docs/01-project-context.md` 의 "SEA 시장 zoom-in") 에 따라, **GTM 우선
모델인 `spot_simple` 회로를 customer signal 없이 model-first 로 구현**한다.

이전 stage 정의 (R4 = customer-first) 와 swap 한 이유: customer 가 선택할
model 이 사실상 정해져 있는 시장 (SEA spot) 에서 "customer signal 기다리기"
가 dead time 이 된다. R3 까지 검증된 universal substrate (`core/spec/` +
`core/circuit/`) 가 두 번째 model 을 빠르게 받을 수 있는지를 검증하는
첫 event 이기도 함 (rule-of-three first event).

산출물:

- `zkpor/core/solvency/spot_simple/spec/*` — types, RiskPolicy,
  SnapshotSource, ConstraintModule, witness. tier_3bucket 의 spec 패턴
  재사용, tier/bucket 개념 제거.
- `zkpor/core/solvency/spot_simple/circuit/*` — BatchCreateUserCircuit
  (spot 버전). math 가 가장 단순: sum equality + Merkle account tree
  + (debt = 0 강제 또는 자동 만족). tier haircut / multi-bucket 없음.
- spot_simple model 의 trusted setup ceremony 절차 정의 (R3 step 3 의
  G1 byte-equivalence 절차를 재사용 — legacy reference 가 없으니 G1
  본체는 N/A, 대신 R1CS hash freeze 만).
- `core/circuit/` substrate 가 두 model 모두 지원하는지 확인 — 만약
  spot_simple 이 새 universal helper 를 필요로 하면 그건 R6 (G11 rule-of-
  three) 의 첫 candidate.

부수 작업:

- 만약 `core/constraint_modules/noop/` promotion 이 R3 step 4 의 부수
  작업으로 처리되지 않았으면, R4 안에서 처리 (spot_simple 도 같은 noop
  module 을 쓰므로 promotion 의 가치가 R4 에서 명확해짐).

Exit criteria:

- spot_simple 회로 audit 완료 (또는 첫 audit-ready 상태).
- spot_simple Setup smoke (Compile + Setup) 통과 + R1CS hash 기록.
- substrate (`core/circuit/`) 가 tier_3bucket 과 spot_simple 두 model 모두
  지원함 확인. 차이가 있는 부분은 R6 promotion 후보로 기록.

### Stage R5 — Second customer profile (SEA reference)

목표: spot_simple model 위에 첫 SEA customer profile 을 구현해서 model →
customer flow 를 end-to-end 로 검증한다. 후보: Indonesia (Indodax /
Tokocrypto / Pintu 류) 또는 Thailand (Bitkub 류) — 둘 다 채택 incentive
"상" 으로 평가됨.

산출물:

- 새 고객사 어댑터 8개 (catalog, pricing, identity, insolvent,
  batch_shape, risk, snapshot, constraint_noop or custom). spot_simple
  의 spec 인터페이스를 구현.
- 해당 고객사가 채택할 model 결정 — SEA 의 경우 거의 자동으로 spot_simple
  (Q3 답).
- 두 고객사 (Binance R3 + SEA R5) 가 다른 model 을 쓰는 경우의 `.vk`
  공유/분리 정책 (G12 closed, Q7 답).
- **Declarative profile.toml 첫 추출** — 두 customer 가 등장하면 자연스
  러운 refactor trigger. asset list / batch shape / multipliers /
  identity scheme ID / insolvent policy / source-type ID 를 toml 로
  뽑는다. 어댑터 Go 코드는 여전히 존재 (procedural 부분). 단일-entry
  registry pattern 도입 — 형태만 잡힌다. `docs/02-module-architecture.md`
  §6 참조.

부수 작업 (이 stage 의 트리거에 따라):

- **Composite 패턴 첫 도입** — SEA customer 가 N module 을 요구하면
  `core/constraint_modules/composite/` 신설 + `ComposeModules([m1, ...])`
  헬퍼. `docs/02-module-architecture.md` §2 의 spec 그대로 impl. SEA 의
  첫 customer 가 noop 만 쓰면 R6 으로 carry.
- **Param-as-public-input 규칙 closure** — 첫 parameterized module
  등장 시점에 `docs/02-module-architecture.md` §4 의 규칙을 spec/witness
  builder 에 박는다. G3 의 ConstraintContext API freeze 와 동반.

Exit criteria:

- SEA 고객사 sample data 로 end-to-end PoR 통과 (witness → proof →
  verifier, spot_simple model 위에서).
- multi-customer 운영 시 `.vk` 공유/분리 정책 문서화.
- 두 customer 의 declarative 데이터가 동일 toml 스키마 위에서 표현됨
  (스키마 freeze 는 R7).

Blocking gates: G12.

### Stage R6 — Third model + core/circuit 보강 (rule-of-three trigger)

목표: 세 번째 model 구현 시점 (tier_3bucket + spot_simple + 새 model).
이때 처음으로 `zkpor/core/circuit/` 에 추가 헬퍼 승격 검토 (G11). 후보:
`merkle_classic` (mid-tier 마진 거래소), `over_collateral_simple` (단순
마진), 또는 시장 신호 기반의 다른 entry. 후보 헬퍼: RLC-based sum
equality helper, account leaf composition helper.

산출물:

- 세 번째 model 의 회로 구현.
- 세 model 모두에 공통으로 적용 가능한 패턴이 `zkpor/core/circuit/` 으로 승격됨.
- substrate API v1 잠정 정착.
- **`core/constraint_modules/` 의 첫 본격 entry 들 promotion** — R5 까지
  거치며 두 customer 이상에서 같은 module 패턴이 사용된 것이 있으면, G11
  의 자연 확장으로 module 도 core 라이브러리로 옮긴다. 후보 prefix:
  `regulator.<jurisdiction>.<rule>_v<v>`, `business.<pattern>_v<v>`.
  `docs/02-module-architecture.md` §5 참조.

Exit criteria:

- 세 model 모두 `zkpor/core/circuit/` 새 헬퍼 호출 형태로 정리.
- G11 closed.
- `core/constraint_modules/` 에 noop 외 최소 1 entry 등록 (rule-of-three
  통과한 module 한정).

Blocking gates: G11.

### Stage R7 — v1 catalog freeze

목표: 5-tier 카탈로그 모두 (또는 우선 정해진 subset) 구현 완료. v1 카탈로그
stable 선언. 추가 model은 v2 카탈로그로 미룬다.

산출물:

- 5개 model 모두 회로·spec·trusted setup 완료 (혹은 일부 deprecated 처리).
- `zkpor/core/spec/solvency_models.go` 가 v1 카탈로그로 freeze.
- LegacyKeyName deprecate 일정 결정 (G10 closed).
- **Profile descriptor schema v1 freeze** — R4 부터 emerging 한
  profile.toml 의 필드·타입·버전 표기 확정. `docs/02-module-architecture.md`
  §8 의 "lock 되지 않은 것" 중 schema 부분 closure.
- **Module 카탈로그 freeze** — `core/constraint_modules/` 의 entry list
  + ID prefix 규약 v1 확정. 신규 module 은 v2 로 미룸.

Exit criteria:

- 카탈로그 stability 선언 문서화.
- 신규 model 제안은 v2 정의서로 격상.
- profile descriptor v1 schema 문서화 (`docs/02-module-architecture.md`
  업데이트 또는 신규 docs/03-...).
- module 카탈로그 v1 list 확정.

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
| **G1** trusted-setup byte-equivalence 검증 방법 + 실행 | closed | R3 step 3 | **(a) R1CS L·R==O matrix SHA256 채택** (commit 1398e04). `bn254.R1CS.GetR1Cs()` 로 L/R/O 추출 후 직렬화 SHA256. Tiny shape (5, 50, 2) 에서 legacy + zkpor 모두 `678eb23f62a9932bb93a8f0811db3b64a4bfd8eadb5e743791d93b27c0b95b32`. (b) `.pk` SHA256 은 `groth16.Setup` 의 toxic-waste randomness 로 deterministic 하지 않음 — production ceremony 의 waste 가 파기되어 재사용 불가, 기각. Hint identifier divergence (legacy `circuit.IntegerDivision` vs zkpor `corecircuit.IntegerDivision` 의 reflect-derived ID) 는 solver-side metadata 라 .pk/.vk 에 무관 — 각 service 가 R3 step 4 에서 zkpor 의 IntegerDivision 을 `solver.RegisterHint` 로 등록. Sample-corpus AccountID byte-equivalence 도 동시 검증 (90 valid + 10 invalid 분류 까지 parity). | 후속 production-shape 검증은 ROADMAP R3 step 3 산출물 박스의 절차 참조 (optional). |
| **G2** AccountIDProvider scheme v1 freeze | closed | R3 step 4 | **`passthrough_hex_bn254_reduced.v0`** 채택. binance/identity.go 의 `DeriveAccountID` 가 hex-decode 후 BN254 fr.Element SetBytes→Marshal 적용 — snapshot 의 G13 정규화와 동일한 출력. 정직한 freeze 위해 함수 동작도 이름과 일치시킴 (과거 placeholder 는 hex passthrough 였고 절반의 입력에서 leaf hash 와 어긋났다). Customer-side derivation 가정 유지 — HMAC/salt 정식화는 V2 이후 별도 결정. | — |
| **G3** ConstraintModule 공개 API freeze | deferred | R3 후 | 현재 `ConstraintContext` 가 minimal surface. 두 번째 module 등장 시 확정. | 첫 비-noop module 등장 시 API surface 검토. |
| **G4** catalog stability 선언 | deferred | R7 | 5-tier 잠정 확정. 회로 구현은 1/5. | 모든 model 구현 후 freeze. |
| **G5** RiskPolicy 데이터 schema | deferred | R2 | 현재 `cex_assets_info.csv` 형식 (legacy). | CSV 유지 vs JSON/YAML schema 도입 결정. |
| **G6** ValueScale invariant assert 위치 | closed | R3 step 4 | **witness service startup assert** 채택 (commit 5332f40). `binance.NewPricing()` 의 default-symbol 경로에서 `PriceMultiplier × BalanceMultiplier == ValueScale` 위반 시 panic. witness 가 첫 PriceScaleProvider 소비자라 자연 call site. 두-자리-자산 경로 등 per-symbol split 은 `profile/binance` 자체 테스트 책임 (services 가 enumerate 하지 않음). | — |
| **G7** InvalidAccountPolicy 운영 정책 | closed | R0 | drop + log (legacy 동등). | customer 요구 시 별도 정책. 변경 시 customer review. |
| **G8** BatchShape v1 정착 (binance) | closed | R0 | `{50,700}` + `{500,92}` (Binance reference). | 다른 customer 시 별도 shape 정의. |
| **G9** module ID 명명 규약 | closed | R0 | `<exchange>.<rule>_v<version>` 형식. filename-safe (lowercase, digits, dots, underscores). | — |
| **G10** LegacyKeyName 폐기 일정 | deferred | R7 | 현재 호환 유지 (`BatchShape.LegacyKeyName()`). | catalog freeze 후 한 release에서 deprecate. |
| **G11** core/circuit 추가 헬퍼 승격 규약 | deferred | R6 | rule-of-three — 3번째 model 등장 시 검토. | 세 번째 model 구현 시점. |
| **G12** multi-customer profile 충돌 정책 | deferred | R5 | profile/<customer>/ 단일 패키지 가정. shape/.vk 공유 정책 미정. R4 model-first swap 이후 customer 는 R5 에 등장. | 두 번째 customer (SEA reference) 등장 시. |
| **G13** AccountID fr.Element 정규화 위치 | closed | R3 step 1 | **(a) snapshot 어댑터** 채택. legacy `src/utils/utils.go:553` 와 동일 layer 에서 `new(fr.Element).SetBytes(id).Marshal()` round-trip. 근거: G1 byte-equivalence 비용 최저 (snapshot 출력 hex 직접 비교 가능), `AccountInfo.AccountID == userproof.AccountID == field input` 단일 형태 유지, R3 step 4 service rewire 시 호출 누락 위험 없음. 트레이드오프: `profile/binance/snapshot.go` 가 bn254 에 직접 결합 — 현재 카탈로그 5 model 전부 bn254 라 실질 충돌 없음, 두 번째 customer profile (R4) 등장 시 R6 helper 승격 후보로 carry. (b)/(c) 는 layering 더 깔끔하나 user-facing inconsistency / interface 확장 / 회귀 위험으로 기각. | impl: R3 step 2 (alpha wiring 과 동반). `AccountIDProvider.Scheme()` 명칭 갱신은 R3 step 4 (G2 closure) 동반. |
| **G14** 사용자-facing verification 분배 책임 | deferred | post-V1 / customer SLA | V1 engine 은 CLI + file artifact + userproof DB 행만 출하. 사용자가 자기 inclusion 을 확인하는 UI / 페이지는 engine 밖 (`## Scope Boundary` 참조). 후보 owner: (a) customer 가 자체 UI 구축, (b) partner / SI 가 reference UI 제공, (c) zkpor 가 reference open-source CLI/static page 부속 제공. | 첫 customer 통합 (R5 진입, model-first swap 이후) 시 SLA 협상 항목으로 surface. V1 안에서는 결정 보류. |
| **G15** Prove-path GPU 가속 backend 채택 여부 | deferred | post-R3 step 4 / first production prove SLA | gnark README 가 ICICLE backend (Ingonyama) 통한 GPU 가속을 **공식 지원** — BN254 + Groth16 호환, 라벨 "Experimental". `.pk`/`.vk` byte-equivalence (G1) 와는 **직교** (accelerator 가 같은 ceremony 출력 사용 — R1CS/`.pk`/`.vk` 모두 그대로). 채택 시 audit 추가 surface = ICICLE backend 자체 (수학적 동치이지만 trust boundary 증가). 결정은 첫 production deployment 의 CPU prove 시간 측정 → 24h snapshot SLA 와 비교 후. pre-결정 작업: ICICLE 공식 docs 에서 (a) PoR-scale R1CS 의 speedup 벤치마크, (b) build/CUDA toolkit 요건, (c) GPU 없는 환경에서의 fallback 동작 확인. | 첫 production prove SLA 측정 시점에 surface. binding 하면 채택 검토 → closed, 그렇지 않으면 CPU 만 사용. |
| **G16** Module composition compatibility 검토 프로세스 | deferred | first multi-module composition (R5 candidate) | `docs/02-module-architecture.md` §1 의 add-only 원칙으로 composition 자체는 수학적으로 안전. 그러나 module 간 hidden assumption 충돌 (한 module 이 system 의 변수 의미를 전제, 다른 module 이 그걸 깸 → unsat) 가능. 방향 lock: (a) 각 module 의 doc/audit note 에 assumed invariants 명시 의무, (b) composition 등록 (= 새 `.vk` ceremony 시작) 전에 reviewer 가 invariant 호환성 검토, (c) 자동화는 future work. process detail (reviewer who, document where, fail-mode) 는 첫 multi-module 등장 시 채움. | 첫 multi-module composition customer 등장 시 process detail 확정 + 이 row 의 status `deferred → closed`. |

## Gate → Stage Dependency

어떤 게이트가 어떤 stage를 막는지.

```text
G1  --> closed at R3 step 3 (trusted-setup byte-equivalence — commit 1398e04)
G2  --> R3 step 4 (identity scheme freeze)
G3  --> R3+ (first non-noop module 등장 시)
G4  --> R7 (catalog freeze)
G5  --> R2 (RiskPolicy schema)
G6  --> R3 step 4 (ValueScale assert)
G10 --> R7 (LegacyKeyName deprecate)
G11 --> R6 (core/circuit promotion)
G12 --> R5 (multi-customer .vk policy — moved from R4 by model-first swap)
G13 --> R3 step 1 (AccountID fr.Element normalization)
G14 --> post-V1 / customer SLA (user-facing verification distribution)
G15 --> post-R3 step 4 / first production prove SLA (GPU acceleration backend)
G16 --> R5 candidate (module composition compatibility process)

(G7, G8, G9 는 R0 시점에 closed)
```

## Parallel Workstreams

병렬 진행 가능한 작업 줄기와 의존.

```text
Foundation                      Catalog Path                 Customer Maturity
───────────                     ─────────────                ─────────────────
R1 (circuit port)
  │
  v
R2 (CSV absorb)
  │
  v
R3 (service rewire) ──> R4 (spot_simple model) ──> R5 (SEA customer profile)
                                  │                          │
                                  v                          v
                                R6 (third model + promotion, rule-of-three)
                                  │
                                  v
                                R7 (catalog freeze)
```

Foundation (R1+R2+R3) 은 직렬. **R4 = spot_simple 회로 (SEA GTM driver,
model-first), R5 = 첫 SEA customer profile (spot_simple 위)** — R4/R5
swap 은 SEA 시장 조사 결과 (`docs/01-project-context.md` SEA zoom-in)
근거. R6 은 세 번째 model 도달 시점 (rule-of-three), R7 = freeze.
