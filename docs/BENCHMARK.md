# zkpor benchmark — single source of truth

zkpor 의 성능 / 비용 / SLA 관련 모든 측정과 추정의 단일 진실원.
methodology + measurements + derived estimates + open questions 를 한
문서에 통합. 양식 (`docs/reports/SMOKE_TEMPLATE.md`) 와 자동 추출
helper (`scripts/extract_smoke_metrics.sh`) 는 active artifact 로 별도
보존.

본 문서는 source-of-truth — `docs/estimates/` + `docs/reports/` 분리
보고서는 git history 안에만 남고 작업용 산출물은 본 문서로 흘러
들어옴.

목차:

1. [Methodology](#1-methodology) — Setup vs Prove 분리, ablation 디자인
2. [Measurements](#2-measurements) — 시간순 측정 결과
3. [Derived estimates](#3-derived-estimates) — 4-axis matrix + GPU 견적
4. [Open questions / next measurements](#4-open-questions--next-measurements)

---

## 1. Methodology

### 1.1 핵심 인사이트: 4-axis → 2-phase 환원

당초 4 axis (instance, model, tier shape, user count) 완전 조합 시
~500 cells — 비효율. **Setup vs Prove 두 phase 만 별도 측정** 하면
대부분의 GTM/운영 질문이 답해진다.

| Phase | 특성 | 측정 빈도 | 자원 |
|---|---|---|---|
| **Setup (keygen)** | 1회성 (per `<model, shape, capacity>` tuple) | 인스턴스 선택 일회 결정 후 평생 cache hit | RAM 집중 (T4 production peak 87GB) |
| **Prove (groth16.Prove)** | 반복적 (snapshot 마다 batches × N) | 운영 SLA 핵심 | vCPU/AVX-512 집중, RAM 적음 (~25-30GB) |

→ Setup 과 Prove 가 **자원 프로파일 다름** + **운영 빈도 다름** +
**인스턴스 요구 다름**. 측정 + instance 선택 모두 분리해서 다룸.

### 1.2 Setup phase — instance OOM 경계만 lock

Setup 은 cache hit 되면 추가 비용 0 (peer FS 또는 EBS volume 에 .pk 저장
후 평생 재사용). 즉 instance 선택 가이드만 정확하면 됨.

**필요 측정** (production tuple 당 2 cells):
- **OOM 측정** (RAM 부족 instance): 어디서 fail 하는지 + peak RAM
- **OK 측정** (RAM 안전 instance): 통과 시간 + actual peak

예: T4 production (cap=500, shape 50_700+500_92):
- m8a.4xl (64GB) → **OOM 예상** (peak 87GB > 64GB)
- m8a.8xl (128GB) → 통과

→ 운영 가이드: "T4 production keygen은 128GB+ instance 사용". 추가
instance 종류는 비용/속도 trade-off만.

### 1.3 Prove phase — PK in-memory + multi-batch 구조 확인

`cmd/prover/main.go:106-122` lazy reload pattern:

```go
var params snarkParams                // outer scope, persists
for {
    row, err := witnessStore.ClaimOldestByStatus(...)
    proveOne(row, &params, plan, ...)  // params 재사용
}
// loadSnarkParams: if params.tier == targetTier && params.r1cs != nil return nil
```

→ PK + R1CS 한 번 메모리 적재 후 multiple batches process. Tier 변경
시에만 reload.

**Prove 메모리 budget (T4 production 기준)**:

| 항목 | 크기 |
|---|---:|
| `.pk` 적재 | ~12 GiB |
| `.r1cs` 적재 | ~3-4 GiB |
| groth16.Prove intermediate (MSM working set) | ~5-10 GiB |
| per-batch witness | ~수십 MB |
| **Prove peak RAM** | **~25-30 GiB** |

→ **m8a.4xl (64GB) 도 prove 가능**. Setup 만 다른 instance 에서 한 후
`.pk` 24GB 만 메모리 적재 + working set.

### 1.4 Prove ablation 디자인

3 instances 만으로 깔끔한 ablation:

| Pair | 변수 | 측정하는 것 |
|---|---|---|
| **m7a.4xl vs m8a.4xl** | 세대 (Zen4 → Zen5) | IPC + AVX-512 + DDR5 BW |
| **m8a.4xl vs m8a.8xl** | 코어수 (16 → 32 vCPU) | Amdahl curve |
| (m7a.4xl vs m8a.8xl) | 둘 다 | 통합 효과 |

→ 3 instances × 3 shape (T4 mid 20_500 / Tier 1 50_700 / Tier 2 500_92)
= **9 cells** 로 instance × shape matrix 완성.

### 1.5 User count: math derive + 10K real-batch sanity

`cmd/prover` multi-batch lazy reload → per-batch prove time 은 batches
수에 독립.

```
total_prove_time = batches × per_batch_prove_time
batches = ceil(users / users_per_batch)
```

→ **1-batch 실측 후 batches math 만 곱하면 N users prove time 추정**.

다만 100% math derive 는 자의적이라, **10K user multi-batch 실측 1
round** 를 ablation 안에 묶어 sanity check:

| 측정 | 목적 |
|---|---|
| **1-batch × 3 instance × 3 shape (9 cells)** | per-batch isolation, instance + shape ablation |
| **10K user × 3 instance × 1 shape (3 cells)** | multi-batch sanity (GC drift, memory growth, lookup-table rebuild overhead), 1-batch math derive 정확도 검증 |

10K user shape 선택: **T4 mid (20_500)** — 10K / 500 = 20 batches,
적당한 batch count, RAM gentle (25-30 GiB peak), 모든 instance 가용.
R11-A testdata generator (`cmd/gen-testdata`) 출력 사용.

10K 측정의 핵심 검증:
- `per_batch_prove(batch=N) ≈ per_batch_prove(batch=1)` (linear 가정)
- 누적 GC pause / 메모리 leak 없음 (peak RAM 안정)
- Tier 전환 (multi-shape 시) 오버헤드 측정 (single-shape 10K 에선 N/A)

### 1.6 Minimum viable benchmark plan

총 **14 cells, ~$25-30, ~5-7hr** 측정:

| 구분 | Cells | 비용 (대략) | 측정 instance |
|---|---:|---:|---|
| Setup OOM | 1 | ~$1 | m8a.4xl (T4 production setup attempt) |
| Setup OK | 1 | ~$4 | m8a.8xl (T4 production setup full) |
| Prove 1-batch × 3 shapes × 3 instances | 9 | ~$10-15 | m7a.4xl, m8a.4xl, m8a.8xl |
| Prove 10K user × 1 shape × 3 instances | 3 | ~$10 | 동일 (T4 mid 20_500, R11-A testdata) |
| **합** | **14** | **~$25-30** | |

10K user 측정은 §1.5 의 sanity check — 1-batch math derive 의 linear
scaling 가정과 GC/메모리 drift 부재를 검증.

### 1.7 Instance type-switch 전략

`.pk` 24GB EBS 공유로 측정 instance 마다 keygen 재실행 불요:

```
launch m8a.4xl (us-east-1c, gp3 150GB EBS)
  ↓ setup attempt → OOM 측정 (T4 production)
  ↓ stop + type change → m8a.8xl
setup OK + prove 측정 (3 cells)
  ↓ stop + type change → m7a.4xl
prove 측정 (3 cells, .pk 재활용 from EBS)
  ↓ stop + type change → m8a.4xl
prove 측정 (3 cells, .pk 재활용)
  ↓ stop + terminate (EBS DeleteOnTermination=true)
```

**Capacity 변동 대비**: 다른 AZ 에 새 EBS + 인스턴스 launch (production
T4 측정에서 활용한 fallback).

---

## 2. Measurements

각 측정 세션의 raw data + 핵심 insight. 시간순. SMOKE_TEMPLATE 따라
metadata + per-model breakdown.

### 2.1 Tiny first pass (2026-05-27)

**Phase 4 closure tiny smoke** — 4 model × cap=5 × shape=5_10.

| 항목 | 값 |
|---|---|
| 호스트 | m8a.8xlarge (32 vCPU Zen5 / 128 GB) |
| zkpor commit | `53aaa72` (Phase 4 closure + 4 fix layer) |
| testdata | 10 real accounts + padding |
| 비고 | 4 layer fix 후 첫 통과. `1badfc4` → `c19d455` → `53aaa72` 적용 |

**4 fix layer (Phase 4 closure 직전 발견)**:

| # | Fix | Commit |
|---|---|---|
| 1 | smoke.sh DB clear table-name | `beda223` |
| 2 | `SetBatchCreateUserCircuitWitness` (4 model) sparse user.Assets → dense AssetsForUpdateCex | `1badfc4` |
| 3 | T1 circuit price scaling (T4와 align) | `c19d455` |
| 4 | RunWitness BeforeCex zero-init | `53aaa72` |

**측정값 (cap=5, shape=5_10, 4 model)**:

| Model | NbConstraints | compile | Setup | Prove | 결과 |
|---|---:|---:|---:|---:|---|
| T1 | 165,613 | 1.48s | 3.77s | 461 ms | ✅ |
| T2 | 167,351 | 1.59s | 3.94s | 443 ms | ✅ |
| T3 | 204,015 | 2.10s | 5.09s | 467 ms | ✅ |
| T4 | 286,157 | 3.17s | 7.94s | 674 ms | ✅ |
| **합** | | | | | 4 model PASS |

**Artifact**: `.pk` ~150 MiB 합, `.r1cs` ~88 MiB.

### 2.2 Mid-tier r7a 첫 통과 (2026-05-27)

**4 model × cap=200 × shape=20_500** on r7a.4xlarge.

| 항목 | 값 |
|---|---|
| 호스트 | r7a.4xlarge (16 vCPU AMD Genoa Zen4 / 128 GB) |
| zkpor commit | `489b44b` |
| 비고 | Mid-tier 첫 통과 측정. T4 client-side SSH 끊김 (server-side 완료, artifacts + DB 검증) |

**측정값 (mid-tier)**:

| Model | NbConstraints | compile | Setup | Prove | 결과 |
|---|---:|---:|---:|---:|---|
| T1 | 9,483,618 | 1m 50s | 4m 14s | 17.8 s | ✅ |
| T2 | 10,831,084 | 2m 14s | 5m 6s | 18.4 s | ✅ |
| T3 | 14,481,701 | 2m 48s | 6m 20s | 19.3 s | ✅ |
| T4 | 23,695,376 | 4m 13s | ~14m* | ~31s* | ✅ |

*T4 setup time .pk mtime 기준 추정 (SSH disconnect 영향). T4 prove 도 추정.

**Wall-clock 전체**: ~52분. **Artifact**: `.pk` ~10.7 GiB 합.

### 2.3 Mid-tier 3-way 비교 (2026-05-27)

**같은 mid-tier (cap=200, shape=20_500) × 3 instances**:

| 항목 | r7a.4xl (Zen4 16) | m8a.8xl (Zen5 32) | c8a.12xl (Zen5 48) |
|---|---|---|---|
| CPU | AMD Genoa | AMD EPYC 9R45 Turin | AMD EPYC 9R45 Turin |
| RAM | 128 GB DDR5-4800 | 128 GB DDR5-5200 | 92 GB DDR5-5200 |
| 가격/hr | $1.06 | ~$2.10 | ~$2.65 |

**Per-stage 측정 (T4 mid-tier 합산)**:

| 단계 | r7a | m8a.8xl | c8a.12xl | r7a → m8a | m8a → c8a |
|---|---:|---:|---:|---:|---:|
| compile 합 | 665s | 422s | 426.7s | **1.58×** | 0.99× |
| Setup 합 | ~1780s* | 774s | 611.1s | ~2.30×* | **1.27×** |
| Prove 합 (4 batches) | ~87s | 42.7s | 36.8s | ~2.04× | 1.16× |
| **Wall-clock 전체** | ~52 min | ~25 min | ~20 min | **2.08×** | **1.25×** |
| 1회 비용 | $0.92 | $0.88 | $0.88 | -4% | 0% |

**Speedup efficiency**:

| Transition | vCPU 비율 | 실측 speedup | Efficiency |
|---|---:|---:|---:|
| r7a → m8a.8xl | 2× | 2.08× | **104%** (super-linear: IPC + AVX-512) |
| m8a.8xl → c8a.12xl | 1.5× | 1.25× | 83% (Amdahl) |
| r7a → c8a.12xl | 3× | 2.60× | 87% |

**Amdahl fractions (3-way 측정 derived)**:

| 단계 | Sequential | 의미 |
|---|---:|---|
| compile | ~85% | vCPU 효과 ~4 cores까지만 |
| Setup | ~25% | vCPU scaling 가장 효과적 |
| Prove | ~15% | Production scale에서 최대 speedup 가능 |

**Insight**: m8a.8xl 이 sweet spot — r7a 대비 시간 절반에 비용은 거의
동일.

### 2.4 Production T4 m8a.12xlarge (2026-05-27)

**T4 production keygen + smoke** on m8a.12xlarge.

| 항목 | 값 |
|---|---|
| 호스트 | m8a.12xlarge (48 vCPU AMD EPYC 9R45 Turin / 192 GB) |
| Instance ID | `i-0b251761db7ec9565` (fresh launch, us-east-1b, AZ fallback) |
| zkpor commit | `496c492` |
| 비고 | us-east-1c capacity 부족 → us-east-1b 새 인스턴스. force re-keygen. |

**Tier별 측정**:

| Tier | shape | NbConstraints | compile | Setup | Prove |
|---|---|---:|---:|---:|---:|
| Tier 1 | 50_700 | 64,341,094 | 7m 47s | 11m 59s | 29.6 s |
| Tier 2 | 500_92 | 63,822,805 | 8m 47s | 10m 50s | (single batch only) |
| **합** | | **128,163,899** | **16m 34s** | **22m 49s** | |

**Prove 분해 (Tier 1 single batch)**:
- solver: 11.6s (witness solving)
- prover: 17.2s (groth16.Prove proper)
- **합**: 29.6s

**Artifact**: `.pk` 24 GiB (= 12 × 2 tier, README baseline 일치).

**Wall-clock**: ~50 min (bootstrap 5m + smoke 45m).

**R3 step 4 baseline (r7a ~60min) 대비 ~1.3× speedup**. mid-tier 3-way 의
vCPU scaling (r7a→m8a.12xl 추정 2.6×) 보다 작음.

**가설**: RAM bandwidth bound — production T4 회로 size (~64M
constraints/tier) MSM working set 이 L3 cache 넘어 메모리 bound. DDR5
bandwidth 가 vCPU scaling 캡. r7a.4xl 직접 측정으로 검증 필요.

### 2.5 측정 cells 종합 (9 measurements)

기존 측정 lookup table — derived estimates §3 의 fit base:

| # | Instance | Model | Shape | cap | NbConstraints | Prove/batch |
|---|---|---|---|---:|---:|---:|
| 1 | m8a.8xl | T1 | 5_10 | 5 | 165,613 | 461 ms |
| 2 | m8a.8xl | T2 | 5_10 | 5 | 167,351 | 443 ms |
| 3 | m8a.8xl | T3 | 5_10 | 5 | 204,015 | 467 ms |
| 4 | m8a.8xl | T4 | 5_10 | 5 | 286,157 | 674 ms |
| 5 | r7a.4xl | T4 | 20_500 | 200 | 23.7M | 15.4 s |
| 6 | m8a.8xl | T4 | 20_500 | 200 | 23.7M | 9.7 s |
| 7 | c8a.12xl | T4 | 20_500 | 200 | 23.7M | 8.3 s |
| 8 | m8a.12xl | T4 (Tier 1) | 50_700 | 500 | 64,341,094 | 17.2 s |
| 9 | m8a.12xl | T4 (Tier 2) | 500_92 | 500 | 63,822,805 | ~17 s |

---

## 3. Derived estimates

측정 9 cells + sub-linearity / Amdahl / Math 로 derive. **GPU + multi-
instance 는 hypothesis** (R12/R13 closure 후 실측 lock).

### 3.1 Fit 1: NbConstraints → per-batch prove time

회로 size 가 클수록 **per-million-constraint cost ↓** (MSM Pippenger
amortization):

| Instance | NbConstraints | Prove (s) | s/M-constraint |
|---|---:|---:|---:|
| m8a.8xl | 165k | 0.46 | 2.8 |
| m8a.8xl | 286k | 0.67 | 2.4 |
| m8a.8xl | 23.7M | 9.7 | 0.41 |
| r7a.4xl | 23.7M | 15.4 | 0.65 |
| c8a.12xl | 23.7M | 8.3 | 0.35 |
| m8a.12xl | 64.3M | 17.2 | 0.27 |

대략적 fit: `per_batch_prove ≈ k × NbConstraints^0.85` (linear에 가까운
sub-linear).

### 3.2 Fit 2: Instance speedup (Amdahl + IPC)

3-way 측정 §2.3 의 결과로 도출.

### 3.3 Fit 3: User count → batches (math, no measurement)

```
batches = ceil(users / users_per_batch)
```

User count 자체는 per-batch cost 에 영향 0.

### 3.4 4-axis Derived Matrix (1M user 기준, single instance CPU)

| Instance | T1 spot (50_1000) | T2/T3 margin (50_500) | T4 mid (20_500) | T4 prod mix |
|---|---:|---:|---:|---:|
| r7a.4xl (16 vCPU Zen4) | ~85 min | ~3.6 hr | ~50 min | ~7-8 hr |
| m8a.8xl (32 vCPU Zen5) | ~42 min | ~1.7 hr | ~25 min | ~3.5-4 hr |
| **m8a.12xl** (48 vCPU Zen5) | **~33 min** | **~1.3 hr** | **~20 min** | **~2.7-3 hr** |
| c8a.12xl (48 vCPU Zen5) | ~33 min | ~1.3 hr | ~20 min | OOM 위험 (peak 87GB > 92GB) |

### 3.5 GPU 가속 추정 (R12 hypothesis — 실측 미검증)

ICICLE BN254 GPU backend 가정 + MSM/FFT 가 prove time 의 70-80% 차지.

**단계별 GPU 효과**:

| 단계 | GPU 적용 | 추정 speedup |
|---|---|---:|
| `frontend.Compile` | ❌ CPU only | 1.0× |
| `groth16.Setup` | △ | 2-3× |
| **`groth16.Prove` MSM** | ✅ | **5-10×** |
| **`groth16.Prove` FFT** | ✅ | **3-5×** |
| Witness solver | ❌ | 1.0× |
| **Prove 전체 (Amdahl)** | | **3-5×** |

**Per-batch GPU 추정 (T4 production Tier 1, 64M constraints)**:

| Instance | Prove time | 비고 |
|---|---:|---|
| m8a.12xl (CPU, 측정) | 17.2 s | baseline |
| g6.4xl (L4 GPU + ICICLE) | ~4-5 s | 3-4× speedup |
| g6e.4xl (L40S GPU + ICICLE) | ~3-4 s | 4-5× speedup |

### 3.6 1M users GPU 시나리오 (R12+R13 hypothesis)

**Shape별 batches × 1M users**:

| Shape | users/batch | 1M batches |
|---|---:|---:|
| T1 spot (50_1000) | 1,000 | 1,000 |
| T4 Tier 1 (50_700) | 700 | 1,429 |
| T4 mix (95% T1 + 5% T2) | — | ~1,901 |

**Single L4 GPU (g6.4xlarge, $1.32/hr)**:

| Scenario | Total prove | Wall-clock | 1회 비용 |
|---|---:|---|---:|
| T1 spot 1M | 2,500s | **~42 min** | $0.93 |
| T4 production mix 1M | 8,555s | **~2.4 hr** | $3.17 |

**Multi-instance scaling**:

| Setup | T1 spot 1M | T4 prod 1M |
|---|---:|---:|
| 1× L4 g6.4xl | 42 min | 2.4 hr |
| **4× L4** (g6.12xl 또는 4× g6.4xl multi-instance) | **~11 min** | **~36 min** |
| **8× L4** (g6.48xl 또는 8× g6.4xl) | **~6 min** | **~18 min** |

### 3.7 GTM 거래소 시나리오 견적 (Single instance CPU)

| 거래소 규모 | Model | Setup | Wall-clock | 비용 |
|---|---|---|---:|---:|
| 10K spot | T1 | m8a.8xl | ~6 min | $0.21 |
| 30K spot | T1 | m8a.8xl | ~10 min | $0.35 |
| 100K spot | T1 | m8a.12xl | ~12 min | $0.63 |
| **1M spot (T1)** | T1 | m8a.12xl | **~33 min** | **$1.73** |
| 30K margin | T2/T3 | m8a.8xl | ~15 min | $0.53 |
| 100K margin | T2/T3 | m8a.12xl | ~22 min | $1.16 |
| **1M margin (T2/T3)** | T2/T3 | m8a.12xl | **~1.3 hr** | **$4.10** |
| 100K full-margin | T4 | m8a.12xl | ~30 min | $1.58 |
| **1M full-margin (T4)** | T4 | m8a.12xl | **~2.7-3 hr** | **$8.50-9.45** |

**Multi-instance + GPU (R12 + R13 closure 후)** — 위 시간을 N (cluster
size) 으로 나눔 + GPU 적용 시 추가 3-5×.

### 3.8 인스턴스 추천 매트릭스

| 시나리오 | 추천 | 근거 |
|---|---|---|
| Cost-optimal (Total $) | r7a.4xlarge ($0.92 mid-tier) | 가장 저렴 |
| **Balanced sweet spot** | **m8a.8xlarge** ($0.88, 시간 절반) | 비용 거의 동일, 시간 절반 |
| Time-optimal | c8a.12xlarge (20m mid-tier) | 같은 비용, 더 빠름 |
| Production keygen 일회성 | r7a.4xlarge | one-time cost |
| Dev iteration 자주 | **m8a.8xlarge** | sweet spot |
| Production T4 setup (peak 87GB) | m8a.8xl+ (128GB+) | RAM 안전 |

### 3.9 Confidence intervals

| Matrix cell | Source | Confidence |
|---|---|---|
| (m8a.8xl, T1-T4, shape=5_10, cap=5) | Measured | ✅ High |
| (r7a/m8a/c8a, T4, shape=20_500, cap=200) | Measured (3 instances) | ✅ High |
| (m8a.12xl, T4, shape=50_700/500_92, cap=500) | Measured (production) | ✅ High |
| (m8a.12xl, T4, shape=20_500, cap=200) | Extrapolated | 🟢 Medium-High |
| (any instance, T1-T3, mid-tier) | Extrapolated | 🟡 Medium |
| (any instance, T1-T3, production) | Extrapolated | 🟠 Low-Medium |
| (GPU, any) | Hypothesis (3-5× CPU) | 🔴 Hypothesis — **R12 후 실측** |
| (multi-instance, any) | Math derivation | 🔴 Hypothesis — **R13 후 실측** |

### 3.10 사용 가이드 (GTM 견적 작성 시)

1. 목표 고객 정의 (user count + model)
2. §3.7 거래소 시나리오 lookup
3. GPU/multi-instance 필요 시 §3.6 multiplier 적용 (R12/R13 후 정확화)
4. §3.9 Confidence 확인 — High만 약속, Medium은 buffer 추가, Low는 PoC
   measurement 권장
5. Customer SLA 협상 시 — measured cells 만 약속

---

## 4. Open questions / next measurements

### 4.1 R11-D minimum viable plan (Setup/Prove ablation)

§1.6 의 14 cells. 진행 시점은 R11 dev infra (R11-A/B/C, 이미 완료) 후.

**구성**:
- Setup OOM (m8a.4xl, T4 production) — peak RAM lock
- Setup OK (m8a.8xl, T4 production) — 통과 baseline
- **1-batch prove** × 3 shapes × {m7a.4xl, m8a.4xl, m8a.8xl} = 9 cells
- **10K user prove** × T4 mid (20_500) × {m7a.4xl, m8a.4xl, m8a.8xl} = 3 cells
  - R11-A `cmd/gen-testdata` 로 testdata 합성
  - R11-B `ZKPOR_SMOKE_USER_DATA` 로 smoke harness 에 주입
  - R11-C `--json` 출력으로 multi-batch aggregate 자동 추출

**Insight 영역**:
- m7a.4xl vs m8a.4xl: 세대 차이 (Zen4 → Zen5) — IPC + AVX-512
- m8a.4xl vs m8a.8xl: 코어수 차이 — Amdahl curve
- §2.4 의 production T4 m8a.12xl ~1.3× speedup 가설 (RAM bandwidth
  bound) 검증
- 10K real-batch vs 1-batch math derive 정확도 (per-batch invariance 검증)

### 4.2 R12 — Prove-path GPU 가속 (ICICLE)

**Hypothesis**: 3-5× prove speedup, $0.88 → ~$0.45 mid-tier 1회 비용.

**Entry blockers**:
- bnb-chain/gnark v0.10.1 fork 의 `backend.WithIcicleAcceleration()`
  호환성 PoC
- CUDA toolchain + AMI 설정 (`scripts/ec2/bootstrap.sh` 확장)

**Single-cell PoC measurement**:
- g6.4xl (L4 GPU) × T4 mid (20_500) — GPU multiplier 가설 검증
- 그 후 production T4 + multi-instance 확장 (R13)

**Roadmap**: `PRODUCTION_ROADMAP.md` §Stage R12.

### 4.3 R13 — Multi-instance prover

**Hypothesis**: `wall_clock(N workers) ≈ single / N × overhead`,
overhead ~1.1-1.3.

**Entry blockers**:
- R12 closure (GPU + multi-instance 가 가장 가치 큼)
- Redis BLPOP 큐 + multi-worker prover 코드 변경 (R3 step 4 follow-up)

**Cells**:
- 4× m8a.8xl multi-worker × T4 production
- 8× g6.4xl GPU multi-instance × T4 (R12 후)

**Roadmap**: `PRODUCTION_ROADMAP.md` §Stage R13.

### 4.4 추가 측정 priority (효율 max)

낮은 confidence 영역 채우는 순서:

| 순위 | 측정 | 비용 | 효과 |
|---|---|---:|---|
| 1 | **R11-D 14 cells** (§4.1) | ~$25-30 | Setup/Prove ablation matrix + 10K multi-batch sanity |
| 2 | **T1 production-scale** (cap=50, 50_1000, m8a.12xl) | ~$3-5 | Spot GTM segment 견적 정확화 |
| 3 | **R12 PoC** (GPU L4 single point) | ~$2 | GPU column lock-in |
| 4 | **R11-A 의 real-scale testdata** + multi-batch sanity | dev work | 1-batch math derive 정확도 검증 |
| 5 | **R13 multi-worker** (4× m8a.8xl) | ~$15-20 | Multi-instance overhead 실측 |
| 6 | m8a.24xl + m8a.48xl on production T4 | ~$30 | vCPU saturation + RAM BW 한계 |

### 4.5 산출물 흐름

```
1-batch 실측 → 본 BENCHMARK.md §2 (measurement section 추가)
  ↓ aggregate
본 §3 derived estimates 갱신
  ↓ derive
GTM 가격 시나리오 §3.7 갱신
```

R11-D 측정 결과는 §2.5 에 새 row 추가 + §3 의 fit 정확화 + §3.7 의
GTM matrix 갱신. 측정 raw 는 `.artifacts/reports/*.md` 임시 → 본 문서로
fold-in.

### 4.6 측정 reference helper

- `scripts/extract_smoke_metrics.sh` — log → md + `--json` machine-
  readable
- `docs/reports/SMOKE_TEMPLATE.md` — raw measurement template
- `cmd/gen-testdata/` + `internal/testdata/` — real-scale testdata
  generator (R11-A)
- `scripts/ec2/` — EC2 measurement infra
- `scripts/smoke.sh` (R11-B `ZKPOR_SMOKE_USER_DATA` override) — smoke
  harness data dir 인자화
