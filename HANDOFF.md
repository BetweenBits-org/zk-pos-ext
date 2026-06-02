# HANDOFF.md

이 문서는 agent 세션이 바뀌어도 작업을 이어가기 위한 **현재 시점의 인수인계**다.
긴 히스토리 로그가 아니다. 작업 시작 전 source priority 상위 문서를 먼저 읽는다.

## Current State

Origin: `https://github.com/BetweenBits-org/zk-pos-ext.git` (zkpor/.git/).
브랜치 layout: `main` 은 stable, `development` 워크트리 (`.worktree/development/`)
에서 in-progress 슬라이스 진행. main 으로 fast-forward merge 후 push.

Latest commits (branch `r12ef-io-inversion`; R12-E + R12-F IO inversion):

```text
docs(zkpor): R12-E/R12-F source-agnostic IO + persistence ports (this commit)
a808d3e refactor(zkpor): keygen receives injected Profile + KeySink (R12-E/keygen)
df69b93 refactor(zkpor): verifier engine receives injected ports (R12-EF/verifier)
7474881 refactor(zkpor): prover engine receives injected ports (R12-EF/prover)
f2cec0b refactor(zkpor): userproof engine receives injected Snapshot/store (R12-E)
b570ea9 refactor(zkpor): witness engine receives injected Snapshot/queue (R12-E)
fd1b3df refactor(zkpor): snapshot reads through vfs.Opener (R12-E/4)
5a491cc feat(zkpor): store port adapter-wrappers (R12-F/1)
b584f3b feat(zkpor): core/host persistence ports + DTOs (R12-F/0)
843ed68 feat(zkpor): osvfs local filesystem adapter (R12-E/3)
b08258c feat(zkpor): core/io/vfs opener ports (R12-E/2)
[... 그 위 config.Parse/declarative.Parse seams + R12-A/B/C + R11-D]
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
| **R12-E (Source-Agnostic Engine Input)** | **✅ closed (2026-06-01, merged → main)** | 입력 구성을 engine `Run` 밖 cmd shim 으로 inversion. profile/config 은 parsed value, snapshot/keys 는 `vfs.Opener`/`KeyOpener`/`KeySink`/`ByteSource` port. `core/io/vfs` + `osvfs` 가 input-side os/filepath 유일 지점. logical stem (no `filepath.Join` in engine). lazy `verifier -hash`. verifier proof CSV 도 `vfs.ByteSource` 로 inversion → **engine pkg+core 직접 file/db IO 0**. on-wire bytes/명명/시그니처 보존. ROADMAP §R12-E |
| **R12-F (Persistence Port Inversion)** | **✅ closed (2026-06-01, merged → main)** | `core/host` 가 3 port (`WitnessQueue`/`ProofStore`/`UserProofStore`) + gorm-free DTO + `ErrNotFound` sentinel 소유. `store` 는 MySQL adapter-wrapper. gorm.Model 은 store row 에만. R12-D 의 backing-agnostic 목표를 **port 로 흡수** — 단 postgres/sqlite gorm driver 는 **추가하지 않음** (MySQL 단일 adapter, Redis/S3/PG 는 port 의 미래 adapter). ROADMAP §R12-F |
| **R12-D (DB engine 다중화, MySQL only 해제)** | ⏳ **R12-F 로 개념 흡수 (잔여 DB-mux 는 optional)** | backing-agnostic 은 R12-F port 가 제공. `store.Open` driver-switch + postgres/sqlite dialector 는 **미실시** (port 가 더 깨끗한 seam). 실제 2nd adapter 가 필요하면 별도 슬라이스. ROADMAP §Stage R12 의 `R12-D — GPU 측정 smoke` 와는 별개 라벨 (둘 다 intact) |
| **R12-G (TreeDB 백킹 주입)** | **✅ closed (2026-06-02, merged → main)** | witness/userproof 가 `tree.NewAccountTree(cfg.TreeDB…)` 를 직접 호출하고 config 에 tree DSN 보유하던 마지막 engine-측 backing 구성 제거. cmd shim 이 tree 빌드 후 `Options.AccountTree` (`bsmt.SparseMerkleTree`) 주입, engine Options 에서 `Config` 삭제. tree 백엔드(memory/redis/future) 도 cmd 선택. T1 tiny smoke green |
| **Standalone module 추출** | **✅ closed (2026-06-02, merged → main)** | zkpor 가 binance OSS 모듈의 overlay subdir (go.mod 없음, import `…/zkmerkle-proof-of-solvency/zkpor/…`) 에서 **자립 Go 모듈**로 전환. 자체 `go.mod` (module `github.com/BetweenBits-org/zk-pos-ext`, gnark/gnark-crypto replace 2개 이관) + 131 파일 import rewrite + legacy parity 테스트 4개 제거 (G1 closed; production 은 이미 legacy-free). `cd zkpor && go build/test ./...` + T1 smoke green. 커밋 `594dc16` |
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
| `zkpor/core/spec/`, `zkpor/core/circuit/`, `zkpor/core/host/`, `zkpor/core/tree/` | ✅ engine universal layer — 변경 적음. `core/host` 가 R12-F persistence ports (`WitnessQueue`/`ProofStore`/`UserProofStore` + gorm-free DTO + `ErrNotFound`) 추가 소유 |
| `zkpor/core/io/vfs` + `zkpor/core/io/vfs/osvfs` | ✅ R12-E — input-side os/filepath 유일 지점. `vfs` ports (`Opener`/`KeyOpener`/`KeySink`/`ByteSource`), `osvfs` local adapter (`Dir`/`KeyDir`/`KeyDirSink`/`File`). S3/DB/in-mem backend 는 port 만 구현하면 꽂힘 |
| `zkpor/core/snapshot/{schema,csv,mapping}` + `{t1,t2,t3,t4}_*` | ✅ R9 closed — 4 model standard CSV connectors |
| `zkpor/core/solvency/{t1,t2,t3,t4}/{spec,circuit,host}` | ✅ 4 model 본체 + host helpers + `*_runner.go` (witness/prover/verifier/userproof dispatch) |
| `zkpor/pkg/{keygen,witness,prover,verifier,userproof}` | ✅ R12-A/B/C/E/F — engine library surface. `Options` 가 injected value/opener/port (profile/config value, `vfs` opener, `corehost` port) 받음. `Run(ctx, opts) error` (verifier: `RunBatch/RunUser(ctx, opts)`, `RunHash` ctx-free). engine `Run` 안에 os/filepath/store import 0. in-process callable, no recover(), cancellable |
| `zkpor/cmd/{keygen,witness,prover,verifier,userproof}` | ✅ thin shim — flag parse + `signal.NotifyContext` + **입력 구성 (R12-E)**: `os.ReadFile`+`Parse` value, `osvfs.{Dir,KeyDir,KeyDirSink,File}` opener, `store.New*Adapter` port. verifier 는 `-hash` 빼고 lazy 구성. prover 는 context.Canceled → exit 0, 나머지 one-shot 은 error → exit 1 |
| `zkpor/cmd/gen-testdata` | ✅ R11-A — `--asset-capacity` + `--asset-count` + `--users` + `--seed`, sum invariant |
| `zkpor/cmd/gen-testdata/internal/testdata/` | ✅ R11-A — model-typed synthesis (R12-A 에서 cmd/gen-testdata 안으로 visibility 축소) |
| `zkpor/profile/{t1,t2,t3,t4}_reference/` | ✅ profile.toml + standard CSV testdata/happy/ — sea/binance 명명 제거됨 (`sea_reference`→`t1_reference`, `binance`→`t4_reference`) |
| `zkpor/profile/declarative/` | ✅ R5/R7/R8/R10 builders + Validate |
| `zkpor/scripts/smoke.sh` | ✅ profile-driven + ZKPOR_SMOKE_USER_DATA env override |
| `zkpor/scripts/extract_smoke_metrics.sh` | ✅ md 양식 + `--json` (multi-batch aggregate) |
| `zkpor/scripts/ec2/{bootstrap,sync,fetch,smoke,r11d,switch_type}.sh` + `_lib.sh` | ✅ R11-D 측정 인프라 |
| `zkpor/store/` | ✅ MySQL gorm — 3 모델 (witness/proof/userproof) + R12-F adapter-wrappers (`New{WitnessQueue,ProofStore,UserProofStore}Adapter`) 가 `core/host` port 만족. gorm.Model 은 store row 에만 격리. **유일 출하 adapter** (postgres/sqlite driver 미추가) |
| `zkpor/scripts/deploy/docker-compose.yml` | ✅ smoke MySQL fixture |
| `zkpor/docs/BENCHMARK.md` | ✅ benchmark single source of truth (R6.5/§2.4/§2.6 측정 fold-in) |
| `zkpor/docs/R11D_RUNBOOK.md` | ✅ R11-D 절차 |
| `circuit/`, `src/` (legacy, binance OSS 모듈) | ✅ untouched. zkpor 는 standalone 추출 후 legacy 와 **별도 Go 모듈** — production 은 원래 legacy-free, parity 테스트(legacy import)는 추출 시 제거됨 |
| `zkpor/go.mod` + `go.sum` | ✅ standalone — module `github.com/BetweenBits-org/zk-pos-ext`, go 1.22, gnark/gnark-crypto → bnb-chain fork replace 보유. `cd zkpor && go build ./...` |

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
2. `git -C zkpor log --oneline -12` (branch `main`) 으로 최근 commit 확인.
   **빌드는 자립 모듈**: `cd zkpor && go build ./...` (outer 에서 `./zkpor/...` 아님).
3. R12-E/R12-F 결과 흡수: engine `Run` 은 입력을 직접 안 연다. cmd shim
   이 profile/config 를 `Parse` value 로, snapshot/keys 를 `osvfs`
   opener 로, store 를 `store.New*Adapter` port 로 구성해 `Options` 에
   주입. `core/io/vfs` + `osvfs` 가 input-side os/filepath 유일 지점.
   `core/host` 가 persistence port 3종 + gorm-free DTO + `ErrNotFound`
   소유. key stem 은 bare logical (engine 이 dir join 안 함), `osvfs.
   KeyDir` 가 `<dir>/<stem>.<ext>` 생성. verifier `-hash A B` 는 빈
   profile 로도 성공 (lazy 구성). on-wire bytes/명명/시그니처 보존.
4. 다음 슬라이스 — 아래 §Next Slice.

### Next Slice: port 의 첫 non-local adapter (DB/S3/Redis)

R12-E/F/G + standalone 추출 완료. T1 tiny smoke green (cmd shim 의
osvfs/Parse/adapter/tree 구성이 기존 path 동작과 무회귀임 확인됨).
engine 은 입력(profile/config value, snapshot/keys opener, tree) +
persistence(queue/proof/userproof port) 를 전부 주입받으므로 **코드
변경 0 으로 backing 만 교체** 가능. 다음은 실제 non-local adapter:

1. **Redis / S3 adapter against the ports** — R12-F port
   가 이미 seam. Redis `WitnessQueue` (BLPOP 큐, R13 의 자연 진입점)
   또는 S3 `vfs.Opener`/`KeySink` (key/snapshot 원격). engine 코드
   변경 0 — adapter package + cmd shim wiring 만.
2. **(optional, still-queued) R12-D DB-mux** — 정말 *같은 SQL gorm
   에서 2nd dialect* 가 필요하면 `store.Open` driver-switch +
   `gorm.io/driver/{postgres,sqlite}`. 단 port 가 더 깨끗하므로
   기본은 #2 (별도 adapter) 권장. (ROADMAP §Stage R12 의
   `R12-D — GPU 측정 smoke` 라벨과는 무관.)

종료 조건: smoke green (무회귀). adapter 슬라이스는 build/vet/test
green + 해당 port round-trip 테스트.

### Deferred slices

- **R12 GPU PoC (ICICLE)**: bnb-chain/gnark v0.10.1 fork 의
  `backend.WithIcicleAcceleration()`. host RAM ≥128 GB (R11-D 발견),
  g6.8xl / g6e.8xl 후보. CPU 측정 audit trail 유지된 채 별도 트랙.
- **R11-D 잔여 cheap measurements** — d70/d80 transition zone
  ($0.30), m8a.4xl 직접 OOM 검증 ($0.10), avg vs peak gap 분석.

## Required Commands

- 작업 워크트리: `cd .worktree/development` (또는 `cd zkpor` for main)
- `git status` + `git log --oneline -10`
- **빌드/테스트 (zkpor 는 이제 자립 Go 모듈)**: `cd zkpor && go build ./... && go vet ./... && go test -short ./...`.
  더는 outer `zkmerkle-proof-of-solvency` 에서 `./zkpor/...` 로 빌드하지 않는다 — `zkpor/go.mod` 가 모듈 루트 (module `github.com/BetweenBits-org/zk-pos-ext`, go 1.22, bnb-chain/gnark+gnark-crypto replace 보유). overlay 절차 불요.
- Smoke 검증 (zkpor/ 에서): `./scripts/smoke.sh profile/<X>_reference/<X>_reference.toml`
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
