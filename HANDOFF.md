# HANDOFF.md

이 문서는 agent 세션이 바뀌어도 작업을 이어가기 위한 **현재 시점의 인수인계**다.
긴 히스토리 로그가 아니다. 작업 시작 전 source priority 상위 문서를 먼저 읽는다.

## Current State

Origin: `https://github.com/BetweenBits-org/zk-pos-ext.git` (zkpor/.git/).
브랜치 layout: `main` 은 stable, `development` 워크트리 (`.worktree/development/`)
에서 in-progress 슬라이스 진행. main 으로 fast-forward merge 후 push.

Latest commits (branch `development`; R12-C closure ff-merged into main):

```text
docs(zkpor): HANDOFF — R12-C closed, R12-D as next slice (this commit)
c7c28cd refactor(zkpor): verifier+keygen — Run*(ctx, ...) (R12-C/3 closure)
e9d3edb refactor(zkpor): witness+userproof — Run(ctx, opts) (R12-C/2)
7f070ff refactor(zkpor): prover — Run(ctx, opts) graceful shutdown (R12-C/1)
6537f5f docs(zkpor): HANDOFF — R12-B closed, R12-C as next slice
ec9dfa9 refactor(zkpor): userproof+witness+prover — panic → error (R12-B closure)
[... 그 위 R12-B/1·2 + R12-A 5개 pkg/* 추출 + R11-D Phase 1/2/2b]
```

**Phase progression**:

| 단계 | 상태 | 핵심 결과 |
|---|---|---|
| R3-R10 (services + profile + standard CSV) | ✅ closed | docs/PRODUCTION_ROADMAP.md §R3-R10 참조 |
| Phase 3a/b/c (keygen + witness + prover 4-model dispatch) | ✅ closed | core/solvency/{t1,t2,t3,t4}/host/*_runner.go |
| Phase 3d/3e (verifier + userproof 4-model dispatch) | ✅ closed | 4 model E2E smoke 통과 |
| Profile rename (sea→t1, binance→t4) | ✅ closed | go module path `github.com/binance/...` 유지 |
| Phase 4 (4-model smoke E2E + 4 fix layers) | ✅ closed | beda223 + 1badfc4 + c19d455 + 53aaa72 |
| R11-A (cmd/gen-testdata) | ✅ closed | uniform dist + BN254-safe ID + sum invariant |
| R11-B (ZKPOR_SMOKE_USER_DATA env) | ✅ closed | smoke 가 외부 testdata dir 받음 |
| R11-C (extract_smoke_metrics --json + multi-batch aggregate) | ✅ closed | per-cell JSON 출력 |
| R11-D prep (asset-count tier control + r11d.sh + switch_type.sh + RUNBOOK) | ✅ closed | dense/sparse cell 정의 |
| **R11-D Phase 1 (dense Setup/Prove ablation, m8a.8xl)** | **✅ closed (2026-05-28)** | docs/BENCHMARK.md §2.6 |
| **R11-D Phase 2 (density ablation, sparse cells d10/d50)** | **✅ closed (2026-05-28)** | BENCHMARK §2.7 — plateau pattern |
| **R11-D Phase 2b (d1 cells + d10 rerun, 2s RSS sampler)** | **✅ closed (2026-05-28)** | BENCHMARK §2.7 — binary step lock-in |
| **R12-A (library extraction — 5 services → pkg/\*)** | **✅ closed (2026-05-28)** | verifier/prover/witness/userproof/keygen 모두 pkg/\* + cmd thin shim. `Options` + `Run(opts)` 통일 |
| **R12-B (panic → error migration)** | **✅ closed (2026-05-29)** | 5 pkg/\* 모두 `Run(opts) error`. in-process 호출 가능 (no recover). 3 commit (B/1 verifier, B/2 keygen, B/3 user+wit+prover closure) |
| **R12-C (context.Context 지원, cancellable Run)** | **✅ closed (2026-05-29)** | 5 pkg/\* 모두 `Run(ctx, ...)`. prover 폴 루프 batch-granular cancel, witness/userproof snapshot stream 중단, verifier worker pool ctx 조기종료, keygen shape-granular. cmd shim `signal.NotifyContext`. 3 commit (C/1 prover, C/2 wit+user, C/3 verifier+keygen) |
| **R12-D (DB engine 다중화, MySQL only 해제)** | ⏳ **다음 진입점 (development 워크트리)** | `store.Open` driver-selection switch + per-driver error translation. Postgres/SQLite 후보 |
| R12 GPU PoC (ICICLE) | deferred | host RAM 요구 8xl+ — 측정 가치 큼, 별도 트랙 |
| R13 (multi-instance prover) | deferred | R12 closure 후 |

**EC2 자산** (region us-east-1, instance `i-05da73a6bb557498e`, **stopped**):

- m8a.8xlarge / 32 vCPU / 128 GB / gp3 150GB EBS (`DeleteOnTermination=true`)
- T4 production keygen artifact: `.artifacts/zkpor.t4_*.{pk,vk,r1cs}` × 2 shape (24 GB 합)
- gen-testdata 산출물: 12 cells 분 testdata (.artifacts/testdata/) — Phase 1 dense + Phase 2 d10/d50 + Phase 2b d1
- 재시작 → 즉시 prove cell 재실행 가능 (keygen 재실행 불요)
- R12 진입 시 GPU 인스턴스 (g6e.8xl 후보) 로 EBS reattach 가능

**R11-D 종합 핵심 발견** (12 cells, ~$5) — 자세한 수치는
`docs/BENCHMARK.md §1.3` (budget) + `§2.6` (Phase 1 dense) +
`§2.7` (Phase 2/2b density), raw report `.artifacts/reports/`.

- T4 prove RSS = **거의 binary step**: sparse (≤50%) ~62-66 GiB plateau,
  dense (100%) ~118-122 GiB step. linear scaling 가설 무너짐.
- prove wall-clock 은 density 무관, constraint-bound only.
- 운영 권장: m8a.8xl 이 density ≤50% 안전, m8a.12xl 이 ≥70% 필수.

## Source Priority

1. `zkpor/AGENTS.md` — 인계 contract
2. `zkpor/docs/01-project-context.md` — 개념 / scope
3. `zkpor/PRODUCTION_ROADMAP.md` — stage register
4. **본 HANDOFF.md** — 현재 시점 snapshot
5. `zkpor/docs/BENCHMARK.md` — 성능 / 메모리 / 견적의 단일 진실원
6. `zkpor/docs/R11D_RUNBOOK.md` — R11-D 측정 절차
7. `zkpor/docs/02-module-architecture.md` — 모듈 wiring
8. `zkpor/docs/04-solvency-models.md` — 4 모델 spec

## Repository Map (요약)

| 영역 | 상태 / 비고 |
|---|---|
| `zkpor/core/spec/`, `zkpor/core/circuit/`, `zkpor/core/host/`, `zkpor/core/tree/` | ✅ engine universal layer — 변경 적음 |
| `zkpor/core/snapshot/{schema,csv,mapping}` + `{t1,t2,t3,t4}_*` | ✅ R9 closed — 4 model standard CSV connectors |
| `zkpor/core/solvency/{t1,t2,t3,t4}/{spec,circuit,host}` | ✅ 4 model 본체 + host helpers + `*_runner.go` (witness/prover/verifier/userproof dispatch) |
| `zkpor/pkg/{keygen,witness,prover,verifier,userproof}` | ✅ R12-A/B/C — engine library surface. `Options` struct + `Run(ctx, opts) error` (verifier: `RunBatch/RunUser(ctx, opts)`, `RunHash` ctx-free). in-process callable, no recover(), cancellable |
| `zkpor/cmd/{keygen,witness,prover,verifier,userproof}` | ✅ thin shim — flag parse + `signal.NotifyContext` (SIGINT/SIGTERM → ctx) 후 pkg/<service>.Run 호출. prover 는 context.Canceled → exit 0 (daemon graceful), 나머지 one-shot 은 모든 error → exit 1 |
| `zkpor/cmd/gen-testdata` | ✅ R11-A — `--asset-capacity` + `--asset-count` + `--users` + `--seed`, sum invariant |
| `zkpor/cmd/gen-testdata/internal/testdata/` | ✅ R11-A — model-typed synthesis (R12-A 에서 cmd/gen-testdata 안으로 visibility 축소) |
| `zkpor/profile/{t1,t2,t3,t4}_reference/` | ✅ profile.toml + standard CSV testdata/happy/ — sea/binance 명명 제거됨 (`sea_reference`→`t1_reference`, `binance`→`t4_reference`) |
| `zkpor/profile/declarative/` | ✅ R5/R7/R8/R10 builders + Validate |
| `zkpor/scripts/smoke.sh` | ✅ profile-driven + ZKPOR_SMOKE_USER_DATA env override |
| `zkpor/scripts/extract_smoke_metrics.sh` | ✅ md 양식 + `--json` (multi-batch aggregate) |
| `zkpor/scripts/ec2/{bootstrap,sync,fetch,smoke,r11d,switch_type}.sh` + `_lib.sh` | ✅ R11-D 측정 인프라 |
| `zkpor/store/` | ✅ MySQL gorm — 3 모델 (witness/proof/userproof) |
| `zkpor/scripts/deploy/docker-compose.yml` | ✅ smoke MySQL fixture |
| `zkpor/docs/BENCHMARK.md` | ✅ benchmark single source of truth (R6.5/§2.4/§2.6 측정 fold-in) |
| `zkpor/docs/R11D_RUNBOOK.md` | ✅ R11-D 절차 |
| `circuit/`, `src/` (legacy) | ✅ untouched |

## Non-Negotiable Rules

- 모든 측정 / 분석은 `docs/BENCHMARK.md` 에 fold-in. 별도 산발 보고서
  금지 — 본 문서 commit history 가 누적 audit trail.
- testdata 합성은 항상 R11-A `cmd/gen-testdata` 경유. raw CSV 손수
  편집 금지 (sum invariant + BN254 reduce-safe ID 보장).
- EC2 측정 sequence 는 `scripts/ec2/r11d.sh <cell>` 통해. cell 정의가
  shape ↔ asset_count 페어링을 강제하므로 invariant 위반 방지.
- 슬라이스 작업은 `development` 워크트리 (`.worktree/development/`) 에서.
  closure 시 main 으로 fast-forward merge + push. Phase / R-stage 단위
  commit (atomic, 자체적 build green + smoke pass).
- 결과 reports (`.artifacts/reports/R11D_*`) 는 commit 가능 (raw log
  audit trail 가치 > 저장 비용).

## Resume Actions

다음 agent 의 진입 순서:

1. `zkpor/AGENTS.md`, `zkpor/PRODUCTION_ROADMAP.md`, **본 문서** 읽기.
2. `cd .worktree/development && git log --oneline -10` 으로 최근 commit 확인.
3. R12-C 결과 흡수: 5개 pkg/<service> 모두 `Run(ctx, ...)`. prover 폴
   루프는 batch-granular cancel (in-flight `groth16.Prove` 는 끝까지
   돌고, 다음 claim 전에 `ctx.Err()` 관측 → 종료). witness/userproof 는
   snapshot stream (AccountStream/CexAssets 가 이미 ctx 준수) 흐름.
   verifier batch 는 worker pool 이 proof 사이마다 ctx 관측 → 조기 종료.
   keygen 은 shape 사이마다 관측. cmd shim 은 `signal.NotifyContext` 로
   SIGINT/SIGTERM 연결, prover 만 `context.Canceled` → exit 0.
4. 다음 슬라이스 — **R12-D DB engine 다중화** (아래 §Next Slice).

### Next Slice: R12-D — DB engine 다중화 (MySQL only 해제)

목표: `store.Open` 의 MySQL-전용 가정 제거. 현재 `gorm.io/driver/mysql`
을 직접 호출하므로 Postgres/SQLite 배포가 불가.

작업 단위 후보:

1. **driver-selection switch** — config 에 driver 필드 추가 (mysql /
   postgres / sqlite). `store.Open` 이 DSN + driver 로 분기.
2. **per-driver error translation** — duplicate-key / not-found 등
   sentinel error 가 driver 마다 다름. `store.ErrNotFound` 매핑 유지.
3. **gorm dialector 의존성** — `gorm.io/driver/postgres`,
   `gorm.io/driver/sqlite` go.mod 추가. SQLite 는 in-process 테스트
   (실 MySQL 없이 store 단위 테스트) 가능성도 검토.
4. **smoke fixture** — 현재 docker-compose MySQL 고정. driver matrix
   smoke 는 별도 (cheap: SQLite in-memory 로 store round-trip).

종료 조건: `store.Open` 이 ≥2 driver 지원, 기존 MySQL 경로 무회귀,
build/vet/test green. R13 (multi-instance prover) 와 결합 가능.

### Deferred slices

- **R12 GPU PoC (ICICLE)**: bnb-chain/gnark v0.10.1 fork 의
  `backend.WithIcicleAcceleration()`. host RAM ≥128 GB (R11-D 발견),
  g6.8xl / g6e.8xl 후보. CPU 측정 audit trail 유지된 채 별도 트랙.
- **R11-D 잔여 cheap measurements** — d70/d80 transition zone
  ($0.30), m8a.4xl 직접 OOM 검증 ($0.10), avg vs peak gap 분석.

## Required Commands

- 작업 워크트리: `cd .worktree/development` (또는 `cd zkpor` for main)
- `git status` + `git log --oneline -10`
- `cd zkmerkle-proof-of-solvency && go test ./zkpor/...`
- `cd zkmerkle-proof-of-solvency && go build ./zkpor/...`
- Smoke 검증: `./scripts/smoke.sh profile/<X>_reference/<X>_reference.toml`
- EC2 측정 (R12 GPU PoC 진입 시): `./scripts/ec2/sync.sh` → `INSTANCE_TAG=<tag> ./scripts/ec2/r11d.sh <cell>` → `./scripts/ec2/fetch.sh`

## Commit Discipline

- Atomic — 한 stage 안의 sub-step 마다 별도 commit (예: R11-D prep,
  chmod fix, measurement fold-in 각각).
- prefix: `feat(zkpor):` / `fix(zkpor):` / `docs(zkpor):` / `refactor(zkpor):`
- 본문 첫 줄 ≤ 70 char, 본문 2-3 줄로 "why" 요약, `Co-Authored-By:
  Claude` 표기.
- 코드 + 문서를 같은 commit 에 fold (history 일관성).
- 측정 commit 은 raw report (`.artifacts/reports/`) 까지 함께 포함.

## Updating This File

본 문서는 stage / phase closure 마다 갱신:

- **Current State table** 의 row 추가 + 변경된 행 갱신
- **Recent commits 헤드** 5-7개 갱신
- **Next Slice** 갱신 — 진입점 + 준비 작업 + 측정 단계
- **EC2 자산** 상태 갱신 (instance state, EBS 보유 artifact)

본 문서 길이는 200 lines 이내 유지 — 그 이상은 BENCHMARK / RUNBOOK /
PRODUCTION_ROADMAP 으로 위임.
