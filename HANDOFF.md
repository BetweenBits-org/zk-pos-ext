# HANDOFF.md

이 문서는 agent 세션이 바뀌어도 작업을 이어가기 위한 **현재 시점의 인수인계**다.
긴 히스토리 로그가 아니다. 작업 시작 전 source priority 상위 문서를 먼저 읽는다.

## Current State

Latest implementation commit (`zkpor/.git/`, branch `main`):

```text
R8-close docs(handoff+roadmap+02): close R8 — registry pattern + G17
83cbfbe feat(zkpor): R8-E/F — profile cleanup + snapshot factory carries pricing
950c728 feat(zkpor): R8-D — verifier + userproof wiring + smoke.sh full integration
ad73d80 feat(zkpor): R8-C/3 — prover wiring (profile.toml-driven)
469c019 feat(zkpor): R8-C/2 — witness wiring (profile.toml-driven)
f427b21 feat(zkpor): R8-C/1 — keygen wiring (profile.toml-driven)
fc8325d feat(zkpor): R8-B/3 — constraint module registry (T1 + T4)
4369a91 feat(zkpor): R8-B/2 — snapshot connector registry (T1 + T4)
d9d7135 feat(zkpor): R8-B/1 — declarative builders (model-blind half)
78710d5 feat(zkpor): R8-A — registry infrastructure (identity + insolvent)
64cb1af docs(roadmap+handoff+arch): add R8 stage — Profile wiring + adapter cleanup
8f62d84 docs(handoff+roadmap+arch): close R7 — V1 catalog freeze 완료
08cce42 feat(zkpor): R7-C — module 카탈로그 governance + empty v1 entries
17429e4 feat(zkpor): R7-B — Profile descriptor schema v1 freeze (wiring carry)
9388694 feat(zkpor): R7-A — catalog v1 freeze + LegacyKeyName 즉시 제거
d2f0f06 docs(zkpor): R6.5 close — bw6 env fix + 4 model setup smoke baseline 기록
0f208d2 docs(zkpor): T2/T3 close — 4 model catalog 구현 완료, R7 prep
159f836 feat(zkpor): T2 — t2_static_haircut_margin 본체 + host
5549fdb feat(zkpor): T3 — t3_tiered_haircut_margin_1pool 본체 + host
44a47d9 feat(zkpor): R6 close — G11 closure (universal AccountLeafHash) + handoff/roadmap
722a133 feat(zkpor): R6-B — t1_simple_margin host helpers + sea_reference debt=0 wire
829e81c feat(zkpor): R6-A — t1_simple_margin spec + circuit (debt absorbed, spot superset)
b0318e1 feat(zkpor): R6 prep — 04-solvency-models docs + Tn naming v1 (5→4 model, versioned change)
4330cbf docs(zkpor): add 03-system-architecture visual overview (R5 snapshot)
b5b3236 docs(handoff+roadmap): close R5 — sea_reference + profile.toml + G12
8fb5b3f docs(zkpor): R5-4 — G12 closure (multi-customer .vk sharing policy)
23566aa feat(zkpor): R5-3 — declarative profile.toml schema + two reference instantiations
f41d36a feat(zkpor): R5-2 — sea_reference snapshot CSV adapter + happy fixture
d2c7f9b feat(zkpor): R5-1 — sea_reference customer profile (6 adapters, no snapshot)
e8eabed feat(zkpor): R5-0 — spot_simple host helpers (off-circuit emitter)
20a1571 docs(handoff+roadmap): close R4 — t1_simple_margin model audited + R5 next
f511dcb feat(zkpor): R4-2 — t1_simple_margin Setup smoke + ComputeFlatUint64Commitment fix
466ef55 feat(zkpor): R4-1 — t1_simple_margin circuit (BatchCreateUserCircuit + witness builder)
e4dc0cb feat(zkpor): R4-0 — t1_simple_margin spec package (model 2 entry)
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
4e85757 refactor(zkpor): UserConfig → t4_tiered_haircut_margin_3pool/host shared type (userproof prep)
8045c37 feat(zkpor): zkpor/cmd/prover — R3 step 4 core-path service + G1 hint closure
16f36bd feat(zkpor/store): proof model + witness state-machine methods (prover prep)
5332f40 feat(zkpor): zkpor/cmd/witness — R3 step 4 core-path service + G6 closure
78acd39 feat(zkpor/store): batch witness model + MySQL connection helper
32b9334 feat(zkpor): t4_tiered_haircut_margin_3pool host — AccountLeafHash + PaddingAccounts + Encode/DecodeBatchWitness
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
| `zkpor/core/host/*` | ✅ off-circuit (native) universal 헬퍼 — `VerifyMerkleProof` (Poseidon BN254 SMT, legacy parity). **R8-A: identity + insolvent registries** (78710d5) — `IdentitySchemePassthroughHexBN254ReducedV0` + `InsolventActionDropAndLogV0` self-register via init(). G17 closure. |
| `zkpor/core/tree/*` | ✅ bsmt depth-28 SMT wrapper + `EmptyAccountLeafHash` (Poseidon(0,0,0,0,0)). memory/redis 백엔드. **default=memory, redis=opt-in** (A0 결정: snapshot-SoT 와 정합). witness + userproof 공유 (commit c96018d) |
| `zkpor/core/snapshot/*` | ✅ **R9-A/B/C/D done** — standard raw snapshot schema metadata + CSV primitives + mapping DSL + 4 model standard canonical CSV connectors. `schema` 패키지 + 4 model schema (`t1_simple_margin`, `t2_static_haircut_margin`, `t3_tiered_haircut_margin_1pool`, `t4_tiered_haircut_margin_3pool`). `csv` 패키지는 canonical CSV header 검증, scalar type validation, primary-key duplicate detection, `ErrInvalidRow` 분류, context-aware streaming 제공. `mapping` 패키지는 raw CSV dialect, direct/wide_assets file mapping, source/constant/source_prefix column rules, type assertion, decimal_scale validation 제공. Standard connector IDs: `t1_standard_csv.v1`, `t2_standard_csv.v1`, `t3_standard_csv.v1`, `t4_standard_csv.v1`. |
| `zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host/*` | ✅ off-circuit model-specific 헬퍼 — `ComputeUserAssetsCommitment` + `ComputeCexAssetsCommitment(slice, capacity)` + `AccountLeafHash` + `PaddingAccounts` + `EncodeBatchWitness`/`DecodeBatchWitness` + 공유 `UserConfig` 타입. **R8-B/2: snapshot connector registry** (4369a91) — factory signature `(dir, id, capacity, PriceScaleProvider)` (pricing tail added R8-E). **R8-B/3: constraint module registry** (fc8325d) — empty ID returns universal noop. 모두 legacy byte-equivalence/round-trip 테스트 통과 |
| `zkpor/store/*` | ✅ gorm 영속화 계층 — `Open` + `ConvertMySQLErr` + 3 모델 (`BatchWitness` 78acd39, `Proof` + witness 상태머신 메서드 16f36bd, `UserProof` b7e57e6) + **`ProofStore.ListAllInOrder()` (A4, f1ba54a)** for verifier DB 직접 읽기 경로. 단일 instance DB-poll (`ClaimOldestByStatus` 트랜잭션) 채택 — Redis BLPOP 큐는 multi-worker scaling 시 follow-up. PostgreSQL adapter 는 slice D (deferred) |
| `zkpor/cmd/verifier/*` | ✅ R3 step 4 첫 service (legacy `src/verifier` 의 zkpor-native 대체, 3-mode CLI: batch / -user / -hash). DB 직접 읽기 모드 (A4). per-asset equity<debt warning (A5). **R8-D (950c728)**: profile.toml-driven — `-profile <toml>` + `-keys-dir <path>` (batch) + `-asset-capacity` override. config.json 슬림 (DB + DbSuffix + ProofTable + CexAssetsInfo). tiers/stems/capacity 는 declarative builder + resolveFromProfile 로 derive. |
| `zkpor/cmd/witness/*` | ✅ R3 step 4 service — snapshot → BatchCreateUserWitness → DB. **R8-C/2 (469c019)**: profile.toml-driven — `-profile` + `-user-data-dir` + `-snapshot-id` + `-asset-capacity` flags. G6 invariant assert는 declarative.BuildPricing 안. snapshot은 `t4host.NewSnapshot(connectorID, dir, id, cap, pricing)`. config.json 슬림 (DB + DbSuffix + TreeDB). 핵심 경로 only — multi-worker 병렬 / DB resume / tree rollback 은 follow-up. |
| `zkpor/cmd/prover/*` | ✅ R3 step 4 service — DB-poll Published → groth16.Prove+Verify → proof 테이블. G1 hint closure (`solver.RegisterHint`). **R8-C/3 (ad73d80)**: profile.toml-driven — `-profile` + `-keys-dir`. `buildResolved(prof, keysDir)` 가 (tiers, stems) 도출. config.json 은 DB-only. Redis BLPOP 큐 / -rerun 모드는 follow-up. |
| `zkpor/cmd/userproof/*` | ✅ R3 step 4 service — self-contained tree 재구축 → per-account inclusion proof → DB. **R8-D (950c728)**: profile.toml-driven (witness 와 같은 flag 세트). config.json 슬림 (DB + DbSuffix + TreeDB). 핵심 경로 only — multi-worker 병렬 / -memory_tree 플래그 / resume 은 follow-up. |
| `zkpor/cmd/keygen/*` | ✅ zkpor-native trusted setup. **R8-C/1 (f427b21)**: profile.toml-driven — `-profile <toml>` + `-asset-capacity <N>` override. model 분기 (T1/T4 newCircuit). 더 이상 binance.NewBatchShape 직접 호출 안 함. Tiny smoke (5,5,10): 286k constraints, ~21s, .pk 113MB. |
| `zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec/*` | ✅ complete — types, RiskPolicy, SnapshotSource (`InvalidCount()` 추가됨, R2/2 step 2), ConstraintModule, witness (BatchCreateUserWitness 등) |
| `zkpor/core/solvency/t4_tiered_haircut_margin_3pool/circuit/*` | ✅ complete — BatchCreateUserCircuit + helpers ported. `SetBatchCreateUserCircuitWitness` 는 `assetCountTiers` 를 인자로 받음. **Alpha wiring (R3 step 2)** + **R1CS byte-equivalence vs legacy (R3 step 3 / G1)**. **A5 fix d7c23f3** — `SetBatchCreateUserCircuitWitness` 의 padding UserAssetInfo entries 가 legacy 처럼 6개 collateral 필드를 명시적 0 으로 초기화 (이전엔 nil 이라 gnark `can't set fr.Element with <nil>` 실패). |
| `zkpor/core/solvency/t1_simple_margin/{spec,circuit,host}/*` | ✅ **R4 + R5-0 done** — spec/circuit (R4) + host helpers (R5-0): `ComputeUserAssetsCommitment` (2-field per asset) + `ComputeCexAssetsCommitment(slice, capacity)` (TotalEquity×2^64+BasePrice 1-field per asset) + `AccountLeafHash` (5-input zero-padded) + `PaddingAccounts` + `EncodeBatchWitness`/`DecodeBatchWitness` (capacity self-describing) + `UserConfig` (no debt/collateral fields). NbConstraints=33,306 at tiny shape. RiskPolicy 부재. |
| `zkpor/core/solvency/{t1_simple_margin,t2_static_haircut_margin,t3_tiered_haircut_margin_1pool}/` | ⏸ doc.go only — 카탈로그 reserved. R6 (3rd model) rule-of-three 대기. |
| `zkpor/profile/sea_reference/*` | ✅ **R8-E/F (83cbfbe)**: dead-code adapters 6개 + profile_test.go 제거. 남은 파일: `snapshot.go` (T1 spot ETL, `sea_csv.v1` 등록), `snapshot_test.go`, `snapshot_connector_test.go`, `helpers_test.go`, `sea_reference.toml`, `doc.go`, `testdata/happy`. SnapshotConfig 에 `Pricing` 필드 추가. |
| `zkpor/profile/declarative/*` | ✅ **R5-3 + R7-B + R8-B/1 (d9d7135)** — `profile.toml` schema (v1 FROZEN) + builders.go: `BuildIdentity` / `BuildInsolvent` (host registry lookup) + `BuildBatchShape` + `BuildBatchShapeProvider(model, shapes)` (model-typed wrapper, R8-C/2 추가) + `BuildPricing` (G6 invariant assert) + `BuildCatalog`. Validate 가 빈 identity.scheme / insolvent.action / snapshot.source_type 거부. 20+ tests. |
| `zkpor/profile/binance/*` | ✅ **R8-E (83cbfbe)**: dead-code adapters 7개 + identity_test/batch_shape_test 제거. 남은 파일: `snapshot.go` (T4 ETL, `binance_csv.v1` 등록), `snapshot_test.go`, `snapshot_connector_test.go`, `legacy_compare_test.go` (G1), `helpers_test.go`, `binance.toml`, `doc.go`, `testdata/`. SnapshotConfig 에 `Pricing` 필드 — nil 거부. `streamAccounts` / `readCexAssetsCSV` 가 외부에서 받은 PriceScaleProvider 사용. |
| `zkpor/deploy/` | ✅ **smoke MySQL fixture (A2, 1d5b2e9)** — `docker-compose.yml` 단일 컨테이너 (mysql:8.0, healthcheck, 영속 볼륨). Memory tree 라 Redis 컨테이너 불필요. 사용: `docker compose -f deploy/docker-compose.yml up -d` |
| `zkpor/scripts/` | ✅ **end-to-end smoke 하네스 (A5, d7c23f3)** — `smoke.sh` 가 docker compose → keygen (캐시) → witness → prover → verifier(batch) → userproof → verifier(-user) 순으로 전체 파이프라인 실행. R3 step 4 exit criteria 검증 완료. |
| `circuit/`, `src/` (legacy) | ✅ untouched, fully functional. trusted setup 그대로 유효 |
| docs (`zkpor/AGENTS.md`, `zkpor/CLAUDE.md`, `zkpor/PRODUCTION_ROADMAP.md`, `zkpor/docs/01-project-context.md`, `zkpor/docs/02-module-architecture.md`) | ✅ complete |

## Current Implementation Snapshot

최근 작업 흐름:

```text
<R9/D1>  feat(zkpor): R9-D/1 — T1/T4 standard CSV snapshot connectors
        (`core/snapshot/t1_simple_margin` + `.../t4...` parser.go:
         canonical accounts.csv/cex_assets.csv[/tier_ratios.csv] →
         model SnapshotSource. Capacity padding, BN254 account_id
         canonicalization, deterministic account grouping, tier-ratio
         padding, InvalidCount handling. Registered host connectors:
         t1_standard_csv.v1, t4_standard_csv.v1. T2/T3 follow same
         parser pattern in R9-D/2.)
<R9/D2>  feat(zkpor): R9-D/2 — T2/T3 standard CSV snapshot connectors
        (T2/T3 host snapshot registries + canonical parser.go.
         T2 static haircut computes TotalCollateral via
         collateral×base_price×haircut_bp/10000. T3 single-pool tier
         parser mirrors T4 tier padding with one collateral curve.)
<R9/C>   feat(zkpor): R9-C — snapshot mapping DSL + profile schema bump
        (`core/snapshot/mapping`: Format + File + Column DSL,
         direct/wide_assets modes, source/constant/source_prefix rules,
         decimal_scale/type validation, CSV option builder. profile.toml
         additive fields: [snapshot.format] + [[snapshot.files]].
         BuildSnapshotMapping validates mappings against the selected
         model's StandardSchema. Reference TOMLs add null_values only;
         profile adapters remain procedural until R9-D/E/F.)
<R9/B>   feat(zkpor): R9-B — standard snapshot CSV primitives
        (`core/snapshot/csv`: schema-bound header parser, dialect
         options, normalized Row, uint/bigint/account-id scalar
         validation, primary-key duplicate detection, context-aware
         Stream, and `ErrInvalidRow` classification for later
         drop_and_log.v0 policy routing. No profile rewrite yet.)
<R9/A>   feat(zkpor): R9-A — standard raw snapshot schema v1
        (core/snapshot/schema metadata package + 4 model-specific
         StandardSchema definitions. docs/04 §12 added as the human
         spec. Contract: standard raw rows are canonical after mapping
         (scaled integers, canonical account_id, optional dense
         account_index derivation, duplicate/omitted-zero policy,
         cex_assets capacity padding, tier monotonicity). R9-B/C/D
         will add CSV primitives, mapping DSL, and model parsers.)
<R5/4>   docs(zkpor): R5-4 — G12 closure (multi-customer .vk sharing policy)
        (commit 8fb5b3f. docs/02-module-architecture.md §6.1 신설.
         `.vk` artifact 는 (model, asset_capacity, batch_shape,
         constraint_module) tuple 단위 — customer profile blind.
         두 customer 가 동일 tuple 이면 같은 ceremony 의 .vk 가
         byte-equivalent. `StandardKeyName` 이 이미 customer-blind.
         asset_capacity 는 stem 에 없어 operator 가 디렉터리 컨벤션
         (`.artifacts/cap-<N>/`) 으로 일관성 보장. PRODUCTION_ROADMAP
         G12 row deferred→closed.)
<R5/3>   feat(zkpor): R5-3 — declarative profile.toml schema draft
        (commit 23566aa. profile/declarative/profile.go — schema
         struct + Load + Validate. profile/binance/binance.toml +
         profile/sea_reference/sea_reference.toml 두 instantiation.
         dep go-toml/v2 v2.3.1. 5 test 통과. service-startup 에서
         toml consume 하는 wiring 은 별도 슬라이스 (R7 freeze 전).
         parent repo go.mod commit 596dae0 (zkmerkle-proof-of-solvency)
         별도 — 두 repo 독립 운영.)
<R5/2>   feat(zkpor): R5-2 — sea_reference snapshot + happy fixture
        (commit f41d36a. profile/sea_reference/snapshot.go — spot
         단순 CSV ETL: `rn,id,<asset>,...,sum` user shard, `symbol,
         usdt_price,total_equity` cex_assets_info. binance/snapshot.go
         패턴 단순화. 5 test 신규 (happy + invalid hex + balance
         overflow + missing symbol + capacity pad).)
<R5/1>   feat(zkpor): R5-1 — sea_reference customer profile (6 adapters)
        (commit d2c7f9b. catalog + pricing (uniform default scales,
         no two-digit list) + identity (G2 same scheme as binance) +
         insolvent + batch_shape ({50,1000} default + same env
         override) + constraint_noop (spot-typed). doc.go 가 'rename
         path on real customer confirm' 명시. 9 test.)
<R5/0>   feat(zkpor): R5-0 — t1_simple_margin host helpers (off-circuit)
        (commit e8eabed. core/solvency/t1_simple_margin/host/{commitment,
         account,serialize}.go. t4_tiered_haircut_margin_3pool/host 패턴 모방, spot
         단순화: 2-field per asset user commit, 1-field per asset
         cex commit, 5-input leaf with nil zero positions (Poseidon
         Bytes converts nil → fr.Element{0}). 6 test.)
<R4/2>   feat(zkpor): R4-2 — t1_simple_margin Setup smoke + Commitment fix
        (commit f511dcb. setup_test.go (Compile+Setup at tiny shape
         5_10_2, NbConstraints=33,306, R1CS sha256 baseline 기록 +
         noop-module zero-cost regression guard). 부수로 core/circuit
         /commitment.go 의 잠재 버그 fix: ComputeFlatUint64Commitment
         가 flatten length % 3 != 0 시 trailing partial field 를
         `_ = last` 로 discard 해 tmp[nEles-1] 이 nil 인 채 Poseidon
         호출 → panic. t4_tiered_haircut_margin_3pool 의 6-field-per-asset 가 3 의
         배수라 안 걸렸음, t1_simple_margin 의 2-field-per-asset 첫 노출.
         t4_tiered_haircut_margin_3pool R1CS sha256 (678eb23f…) + coefficients sha256
         불변 확인 — 새 partial-field 분기가 t4_tiered_haircut_margin_3pool 에선 미진입.)
<R4/1>   feat(zkpor): R4-1 — t1_simple_margin circuit 본체
        (commit 466ef55. BatchCreateUserCircuit + Define + 
         SetBatchCreateUserCircuitWitness + paddingAsset 헬퍼.
         t4_tiered_haircut_margin_3pool circuit 구조 모방하되 simplify: no tier table /
         no haircut / no 3-bucket collateral, AssetsForUpdateCex 가
         1-field-per-slot (vs t4_tiered_haircut_margin_3pool 의 5). Random-linear-
         combination cross-check 도 1-power-per-slot. 5-input account
         leaf 로 substrate 호환. PowersOfSixteenBits 는 t4_tiered_haircut_margin_3pool
         에서 복제 (R6 promotion candidate).)
<R4/0>   feat(zkpor): R4-0 — t1_simple_margin spec 패키지 신설
        (commit e4dc0cb. types (1-tuple AccountAsset, no debt/
         collateral) + snapshot (SnapshotSource interface) + witness
         (BatchCreateUserWitness + helpers) + constraint (Constraint
         Module + slim Context). RiskPolicy 부재 — doc 의 "Notably
         absent (vs t4_tiered_haircut_margin_3pool)" 정합.)
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
         panic → warning (t4_tiered_haircut_margin_3pool 은 자산별 차용 허용).)
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
         t4_tiered_haircut_margin_3pool/host 공유 타입으로 이동 — userproof writer +
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
         import 0. cmd/verifier/config 가 t4_tiered_haircut_margin_3pool spec 타입.
         worker-pool small-input panic 가드. main_test.go —
         assetCountTiers {50,500} + decodeBatchMetadata. G2/G6 는
         call site 없어 witness/userproof 로 이연. proof-table
         end-to-end 는 witness+prover 후.)
<R3/4a>  feat(zkpor): extract off-circuit host helpers (R3 step 4 prep)
        (core/host/merkle.go — VerifyMerkleProof, universal,
         Poseidon BN254 SMT. t4_tiered_haircut_margin_3pool/host/commitment.go —
         ComputeUserAssetsCommitment + ComputeCexAssetsCommitment,
         trusted-setup byte packing. 4 테스트 legacy byte-equivalence
         통과. 발견: gnark-crypto bn254 poseidon Write 가
         ≥fr.Modulus() 입력을 silent drop — test fixture 가
         mod-safe 해야 함.)
<R3/3>   test(zkpor): legacy↔zkpor R1CS + AccountID byte-equivalence
        (G1 closure. t4_tiered_haircut_margin_3pool/circuit/legacy_compare_test.go —
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
<R3/0>   test(circuit): add t4_tiered_haircut_margin_3pool Compile+Setup smoke
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
         core/solvency catalog with t4_tiered_haircut_margin_3pool spec+circuit ported,
         profile/binance adapter set)
```

구현된 것:

- 모든 universal 인터페이스 (`zkpor/core/spec/`).
- universal zk 헬퍼 (`zkpor/core/circuit/`) — legacy `circuit/utils.go` 에서
  Merkle/commitment/arithmetic 부분만 추출.
- t4_tiered_haircut_margin_3pool model spec (`zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec/`).
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

- **R4 — second model 회로 `t1_simple_margin`** (SEA GTM driver, model-first
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
  강제했으나 t4_tiered_haircut_margin_3pool 모델은 자산별 차용을 허용 (account-level
  collateral≥debt 만 불변식). 자산별 위반 시 warning log 만 출력
  하도록 강등.

- **Store driver 인터페이스 + PG adapter (slice D, deferred)** — store
  층의 `gorm.io/driver/mysql` 직접 의존 + MySQL 전용 hints + error
  number 매핑을 driver-aware 추상화로 분리하면 PostgreSQL adapter
  가 ~1 commit. Smoke 와 무관하게 진행 가능 — DEFERRED 작업 표.

- **R4 substrate audit (R4-3 done)** — core/circuit substrate 가
  t1_simple_margin 을 신규 helper 없이 수용함 확인. 두 model 양쪽이 쓰는
  universal symbols 7개 (`API`, `Variable`, `BatchCommitment`,
  `ComputeFlatUint64Commitment`, `AccountIndexToMerkleHelper`,
  `VerifyMerkleProof`, `UpdateMerkleProof`, `TwoToTheSixtyFour`).
  tier-only 3개 (`CheckedDivByConstant`, `IntegerDivision`,
  `TwoToTheOneTwentyEight`) 는 spot 미사용일 뿐 model-bound 가 아니라
  잠재 universal (3rd model 이 division 등을 쓰면 자연 활용).
  Substrate 가 sound — 추가 promotion 불필요.

- **R6 promotion candidates (rule-of-three first event)** — 두 model
  사이에 중복된 항목들 (R6 G11 trigger 시 promote):
  · `PowersOfSixteenBits` 가 t4_tiered_haircut_margin_3pool/circuit/constants.go +
    t1_simple_margin/circuit/constants.go 양쪽에 동일 정의로 존재. 의미는
    universal (2^16 powers — asset id packing). R6 에서 core/circuit
    로 옮긴다.
  · R1CS hash test helpers (`bn254R1Cs`, `hashR1Cs`, `hashCoefficients`,
    `writeLinearExpr`, `writeUint64`) 가 t4_tiered_haircut_margin_3pool legacy_compare
    _test.go + t1_simple_margin setup_test.go 양쪽에 중복. R6 에서 test
    helper 패키지 또는 core/circuit/testhelper 로 promote.

- **noop ConstraintModule promotion 보류 (R4-4 reassessed)** — 두 model
  의 `ConstraintContext` 가 다른 field 셋을 가지므로 (t4_tiered_haircut_margin_3pool 은
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
- **거래소 이름을 model id에 박지 않는다** — `t4_tiered_haircut_margin_3pool` ≠ `binance_v2`.

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
    │       ├── t1_simple_margin/doc.go            (★ R4 model-first priority — SEA GTM driver)
    │       ├── t1_simple_margin/doc.go
    │       ├── t2_static_haircut_margin/doc.go
    │       ├── t3_tiered_haircut_margin_1pool/doc.go
    │       └── t4_tiered_haircut_margin_3pool/                 (★ 유일 spec+circuit+host 구현)
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
| off-circuit host 헬퍼 추출 (Merkle verify + commitment) | ✅ done — `core/host` + `t4_tiered_haircut_margin_3pool/host`, legacy byte-equivalence | R3 step 4 prep |
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
| Second model 회로 구현 — `t1_simple_margin` (SEA GTM driver, model-first) | ✅ done — spec + circuit + setup smoke (NbConstraints=33,306 baseline). Substrate audit 통과 (R4-3) | **R4 종결** |
| Second customer profile — SEA reference (Indonesia/Thailand 우선) | ✅ done (sea_reference, hypothetical) — host helpers (R5-0) + 7 어댑터 (R5-1) + snapshot CSV ETL + fixture (R5-2). 실제 customer 결정 시 rename | **R5 종결** |
| Declarative `profile.toml` 첫 추출 | ✅ done — `profile/declarative/profile.go` schema + Load + Validate, binance/binance.toml + sea_reference/sea_reference.toml (R5-3) | R5 |
| G12 multi-customer `.vk` 공유/분리 정책 | ✅ closed — (model, asset_capacity, batch_shape, module) tuple 단위, customer-blind. docs/02-module-architecture.md §6.1 (R5-4) | R5 |
| sea_reference end-to-end smoke (witness→prover→verifier 풀 파이프라인) | pending — R3 step 4 의 binance smoke 패턴 reuse, customer 별 sample data 차이만 wire-up | R5 follow-up |
| service startup 이 profile.toml 을 직접 consume 하도록 wiring | pending — 각 adapter 의 constructor 가 toml 값을 인자로 받게 refactor 필요 | R7 freeze 직전 candidate |
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

**R3 step 4 + R4 + R5 + R6 + R7 + R8 종결**. V1 catalog freeze + profile wiring 완료.

R8 산출물 (commit chain 78710d5 → 83cbfbe, 11 slices):

- **R8-A** (78710d5) — `core/host` 안 identity + insolvent 두 registry.
  G17 lock 의 (a)/(b)/(c) 적용: in-process build-time, `<id>.v<version>`
  ID format, missing/dup → panic. 첫 entry `passthrough_hex_bn254_reduced.v0`
  + `drop_and_log.v0` self-register.
- **R8-B/1** (d9d7135) — `profile/declarative/builders.go` 의 model-blind 절반:
  `BuildIdentity`, `BuildInsolvent`, `BuildBatchShape`, `BuildPricing` (G6
  invariant assert 내장), `BuildCatalog`. universal pricing/catalog 구현이
  declarative 에 inline.
- **R8-B/2** (4369a91) — `core/solvency/<model>/host` 안 snapshot connector
  registry (T1 + T4 각각). factory signature `(dir, id, capacity)` —
  R8-E 에서 `PriceScaleProvider` tail 추가. binance_csv.v1 + sea_csv.v1
  self-register.
- **R8-B/3** (fc8325d) — constraint module registry per model. 빈 ID 는
  universal noop fast-path. v1 catalog 에 non-noop entry 0개 (R7-C 정합).
- **R8-C/1..3** (f427b21 → 469c019 → ad73d80) — keygen + witness + prover
  service-startup 가 `declarative.Load(profile) → builders` 로 wiring.
  config.json 슬림화: DB DSN + (TreeDB / per-snapshot) 만 남음.
- **R8-D** (950c728) — verifier + userproof + smoke.sh 풀 통합. verifier 의
  ZkKeyName/AssetsCountTiers/AssetCapacity 가 `-keys-dir` + profile.toml 로
  derive.
- **R8-E/F** (83cbfbe) — profile/binance + profile/sea_reference 의 dead
  adapter 15개 제거. snapshot factory signature 가 PriceScaleProvider 받도록
  확장; snapshot ETL 이 그 provider 사용. 두 profile 에 `helpers_test.go`
  (declarative-built testPricing) 추가.
- **R8-close** — G17 deferred→closed, docs/02 §6.2 (registry pattern)
  신설, ROADMAP §R8 종결 마킹, HANDOFF.md 갱신 (이 슬라이스).

**G17 closed**: 네 가지 adapter category (identity, insolvent, snapshot
connector, constraint module) 의 registry 패턴이 v1 frozen. 새 customer
통합 비용 = `<customer>.toml` 작성 + (필요 시) snapshot ETL 코드만.


R3 step 4 산출물: 4 services + 3 gate (G1/G2/G6) + AssetCounts
재배치 (E) + end-to-end smoke (A5) — `scripts/smoke.sh` 풀 파이프
라인 통과 (commit chain a6469c6 … 11f2d0a).

R4 산출물: t1_simple_margin model (spec + circuit + setup smoke) — 두번째
model 이 substrate 위에 안정 안착, NbConstraints=33,306 (tiny shape).
core/circuit 의 universal helpers 가 신규 추가 없이 수용. 부수로
`ComputeFlatUint64Commitment` 잠재 버그 fix (commit chain f511dcb →
466ef55 → e4dc0cb).

R5 산출물: host helpers (R5-0) + sea_reference 7 어댑터 (R5-1) + snapshot
CSV ETL (R5-2) + declarative profile.toml (R5-3) + G12 closure (R5-4).
두 customer 가 같은 toml schema + 같은 universal identity scheme 위에서
표현됨. `.vk` 공유 정책 결정 (tuple-based, customer-blind). commit chain
8fb5b3f → 23566aa → f41d36a → d2c7f9b → e8eabed.

R6 산출물 (이번 회): **카탈로그 5→4 통합** (`spot_simple` + `merkle_classic`
→ `t1_simple_margin` superset, debt=0 spot 흡수) + **Tn naming v1**
(`t1_simple_margin` / `t2_static_haircut_margin` /
`t3_tiered_haircut_margin_1pool` / `t4_tiered_haircut_margin_3pool`) +
**4 model 통일 5-input Poseidon AccountLeaf signature** +
**`docs/04-solvency-models.md`** (industry reference + 일반화 트레일) +
**G11 closure** (첫 universal helper `core/host.AccountLeafHash`
promotion). 5-tier marketing 은 `ModelDisplay` map 으로 유지. commit chain
b0318e1 → 829e81c → 722a133 → 44a47d9.

R6 follow-up — **T2 + T3 구현** (T4 → T3 → T2 단순화 chain):

- **T3** (`t3_tiered_haircut_margin_1pool`) — T4 의 3-bucket collateral
  loop 를 single pool 로 collapse. per-asset 4-tuple (Index, Equity, Debt,
  Collateral). 같은 tier-curve evaluation. RLC 3 powers/slot.
- **T2** (`t2_static_haircut_margin`) — T3 의 tier 곡선을 single Haircut
  basis-points 상수로 collapse. per-asset 1 multiply + 1 division.
  assetHaircutTable lookup. 회로 가장 가벼움 (T1 < T2 < T3 < T4).

**4 model 모두 구현 완료**. G4 (catalog stability) 의 회로 측 prereq 충족.

R7 산출물 (catalog v1 FROZEN, commit chain 9388694 → 17429e4 → 08cce42):

- **R7-A** (9388694) — `core/spec/solvency_models.go` v1 FROZEN 헤더 +
  CatalogedModels add-only. `BatchShape.LegacyKeyName()` 즉시 제거 +
  cmd/keygen `-legacy-names` 플래그 제거 (G10 closed).
- **R7-B** (17429e4) — `profile/declarative/profile.go` v1 FROZEN docstring.
  schema 변경 규약 (additive = minor bump, removal/rename disallowed).
  docs/02 §6.0 신설. service-startup wiring 은 R7+1 carry.
- **R7-C** (08cce42) — `core/constraint_modules/` 디렉터리 + governance
  doc.go. ID prefix 규약 (regulator/business/customer-local) + rule-of-
  three promotion gate + noop v2 carry. v1 entry 0개 (intentional).

**G4 closed**, **G10 closed**. v1 catalog freeze 완전 충족.

**R6.5 — bw6 env fix + setup smoke baseline 기록** (closed):

원인 분석: fork 의 zip 에는 bw6-633/761 + poseidon 패키지 모두 존재했으나
Go module *build cache* 의 stale package list 가 stale extraction 을 가리킴.
또 cache extraction 일부 sub-package 가 외부 trigger 로 invalidated.

Fix: `chmod -R u+w` 후 `rm -rf` extracted dirs + `rm` cache zip metadata
→ `go clean -cache` → `go build` 으로 fresh download/extract. 즉시 통과.

검증 footprint (4 model setup smoke + Setup 모두 통과, tiny shape 5_10_2):

| Model | NbConstraints | Setup time |
|---|---:|---:|
| T1 | 38,149 | 2.79s |
| T2 | 48,886 | 6.11s |
| T3 | 274,650 | 22.90s |
| T4 | 723,790 | 58.15s |

T1 R1CS sha256: `d2df98c8969280900ac36424358454af5a223b331839b9f2080cbc548aebe0b0`.
4 model 모두 noop-module zero-cost 검증 통과. 전체 `go test -short` 통과.

**정정**: R6/prep ~ T2/T3 close 의 commit message 의 "build clean" claim 은
부분적으로 false positive 였음 (cache stale 로 fail 가능). R6.5 fix 후
*실제로 통과 확인됨* — 코드 자체는 일관성 있게 작성됐었음.

남은 R7 entry 항목: profile descriptor schema freeze + module 카탈로그
freeze 만.

R6 후 carry (3rd model 또는 후속 promote):

- `PowersOfSixteenBits` (양 model `circuit/constants.go`)
- R1CS hash test helpers (양 model `setup_test.go`)
- ~~`parseShapeOverride` (양 profile `batch_shape.go`)~~ — **R8-B/1 에서
  `profile/declarative/builders.go` 로 promote 완료**.
- `convertFloatStrToUint64` + `errInvalidRow`/`invalidf` 패턴 (양
  profile `snapshot.go`) — 여전히 profile-local. snapshot 자체가
  customer-specific 이라 큰 부담 아님.
- ~~Identity DeriveAccountID 64-hex → fr.Element body~~ — **R8-A 에서
  `core/host/identity_passthrough.go` 로 promote 완료**.

R6 후 surface 된 환경 이슈 (별 슬라이스 후보):

- **bw6 환경 fix** — `bnb-chain/gnark-crypto` fork 의 bw6-633/bw6-761
  패키지 부재로 `gnark/backend/groth16` transitive import 가 fail.
  R6 의 build/vet 는 통과하나 `go test` 가 일부 패키지 (circuit /
  profile/binance / host) 의 setup_test, host_test, legacy_compare_test
  실행 불가. fix 안: gnark / gnark-crypto fork 버전 매칭, 또는
  backend/groth16/bn254 명시 import, 또는 module cache 의 bw6 패키지
  복원. 별 슬라이스 (R6.5 env fix) 로 처리.

다음 슬라이스 갈래 (post-R8) — agent 가 선택 (또는 user 와 합의):

**갈래 R9 — Customer raw data standardization (RECOMMENDED, V1-PROD 직전 자연 다음)**:

```text
R8 종결 시점 (현재) 의 customer-specific 코드 = profile/<customer>/
snapshot.go (binance ~31k LoC, sea_reference ~16k LoC). 거래소마다
raw data (CSV/DB/JSONL) 포맷이 달라 어쩔 수 없는 형태였음.

R9 의 목표는 그 customer-specific 부분도 *모델별 표준 raw schema +
mapping config* 로 축소. snapshot.go 가 thin (~10-30 LoC) 으로 수렴.
Mapping 으로 표현 불가능한 transform 은 thin adapter 코드 (escape
hatch) 로 흡수.

자세한 내용 PRODUCTION_ROADMAP §R9. 작업 분해 ~6-8 슬라이스:
  R9-A   ✅ 모델별 표준 schema 정의 (4 model) + docs/04 §12
  R9-B   ✅ Core CSV parser primitives
  R9-C   ✅ Mapping config DSL (toml [snapshot.format] + [[snapshot.files]])
  R9-D   ✅ 4 model standard CSV connector + registry 적응
  R9-E   profile/binance snapshot.go thin rewrite
  R9-F   profile/sea_reference snapshot.go thin rewrite
  R9-close  G18 closure + handoff/roadmap

R9 종료 후 customer onboarding 비용:
  toml (mapping 포함) + (필요 시) thin adapter ~10-30 LoC
```

**갈래 V1-PROD — Production deployment (R9 후 진짜 최적)**:

```text
R8 종결 시점에도 V1-PROD 진행 가능 — customer adapter 가 thick 인 상태
(현재 binance / sea_reference snapshot.go 패턴). 그러나 R9 종료 후가
*진짜 최적* — raw data adapter 비용 최소화.

V1-PROD 의 핵심 작업:
  - 첫 real customer 통합 (SEA reference 또는 다른)
  - production .pk 마이그레이션 (legacy stem → StandardKeyName, byte
    불변 단순 rename)
  - production keygen (capacity=500, shape={50_700, 500_92})
  - scripts/smoke.sh 풀 파이프라인 production 환경 검증
  - 권장 사양: m6i.{2,4}xlarge

R9 와 V1-PROD 의 우선순위:
  - 첫 customer signal 이 *지금 들어옴* → V1-PROD 먼저, R9 carry
  - 첫 customer signal 없음 + engine 성숙도 우선 → R9 먼저, V1-PROD 후
```

**갈래 R5-FU — sea_reference end-to-end smoke**:

```text
scripts/smoke.sh 를 sea_reference 변형. 차이:
  - profile/binance → profile/sea_reference 로 import 교체 (또는
    smoke 가 profile.toml 선택 가능하게 generic 화 — R8 후 자연 달성)
  - sample data: src/sampledata/ 대신 profile/sea_reference/testdata/
    happy/ 사용 (또는 별도 sea sample 작성)
  - keygen 의 모델/capacity 인자 t1_simple_margin, capacity=10 정도

R5 의 "고객사 sample data 로 end-to-end PoR 통과" exit criterion 의
명시적 마감. T1 의 production-grade 검증. R8 와 독립 가능.
```

**갈래 R5-FU — sea_reference end-to-end smoke**:

```text
scripts/smoke.sh 를 sea_reference 변형. 차이:
  - profile/binance → profile/sea_reference 로 import 교체 (또는
    smoke 가 profile.toml 선택 가능하게 generic 화)
  - sample data: src/sampledata/ 대신 profile/sea_reference/testdata/
    happy/ 사용 (또는 별도 sea sample 작성)
  - keygen 의 모델/capacity 인자 t1_simple_margin, capacity=10 정도

R5 의 "고객사 sample data 로 end-to-end PoR 통과" exit criterion 의
명시적 마감. R4-R5 종결 후 R6 진입 전 자연 연결고리.
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
  parallelism, host helpers for t1_simple_margin) 는 production scale
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
# 예: sha256sum legacy/zkpor50_700.pk new/zkpor.t4_tiered_haircut_margin_3pool.50_700.pk
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
