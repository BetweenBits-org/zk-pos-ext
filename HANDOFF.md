# HANDOFF.md

이 문서는 agent 세션이 바뀌어도 작업을 이어가기 위한 **현재 시점의 인수인계**다.
긴 히스토리 로그가 아니다. 작업 시작 전 source priority 상위 문서를 먼저 읽는다.

## Current State

Latest implementation commit (`zkpor/.git/`, branch `main`):

```text
1398e04 test(zkpor): legacy↔zkpor R1CS + AccountID byte-equivalence (R3 step 3 / G1)
ccc3fe4 feat(zkpor): alpha wiring + AccountID fr.Element @ snapshot (R3 step 2)
1a0d820 test(circuit): add tier_3bucket Compile+Setup smoke (R3 step 0)
cd50632 test(profile): full AccountStream fixture coverage
a26a7d2 feat(profile): classify+skip invalid account rows
d27cde7 feat(profile): stream user-shard rows into AccountStream
ae623f4 test(profile): cover CexAssets() happy + tamper fixtures
8813999 feat(profile): absorb cex_assets_info.csv loader into binance snapshot
8aaf4c3 feat: scaffold zkpor engine — productization of Binance OSS PoR v2
```

`zkpor/` 는 자체 git 저장소 (`zkpor/.git/`). parent (`zkmerkle-proof-of-solvency`)
저장소는 `zkpor/` 를 untracked dir로 봄 — 두 저장소가 독립 운영된다.

| 영역 | 상태 |
|---|---|
| `zkpor/core/spec/*` | ✅ complete — 8 인터페이스/상수 파일 |
| `zkpor/core/circuit/*` | ✅ complete — universal 헬퍼 4 파일 (Merkle, commitment, arith) |
| `zkpor/core/solvency/tier_3bucket/spec/*` | ✅ complete — types, RiskPolicy, SnapshotSource (`InvalidCount()` 추가됨, R2/2 step 2), ConstraintModule, witness (BatchCreateUserWitness 등) |
| `zkpor/core/solvency/tier_3bucket/circuit/*` | ✅ complete — BatchCreateUserCircuit + helpers ported. `SetBatchCreateUserCircuitWitness` 는 `assetCountTiers` 를 인자로 받음 (global 의존 제거). **Alpha wiring 적용 (R3 step 2)** — unexported `module` 필드 + `SetConstraintModule` setter, Define 끝에서 `ConstraintModule.Define(api, ctx)` 호출. noopModule 일 때 NbConstraints == 723790 (R3 step 0 baseline 과 동일). **R1CS byte-equivalence vs legacy 통과 (R3 step 3 / G1, tiny shape, commit 1398e04)** — `bn254.R1CS.GetR1Cs()` L·R==O SHA256 일치 (`678eb23f…`). |
| `zkpor/core/solvency/{spot_simple,merkle_classic,over_collateral_simple,tier_1bucket}/` | ⏸ doc.go only — 카탈로그 reserved, rule-of-three 대기 |
| `zkpor/profile/binance/*` | ✅ snapshot ETL 흡수 완료 — `CexAssets()` + `AccountStream()` happy + invalid 분류 + full-coverage 테스트 (multi-shard / ctx cancel / numeric overflow / collateral sum overflow / fatal column count) + AccountID 정규화. **`parseAccountRow` 에서 bn254 `fr.Element` SetBytes→Marshal round-trip 적용 (R3 step 2, G13 impl)** — legacy `src/utils/utils.go:553` 와 동일 layer. **R3 step 3 sample-corpus parity 통과** — legacy `ReadUserDataFromCsvFile` 과 zkpor `csvSnapshot.AccountStream` 이 sample_users0.csv (100 rows) 에서 90 valid AccountID byte-equal + 10 invalid 분류 parity. 17개 테스트 통과 (binance). multi-shard *concurrency* 는 여전히 R3 step 4 (현재는 sequential). 나머지 7개 어댑터는 constructor 형태 |
| `circuit/`, `src/` (legacy) | ✅ untouched, fully functional. trusted setup 그대로 유효 |
| docs (`zkpor/AGENTS.md`, `zkpor/CLAUDE.md`, `zkpor/PRODUCTION_ROADMAP.md`, `zkpor/docs/01-project-context.md`, `zkpor/docs/02-module-architecture.md`) | ✅ complete |

## Current Implementation Snapshot

최근 작업 흐름:

```text
<R3/3>   test(zkpor): legacy↔zkpor R1CS + AccountID byte-equivalence
        (G1 closure. tier_3bucket/circuit/legacy_compare_test.go —
         tiny shape (5, 50, 2) 에서 legacy + zkpor R1CS L·R==O 행렬을
         `bn254.R1CS.GetR1Cs()` 로 추출, SHA256 동일 (678eb23f…).
         Coefficient table SHA256 도 동일. Hint identifier 차이는
         solver metadata 라 .pk/.vk 에 무관 — 의도적 제외. profile/
         binance/legacy_compare_test.go — sample_users0.csv 100 rows
         에서 legacy `ReadUserDataFromCsvFile` 과 zkpor
         `csvSnapshot.AccountStream` 이 90 valid + 10 invalid 분류
         까지 byte-parity. 두 테스트 모두 -short 에서 skip.)
<R3/2>   feat(zkpor): alpha wiring + AccountID fr.Element @ snapshot
        (BatchCreateUserCircuit 에 unexported `module` 필드 +
         SetConstraintModule setter 추가. Define() 가 base 제약 emit 후
         module.Define(api, ctx) 호출 — per-user 총액 (Equity/Debt/
         CollateralReal) 을 user 루프 안에서 capture 해 ConstraintContext
         로 전달. toCircuitCexAssetView/toCircuitTierRatioView flat-copy
         helper 2개 추가. profile/binance/snapshot.go::parseAccountRow
         에 `new(fr.Element).SetBytes(id).Marshal()` 1줄 — legacy 와
         같은 layer 에서 bn254 정규화. setup smoke 가 nil-module +
         noop-module 두 경로에서 NbConstraints=723790 동일함을 assert.
         AccountStream multi-shard 테스트 (0xaa…aa / 0xbb…bb) 는 같은
         round-trip 으로 expected 계산. parseAccountRow 단위 테스트
         1건 추가 (all-FF 입력 → reduced output).)
<R3/0>   test(circuit): add tier_3bucket Compile+Setup smoke
        (NewBatchCreateUserCircuit(5,50,2) → frontend.Compile +
         groth16.Setup. tiny shape — IR-defect smoke 한 건. 8s compile
         + 62s setup, NbConstraints=723790 (informational only). 정확
         NbConstraints + byte-equivalence vs legacy 는 R3 step 3 (G1).
         production 코드 변경 0.)
<R2/2/3> test(profile): full AccountStream fixture coverage
        (5건 신규 — MultiShardSequential, CtxCancelCloses,
         InvalidNumericOverflow, InvalidCollateralSumOverflow,
         FatalColumnCount. testdata/multi_shard/{cex_assets_info,
         a, b}.csv 신설. goroutine leak guard 는 deferred —
         goleak dep 도입 부담 회피.)
<R2/2/2> feat(profile): classify+skip invalid account rows
        (spec 확장 — SnapshotSource.InvalidCount() 추가, atomic-safe.
         csvSnapshot — errInvalidRow sentinel + invalidf 헬퍼로 데이터
         에러를 invalid 로 분류. per-asset (collateral>equity, overflow)
         + account-level (TotalCollateral<TotalDebt) 불변식 추가.
         streamShard 가 invalid 행은 log + counter + skip, fatal 만 close.
         3개 invalid 테스트 추가 — 총 10건.)
<R2/2/1> feat(profile): stream user-shard rows into AccountStream
        (errAccountStreamPending → 실제 구현. listUserShards 추출 →
         readUserAssetOrder + AccountStream 공용. 헬퍼:
         streamAccounts/streamShard/parseAccountRow/assetCollateralValue/
         haircutValue. CalculateAssetValueForCollateral +
         CalculateAssetValueViaTiersRatio legacy 포팅 — byte-equivalence.
         smoke test 1건 추가 (2 rows × 3 assets).)
<R2/1f> test(profile): cover CexAssets() happy + tamper fixtures
        (single _test.go — happy + TwoDigitMultiplier + MissingSymbol
         + MalformedHeader + NonMonotonicBoundary + BoundaryOverflow.
         testdata/happy/ 베이스 + t.TempDir overlay 헬퍼.)
<R2/1>  feat(profile): absorb cex_assets_info.csv loader into binance snapshot
        (CexAssets() 구현 — user CSV header → asset 순서, cex_assets_info.csv
         → BasePrice + 3 buckets × TierCount, reserved 슬롯으로 AssetCounts
         까지 패딩. AccountStream() stub 유지.)
8aaf4c3 feat: scaffold zkpor engine — productization of Binance OSS PoR v2
        (root-commit: methodology docs, core/spec, core/circuit,
         core/solvency catalog with tier_3bucket spec+circuit ported,
         profile/binance adapter set)
```

구현된 것:

- 모든 universal 인터페이스 (`zkpor/core/spec/`).
- universal zk 헬퍼 (`zkpor/core/circuit/`) — legacy `circuit/utils.go` 에서
  Merkle/commitment/arithmetic 부분만 추출.
- tier_3bucket model spec (`zkpor/core/solvency/tier_3bucket/spec/`).
- Binance 어댑터 8개 (constructor 형태) — 단일 Go 패키지.
- `binance.csvSnapshot.CexAssets()` — legacy `ParseAssetIndexFromUserFile` +
  `ParseCexAssetInfoFromFile` 흡수. sync.Once 캐싱, 두 자리 가격
  multiplier (`pricing.PriceMultiplier`) 재사용, tier boundary는
  `corespec.DefaultValueScale` (1e16) 로 스케일, TierCount/AssetCounts
  패딩 모두 적용.
- 5-tier 카탈로그 (`zkpor/core/spec/solvency_models.go`).
- 명명 규약: SolvencyModelID, BatchShape, key file naming, ConstraintModuleID.

아직 의도적으로 닫지 않은 것:

- multi-shard / multi-worker concurrency — R3 step 4 production wiring.
  현재 streamShard 는 sequential. legacy 는 goroutine 풀 + GC trigger.
- 4개 service main.go 의 wiring — R3 step 4.
- `solver.RegisterHint(corecircuit.IntegerDivision)` — 각 service
  의 main 에 들어가야 zkpor circuit 으로 witness solving 이 동작.
  G1 closure 의 의도적 제외 항목 (hint identifier divergence) 가 R3
  step 4 에서 service-side wiring 으로 해소된다.
- `AccountIDProvider.Scheme()` rename (정규화 사실 노출) — R3 step 4
  (G2 closure 동반).
- ValueScale startup assert — R3 step 4 (G6).
- goroutine leak guard 테스트 — uber-go/goleak dep 도입 시점에.
- 나머지 4개 model 회로 — R4+ (시장 신호 대기).
- 사용자-facing verification UI / 페이지 — engine boundary 밖, V1 scope
  미포함. customer / partner 영역. PRODUCTION_ROADMAP `## Scope
  Boundary` + G14 참조.

발견 사항 (작업 중 surface된 것, 의사결정 보류 / 일부 closure):

- **G1 closed (R3 step 3) — R1CS L·R==O 행렬의 SHA256 비교**.
  `bn254.R1CS.GetR1Cs()` 로 추출한 L/R/O term stream 을 직렬화
  후 SHA256. 후보 (b) `.pk` SHA256 은 `groth16.Setup` 의
  `sampleToxicWaste()` 가 deterministic 하지 않아 기각. Tiny shape
  (5, 50, 2) 일치 + Define 이 shape-invariant 라는 구조적 논증으로
  production shape 일치를 내포. Sample-corpus AccountID byte-parity
  (90 valid + 10 invalid 분류) 가 snapshot-layer 보조 증거. 의도적
  제외: hint identifier (legacy `circuit.IntegerDivision` 과 zkpor
  `corecircuit.IntegerDivision` 의 reflect-derived ID 가 다름 — 단,
  solver-only metadata 이며 .pk/.vk 매트릭스에 무관, R3 step 4 service
  wiring 에서 `solver.RegisterHint` 로 등록). gnark debug metadata
  (SymbolTable, DebugInfo, MDebug, Logs) 도 source path / line number
  를 담아 byte 비교를 잘못 깨뜨리므로 제외.


- `parseTierRatios` 의 `MaxTierBoundary` 체크는 CSV 입력 경로에서
  도달 불가능 — `uint64.Max · 1e16 ≈ 1.84e35` 가
  `maxTierBoundary ≈ 3.32e35` 아래에 있어 uint64 변환이 항상 먼저
  실패한다. 코드는 defense-in-depth 로 보존 (`convertFloatStrToUint64`
  가 넓은 정수로 바뀌면 다시 살아남). R2/1f 테스트는 실제 도달 가능한
  uint64 overflow 경로를 검증.
- **G13 closed + impl in (R3 step 2) — AccountID bn254 fr.Element
  정규화는 snapshot 어댑터에서 (a)**. legacy `src/utils/utils.go:553`
  와 동일 layer 에 `new(fr.Element).SetBytes(id).Marshal()` round-trip
  배치 (ccc3fe4). 근거: G1 byte-equivalence 비용 최저 (snapshot 출력
  hex 직접 비교), user-facing `AccountInfo.AccountID == userproof.
  AccountID == field input` 단일 형태, R3 step 4 service rewire 시
  호출 누락 위험 없음. 트레이드오프: `profile/binance/snapshot.go` 가
  bn254 에 직접 결합 — 5-tier catalog 모두 bn254 라 실질 충돌 없음,
  두 번째 customer profile 등장 시 R6 helper 승격 후보 (G11). (b)/(c)
  는 layering 더 깔끔하나 user-facing inconsistency / interface 확장 /
  R3 step 4 회귀 위험으로 기각. **Scheme rename 은 step 4 (G2)**.
  PRODUCTION_ROADMAP G13 참조.
- **Alpha wiring 적용 (R3 step 2)** — `BatchCreateUserCircuit` 에
  unexported `module tier3spec.ConstraintModule` 필드 + 동명의
  pointer-receiver setter. Define() 끝 (모든 base 제약 emit 후) 에서
  `module.Define(api, ctx)` 호출. gnark frontend 가 exported
  Variable-bearing 필드만 reflect 하므로 unexported `module` 은
  Compile 에 비가시이며 in-circuit 비용 0. nil-module (R3 step 0
  baseline) 과 noop-module 두 경로에서 NbConstraints == 723790 동일함을
  setup smoke 가 assert. trusted setup 분기 정책 (별도 module → 별도
  .pk/.vk pair) 은 spec docstring 에 이미 명시됨.

## Non-Negotiable Rules

작업 내내 어기면 안 되는 규칙.

- **frozen 계약 경계 우선** — `zkpor/core/spec/` 인터페이스 시그니처, 카탈로그
  상수, key file naming 은 versioned change 만.
- **legacy 코드 직접 수정 금지** — `circuit/`, `src/` 는 reference.
- **sum equality 는 모든 model에서 mandatory** — base PoR claim.
- **ConstraintModule 은 add only** — base-circuit 제약 weaken/remove 금지.
- **`PriceMultiplier × BalanceMultiplier == ValueScale` 불변식** — startup
  assert (R3 G6 closed 시점).
- **미결정·spec 공백·계약 불일치는 debate/question으로 surface** — agent
  임의 결정 금지.
- **검증 명령 실제 실행 없이 완료 선언 금지** — go build / go vet 통과 필수.
- **거래소 이름을 model id에 박지 않는다** — `tier_3bucket` ≠ `binance_v2`.

## Source Priority

문서 충돌 시 우선순위. 자세한 내용은 `zkpor/PRODUCTION_ROADMAP.md` 참조.

1. `zkpor/core/spec/` 코드 (frozen 계약)
2. `zkpor/docs/01-project-context.md`
3. `zkpor/docs/02-module-architecture.md`
4. `zkpor/PRODUCTION_ROADMAP.md`
5. `zkpor/AGENTS.md`, `zkpor/CLAUDE.md`
6. `zkpor/HANDOFF.md` (이 문서 — 휘발성)
7. `docs/*.md` (legacy 참고 자료)

## Repository Map

세션 cwd는 project root (`zkmerkle-proof-of-solvency/`).

```text
zkmerkle-proof-of-solvency/                   (cwd — parent repo)
├── circuit/                                  (legacy Binance OSS PoR v2 — 수정 금지)
├── src/                                      (legacy Binance OSS PoR v2 — 수정 금지)
├── docs/                                     (legacy historical notes)
└── zkpor/                                    (★ 신규 엔진 — 자체 git 저장소)
    ├── AGENTS.md                             ← agent contract (가장 먼저 읽음)
    ├── CLAUDE.md                             ← Claude 자동 로드 메모리 (AGENTS.md redirect)
    ├── HANDOFF.md                            ← 이 문서 (현재 시점 인수인계)
    ├── PRODUCTION_ROADMAP.md                 ← Part 3 (stages + gates)
    ├── docs/
    │   ├── 01-project-context.md             ← Part 1 (컨셉/scope/guarantee)
    │   └── 02-module-architecture.md         ← Module + Profile architecture lock
    ├── core/
    │   ├── spec/                             universal 인터페이스 + 상수 + 카탈로그
    │   │   ├── batch_shape.go
    │   │   ├── catalog.go                    (AssetCatalog interface)
    │   │   ├── constants.go
    │   │   ├── constraint_id.go
    │   │   ├── identity.go
    │   │   ├── insolvent.go
    │   │   ├── price.go
    │   │   └── solvency_models.go            (5-tier 카탈로그)
    │   ├── circuit/                          universal zk 헬퍼
    │   │   ├── arithmetic.go
    │   │   ├── commitment.go
    │   │   ├── constants.go
    │   │   └── merkle.go
    │   └── solvency/                         audited math 카탈로그
    │       ├── spot_simple/doc.go
    │       ├── merkle_classic/doc.go
    │       ├── over_collateral_simple/doc.go
    │       ├── tier_1bucket/doc.go
    │       └── tier_3bucket/                 (★ 유일 spec+circuit 구현)
    │           ├── doc.go
    │           ├── spec/                     (types, risk, snapshot, constraint, witness)
    │           └── circuit/                  (BatchCreateUserCircuit + helpers)
    └── profile/
        └── binance/                          (★ 유일 customer profile)
            ├── doc.go
            ├── batch_shape.go
            ├── catalog.go
            ├── constraint_noop.go
            ├── identity.go
            ├── insolvent.go
            ├── pricing.go
            ├── risk.go
            ├── snapshot.go                   (CexAssets + AccountStream + invalid 분류 done)
            ├── snapshot_test.go              (CexAssets 6 + AccountStream happy 1 + invalid 3 + coverage 5 + parseAccountRow normalization 1 — 16 total)
            ├── legacy_compare_test.go        (R3 step 3 / G1 — sample-corpus AccountID byte-equivalence vs legacy ETL; -short skip)
            └── testdata/
                ├── happy/                    (cex_assets_info.csv + user_shard.csv 헤더 + 2 rows)
                └── multi_shard/              (cex_assets_info.csv + a.csv + b.csv)
```

## Deferred Work

| Item | 상태 | track 위치 |
|---|---|---|
| CSV ETL absorb — CexAssets 부분 | ✅ done | R2 / G5 (step 1) |
| `CexAssets()` 픽스처 테스트 (happy + tamper) | ✅ done | R2 / G5 (step 1 follow-up) |
| CSV ETL absorb — AccountStream happy-path | ✅ done | R2 / G5 (step 2 / sub 1) |
| invalid-account 분류 (skip+log+counter) | ✅ done | R2 / G5 (step 2 / sub 2) |
| `AccountStream` 픽스처 테스트 (full coverage) | ✅ done | R2 / G5 (step 2 / sub 3) |
| Setup smoke test (Compile + Setup) | ✅ done (tiny shape) | R3 step 0 |
| AccountID fr.Element 정규화 위치 결정 (G13) | ✅ closed — (a) snapshot 어댑터 | R3 step 1 |
| Constraint Architecture alpha wiring + fr.Element impl | ✅ done — `module` 필드 + setter, snapshot round-trip, noop-baseline regression guard | R3 step 2 |
| G1 byte-equivalence 절차 합의 + 실행 | ✅ closed — (a) R1CS L·R==O SHA256 채택, tiny shape match + sample-corpus AccountID parity (commit 1398e04) | R3 step 3 |
| 4개 service rewiring + ValueScale assert + Scheme freeze | pending | R3 step 4 / G2 + G6 (agent 가 4 서비스 별 commit 으로 자율 분해) |
| AccountIDProvider derivation 정식화 | deferred | R3 / G2 |
| 두 번째 customer profile | awaits signal | R4 / G12 |
| 두 번째 model 회로 구현 | awaits signal | R5 |
| core/circuit/ 추가 헬퍼 승격 | awaits signal | R6 / G11 |
| 카탈로그 v1 freeze | awaits R7 | R7 / G4 |
| 사용자-facing verification 분배 책임 (UI / 페이지) | deferred | post-V1 / customer SLA / G14 |
| Module composition compatibility 검토 프로세스 (G16) | deferred | R5 candidate / 첫 multi-module composition |
| `core/constraint_modules/noop/` promotion (universal noop 분리) | pending | R3 step 4 직후 또는 R4 진입 시 |
| Composite 패턴 (`ComposeModules` 헬퍼) 도입 | pending | R5 candidate / 첫 N≥2 module deployment |
| Param-as-public-input 규칙 closure | pending | R5 candidate / 첫 parameterized module |
| Declarative `profile.toml` 첫 추출 | pending | R4 / 두 번째 customer 도입 |
| profile descriptor schema v1 freeze | pending | R7 / G4 |
| module 카탈로그 v1 list freeze | pending | R7 / G4 |

## Resume Actions

다음 agent는 아래 순서로 이어간다.

1. `zkpor/AGENTS.md`, `zkpor/docs/01-project-context.md`,
   `zkpor/PRODUCTION_ROADMAP.md` 읽기.
2. `git -C zkpor status` + `git -C zkpor log --oneline -10` 으로
   zkpor 저장소 상태 확인.
3. baseline 검증 명령 실행 (Required Commands 참고).
4. 다음 슬라이스 진입.

**R3 step 3 closed (G1)**. Tiny-shape R1CS L·R==O byte-equivalence
(SHA256 `678eb23f…`) + sample-corpus AccountID 90/90 + invalid 10/10
parity. 두 영구 회귀 테스트 (`-short` 에서 skip) 가 회로/snapshot
양쪽의 legacy 정합성을 지속 가드. 다음은 R3 본체 — **4개 service
main.go 를 zkpor 어댑터로 재배선** + G2 (Scheme rename) + G6
(ValueScale assert) closure.

권장 다음 슬라이스 — **R3 step 4 (service rewiring, agent 가 자율
분해)**:

```text
관여 서비스 (PRODUCTION_ROADMAP R3 step 4 참조):
  - src/witness    : snapshot → AccountInfo stream → BatchCreateUserWitness
  - src/prover     : witness file → groth16.Prove → proof file
  - src/userproof  : per-user inclusion proof → DB 행
  - src/verifier   : groth16.Verify → exit code

같은 commit 에 묶을 후보 (low-coupling 변경):
  - 4 서비스 main.go 의 legacy `src/utils` import 제거,
    `zkpor/profile/binance` + `zkpor/core/solvency/tier_3bucket/...`
    import 으로 교체.
  - `solver.RegisterHint(corecircuit.IntegerDivision)` 등록 — G1
    의 의도적 제외 항목 (hint identifier divergence) 의 service-side
    해소.
  - ValueScale startup assert (G6 closure) — service init 에 한 줄.
    `PriceMultiplier × BalanceMultiplier == ValueScale` 불변식.

분해 후보 (의존도/결합도가 코드를 만져봐야 드러나는 영역, 한 commit
에 묶지 않는다):
  - witness → prover artifact 의존 (file format / serialization
    경계).
  - userproof 의 DB 스키마 — schema 변경이 필요한지, legacy 와
    공유 가능한지.
  - multi-shard concurrency (현재 sequential streamShard) — legacy
    의 goroutine pool + GC trigger 패턴을 zkpor 어댑터에 도입할지,
    R3 step 4 안에 묶을지 별 슬라이스로 분리할지.
  - goroutine leak guard 테스트 (uber-go/goleak 도입 가치 평가).

G2 closure (별도 commit 또는 service rewiring 동반):
  - `AccountIDProvider.Scheme()` 의 v1 이름 결정. 현재 placeholder
    `passthrough_hex.v0` 가 G13 의 fr.Element 정규화 사실을 반영하지
    못함. 후보: `passthrough_hex_bn254_reduced.v0` 또는 그
    derivative. 한 번 freeze 되면 service / artifact 명명에 박힘.

진입 시 agent 가 자기 슬라이스를 HANDOFF Resume Actions 에 박는다.
4 서비스를 한 commit 에 묶지 않는다 — service-별 commit 분해가
default. legacy + 신규의 결합도가 드러나는 순간 슬라이스 경계 재조정.
```

그 다음 진입:

```text
R3 step 4 closure 이후 — R3 본체 종료.
R4    — second customer profile (G12 closing).
R5    — second model 회로 구현 (rule-of-three first event).
```

목표 / 범위 제외:

- 다음 슬라이스 (R3 step 4): service rewiring + G2 + G6 closure.
  R3 step 4 는 한 commit 이 아니라 4-서비스 별 commit 권장.

## Required Commands

Start of work:

```bash
git -C zkpor status
git -C zkpor log --oneline -10
```

Baseline 검증 (슬라이스마다 — project root에서 실행):

```bash
go build ./zkpor/...
go vet ./zkpor/...
go build ./...              # legacy + 신규 — legacy 영향 없음 확인
```

회로 이식 후 (R1 진행 중부터):

```bash
# trusted setup byte-equivalence (G1 검증 절차 — R1 진입 전 결정 필요)
# 예: sha256sum legacy/zkpor50_700.pk new/zkpor.tier_3bucket.50_700.pk
```

End-to-end (R3 부터):

```bash
# sample data 기준 PoR 생성·검증 (구체 절차는 R3 진입 시 결정)
```

## Commit Discipline

- slice = commit. 작은 단위로.
- 순서: `docs/scaffold` → `implementation` → `tests`.
- 커밋 메시지 prefix:
  - `feat:` 새 기능 / 새 model / 새 customer
  - `refactor:` 동작 불변 구조 변경
  - `docs:` 문서만
  - `build:` build/CI 변경
  - `test:` 테스트 추가/수정
  - `chore:` 잡무

## Updating This File

아래 시점에 이 문서를 갱신한다.

- stage 진입/종료 시.
- decision gate 가 닫힐 때.
- deferred work 가 완료/재분류될 때.
- 다음 진입 action (Resume Actions) 이 바뀔 때.

이 문서를 긴 히스토리 로그로 쓰지 않는다. 과거 작업 흐름은 git log 가 source.
