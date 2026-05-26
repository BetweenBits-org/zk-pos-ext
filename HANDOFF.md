# HANDOFF.md

이 문서는 agent 세션이 바뀌어도 작업을 이어가기 위한 **현재 시점의 인수인계**다.
긴 히스토리 로그가 아니다. 작업 시작 전 source priority 상위 문서를 먼저 읽는다.

## Current State

Latest implementation commit (`zkpor/.git/`, branch `main`):

```text
f511dcb feat(zkpor): R4-2 — spot_simple Setup smoke + ComputeFlatUint64Commitment fix
466ef55 feat(zkpor): R4-1 — spot_simple circuit (BatchCreateUserCircuit + witness builder)
e4dc0cb feat(zkpor): R4-0 — spot_simple spec package (model 2 entry)
a6469c6 feat(zkpor): F — EC2 remote-test helpers + smoke.sh shape parametrisation
6cb6a37 docs(handoff+roadmap): close R3 step 4 with smoke + 3 supporting slices
d7c23f3 feat(zkpor): A5 — end-to-end smoke harness + 3 bug fixes surfaced en route
1d5b2e9 feat(zkpor): keygen service + smoke MySQL fixture (A2 + A3)
1d5571b refactor(zkpor): AssetCounts — profile-owned, catalog as source of truth (E)
f1ba54a feat(zkpor): verifier — DB direct proof-table read path (Option B for A4)
11f2d0a feat(zkpor): binance.NewBatchShape — ZKPOR_BATCH_SHAPE_OVERRIDE for smoke (A1)
cd6a0db docs(handoff+roadmap): catch up to R3 step 4 closure + 2 next-slice 갈래
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
| `zkpor/core/spec/*` | ✅ complete — engine 표준만 잔존. **`AssetCounts=500` 제거 (E refactor, 1d5571b)** — profile capacity 였으므로 `AssetCatalog.Capacity()` 가 단일 진실원, R5 `profile.toml` 슬롯과 정합. |
| `zkpor/core/circuit/*` | ✅ complete — universal 헬퍼 4 파일 (Merkle, commitment, arith) |
| `zkpor/core/host/*` | ✅ off-circuit (native) universal 헬퍼 — `VerifyMerkleProof` (Poseidon BN254 SMT, legacy parity). R3 step 4 prep (commit 5f98fdd) |
| `zkpor/core/tree/*` | ✅ bsmt depth-28 SMT wrapper + `EmptyAccountLeafHash` (Poseidon(0,0,0,0,0)). memory/redis 백엔드. **default=memory, redis=opt-in** (A0 결정: snapshot-SoT 와 정합). witness + userproof 공유 (commit c96018d) |
| `zkpor/core/solvency/tier_3bucket/host/*` | ✅ off-circuit model-specific 헬퍼 — `ComputeUserAssetsCommitment` + `ComputeCexAssetsCommitment(slice, capacity)` (5f98fdd + E refactor 1d5571b) + `AccountLeafHash` + `PaddingAccounts` + `EncodeBatchWitness`/`DecodeBatchWitness` (32b9334; **DecodeBatchWitness 가 capacity 를 witness 의 BeforeCexAssets 길이로 자기-기술**, E refactor) + 공유 `UserConfig` 타입 (4e85757). 모두 legacy byte-equivalence/round-trip 테스트 통과 |
| `zkpor/store/*` | ✅ gorm 영속화 계층 — `Open` + `ConvertMySQLErr` + 3 모델 (`BatchWitness` 78acd39, `Proof` + witness 상태머신 메서드 16f36bd, `UserProof` b7e57e6) + **`ProofStore.ListAllInOrder()` (A4, f1ba54a)** for verifier DB 직접 읽기 경로. 단일 instance DB-poll (`ClaimOldestByStatus` 트랜잭션) 채택 — Redis BLPOP 큐는 multi-worker scaling 시 follow-up. PostgreSQL adapter 는 slice D (deferred) |
| `zkpor/cmd/verifier/*` | ✅ R3 step 4 첫 service — legacy `src/verifier` 의 zkpor-native 대체 (3-mode CLI: batch / -user / -hash). src/utils + legacy circuit import 0. `UserConfig` 는 tier_3bucket/host 공유 타입 (4e85757). **DB 직접 읽기 모드 추가 (A4, f1ba54a)** — `MysqlDataSource` 설정 시 proof.csv hop 제거, `ProofStore.ListAllInOrder` 로 직접 ingest. **AssetCapacity config (E)**. **per-asset equity<debt 가 panic → warning** (A5 d7c23f3) — tier_3bucket 은 자산별 차용 허용 |
| `zkpor/cmd/witness/*` | ✅ R3 step 4 service — snapshot → BatchCreateUserWitness → DB (commit 5332f40). **G6 closure 동반** (`PriceMultiplier × BalanceMultiplier == ValueScale` startup assert). `AssetCapacity` config (E refactor). `BeforeCexAssets` slice 가 snapshot 길이로 sizing. `accountTree.Commit(nil)` (pruning off; A5 fix d7c23f3). `-dump-final-cex <path>` smoke harness 플래그. 핵심 경로 only — multi-worker 병렬 / DB resume / tree rollback 은 follow-up |
| `zkpor/cmd/prover/*` | ✅ R3 step 4 service — DB-poll Published → groth16.Prove+Verify → proof 테이블 (commit 8045c37). **G1 hint closure** (`solver.RegisterHint(corecircuit.IntegerDivision)`). idempotent persist + lazy snarkParams cache. Decode self-infers capacity from witness data (E). Redis BLPOP 큐 / -rerun 모드는 follow-up |
| `zkpor/cmd/userproof/*` | ✅ R3 step 4 service — self-contained tree 재구축 (witness redis 의존 제거) → per-account inclusion proof → DB (commit fdf4a63). 동일 padding 으로 root parity. `AssetCapacity` config (E refactor). `-dump-user-index/-dump-user-path` smoke harness 플래그. 핵심 경로 only — multi-worker 병렬 / -memory_tree 플래그 / resume 은 follow-up |
| `zkpor/cmd/keygen/*` | ✅ **새 service (A3, 1d5b2e9)** — zkpor-native trusted setup. `binance.NewBatchShape()` (override 가능) 와 `-asset-capacity` 플래그로 (userAssetCounts, assetCapacity, batchCounts) 회로 compile + groth16.Setup. StandardKeyName 파일 (`zkpor.tier_3bucket.<tier>_<users>.{pk,vk,r1cs}`), `-legacy-names` 옵션. Tiny smoke (5,5,10): 286k constraints, ~21s, .pk 113MB. |
| `zkpor/core/solvency/tier_3bucket/spec/*` | ✅ complete — types, RiskPolicy, SnapshotSource (`InvalidCount()` 추가됨, R2/2 step 2), ConstraintModule, witness (BatchCreateUserWitness 등) |
| `zkpor/core/solvency/tier_3bucket/circuit/*` | ✅ complete — BatchCreateUserCircuit + helpers ported. `SetBatchCreateUserCircuitWitness` 는 `assetCountTiers` 를 인자로 받음. **Alpha wiring (R3 step 2)** + **R1CS byte-equivalence vs legacy (R3 step 3 / G1)**. **A5 fix d7c23f3** — `SetBatchCreateUserCircuitWitness` 의 padding UserAssetInfo entries 가 legacy 처럼 6개 collateral 필드를 명시적 0 으로 초기화 (이전엔 nil 이라 gnark `can't set fr.Element with <nil>` 실패). |
| `zkpor/core/solvency/spot_simple/{spec,circuit}/*` | ✅ **R4 done** — spec (types, snapshot, witness, constraint) + circuit (BatchCreateUserCircuit + Define + SetBatchCreateUserCircuitWitness) + setup smoke (NbConstraints=33,306 at tiny shape (5,10,2), R1CS sha256 baseline 기록). 5-input account leaf (`Poseidon(accountID, totalEquity, 0, 0, assetsCommitment)`) 로 substrate 의 universal `EmptyAccountLeafHash` 공유. RiskPolicy 부재 (doc 명시). |
| `zkpor/core/solvency/{merkle_classic,over_collateral_simple,tier_1bucket}/` | ⏸ doc.go only — 카탈로그 reserved. R6 (3rd model) rule-of-three 대기. |
| `zkpor/profile/binance/*` | ✅ snapshot ETL 흡수 완료. **G2 closed** (Scheme `passthrough_hex_bn254_reduced.v0`). **G13 closed** (snapshot AccountID fr.Element 정규화). `NewCatalog(orderedSymbols, capacity)` (E refactor — capacity 가 catalog 인스턴스 필드). `SnapshotConfig.AssetCapacity` 추가. `NewBatchShape()` 가 `ZKPOR_BATCH_SHAPE_OVERRIDE` env var 지원 (A1 11f2d0a). multi-shard concurrency 는 여전히 sequential (follow-up) |
| `zkpor/deploy/` | ✅ **smoke MySQL fixture (A2, 1d5b2e9)** — `docker-compose.yml` 단일 컨테이너 (mysql:8.0, healthcheck, 영속 볼륨). Memory tree 라 Redis 컨테이너 불필요. 사용: `docker compose -f deploy/docker-compose.yml up -d` |
| `zkpor/scripts/` | ✅ **end-to-end smoke 하네스 (A5, d7c23f3)** — `smoke.sh` 가 docker compose → keygen (캐시) → witness → prover → verifier(batch) → userproof → verifier(-user) 순으로 전체 파이프라인 실행. R3 step 4 exit criteria 검증 완료. |
| `circuit/`, `src/` (legacy) | ✅ untouched, fully functional. trusted setup 그대로 유효 |
| docs (`zkpor/AGENTS.md`, `zkpor/CLAUDE.md`, `zkpor/PRODUCTION_ROADMAP.md`, `zkpor/docs/01-project-context.md`, `zkpor/docs/02-module-architecture.md`) | ✅ complete |

## Current Implementation Snapshot

최근 작업 흐름:

```text
<R4/2>   feat(zkpor): R4-2 — spot_simple Setup smoke + Commitment fix
        (commit f511dcb. setup_test.go (Compile+Setup at tiny shape
         5_10_2, NbConstraints=33,306, R1CS sha256 baseline 기록 +
         noop-module zero-cost regression guard). 부수로 core/circuit
         /commitment.go 의 잠재 버그 fix: ComputeFlatUint64Commitment
         가 flatten length % 3 != 0 시 trailing partial field 를
         `_ = last` 로 discard 해 tmp[nEles-1] 이 nil 인 채 Poseidon
         호출 → panic. tier_3bucket 의 6-field-per-asset 가 3 의
         배수라 안 걸렸음, spot_simple 의 2-field-per-asset 첫 노출.
         tier_3bucket R1CS sha256 (678eb23f…) + coefficients sha256
         불변 확인 — 새 partial-field 분기가 tier_3bucket 에선 미진입.)
<R4/1>   feat(zkpor): R4-1 — spot_simple circuit 본체
        (commit 466ef55. BatchCreateUserCircuit + Define + 
         SetBatchCreateUserCircuitWitness + paddingAsset 헬퍼.
         tier_3bucket circuit 구조 모방하되 simplify: no tier table /
         no haircut / no 3-bucket collateral, AssetsForUpdateCex 가
         1-field-per-slot (vs tier_3bucket 의 5). Random-linear-
         combination cross-check 도 1-power-per-slot. 5-input account
         leaf 로 substrate 호환. PowersOfSixteenBits 는 tier_3bucket
         에서 복제 (R6 promotion candidate).)
<R4/0>   feat(zkpor): R4-0 — spot_simple spec 패키지 신설
        (commit e4dc0cb. types (1-tuple AccountAsset, no debt/
         collateral) + snapshot (SnapshotSource interface) + witness
         (BatchCreateUserWitness + helpers) + constraint (Constraint
         Module + slim Context). RiskPolicy 부재 — doc 의 "Notably
         absent (vs tier_3bucket)" 정합.)
<R3/4n>  feat(zkpor): A5 — end-to-end smoke harness + 3 bug fixes
        (commit d7c23f3. R3 step 4 exit criteria 종결. scripts/smoke.sh
         가 docker mysql → keygen (cache) → witness → prover → verifier
         (batch, DB direct) → userproof → verifier(-user) 풀 파이프
         라인을 sample data 로 통과. Last run: account tree root
         142c03677f6f… 가 모든 stage 에서 일치, 17/17 proofs verify,
         account #0 inclusion "verify pass!!!".
         Bug fix 3건: (1) witness.Commit(&v) → Commit(nil) — bsmt
         recentVersion 은 pruning 버전이라 ≥ newVersion 일 때 ErrVersion
         TooHigh. (2) SetBatchCreateUserCircuitWitness padding entries
         의 6개 collateral 필드가 nil 이라 gnark 실패 — legacy 처럼
         명시적 0 으로 초기화. (3) verifier 의 per-asset equity<debt
         panic → warning (tier_3bucket 은 자산별 차용 허용).)
<R3/4m>  feat(zkpor): keygen service + smoke MySQL fixture (A2 + A3)
        (commit 1d5b2e9. zkpor/cmd/keygen — binance.NewBatchShape() 의
         shape 들에 대해 frontend.Compile + groth16.Setup, .pk/.vk/.r1cs
         를 -out 에 출력. `-asset-capacity` 필수 (E refactor 와 정합),
         StandardKeyName 파일 stem. Smoke shape (5,5,10): 286,157
         constraints, compile 2s + setup 19s, .pk 113MB.
         deploy/docker-compose.yml — single MySQL 8.0 + healthcheck.
         Memory tree (A0 결정) 라 Redis 컨테이너 불필요.)
<R3/4l>  refactor(zkpor): AssetCounts — profile-owned (slice E)
        (commit 1d5571b. `corespec.AssetCounts = 500` 제거. AssetCounts
         는 deployment 결정값이지 engine 표준이 아니었음. AssetCatalog.
         Capacity() 가 단일 진실원. NewCatalog(orderedSymbols, capacity)
         로 capacity 가 catalog 인스턴스 필드. SnapshotConfig.AssetCapacity
         추가, loadCSVSnapshot 이 그 값으로 pad. ComputeCexAssets
         Commitment(slice, capacity) — capacity 명시 인자. Decode
         BatchWitness(data) — capacity 를 BeforeCexAssets 길이에서 추론.
         cmd/{witness,prover,userproof,verifier} 모두 AssetCapacity
         config. Smoke 효과: tiny capacity=5 가능, keygen 6.8M → 286k
         constraints, ~20분 → ~21초. R5 declarative profile.toml 슬롯
         과 정합. Legacy parity 테스트가 capacity=500 으로 byte-neutral
         검증.)
<R3/4k>  feat(zkpor): A4 — verifier DB direct proof-table read
        (commit f1ba54a. Option B (A0 user agreement) — proof.csv hop
         제거. verifier config 에 optional MysqlDataSource + DbSuffix.
         loadProofs 가 CSV/DB 분기. convertStoredProof 가 store.Proof →
         proofRow seam (json.Marshal of [][]byte 이 base64-string
         이라 CSV 와 byte-identical). ProofStore.ListAllInOrder 신설.
         legacy ProofTable CSV 경로 backward-compat 유지.)
<R3/4j>  feat(zkpor): A1 — binance.NewBatchShape shape override env
        (commit 11f2d0a. `ZKPOR_BATCH_SHAPE_OVERRIDE` env var 로
         production shapes {50_700, 500_92} 를 임의 shape (e.g. 5_10)
         으로 swap. Smoke 하네스만 set, 서비스 코드/테스트/config 무변경.
         "<tier>_<users>[,…]" 형식, parse error 는 NewBatchShape
         panic. 13 unit test 추가 (default/single/multi-sort/9 reject
         cases).)
<R3/4i>  docs(handoff+roadmap): catch up to R3 step 4 closure + 2 next-slice 갈래
        (commit cd6a0db — 이전 handoff doc 슬라이스. 본 슬라이스 이전
         의 상태를 묘사. 이 라인 아래는 그 시점 기준의 진행 흐름.)
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

**R3 step 4 본체는 종결** — 4 services + G1/G2/G6 closure + AssetCounts
catalog 재배치 (E) + end-to-end smoke (A5) 모두 done. Exit criteria
"sample data 기준 CLI end-to-end PoR 생성·검증 통과" 충족.

R3 step 4 follow-ups (post-smoke):

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

- **AssetCounts 재배치 (slice E, commit 1d5571b)** — 기존 `corespec.
  AssetCounts = 500` 은 core spec 에 박혔으나 실제로는 Binance
  deployment cap (다른 profile 은 다른 capacity 를 가질 수 있음).
  E 슬라이스가 const 를 제거하고 `AssetCatalog.Capacity()` 를 단일
  진실원으로 격상, 모든 model/cmd 코드는 catalog 인스턴스 또는
  config 의 `AssetCapacity` 를 통해 capacity 를 받음. R5 의
  declarative `profile.toml` (G12) 이 이 슬롯에 자연스럽게 매핑됨.
  Side effect: smoke 가 capacity=5 로 가능해져 keygen 6.8M → 286k
  constraints (~20분 → 21초). 기존 production 경로는 capacity=500
  으로 byte-neutral (legacy parity 테스트가 잠금).

- **bsmt.Commit 의 인자는 pruning version (A5 fix d7c23f3)** — witness
  최초 작성 시 `Commit(&v)` 로 v=height+1 을 전달해 "the version is
  higher than the latest" 로 panic 했다. 실제로는 bsmt 가 newVersion
  을 `tree.version+1` 로 auto-increment 하고 인자는 *pruning* version.
  Fix: `Commit(nil)` (pruning off; memory tree 는 무영향, redis multi-
  batch resume 시 도입 검토). userproof 도 같은 방식 (Commit(nil)).

- **SetBatchCreateUserCircuitWitness padding zero-init (A5 fix d7c23f3)**
  — padding UserAssetInfo 에서 AssetIndex 만 set 하고 6개 collateral
  필드를 leave-nil 한 결과 gnark frontend 가 "can't set fr.Element
  with <nil>" 로 거부. legacy 회로 코드는 명시적 0 을 set 함 — port
  과정에서 빠진 줄을 복원. `paddingAsset` 헬퍼로 통일.

- **verifier per-asset equity≥debt 가정 완화 (A5 d7c23f3)** — legacy
  verifier 가 모든 자산에서 TotalEquity ≥ TotalDebt 를 panic 으로
  강제했으나 tier_3bucket 모델은 자산별 차용을 허용 (account-level
  collateral≥debt 만 불변식). 자산별 위반 시 warning log 만 출력
  하도록 강등.

- **Store driver 인터페이스 + PG adapter (slice D, deferred)** — store
  층의 `gorm.io/driver/mysql` 직접 의존 + MySQL 전용 hints + error
  number 매핑을 driver-aware 추상화로 분리하면 PostgreSQL adapter
  가 ~1 commit. Smoke 와 무관하게 진행 가능 — DEFERRED 작업 표.

- **R4 substrate audit (R4-3 done)** — core/circuit substrate 가
  spot_simple 을 신규 helper 없이 수용함 확인. 두 model 양쪽이 쓰는
  universal symbols 7개 (`API`, `Variable`, `BatchCommitment`,
  `ComputeFlatUint64Commitment`, `AccountIndexToMerkleHelper`,
  `VerifyMerkleProof`, `UpdateMerkleProof`, `TwoToTheSixtyFour`).
  tier-only 3개 (`CheckedDivByConstant`, `IntegerDivision`,
  `TwoToTheOneTwentyEight`) 는 spot 미사용일 뿐 model-bound 가 아니라
  잠재 universal (3rd model 이 division 등을 쓰면 자연 활용).
  Substrate 가 sound — 추가 promotion 불필요.

- **R6 promotion candidates (rule-of-three first event)** — 두 model
  사이에 중복된 항목들 (R6 G11 trigger 시 promote):
  · `PowersOfSixteenBits` 가 tier_3bucket/circuit/constants.go +
    spot_simple/circuit/constants.go 양쪽에 동일 정의로 존재. 의미는
    universal (2^16 powers — asset id packing). R6 에서 core/circuit
    로 옮긴다.
  · R1CS hash test helpers (`bn254R1Cs`, `hashR1Cs`, `hashCoefficients`,
    `writeLinearExpr`, `writeUint64`) 가 tier_3bucket legacy_compare
    _test.go + spot_simple setup_test.go 양쪽에 중복. R6 에서 test
    helper 패키지 또는 core/circuit/testhelper 로 promote.

- **noop ConstraintModule promotion 보류 (R4-4 reassessed)** — 두 model
  의 `ConstraintContext` 가 다른 field 셋을 가지므로 (tier_3bucket 은
  collateral/tier ratios, spot 은 equity only) 진정한 universal noop
  은 generic constraint 가 필요. 단순 promotion 가치가 limited —
  binance/constraint_noop.go 는 profile-specific 으로 유지, R6 (3rd
  model) 시 재검토.

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
    │   ├── verifier/                         (batch / -user / -hash; DB direct read mode)
    │   │   ├── config/config.go              (+MysqlDataSource / DbSuffix / AssetCapacity)
    │   │   ├── main.go
    │   │   └── main_test.go
    │   ├── witness/                          (snapshot → BatchCreateUserWitness → DB. G6 assert)
    │   │   ├── config/config.go              (+AssetCapacity)
    │   │   ├── main.go                       (-dump-final-cex smoke flag, Commit(nil))
    │   │   └── main_test.go
    │   ├── prover/                           (DB-poll → groth16.Prove+Verify → DB. G1 hint register)
    │   │   ├── config/config.go
    │   │   ├── main.go
    │   │   └── main_test.go
    │   ├── userproof/                        (self-contained tree → per-user proof → DB)
    │   │   ├── config/config.go              (+AssetCapacity)
    │   │   ├── main.go                       (-dump-user-index/-dump-user-path smoke flags)
    │   │   └── main_test.go
    │   └── keygen/                           (★ A3 — zkpor-native trusted setup)
    │       └── main.go                       (frontend.Compile + groth16.Setup per shape)
    ├── deploy/                               (★ A2 — smoke MySQL fixture)
    │   └── docker-compose.yml                (mysql:8.0 + healthcheck + persistent volume)
    ├── scripts/                              (★ A5 — smoke harness)
    │   └── smoke.sh                          (docker→keygen→witness→prover→verifier→userproof→verifier-user)
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
| Shape override env var (A1) | ✅ done — `ZKPOR_BATCH_SHAPE_OVERRIDE` (commit 11f2d0a) | R3 step 4 / smoke prep |
| Verifier DB direct read (A4 / Option B) | ✅ done — `ProofStore.ListAllInOrder` + verifier loadProofs 분기 (commit f1ba54a) | R3 step 4 |
| AssetCounts → profile-owned + catalog source-of-truth (E) | ✅ done — `spec.AssetCounts` 제거, capacity 가 `AssetCatalog.Capacity()` 및 SnapshotConfig/Service configs 의 `AssetCapacity` 로 흐름 (commit 1d5571b) | R3 step 4 / arch correction |
| zkpor/cmd/keygen + smoke MySQL fixture (A2 + A3) | ✅ done — `zkpor/cmd/keygen` (binance.NewBatchShape 기반, `-asset-capacity` 필수) + `deploy/docker-compose.yml` (commit 1d5b2e9) | R3 step 4 / smoke infra |
| End-to-end sample-data smoke (A5) | ✅ done — `scripts/smoke.sh` 가 풀 파이프라인 (witness→prover→verifier(batch)→userproof→verifier(-user)) 통과 + 3 버그 fix (commit d7c23f3) | R3 step 4 / **exit criteria 종결** |
| witness multi-worker 병렬 hashing | pending | R3 step 4 follow-up |
| witness DB resume + tree rollback | pending | R3 step 4 follow-up |
| prover Redis BLPOP 큐 + multi-worker scaling | pending | R3 step 4 follow-up |
| prover -rerun 모드 (claimed-but-not-finished 회수) | pending | R3 step 4 follow-up |
| userproof multi-worker 병렬 + resume + -memory_tree 플래그 | pending | R3 step 4 follow-up |
| snapshot multi-shard concurrency (`csvSnapshot.streamShard` sequential) | pending | R3 step 4 follow-up or R4 |
| AccountIDProvider derivation 정식화 (HMAC/salt) | deferred (V2 candidate) | post-V1 |
| Store driver abstraction + PG adapter (slice D) | pending — `store.Open(driver, dsn)` + ConvertDriverErr 매핑 + MaxExecutionTime context 추상화 | post-A / DEFERRED |
| EC2 원격 sync 스크립트 (slice F) | pending — rsync + ssh helper, m6i.{2,4}xlarge 권장 | post-A / DEFERRED |
| Second model 회로 구현 — `spot_simple` (SEA GTM driver, model-first) | ✅ done — spec + circuit + setup smoke (NbConstraints=33,306 baseline). Substrate audit 통과 (R4-3) | **R4 종결** |
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

**R3 step 4 + R4 종결**.

R3 step 4 산출물: 4 services + 3 gate (G1/G2/G6) + AssetCounts
재배치 (E) + end-to-end smoke (A5) — `scripts/smoke.sh` 풀 파이프
라인 통과, 17/17 proofs verify, account #0 inclusion "verify
pass!!!" (commit chain a6469c6 … 11f2d0a).

R4 산출물: spot_simple model (spec + circuit + setup smoke) — 두번째
model이 substrate 위에 안정 안착. NbConstraints=33,306 (tiny shape),
core/circuit 의 universal helpers 가 신규 추가 없이 수용. 부수로
`ComputeFlatUint64Commitment` 잠재 버그 1건 fix (commit chain
f511dcb → 466ef55 → e4dc0cb).

R6 promotion candidates 식별 (substrate audit): `PowersOfSixteenBits`,
R1CS hash test helpers — 두 model 양쪽에 중복, rule-of-three 첫
event. 3rd model 시 promote.

다음 슬라이스 갈래 — agent 가 선택 (또는 user 와 합의):

**갈래 B' — R5 진입 (SEA reference customer + declarative profile)**:

```text
R4 가 spot_simple model 위에 customer 어댑터를 빠르게 받을 수 있는지를
검증할 차례. R5 산출물:
  - SEA reference customer (Indonesia/Thailand 후보) profile 패키지
    (`zkpor/profile/<customer>/*`) — 어댑터 8개.
  - 그 customer 의 spot_simple host helpers
    (`zkpor/core/solvency/spot_simple/host/{commitment,account,serialize}.go`)
    — tier_3bucket/host 패턴 모방. ComputeUserAssetsCommitment +
    ComputeCexAssetsCommitment + AccountLeafHash + Encode/Decode
    BatchWitness.
  - declarative `profile.toml` 첫 추출 — binance + SEA 두 어댑터를
    같은 toml schema 위에서 표현. schema freeze 는 R7.
  - multi-customer `.vk` 공유/분리 정책 (G12 closure).

전제: SEA customer 후보 narrow-down 이 user/partner 측에서 진행돼
있어야 함. 코드만이면 hypothetical SEA fixture 로 진행 가능 (실제
customer 데이터 없이도 model→customer flow 검증).
```

**갈래 C — 작은 정리 작업** (low-risk, 언제든 가능):

```text
- goroutine leak guard (uber-go/goleak dep 도입 검토).
- R4 가 surface 한 R6 promotion candidates 의 사전 정리
  (PowersOfSixteenBits, R1CS hash helpers — 단순 복제 → core 로
  이동 — rule-of-three 엄격 적용 안 하면 가능, 보수적이면 R6 대기).
```

**갈래 D — Store Driver 추상화 + PG adapter** (DEFERRED 슬라이스):

```text
- store.Open(driver, dsn) — gorm.io/driver/{mysql,postgres} 분기
- ConvertDriverErr (MySQL number ↔ PG SQLSTATE 매핑 테이블)
- MaxExecutionTime: MySQL hint vs PG `gorm.WithContext` 시간 제한
- 1 commit. Smoke 와 무관, 별도 진행 가능.
```

**갈래 F — EC2 원격 테스트 sync 스크립트** (사용자 추가 요청, infra):

```text
- scripts/ec2-sync.sh — rsync + ssh helper, exclude .artifacts/.git/
- scripts/ec2-keygen.sh — 원격에서 production capacity (500) keygen
- 권장 사양: m6i.{2,4}xlarge — capacity=500 production shape (50_700)
  은 .pk 12GB / 8 vCPU/32GB 면 충분. 4xlarge 가 더 여유.
```

R3 step 4 follow-ups (각각 별 슬라이스, 우선순위 user 결정):
  - witness multi-worker 병렬 + DB resume + tree rollback
  - prover Redis BLPOP 큐 + -rerun 모드 + multi-worker
  - userproof multi-worker + resume + -memory_tree 플래그
  - snapshot multi-shard concurrency

갈래 선택 기준 (suggested):
- R3 step 4 + R4 종결됐으므로 다음 큰 가치 흐름은 **B' (R5 SEA
  reference customer 진입)** — model 위에 customer adapter 가 빠르게
  올라가는지를 검증하는 첫 event 이자 declarative profile.toml 추출
  trigger.
- R5 진입 전제로 SEA customer 후보 narrow-down 이 user/partner 측
  결정 필요. code-only flow 검증이 우선이라면 hypothetical SEA
  fixture (Indonesia/Thailand 합성 데이터) 로도 진행 가능.
- R3/R4 follow-ups (multi-worker scaling, prover/userproof
  parallelism, host helpers for spot_simple) 는 production scale
  prove SLA 가 측정될 때 필요. F 의 EC2 환경에서 한 번 production
  keygen + smoke 하면 SLA 데이터 확보 가능 — follow-up 우선순위가
  그 시점에 정해짐.
- D 는 DBaaS 선택지가 정해질 때 (Supabase/RDS PG vs Aurora MySQL 등)
  진행. 그 전까지는 deferred.

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

End-to-end smoke (R3 step 4 exit criteria — ✅ A5 슬라이스로 마감):

```bash
# 한 줄 실행 — sample data + tiny capacity=5 + shape=5_10
./zkpor/scripts/smoke.sh

# 또는 stage 분해:
docker compose -f zkpor/deploy/docker-compose.yml up -d
ZKPOR_BATCH_SHAPE_OVERRIDE=5_10 go run ./zkpor/cmd/keygen -asset-capacity 5 -out ./zkpor/.artifacts/
# (이후 scripts/smoke.sh 가 service configs 생성 + 5 services 순차 실행)
docker compose -f zkpor/deploy/docker-compose.yml down -v   # 정리

# Production capacity smoke (m6i.{2,4}xlarge 권장):
ZKPOR_BATCH_SHAPE_OVERRIDE=50_700 go run ./zkpor/cmd/keygen -asset-capacity 500 -out ./zkpor/.artifacts/
# ... 동일 흐름, 다만 keygen 가 분 단위, prove 가 batch 당 분 단위
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
