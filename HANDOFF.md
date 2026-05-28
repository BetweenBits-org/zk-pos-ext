# HANDOFF.md

이 문서는 agent 세션이 바뀌어도 작업을 이어가기 위한 **현재 시점의 인수인계**다.
긴 히스토리 로그가 아니다. 작업 시작 전 source priority 상위 문서를 먼저 읽는다.

## Current State

Latest commits (`zkpor/.git/`, branch `main`):

```text
0b4735a docs(zkpor): R11-D dense phase results — prove memory budget 4× correction
25cefd7 fix(zkpor): chmod +x R11-D scripts so rsync→EC2 preserves exec bit
5ce4df2 feat(zkpor): R11-D prep — gen-testdata tier control + ec2 helpers + runbook
1c022e6 docs(zkpor): R11-D plan — Setup artifact reuse (14→8 cells, $25-30→$8-10)
5f01e5e docs(zkpor): R11-D plan — add 10K user multi-batch sanity (+3 cells)
3f65b0c docs(zkpor): consolidate benchmark docs into single BENCHMARK.md
6c196b3 feat(zkpor): R11-C — extract_smoke_metrics multi-batch aggregate + JSON
[... 그 위 R11-A/B + Phase 4 fix layers + Phase 3d/3e refactors + profile rename + R10]
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
| **R11-D Phase 2 (density ablation + sparse cells + 4xl 재가능)** | ⏳ **다음 진입점** | BENCHMARK §4.1 + RUNBOOK 갱신 필요 |
| R12 (GPU PoC, ICICLE) | deferred | host RAM 요구 8xl+ — g6e.8xl 후보 |
| R13 (multi-instance prover) | deferred | R12 closure 후 |

**EC2 자산** (region us-east-1, instance `i-05da73a6bb557498e`, **stopped**):

- m8a.8xlarge / 32 vCPU / 128 GB / gp3 150GB EBS (`DeleteOnTermination=true`)
- T4 production keygen artifact: `.artifacts/zkpor.t4_*.{pk,vk,r1cs}` × 2 shape (24 GB 합)
- gen-testdata 산출물: `.artifacts/testdata/{t1_700,t2_92,t1_10k,t2_10k}/`
- 재시작 → 즉시 prove cell 재실행 가능 (keygen 재실행 불요)

**R11-D Phase 1 핵심 발견**:

- T4 production dense workload 의 prove peak RSS **~120 GiB** (4×
  §1.3 의 원 추정 ~25-30 GiB)
- 회로 size × density 비례 함수 — sparse testdata 만으로는 underestimate
- multi-batch 메모리 누적 없음 (prove 종료 시 정상 회수)
- per-batch invariance ✓ (1-batch vs 15/109-batch 차이 +2-3%)
- m8a.4xl/m7a.4xl ablation 은 dense workload 에서 OOM 불가피 → Phase 2
  에서 sparse cell 로 instance ablation 재시도

자세한 측정 결과 + 보정된 memory budget: `docs/BENCHMARK.md §1.3` + `§2.6`.

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
| `zkpor/internal/testdata/` | ✅ R11-A — model-typed synthesis + uniform dist + BN254-safe ID |
| `zkpor/profile/{t1,t2,t3,t4}_reference/` | ✅ profile.toml + standard CSV testdata/happy/ — sea/binance 명명 제거됨 (`sea_reference`→`t1_reference`, `binance`→`t4_reference`) |
| `zkpor/profile/declarative/` | ✅ R5/R7/R8/R10 builders + Validate |
| `zkpor/scripts/smoke.sh` | ✅ profile-driven + ZKPOR_SMOKE_USER_DATA env override |
| `zkpor/scripts/extract_smoke_metrics.sh` | ✅ md 양식 + `--json` (multi-batch aggregate) |
| `zkpor/scripts/ec2/{bootstrap,sync,fetch,smoke,r11d,switch_type}.sh` + `_lib.sh` | ✅ R11-D 측정 인프라 |
| `zkpor/store/` | ✅ MySQL gorm — 3 모델 (witness/proof/userproof) |
| `zkpor/deploy/docker-compose.yml` | ✅ smoke MySQL fixture |
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
3. `docs/BENCHMARK.md §2.6` + `§1.3` 로 메모리 발견 / Phase 1 결과 흡수.
4. 다음 슬라이스 — **R11-D Phase 2 density ablation** (아래 §Next Slice).

### Next Slice: R11-D Phase 2

목표: dense workload 에서 좌초된 4xl-class instance ablation 을 sparse
cell 로 재가능 + 실제 거래소 sparse benefit 정량화.

준비 작업 (코드 변경 + 문서):

1. **r11d.sh 에 RSS sampling 추가** — prove process PID 의 RSS 를 10s
   주기로 `${REPORT_ROOT}/run_*.mem.tsv` 에 기록. cell 종료 시 peak /
   avg / min / max 추출 → metrics output 에 포함. 32s 1-batch prove
   에서도 sample 가능.
2. **새 cell 정의** (r11d.sh `case` 에 추가):
   - `t1_700_d10` — Tier 1, 700 user × asset_count=5 (~10% density)
   - `t1_700_d50` — Tier 1, 700 user × asset_count=25 (~50%)
   - `t2_92_d10` — Tier 2, 92 user × asset_count=50 (~10%)
   - `t2_92_d50` — Tier 2, 92 user × asset_count=250 (~50%)
3. **BENCHMARK §1.5 / §1.6 / §4.1** density-aware plan 갱신
4. **R11D_RUNBOOK** Phase 2 runbook 추가 — sparse cells × 3 instance
   (m7a.4xl, m8a.4xl, m8a.8xl) 매트릭스

측정 실행:

5. Instance start (`aws ec2 start-instances --instance-ids
   i-05da73a6bb557498e`) → IP 확보 → `.env` 갱신 (사용자 수동, 권한
   막힘 — agent 가 .env 직접 못 씀)
6. dense cells 의 RSS retroactive 측정 (sparse-density baseline 확보)
   — 가능하면 t1_700, t2_92 (1-batch, 32s/30s prove) 의 RSS 직접 측정
7. sparse cells × 3 instance 측정 (type-switch sequence 동일)
8. fetch + fold-in to BENCHMARK §2.7 (Phase 2 measurement)

예상 비용 / 시간: ~$3-5, 2-3hr.

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
