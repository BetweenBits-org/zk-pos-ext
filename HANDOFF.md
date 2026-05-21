# HANDOFF.md

이 문서는 agent 세션이 바뀌어도 작업을 이어가기 위한 **현재 시점의 인수인계**다.
긴 히스토리 로그가 아니다. 작업 시작 전 source priority 상위 문서를 먼저 읽는다.

## Current State

Latest implementation commit:

```text
de794a4 add IDR tokens with two digits (#97)
```

(현재 zkpor/ 전체는 uncommitted local changes. 첫 commit이 아직 없음.)

| 영역 | 상태 |
|---|---|
| `core/spec/*` | ✅ complete — 8 인터페이스/상수 파일 |
| `core/circuit/*` | ✅ complete — universal 헬퍼 4 파일 (Merkle, commitment, arith) |
| `core/solvency/tier_3bucket/spec/*` | ✅ complete — types, RiskPolicy, SnapshotSource, ConstraintModule, witness (BatchCreateUserWitness 등) |
| `core/solvency/tier_3bucket/circuit/*` | ✅ complete — BatchCreateUserCircuit + helpers ported. `SetBatchCreateUserCircuitWitness` 는 `assetCountTiers` 를 인자로 받음 (global 의존 제거). `.pk`/`.vk` byte-equivalence 런타임 검증 pending (R3 와 함께) |
| `core/solvency/{spot_simple,merkle_classic,over_collateral_simple,tier_1bucket}/` | ⏸ doc.go only — 카탈로그 reserved, rule-of-three 대기 |
| `profile/binance/*` | ⚠ stubs — 8개 어댑터 constructor 존재. `snapshot.go`는 `errStubSnapshot` 반환 (R2에서 CSV loader 흡수) |
| `../circuit/`, `../src/` (legacy) | ✅ untouched, fully functional. trusted setup 그대로 유효 |
| docs (`AGENTS.md`, `CLAUDE.md`, `PRODUCTION_ROADMAP.md`, `docs/01-project-context.md`) | ✅ complete |

## Current Implementation Snapshot

최근 작업 흐름 (uncommitted):

```text
methodology 적용 — AGENTS.md, project-context, production-roadmap, HANDOFF 정착
catalog 정리 — binance_v2 → tier_3bucket 리네임 (model = math, profile = deployment)
어댑터 통합 — profile/binance/ 단일 패키지로 모음
core/spec/ + core/circuit/ + profile/binance/ 스켈레톤 구축
```

구현된 것:

- 모든 universal 인터페이스 (`core/spec/`).
- universal zk 헬퍼 (`core/circuit/`) — legacy `../circuit/utils.go` 에서
  Merkle/commitment/arithmetic 부분만 추출.
- tier_3bucket model spec (`core/solvency/tier_3bucket/spec/`).
- Binance 어댑터 8개 (constructor 형태) — 단일 Go 패키지.
- 5-tier 카탈로그 (`core/spec/solvency_models.go`).
- 명명 규약: SolvencyModelID, BatchShape, key file naming, ConstraintModuleID.

아직 의도적으로 닫지 않은 것:

- CSV ETL 실제 로직 — R2 (현재 `errStubSnapshot`).
- 4개 service main.go 의 wiring — R3.
- `.pk`/`.vk` byte-equivalence 런타임 검증 — R3 와 함께 (G1 closing).
- 나머지 4개 model 회로 — R4+ (시장 신호 대기).

## Non-Negotiable Rules

작업 내내 어기면 안 되는 규칙.

- **frozen 계약 경계 우선** — `core/spec/` 인터페이스 시그니처, 카탈로그
  상수, key file naming 은 versioned change 만.
- **legacy 코드 직접 수정 금지** — `../circuit/`, `../src/` 는 reference.
- **sum equality 는 모든 model에서 mandatory** — base PoR claim.
- **ConstraintModule 은 add only** — base-circuit 제약 weaken/remove 금지.
- **`PriceMultiplier × BalanceMultiplier == ValueScale` 불변식** — startup
  assert (R3 G6 closed 시점).
- **미결정·spec 공백·계약 불일치는 debate/question으로 surface** — agent
  임의 결정 금지.
- **검증 명령 실제 실행 없이 완료 선언 금지** — go build / go vet 통과 필수.
- **거래소 이름을 model id에 박지 않는다** — `tier_3bucket` ≠ `binance_v2`.

## Source Priority

문서 충돌 시 우선순위. 자세한 내용은 `PRODUCTION_ROADMAP.md` 참조.

1. `core/spec/` 코드 (frozen 계약)
2. `docs/01-project-context.md`
3. `PRODUCTION_ROADMAP.md`
4. `AGENTS.md`, `CLAUDE.md`
5. `HANDOFF.md` (이 문서 — 휘발성)
6. `../docs/*.md` (legacy 참고 자료)

## Repository Map

```text
zkpor/
├── AGENTS.md                              ← agent contract (가장 먼저 읽음)
├── CLAUDE.md                              ← Claude 자동 로드 메모리
├── HANDOFF.md                             ← 이 문서 (현재 시점 인수인계)
├── PRODUCTION_ROADMAP.md                  ← Part 3 (stages + gates) — root 격상
├── docs/
│   └── 01-project-context.md              ← Part 1 (컨셉/scope/guarantee)
├── core/
│   ├── spec/                              universal 인터페이스 + 상수 + 카탈로그
│   │   ├── batch_shape.go
│   │   ├── catalog.go                     (AssetCatalog interface)
│   │   ├── constants.go
│   │   ├── constraint_id.go
│   │   ├── identity.go
│   │   ├── insolvent.go
│   │   ├── price.go
│   │   └── solvency_models.go             (5-tier 카탈로그)
│   ├── circuit/                           universal zk 헬퍼
│   │   ├── arithmetic.go
│   │   ├── commitment.go
│   │   ├── constants.go
│   │   └── merkle.go
│   └── solvency/                          audited math 카탈로그
│       ├── spot_simple/doc.go
│       ├── merkle_classic/doc.go
│       ├── over_collateral_simple/doc.go
│       ├── tier_1bucket/doc.go
│       └── tier_3bucket/                  (★ 유일 spec 구현)
│           ├── doc.go
│           └── spec/                      (types, risk, snapshot, constraint)
└── profile/
    └── binance/                           (★ 유일 customer profile)
        ├── doc.go
        ├── batch_shape.go
        ├── catalog.go
        ├── constraint_noop.go
        ├── identity.go
        ├── insolvent.go
        ├── pricing.go
        ├── risk.go
        └── snapshot.go                    (★ errStubSnapshot — R2 작업)

../circuit/, ../src/   (legacy Binance OSS PoR v2 — 수정 금지)
../docs/               (legacy historical notes)
```

## Deferred Work

| Item | 상태 | track 위치 |
|---|---|---|
| CSV ETL absorb | pending | R2 / G5 |
| 4개 service rewiring + `.pk`/`.vk` byte-equivalence 검증 | pending | R3 / G1 + G2 + G6 |
| AccountIDProvider derivation 정식화 | deferred | R3 / G2 |
| 두 번째 customer profile | awaits signal | R4 / G12 |
| 두 번째 model 회로 구현 | awaits signal | R5 |
| core/circuit/ 추가 헬퍼 승격 | awaits signal | R6 / G11 |
| 카탈로그 v1 freeze | awaits R7 | R7 / G4 |

## Resume Actions

다음 agent는 아래 순서로 이어간다.

1. `AGENTS.md`, `docs/01-project-context.md`, `PRODUCTION_ROADMAP.md` 읽기.
2. `git status` 로 워크트리 상태 확인 (현재 `zkpor/` 전체가 untracked).
3. baseline 검증 명령 실행 (Required Commands 참고).
4. 다음 슬라이스 진입.

권장 다음 슬라이스:

```text
R2 진입 — CSV ETL absorb 1단계.
legacy ../src/utils/utils.go 의 ParseAssetIndexFromUserFile 와
ParseCexAssetInfoFromFile 를 profile/binance/snapshot.go 의
CexAssets(ctx) 구현으로 흡수한다 (AssetCatalog + RiskPolicy 도 같은
CSV 출처에서 일관 구축).
```

목표 / 범위 제외:

- 이 슬라이스: cex_assets_info.csv 로딩 — CexAssets() stub 제거, AssetCatalog
  symbol 리스트 추출, RiskPolicy tier ratio 값 채움.
- 같은 커밋에 넣지 않을 것: 사용자 CSV 스트리밍 (`AccountStream`),
  invalid-account 처리 wiring, 서비스 main.go 변경.

## Required Commands

Start of work:

```bash
git status
git log --oneline -10
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
