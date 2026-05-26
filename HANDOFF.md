# HANDOFF.md

이 문서는 agent 세션이 바뀌어도 작업을 이어가기 위한 **현재 시점의 인수인계**다.
긴 히스토리 로그가 아니다. 작업 시작 전 source priority 상위 문서를 먼저 읽는다.

## Current State

Latest implementation commit (`zkpor/.git/`, branch `main`):

```text
3dae8da docs(strategy): realign R4/R5 to SEA spot-first + market context zoom-in
3c691cb feat(zkpor): close G2 — AccountIDProvider scheme v1 freeze
fdf4a63 feat(zkpor): zkpor/cmd/userproof — R3 step 4 core-path service
b7e57e6 feat(zkpor/store): user-proof model (userproof prep)
4e85757 refactor(zkpor): UserConfig → tier_3bucket/host shared type (userproof prep)
8045c37 feat(zkpor): zkpor/cmd/prover — R3 step 4 core-path service + G1 hint closure
16f36bd feat(zkpor/store): proof model + witness state-machine methods (prover prep)
5332f40 feat(zkpor): zkpor/cmd/witness — R3 step 4 core-path service + G6 closure
78acd39 feat(zkpor/store): batch witness model + MySQL connection helper
32b9334 feat(zkpor): tier_3bucket host — AccountLeafHash + PaddingAccounts + Encode/DecodeBatchWitness
c96018d feat(zkpor/core/tree): SMT account tree wrapper + empty-leaf hash
ea3244c docs(handoff): close verifier slice, frame witness as next R3 step 4
9f889ad feat(zkpor): zkpor/cmd/verifier — first R3 step 4 service
5f98fdd feat(zkpor): extract off-circuit host helpers (R3 step 4 prep)
8aaf4c3 feat: scaffold zkpor engine — productization of Binance OSS PoR v2
```

`zkpor/` 는 자체 git 저장소 (`zkpor/.git/`). parent (`zkmerkle-proof-of-solvency`)
저장소는 `zkpor/` 를 untracked dir로 봄 — 두 저장소가 독립 운영된다.

| 영역 | 상태 |
|---|---|
| `zkpor/core/spec/*` | ✅ complete — 8 인터페이스/상수 파일 |
| `zkpor/core/circuit/*` | ✅ complete — universal 헬퍼 4 파일 (Merkle, commitment, arith) |
| `zkpor/core/host/*` | ✅ off-circuit (native) universal 헬퍼 — `VerifyMerkleProof` (Poseidon BN254 SMT, legacy parity). R3 step 4 prep (commit 5f98fdd) |
| `zkpor/core/tree/*` | ✅ bsmt depth-28 SMT wrapper + `EmptyAccountLeafHash` (Poseidon(0,0,0,0,0)). memory/redis 백엔드. witness + userproof 공유 (commit c96018d) |
| `zkpor/core/solvency/tier_3bucket/host/*` | ✅ off-circuit model-specific 헬퍼 — `ComputeUserAssetsCommitment` + `ComputeCexAssetsCommitment` (5f98fdd) + `AccountLeafHash` + `PaddingAccounts` + `EncodeBatchWitness`/`DecodeBatchWitness` (32b9334) + 공유 `UserConfig` 타입 (4e85757). 모두 legacy byte-equivalence/round-trip 테스트 통과 |
| `zkpor/store/*` | ✅ gorm 영속화 계층 — `Open` + `ConvertMySQLErr` + 3 모델 (`BatchWitness` 78acd39, `Proof` + witness 상태머신 메서드 16f36bd, `UserProof` b7e57e6). 단일 instance DB-poll (`ClaimOldestByStatus` 트랜잭션) 채택 — Redis BLPOP 큐는 multi-worker scaling 시 follow-up |
| `zkpor/cmd/verifier/*` | ✅ R3 step 4 첫 service — legacy `src/verifier` 의 zkpor-native 대체 (3-mode CLI: batch / -user / -hash). src/utils + legacy circuit import 0. `UserConfig` 는 tier_3bucket/host 공유 타입 (4e85757). proof-table end-to-end 검증은 witness+prover artifact 의존 |
| `zkpor/cmd/witness/*` | ✅ R3 step 4 service — snapshot → BatchCreateUserWitness → DB (commit 5332f40). **G6 closure 동반** (`PriceMultiplier × BalanceMultiplier == ValueScale` startup assert). 핵심 경로 only — multi-worker 병렬 / DB resume / tree rollback 은 follow-up |
| `zkpor/cmd/prover/*` | ✅ R3 step 4 service — DB-poll Published → groth16.Prove+Verify → proof 테이블 (commit 8045c37). **G1 hint closure** (`solver.RegisterHint(corecircuit.IntegerDivision)`). idempotent persist + lazy snarkParams cache. Redis BLPOP 큐 / -rerun 모드는 follow-up |
| `zkpor/cmd/userproof/*` | ✅ R3 step 4 마지막 service — self-contained tree 재구축 (witness redis 의존 제거) → per-account inclusion proof → DB (commit fdf4a63). 동일 padding 으로 root parity. 핵심 경로 only — multi-worker 병렬 / -memory_tree 플래그 / resume 은 follow-up |
| `zkpor/core/solvency/tier_3bucket/spec/*` | ✅ complete — types, RiskPolicy, SnapshotSource (`InvalidCount()` 추가됨, R2/2 step 2), ConstraintModule, witness (BatchCreateUserWitness 등) |
| `zkpor/core/solvency/tier_3bucket/circuit/*` | ✅ complete — BatchCreateUserCircuit + helpers ported. `SetBatchCreateUserCircuitWitness` 는 `assetCountTiers` 를 인자로 받음 (global 의존 제거). **Alpha wiring 적용 (R3 step 2)** — unexported `module` 필드 + `SetConstraintModule` setter, Define 끝에서 `ConstraintModule.Define(api, ctx)` 호출. noopModule 일 때 NbConstraints == 723790 (R3 step 0 baseline 과 동일). **R1CS byte-equivalence vs legacy 통과 (R3 step 3 / G1, tiny shape, commit 1398e04)** — `bn254.R1CS.GetR1Cs()` L·R==O SHA256 일치 (`678eb23f…`). |
| `zkpor/core/solvency/{spot_simple,merkle_classic,over_collateral_simple,tier_1bucket}/` | ⏸ doc.go only — 카탈로그 reserved. **`spot_simple` 은 R4 model-first 우선순위 (SEA GTM driver, `docs/01-project-context.md` SEA zoom-in 참조)**. 나머지는 rule-of-three 대기. |
| `zkpor/profile/binance/*` | ✅ snapshot ETL 흡수 완료 — `CexAssets()` + `AccountStream()` happy + invalid 분류 + full-coverage 테스트. **`parseAccountRow` 에서 bn254 `fr.Element` SetBytes→Marshal round-trip (R3 step 2, G13 impl)**. **R3 step 3 sample-corpus parity 통과**. **G2 closure (commit 3c691cb)** — `identity.Scheme()` 가 `passthrough_hex_bn254_reduced.v0` 로 freeze 됨, `DeriveAccountID` 도 fr.Element 정규화 적용해 snapshot 출력과 byte-equal. `identity_test.go` 4건 (Scheme freeze + below-modulus passthrough + above-modulus reduces + 입력 가드 panic). multi-shard concurrency 는 여전히 sequential (witness 슬라이스 follow-up). 나머지 7개 어댑터는 constructor 형태 |
| `circuit/`, `src/` (legacy) | ✅ untouched, fully functional. trusted setup 그대로 유효 |
| docs (`zkpor/AGENTS.md`, `zkpor/CLAUDE.md`, `zkpor/PRODUCTION_ROADMAP.md`, `zkpor/docs/01-project-context.md`, `zkpor/docs/02-module-architecture.md`) | ✅ complete |

## Current Implementation Snapshot

최근 작업 흐름:

```text
<R3/4h>  feat(zkpor): close G2 — AccountIDProvider scheme v1 freeze
        (binance.identity.Scheme() = "passthrough_hex_bn254_reduced.v0".
         DeriveAccountID 가 hex-decode 후 BN254 fr.Element
         SetBytes→Marshal — snapshot G13 정규화와 동일 출력. 과거
         placeholder 는 hex passthrough 라 절반 입력에서 leaf hash
         와 어긋났음. identity_test.go 4건 신규: Scheme freeze +
         below/above modulus + 입력 가드 panic.)
<R3/4g>  feat(zkpor): zkpor/cmd/userproof — R3 step 4 마지막 service
        (self-contained tree 재구축 — witness redis 상태 의존 제거.
         동일 padding 으로 root parity. tree.Set 모든 leaf (real+
         padding) → tree.GetProof real-only → UserProof + 임베디드
         UserConfig JSON. dbBatchSize 100. main_test 2건 — tier 보호
         + buildUserProofRow Config round-trip via tier3host.UserConfig.)
<R3/4f>  feat(zkpor/store): user-proof model + refactor UserConfig
        (store.UserProof gorm 모델 + UserProofStore. UserConfig 가
         tier_3bucket/host 공유 타입으로 이동 — userproof writer +
         verifier -user reader 단일 소스, base64 hop 제거. verifier
         도 tier3host.UserConfig 직접 import.)
<R3/4e>  feat(zkpor): zkpor/cmd/prover — R3 step 4 service + G1 hint closure
        (DB-poll ClaimOldestByStatus Published→Received 트랜잭션 →
         DecodeBatchWitness → SetBatchCreateUserCircuitWitness →
         groth16.Prove + Verify → proof 테이블 + witness Finished.
         solver.RegisterHint(corecircuit.IntegerDivision) — G1 의
         의도적 제외 항목 service-side 해소. snarkParams lazy cache
         per tier. idempotent persist (GetByBatchNumber 사전 probe).
         Redis BLPOP 큐 / -rerun 모드는 multi-worker 시 follow-up.
         main_test 2건 — loadConfig round-trip + proof metadata JSON
         shape.)
<R3/4d>  feat(zkpor/store): proof model + witness 상태머신 메서드
        (store.Proof gorm 모델 + ProofStore (CreateTable / Create /
         GetByBatchNumber). witness 측 ClaimOldestByStatus (트랜잭션
         atomic flip) + LatestByStatus + MarkStatus 추가.)
<R3/4c>  feat(zkpor): zkpor/cmd/witness + 의존 — R3 step 4 service + G6 closure
        (3-commit 묶음: c96018d core/tree (bsmt depth-28 wrapper +
         EmptyAccountLeafHash) + 32b9334 tier3host (AccountLeafHash
         + PaddingAccounts + Encode/DecodeBatchWitness, 모두 legacy
         byte-equivalence) + 78acd39 store (BatchWitness gorm 모델 +
         MySQL error sentinel translation) + 5332f40 cmd/witness 본체.
         witness 가 첫 PriceScaleProvider 소비자라 G6 startup assert
         자연 call site. 핵심 경로: snapshot bucket → padding → tree
         populate → batch loop (commit per height). multi-worker
         병렬 / DB resume / tree rollback 은 follow-up.)
<R3/4b>  feat(zkpor): zkpor/cmd/verifier — first R3 step 4 service
        (legacy src/verifier 의 zkpor-native 대체. 3-mode CLI:
         batch (proof-table groth16 검증 + 체이닝 + 최종 CEX
         commitment) / -user (단일 계정 leaf 재계산 + Merkle path)
         / -hash (Poseidon of 2 base64). src/utils + legacy circuit
         import 0. cmd/verifier/config 가 tier_3bucket spec 타입.
         worker-pool small-input panic 가드. main_test.go —
         assetCountTiers {50,500} + decodeBatchMetadata. G2/G6 는
         call site 없어 witness/userproof 로 이연. proof-table
         end-to-end 는 witness+prover 후.)
<R3/4a>  feat(zkpor): extract off-circuit host helpers (R3 step 4 prep)
        (core/host/merkle.go — VerifyMerkleProof, universal,
         Poseidon BN254 SMT. tier_3bucket/host/commitment.go —
         ComputeUserAssetsCommitment + ComputeCexAssetsCommitment,
         trusted-setup byte packing. 4 테스트 legacy byte-equivalence
         통과. 발견: gnark-crypto bn254 poseidon Write 가
         ≥fr.Modulus() 입력을 silent drop — test fixture 가
         mod-safe 해야 함.)
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

아직 의도적으로 닫지 않은 것 (R3 step 4 follow-ups 와 stage 미진입 항목):

R3 step 4 follow-ups (본체 서비스 4개 + G1/G2/G6 closure 는 모두 done):

- **End-to-end sample-data smoke** — witness → prover → verifier(batch)
  + userproof → verifier(-user) 전체 파이프라인을 sample data 로
  통과시켜 R3 step 4 exit criteria 의 마지막 가드를 닫는다. 작은
  스크립트로 묶을 후보.
- **witness multi-worker 병렬 hashing** — 현재 sequential 단일 worker.
  legacy 는 cpuCores-2 worker + 채널 동기화로 account hash 와 main
  batch 루프 overlap.
- **witness DB resume** — `BatchWitness.Latest()` (이미 store 에 존재)
  를 읽어 currentBatchNumber 결정, 기존 batch 들 skip. 현재는 fresh
  start only.
- **witness tree rollback** — `accountTree.LatestVersion()` 이 DB
  height 보다 앞서면 rollback. resume 과 동반.
- **prover Redis BLPOP 큐 + multi-worker** — 현재 단일 instance DB-poll
  `ClaimOldestByStatus`. multi-worker scaling 시 Redis 도입 + 큐 producer
  (witness or dbtool) 결정.
- **prover -rerun 모드** — `LatestByStatus(StatusReceived)` 로 claim
  되었으나 finished 되지 못한 batch 회수. 현재는 within-process retry
  만 idempotency probe 로 안전.
- **userproof -memory_tree 플래그** — root-only 빠른 계산 ops 유틸.
- **userproof multi-worker 병렬** — 현재 sequential GetProof + write.
- **userproof resume** — `store.UserProofStore.Count()` (이미 추가됨)
  로 written rows 건너뛰기. 핵심 경로 외.
- **multi-shard concurrency** (snapshot adapter) — `binance.csvSnapshot.
  streamShard` 는 여전히 sequential. legacy 는 goroutine 풀 + GC
  trigger 패턴. R3 step 4 안에 묶지 않음.
- **goroutine leak guard 테스트** — uber-go/goleak dep 도입 시점에.

Stage 미진입:

- **R4 — second model 회로 `spot_simple`** (SEA GTM driver, model-first
  swap). docs/01-project-context.md SEA zoom-in 참조. customer signal
  안 기다림.
- **R5 — SEA reference customer profile** (Indonesia/Thailand 우선) +
  declarative `profile.toml` 첫 추출 (G12 closing).
- **R6 — third model + core/circuit 헬퍼 승격** (rule-of-three trigger,
  G11).
- **R7 — v1 catalog freeze**.

Engine boundary 외 (V1 scope 미포함):

- 사용자-facing verification UI / 페이지 — customer / partner 영역.
  PRODUCTION_ROADMAP `## Scope Boundary` + G14 참조.
- Prove-path GPU 가속 (ICICLE backend) — G15. 첫 production prove SLA
  측정 후 결정.

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

- **G6 closed (R3 step 4) — ValueScale invariant assert at witness
  startup** (commit 5332f40). `binance.NewPricing()` 의 default-symbol
  경로에서 `PriceMultiplier × BalanceMultiplier == ValueScale` 위반 시
  panic. witness 가 첫 PriceScaleProvider 소비자라 자연 call site.
  per-symbol split (두-자리-자산) enumeration 은 `profile/binance` 자체
  테스트 책임 — services 가 known-symbol list 를 들고 다니지 않음.
- **G2 closed (R3 step 4) — `passthrough_hex_bn254_reduced.v0`** (commit
  3c691cb). 이름 변경 + 함수 동작 정렬 동시에 — `DeriveAccountID` 가
  hex-decode 후 fr.Element SetBytes/Marshal 적용해 snapshot G13
  정규화와 byte-equal. 과거 placeholder `passthrough_hex.v0` 는 함수가
  passthrough 였고 입력의 절반에서 (raw 32B > fr.Modulus) leaf hash 와
  silently mismatch. Customer-side derivation (HMAC/salt) 정식화는 V2
  이후 결정으로 보류.
- **G1 hint identifier closure (R3 step 4) — service-side resolution**
  (commit 8045c37). `solver.RegisterHint(corecircuit.IntegerDivision)`
  를 prover 시작 시 호출. G1 byte-equivalence 가 의도적으로 제외한
  hint identifier divergence (legacy `circuit.IntegerDivision` vs zkpor
  `corecircuit.IntegerDivision` 의 reflect-derived ID) 의 solver-side
  해소.
- **gnark-crypto bn254 poseidon `Write` 가 ≥fr.Modulus() 입력을 silent
  drop** (R3 step 4 prep 중 발견). `hash.Hash.Write` interface
  contract 상 caller 가 error 무시하기 쉽고, legacy `src/utils.
  VerifyMerkleProof` 도 무시함. 영향: Poseidon 으로 hash 하는 입력은
  반드시 < fr.Modulus 여야 함 — production hash 출력은 항상 만족하지만
  랜덤 byte (SHA-256 등) test fixture 는 fixture 측에서 top 3-bit 마스킹
  필요. `core/host/merkle_test.go` 의 `modSafeBytes` 헬퍼가 그 패턴.
- **DB-poll vs Redis BLPOP queue 결정 (R3 step 4 prover)** — 핵심 경로는
  단일 instance DB-poll (`store.ClaimOldestByStatus` 트랜잭션) 채택.
  legacy 는 Redis BLPOP + `GetAndUpdateBatchesWitnessByHeight` 쌍.
  multi-worker scaling 시 Redis 재도입 + 큐 producer 결정. 결정 근거:
  Redis dep 도입 비용 대비 단일 instance 환경에서 효익 없음, witness↔
  prover 의 well-defined ordering 이 트랜잭션 한 번이면 만족.
- **userproof self-contained tree 결정 (R3 step 4 userproof)** — legacy
  는 witness 가 persistent (redis) 트리 쓰면 userproof 가 같은 트리
  읽음 (cross-process state coupling). zkpor 는 snapshot + 동일 padding
  으로 트리를 처음부터 재구축 (root parity 보장). 근거: redis 의존
  없는 dev 환경에서도 동작, deployment topology 결정을 도구 안에 박지
  않음. 비용: tree population 한 번 (~accounts 회 Set) — 그러나
  userproof 의 dominant cost 는 어차피 GetProof + Marshal + DB write
  per account 라 무시 가능.

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
    │   ├── circuit/                          universal in-circuit 헬퍼
    │   │   ├── arithmetic.go
    │   │   ├── commitment.go
    │   │   ├── constants.go
    │   │   └── merkle.go
    │   ├── host/                             universal off-circuit 헬퍼
    │   │   ├── merkle.go                     (VerifyMerkleProof, Poseidon BN254 path-bit walk)
    │   │   └── merkle_test.go                (legacy parity + 5 tamper paths + mod-safe fixture)
    │   ├── tree/                             bsmt depth-28 SMT wrapper
    │   │   ├── tree.go                       (NewAccountTree memory/redis, EmptyAccountLeafHash)
    │   │   └── tree_test.go                  (memory round-trip via corehost.VerifyMerkleProof)
    │   └── solvency/                         audited math 카탈로그
    │       ├── spot_simple/doc.go            (★ R4 model-first priority — SEA GTM driver)
    │       ├── merkle_classic/doc.go
    │       ├── over_collateral_simple/doc.go
    │       ├── tier_1bucket/doc.go
    │       └── tier_3bucket/                 (★ 유일 spec+circuit+host 구현)
    │           ├── doc.go
    │           ├── spec/                     (types, risk, snapshot, constraint, witness)
    │           ├── circuit/                  (BatchCreateUserCircuit + helpers)
    │           └── host/                     (off-circuit, model-specific)
    │               ├── commitment.go         (ComputeUserAssetsCommitment + ComputeCexAssetsCommitment)
    │               ├── account.go            (AccountLeafHash + PaddingAccounts + UserConfig)
    │               └── serialize.go          (Encode/DecodeBatchWitness — gob+s2)
    ├── store/                                gorm 영속화 (cross-service 공유)
    │   ├── store.go                          (Open + ConvertMySQLErr + sentinels)
    │   ├── witness.go                        (BatchWitness 모델 + 상태머신)
    │   ├── proof.go                          (Proof 모델)
    │   ├── userproof.go                      (UserProof 모델)
    │   └── store_test.go                     (MySQL 번호→sentinel 매핑)
    ├── cmd/                                  zkpor-native service entries
    │   ├── verifier/                         (batch / -user / -hash)
    │   │   ├── config/config.go
    │   │   ├── main.go
    │   │   └── main_test.go
    │   ├── witness/                          (snapshot → BatchCreateUserWitness → DB. G6 assert)
    │   │   ├── config/config.go
    │   │   ├── main.go
    │   │   └── main_test.go
    │   ├── prover/                           (DB-poll → groth16.Prove+Verify → DB. G1 hint register)
    │   │   ├── config/config.go
    │   │   ├── main.go
    │   │   └── main_test.go
    │   └── userproof/                        (self-contained tree → per-user proof → DB)
    │       ├── config/config.go
    │       ├── main.go
    │       └── main_test.go
    └── profile/
        └── binance/                          (★ 유일 customer profile)
            ├── doc.go
            ├── batch_shape.go
            ├── catalog.go
            ├── constraint_noop.go
            ├── identity.go                   (Scheme "passthrough_hex_bn254_reduced.v0" — G2 frozen)
            ├── identity_test.go              (Scheme freeze + below/above modulus + 입력 panic)
            ├── insolvent.go
            ├── pricing.go
            ├── risk.go
            ├── snapshot.go                   (CexAssets + AccountStream + invalid 분류 done)
            ├── snapshot_test.go              (16 tests)
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
| off-circuit host 헬퍼 추출 (Merkle verify + commitment) | ✅ done — `core/host` + `tier_3bucket/host`, legacy byte-equivalence | R3 step 4 prep |
| service rewiring — verifier | ✅ done — `zkpor/cmd/verifier` (9f889ad) | R3 step 4 |
| service rewiring — witness | ✅ done — `zkpor/cmd/witness` (5332f40) + 의존 (c96018d core/tree, 32b9334 tier3host, 78acd39 store) | R3 step 4 |
| service rewiring — prover | ✅ done — `zkpor/cmd/prover` (8045c37) + store 확장 (16f36bd) | R3 step 4 |
| service rewiring — userproof | ✅ done — `zkpor/cmd/userproof` (fdf4a63) + store 확장 (b7e57e6) + UserConfig refactor (4e85757) | R3 step 4 |
| G1 hint identifier service-side closure (`solver.RegisterHint`) | ✅ done — prover 에서 등록 (8045c37) | R3 step 4 |
| G6 ValueScale startup assert | ✅ closed — witness startup 에서 default-symbol 검사 (5332f40) | R3 step 4 / G6 |
| G2 AccountIDProvider scheme v1 freeze | ✅ closed — `passthrough_hex_bn254_reduced.v0` + DeriveAccountID fr.Element 정규화 (3c691cb) | R3 step 4 / G2 |
| End-to-end sample-data smoke (witness → prover → verifier + userproof → verifier -user) | pending — R3 step 4 exit criteria 의 마지막 가드 | R3 step 4 / follow-up |
| witness multi-worker 병렬 hashing | pending | R3 step 4 follow-up |
| witness DB resume + tree rollback | pending | R3 step 4 follow-up |
| prover Redis BLPOP 큐 + multi-worker scaling | pending | R3 step 4 follow-up |
| prover -rerun 모드 (claimed-but-not-finished 회수) | pending | R3 step 4 follow-up |
| userproof multi-worker 병렬 + resume + -memory_tree 플래그 | pending | R3 step 4 follow-up |
| snapshot multi-shard concurrency (`csvSnapshot.streamShard` sequential) | pending | R3 step 4 follow-up or R4 |
| AccountIDProvider derivation 정식화 (HMAC/salt) | deferred (V2 candidate) | post-V1 |
| Second model 회로 구현 — `spot_simple` (SEA GTM driver, model-first) | pending | **R4 (model-first swap)** |
| Second customer profile — SEA reference (Indonesia/Thailand 우선) | pending | **R5 (was R4)** / G12 |
| Third model + core/circuit/ 추가 헬퍼 승격 | awaits signal | R6 / G11 |
| 카탈로그 v1 freeze | awaits R7 | R7 / G4 |
| 사용자-facing verification 분배 책임 (UI / 페이지) | deferred | post-V1 / customer SLA / G14 |
| Prove-path GPU 가속 (ICICLE) 채택 여부 (G15) | deferred | post-R3 step 4 / first production prove SLA |
| Module composition compatibility 검토 프로세스 (G16) | deferred | R5 candidate / 첫 multi-module composition |
| `core/constraint_modules/noop/` promotion (universal noop 분리) | pending | R3 step 4 직후 또는 R4 진입 시 |
| Composite 패턴 (`ComposeModules` 헬퍼) 도입 | pending | R5 candidate / 첫 N≥2 module deployment |
| Param-as-public-input 규칙 closure | pending | R5 candidate / 첫 parameterized module |
| Declarative `profile.toml` 첫 추출 | pending | R5 / 두 번째 customer (SEA reference) 도입 |
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

**R3 step 4 본체 마감**. 4 services 모두 zkpor-native 로 착지, 3 gate
모두 closed:

- service: verifier (9f889ad) / witness (5332f40) / prover (8045c37) /
  userproof (fdf4a63)
- gate: G1 hint service-side (8045c37) / G6 ValueScale assert
  (5332f40) / G2 Scheme freeze (3c691cb)
- 지원 모듈: core/tree (c96018d) / core/host (5f98fdd) / tier_3bucket
  /host (32b9334 + 4e85757) / store (78acd39 + 16f36bd + b7e57e6)

다음 슬라이스 두 갈래 — agent 가 선택 (또는 user 와 합의):

**갈래 A — R3 step 4 exit criteria 의 마지막 가드 닫기**:

```text
End-to-end sample-data smoke (R3 step 4 exit criteria 의 "sample data
기준 CLI end-to-end PoR 생성·검증 통과"):

  src/sampledata/ 또는 testdata 의 sample CSV 로 4 service 풀
  파이프라인 1회 실행 + verify 통과:
    1. zkpor/cmd/witness     (DB ↦ BatchWitness rows)
    2. zkpor/cmd/prover      (DB BatchWitness → Proof rows)
    3. zkpor/cmd/verifier    (batch 모드, proof_table.csv export 가
                              필요 — dbtool 슬라이스 후보)
    4. zkpor/cmd/userproof   (per-account UserProof + UserConfig JSON)
    5. zkpor/cmd/verifier -user  (user_config.json 으로 inclusion 통과)

  요건:
  - MySQL fixture (Docker compose or sqlite-in-memory 대체) — 결정 필요
  - .pk/.vk/.r1cs 파일 (keygen 실행 필요 — 첫 실행 비용 큼; 기존 legacy
    artifact 재사용 vs 새 ceremony 실행)
  - dbtool DB→proof_table.csv export — legacy 에서 포팅 필요할 수 있음
  - 자동화 script (Makefile target 또는 shell)

  의존 결정 — 위 요건들이 R3 step 4 의 자연 마감일지, 별도 stage(R3.5
  ops automation) 로 뺄지. user 와 합의.

R3 step 4 follow-ups (각각 별 슬라이스, 우선순위 user 결정):
  - witness multi-worker 병렬 + DB resume + tree rollback
  - prover Redis BLPOP 큐 + -rerun 모드
  - userproof multi-worker + resume + -memory_tree 플래그
  - snapshot multi-shard concurrency
```

**갈래 B — R4 진입 (SEA GTM driver model-first)**:

```text
spot_simple model spec + circuit 구현:

  zkpor/core/solvency/spot_simple/{spec,circuit}/* 신규 구현. 현재는
  doc.go only. tier_3bucket 의 spec/circuit/host 패턴을 reference 로
  하되, spot_simple 의 단순한 (no tier table, no 3-bucket collateral)
  수학을 가진다.

  핵심 차이 (tier_3bucket vs spot_simple):
    - per-user per-asset 5-tuple (Equity, Debt, Loan, Margin, PM) →
      2-tuple (Equity, Debt). 단순.
    - 3-bucket collateral haircut 사라짐.
    - tier ratio table 사라짐.
    - 하지만 sum equality + Merkle proof + AccountID + CexCommitment
      구조는 동일 (universal).

  검증 순서 (R3 와 유사한 sub-slice 구조 추천):
    R4 step 0 — Compile + Setup smoke
    R4 step 1 — alpha wiring (tier_3bucket 에서 패턴 검증됨, 재사용)
    R4 step 2 — trusted setup ceremony + .pk/.vk publish

  customer profile (SEA reference) 는 R5 — model 위에 어댑터 8개 +
  declarative profile.toml 첫 추출.

  R4 진입 전 권장:
  - docs/01-project-context.md 의 SEA zoom-in 재확인
  - tier_3bucket 의 4 layer 구조 (spec / circuit / host / cmd) 가
    universal substrate 로 잘 작동하는지 audit (R6 의 core/circuit
    promotion 후보 식별 시작)
```

**갈래 C — 작은 정리 작업** (low-risk, 언제든 가능):

```text
- core/constraint_modules/noop/ promotion (현재 profile/binance/
  constraint_noop.go 에 있는 noopModule 을 universal layer 로). 1 commit.
- HANDOFF + PRODUCTION_ROADMAP doc sweep (다음 큰 변경 직전 정렬).
- goroutine leak guard (uber-go/goleak dep 도입 검토).
```

갈래 선택 기준 (suggested):
- end-to-end smoke 없이는 R3 step 4 "끝났다" 라고 단언 못 함 → A 권장.
- 단, A 가 MySQL fixture / keygen / dbtool 등 ops 의존이 많음. user
  가 그 의존을 감수할 의향 있는지 사전 확인.
- A 가 부담스럽다면 B (R4 spot_simple 진입) 가 다음 큰 가치 흐름.
  R3 step 4 follow-ups 와 R4 는 병렬 가능 (다른 영역).

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

End-to-end (R3 step 4 exit criteria — 아직 미마감, 갈래 A 진입 시 채움):

```bash
# 1) MySQL fixture 기동 (Docker compose or 동등) — TBD
# 2) keygen 으로 .pk/.vk/.r1cs 생성 (or legacy artifact 재사용) — TBD
# 3) cd zkpor/cmd/witness   && go run . (BatchWitness rows 쓰기)
# 4) cd zkpor/cmd/prover    && go run . (Proof rows 쓰기)
# 5) dbtool 로 proof_table.csv export — dbtool 슬라이스 후보
# 6) cd zkpor/cmd/verifier  && go run . (batch 모드 verify 통과)
# 7) cd zkpor/cmd/userproof && go run . (UserProof rows 쓰기)
# 8) cd zkpor/cmd/verifier  && go run . -user (단일 사용자 inclusion)
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
