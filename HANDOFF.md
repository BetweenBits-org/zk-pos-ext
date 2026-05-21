# HANDOFF.md

이 문서는 agent 세션이 바뀌어도 작업을 이어가기 위한 **현재 시점의 인수인계**다.
긴 히스토리 로그가 아니다. 작업 시작 전 source priority 상위 문서를 먼저 읽는다.

## Current State

Latest implementation commit (`zkpor/.git/`, branch `main`):

```text
<R2/1 commit>  feat(profile): absorb cex_assets_info.csv loader into binance snapshot
e2e0f1c       docs: shift paths to root-cwd perspective + refresh HANDOFF after init
8aaf4c3       feat: scaffold zkpor engine — productization of Binance OSS PoR v2
```

`zkpor/` 는 자체 git 저장소 (`zkpor/.git/`). parent (`zkmerkle-proof-of-solvency`)
저장소는 `zkpor/` 를 untracked dir로 봄 — 두 저장소가 독립 운영된다.

| 영역 | 상태 |
|---|---|
| `zkpor/core/spec/*` | ✅ complete — 8 인터페이스/상수 파일 |
| `zkpor/core/circuit/*` | ✅ complete — universal 헬퍼 4 파일 (Merkle, commitment, arith) |
| `zkpor/core/solvency/tier_3bucket/spec/*` | ✅ complete — types, RiskPolicy, SnapshotSource, ConstraintModule, witness (BatchCreateUserWitness 등) |
| `zkpor/core/solvency/tier_3bucket/circuit/*` | ✅ complete — BatchCreateUserCircuit + helpers ported. `SetBatchCreateUserCircuitWitness` 는 `assetCountTiers` 를 인자로 받음 (global 의존 제거). `.pk`/`.vk` byte-equivalence 런타임 검증 pending (R3 와 함께) |
| `zkpor/core/solvency/{spot_simple,merkle_classic,over_collateral_simple,tier_1bucket}/` | ⏸ doc.go only — 카탈로그 reserved, rule-of-three 대기 |
| `zkpor/profile/binance/*` | ⚠ partial — `snapshot.go`의 `CexAssets()` 구현 완료 (cex_assets_info.csv + user CSV header 흡수, 캐싱). `AccountStream()`은 여전히 stub (`errAccountStreamPending`) — R2 step 2에서 사용자 shard 스트리밍 흡수. 나머지 7개 어댑터는 constructor 형태 |
| `circuit/`, `src/` (legacy) | ✅ untouched, fully functional. trusted setup 그대로 유효 |
| docs (`zkpor/AGENTS.md`, `zkpor/CLAUDE.md`, `zkpor/PRODUCTION_ROADMAP.md`, `zkpor/docs/01-project-context.md`) | ✅ complete |

## Current Implementation Snapshot

최근 작업 흐름:

```text
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

- 사용자 shard CSV 스트리밍 — R2 step 2 (`AccountStream` 현재
  `errAccountStreamPending` 반환).
- `CexAssets()` happy-path/tamper 픽스처 테스트 — R2/1 후속 commit (
  docs/scaffold → impl → tests 분리 원칙).
- 4개 service main.go 의 wiring — R3.
- `.pk`/`.vk` byte-equivalence 런타임 검증 — R3 와 함께 (G1 closing).
- 나머지 4개 model 회로 — R4+ (시장 신호 대기).

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
3. `zkpor/PRODUCTION_ROADMAP.md`
4. `zkpor/AGENTS.md`, `zkpor/CLAUDE.md`
5. `zkpor/HANDOFF.md` (이 문서 — 휘발성)
6. `docs/*.md` (legacy 참고 자료)

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
    │   └── 01-project-context.md             ← Part 1 (컨셉/scope/guarantee)
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
            └── snapshot.go                   (★ errStubSnapshot — R2 작업)
```

## Deferred Work

| Item | 상태 | track 위치 |
|---|---|---|
| CSV ETL absorb — CexAssets 부분 | ✅ done | R2 / G5 (step 1) |
| CSV ETL absorb — AccountStream + invalid-account 처리 | pending | R2 / G5 (step 2) |
| `CexAssets()` 픽스처 테스트 (happy + tamper) | pending | R2 / G5 (step 1 follow-up) |
| 4개 service rewiring + `.pk`/`.vk` byte-equivalence 검증 | pending | R3 / G1 + G2 + G6 |
| AccountIDProvider derivation 정식화 | deferred | R3 / G2 |
| 두 번째 customer profile | awaits signal | R4 / G12 |
| 두 번째 model 회로 구현 | awaits signal | R5 |
| core/circuit/ 추가 헬퍼 승격 | awaits signal | R6 / G11 |
| 카탈로그 v1 freeze | awaits R7 | R7 / G4 |

## Resume Actions

다음 agent는 아래 순서로 이어간다.

1. `zkpor/AGENTS.md`, `zkpor/docs/01-project-context.md`,
   `zkpor/PRODUCTION_ROADMAP.md` 읽기.
2. `git -C zkpor status` + `git -C zkpor log --oneline -10` 으로
   zkpor 저장소 상태 확인.
3. baseline 검증 명령 실행 (Required Commands 참고).
4. 다음 슬라이스 진입.

권장 다음 슬라이스:

```text
R2/1 후속 — CexAssets() 픽스처 테스트.
zkpor/profile/binance/testdata/ 아래 작은 cex_assets_info.csv +
가짜 user CSV 헤더 fixture 를 두고 happy-path 와 tamper 케이스
(missing symbol, malformed header, non-monotonic boundary,
boundary overflow, two-digit multiplier) 를 single _test.go 로
검증한다.

그 다음 R2/2 — AccountStream 흡수.
legacy ReadUserDataFromCsvFile 의 핵심 의미(equity/debt/loan/margin/
PM 파싱, ValueScale 적용, invalid-account 분류, AccountInfo 채널화)를
csvSnapshot.AccountStream(ctx) 으로 흡수. workers/GC 디테일은
production wiring 단계(R3)에서 결정.
```

목표 / 범위 제외:

- 다음 슬라이스 (R2/1 후속): `CexAssets()` 단위 테스트만 — implementation
  파일은 손대지 않음.
- 같은 commit 에 넣지 않을 것: 사용자 CSV 스트리밍, invalid-account
  분류 로직, 서비스 main.go 변경.

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
