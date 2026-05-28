# HANDOFF.md

이 문서는 agent 세션이 바뀌어도 작업을 이어가기 위한 **현재 시점의 인수인계**다.
긴 히스토리 로그가 아니다. 작업 시작 전 source priority 상위 문서를 먼저 읽는다.

## Current State

Latest commits (`zkpor/.git/`, branch `main`):

```text
7f2c926 docs(zkpor): R11-D Phase 2b — d1 cells + d10 rerun → binary step pattern
da1799d docs(zkpor): R11-D Phase 2 fold-in — density plateau pattern
13234b7 feat(zkpor): r11d.sh — add d1 cells (extreme sparse, density ~1-2%)
2dc4bbd fix(zkpor): r11d.sh RSS sampler — disable strict mode in subshell
2a6773c docs(zkpor): R11-D Phase 2 plan — single m8a.8xl × density ablation
48d5b5e feat(zkpor): r11d.sh — RSS sampler + Phase 2 density cells
0f40bcd docs(zkpor): HANDOFF rewrite — 1181 → 172 lines, R11-D Phase 1 closure
0b4735a docs(zkpor): R11-D dense phase results — prove memory budget 4× correction
25cefd7 fix(zkpor): chmod +x R11-D scripts so rsync→EC2 preserves exec bit
5ce4df2 feat(zkpor): R11-D prep — gen-testdata tier control + ec2 helpers + runbook
[... 그 위 Phase 4 fix layers + Phase 3d/3e refactors + profile rename + R10]
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
| **R12 (GPU PoC, ICICLE)** | ⏳ **다음 진입점** | host RAM 요구 8xl+ — g6e.8xl 후보 |
| R13 (multi-instance prover) | deferred | R12 closure 후 |

**EC2 자산** (region us-east-1, instance `i-05da73a6bb557498e`, **stopped**):

- m8a.8xlarge / 32 vCPU / 128 GB / gp3 150GB EBS (`DeleteOnTermination=true`)
- T4 production keygen artifact: `.artifacts/zkpor.t4_*.{pk,vk,r1cs}` × 2 shape (24 GB 합)
- gen-testdata 산출물: 12 cells 분 testdata (.artifacts/testdata/) — Phase 1 dense + Phase 2 d10/d50 + Phase 2b d1
- 재시작 → 즉시 prove cell 재실행 가능 (keygen 재실행 불요)
- R12 진입 시 GPU 인스턴스 (g6e.8xl 후보) 로 EBS reattach 가능

**R11-D 종합 핵심 발견** (Phase 1 + 2 + 2b, 12 cells, ~$5):

- T4 production prove RSS = **거의 binary step function**:
  - density 1% ~ 50%: **~62-66 GiB plateau** (sparse — 실제 거래소 영역)
  - density 100%: **~118-122 GiB step** (full-dense worst-case)
  - 가설 (`floor + slope × density`) 의 linear scaling 완전 무너짐
- prove **wall-clock 은 density 무관** (±0.5%) — constraint-bound only
- multi-batch 메모리 누적 없음 (prove 종료 시 정상 회수)
- m8a.4xl (64 GB) **boundary — sparse 에서도 peak 65.6 GiB 가 한계 초과**.
  OOM 확률 높음, 직접 측정 미시행
- **운영 권장**: m8a.8xl (128 GB) 가 density ≤50% 안전, m8a.12xl
  (192 GB) 는 density ≥70% 필수
- 원래 §1.3 prove memory budget (`~25-30 GiB`) → `~60-65 GiB
  (sparse) / ~120 GiB (dense)` 로 보정

자세한 측정 결과: `docs/BENCHMARK.md §1.3` (budget 공식) + `§2.6`
(Phase 1 dense) + `§2.7` (Phase 2 + 2b 통합). 12 cell raw report 는
`.artifacts/reports/R11D_m8a.8xl_*/` audit trail.

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
| `zkpor/cmd/{keygen,witness,prover,verifier,userproof}` | ✅ profile.toml-driven, 4 model dispatch |
| `zkpor/cmd/gen-testdata` | ✅ R11-A — `--asset-capacity` + `--asset-count` + `--users` + `--seed`, sum invariant |
| `zkpor/cmd/gen-testdata/internal/testdata/` | ✅ R11-A — model-typed synthesis + uniform dist + BN254-safe ID |
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
- 분기 `main` 만 사용. Phase / R-stage 단위 commit (atomic, 자체적
  build green + smoke pass).
- 결과 reports (`.artifacts/reports/R11D_*`) 는 commit 가능 (raw log
  audit trail 가치 > 저장 비용).

## Resume Actions

다음 agent 의 진입 순서:

1. `zkpor/AGENTS.md`, `zkpor/PRODUCTION_ROADMAP.md`, **본 문서** 읽기.
2. `git -C zkpor log --oneline -10` 으로 최근 commit 확인.
3. `docs/BENCHMARK.md §2.6` + `§2.7` + `§1.3` 로 R11-D 종합 발견 흡수
   (binary step pattern, density-independent prove time).
4. 다음 슬라이스 — **R12 GPU PoC** (아래 §Next Slice).

### Next Slice: R12 GPU PoC (ICICLE)

목표: bnb-chain/gnark v0.10.1 fork 의 `backend.WithIcicleAcceleration()`
PoC + GPU 가속 prove time / 비용 효율 측정.

R11-D 발견의 R12 영향:
- CPU prove RSS = sparse ~65 GiB / dense ~120 GiB → GPU 인스턴스도
  **host RAM ≥128 GB 필수**. g6.4xl/g6e.4xl (64 GB host) **부적합**
- 권장 entry instance: **g6.8xl (128 GB host + L4 24 GB VRAM)** 또는
  **g6e.8xl (128 GB + L40S 48 GB)**
- VRAM 24 GB 는 chunked MSM 으로 64M constraints 가능 (혹은 L40S 48 GB
  면 single-pass)

준비 작업 (코드 + 인프라):

1. **ICICLE 호환성 PoC** — 로컬에서 `backend.WithIcicleAcceleration()`
   호출 가능한지 검증. gnark fork 의 ICICLE wrapper 확인. CUDA 12+
   필요.
2. **`scripts/ec2/bootstrap.sh` GPU 확장** — NVIDIA driver + CUDA
   toolkit 설치 단계 추가. AL2023 deep learning AMI 가 더 쉬울 수도
   (gpu-ready, ami-id 별도 확인).
3. **r11d.sh GPU cell 정의** (별도 분기 또는 새 wrapper):
   - `t1_700_gpu` — Tier 1 dense on GPU, prove time 비교
   - `t2_92_gpu` — Tier 2 dense
   - 비용/시간 추정: prove ~10s/batch (CPU 32s 의 3×) 가설
4. **BENCHMARK §3.5 / §3.6 GPU 추정 갱신** — host RAM 요구 반영, g6e.8xl
   비용 ~$3.2/hr 기준 재계산

측정 실행:

5. Instance launch — g6e.8xl 또는 g6.12xl (Tesla L4 + AL2023 GPU AMI)
6. T4 production keygen 새로 실행 (CPU keygen, GPU 없이 — keygen 은
   GPU 대상 아님). 또는 기존 EBS 의 .pk 재사용 (region 같으면 attach)
7. dense + sparse cells GPU 측정 (8 cells, ~$5-7)
8. fold-in to BENCHMARK §2.8 (R12 measurement)

**EBS 재사용 옵션** — `i-05da73a6bb557498e` 의 EBS 를 GPU 인스턴스에
attach 하면 keygen 재실행 불요. detach + attach 절차 새 instance launch
시 가능 (단, AZ 동일 필요).

예상 비용 / 시간: ~$5-8, 2-3hr.

### R11-D 완전 종료 이후 — 선택적 후속

R11-D 자체는 closed 이나 다음 cheap measurements 가 정확도 보강 가치:

- **d70 / d80 cells** — plateau (≤50%) → step (100%) transition zone
  ($0.30, 30min)
- **m8a.4xl 직접 OOM/OK 측정** — Phase 2b inference 검증 ($0.10)
- **avg vs peak gap 분석** — `.artifacts/reports/R11D_*/run_*.mem.tsv`
  의 시간 패턴 (post-hoc, 무비용)

## Required Commands

- `git -C zkpor status` + `git -C zkpor log --oneline -10`
- `cd zkmerkle-proof-of-solvency && go test github.com/binance/zkmerkle-proof-of-solvency/zkpor/...`
- `cd zkmerkle-proof-of-solvency && go build ./...`
- Smoke 검증: `./scripts/smoke.sh profile/<X>_reference/<X>_reference.toml`
- EC2 측정: `./scripts/ec2/sync.sh` → `INSTANCE_TAG=<tag> ./scripts/ec2/r11d.sh <cell>` → `./scripts/ec2/fetch.sh`

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
