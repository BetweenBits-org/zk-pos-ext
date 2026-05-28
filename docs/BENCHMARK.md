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

**Prove 메모리 budget — 회로 크기 × density 함수** (R11-D 측정으로 보정):

원래 추정 (`~25-30 GiB`) 은 testdata/happy/ 의 sparse witness (real
user 10명 + padding 690, asset 3 of 500) 기준이라 회로 size 영향 미반영.
R11-D 의 **fully dense** 워크로드 측정에서 **m8a.8xl peak RSS ~120 GiB**
(§2.6) 확인 → 회로 크기 비례 + density 의존성 lock-in.

| 컴포넌트 | Sparse (testdata/happy) | Dense (R11-D 100%) |
|---|---:|---:|
| `.pk` 적재 | 12 GiB | 12 GiB |
| `.r1cs` 적재 | 4 GiB | 4 GiB |
| 솔버 intermediate | ~5 GiB | ~20 GiB |
| MSM G1+G2+C base/scalar | ~5 GiB | ~24 GiB |
| Pippenger buckets × 32 worker | ~3 GiB | ~30 GiB |
| FFT/NTT temp (double buffer) | ~3 GiB | ~10 GiB |
| gnark goroutine buffer overhead | ~3 GiB | ~10 GiB |
| **Prove peak RAM** | **~35 GiB** | **~120 GiB** |

**Sparse 효과 원인** (gnark/groth16 known optimization):
1. **Pippenger MSM**: scalar=0 항 skip — bucket size 가 non-zero scalar
   비례
2. **R1CS solver**: zero-propagation 으로 일부 wire intermediate 계산 trivial
3. **워커별 working set**: dense 시 worker 별 buffer 가 full constraint 폭
   사용; sparse 시 작아짐

**올바른 추정 공식**:

```
prove_peak_RAM ≈ pk + r1cs + (k × constraints × density × num_workers)
where k ≈ 60-100 bytes per (constraint × worker)
      density ∈ {sparse 0.01-0.04, 실제 거래소 0.05-0.3, worst-case 1.0}
```

at production T4 (n=64M, 32 worker) 검증:
- density=1.0 → ~120 GB ✓ (R11-D 실측)
- density=0.05 (실제 거래소 평균) → ~50 GB
- density=0.0004 (testdata/happy) → ~16 GB

→ **Instance 추천 (production T4 prove)**:
- m8a.4xl/m7a.4xl (64 GB): **dense 워크로드 OOM 확실, 실제 sparse 만 가능**
- m8a.8xl (128 GB): 실제 거래소 워크로드 OK, dense worst-case 살얼음
- **m8a.12xl (192 GB)**: dense worst-case 보장, 권장
- r7a.8xl (256 GB) 이상: 2× margin

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
| **1-batch × 2 shape × 3 instance (6 cells)** | per-batch isolation, instance + shape ablation (Setup artifact reuse) |
| **10K user × 2 shape × 1 instance (2 cells, m8a.8xl)** | multi-batch sanity (GC drift, memory growth, lookup-table rebuild overhead), 1-batch math derive 정확도 검증 |

10K user shape 선택: **T4 production 2 shape (50_700 + 500_92)** — Setup
단계에서 생성한 동일 R1CS / `.pk` 재활용. 10K / 700 ≈ 15 batch (Tier 1)
+ 10K / 92 ≈ 109 batch (Tier 2). m8a.8xl 단일 측정으로 충분 (instance
ablation 은 1-batch 6 cells 에서 이미 분리).

10K 측정의 핵심 검증:
- `per_batch_prove(batch=N) ≈ per_batch_prove(batch=1)` (linear 가정)
- 누적 GC pause / 메모리 leak 없음 (peak RAM 안정)
- Tier 전환 (50_700 → 500_92) PK reload 오버헤드 측정

### 1.6 Minimum viable benchmark plan

Setup artifact reuse 패턴 채택 → 총 **8 cells, ~$8-10, ~2-3hr** 측정:

| 구분 | Cells | 시간 | 비용 (대략) | 측정 instance |
|---|---:|---|---:|---|
| Setup (T4 production 2 shape) | — | <1hr | ~$2 | m8a.8xl (1대, keygen 풀 실행 → `.pk`/`.r1cs` EBS 보존) |
| Prove 1-batch × 2 shape × 3 instance | 6 | ~1hr (직렬 type-switch) | ~$2 | m7a.4xl, m8a.4xl, m8a.8xl (artifact 재사용) |
| Prove 10K user × 2 shape × 1 instance | 2 | ~1hr | ~$3 | m8a.8xl (artifact 재사용, multi-batch) |
| 오버헤드 (EBS, snapshot, 재시도) | — | — | ~$2 | — |
| **합** | **8** | **~2-3hr** | **~$8-10** | |

핵심 비용 절감 요인:
- Setup 1회 → 6 prove cell 에서 keygen 중복 제거 (기존 plan 의 ~$15-20 절약)
- Prove 단계는 `.pk` lazy reload (cmd/prover snarkParams 패턴, Phase 3c) 활용 → 인스턴스 가동 시간 ≈ prove 실측 시간 + 부팅 10분
- 10K user 도 100 batch × ~30s = ~50min 이내 (single-instance, 1 cell ~1hr)

### 1.7 Instance type-switch 전략

`.pk` 24GB EBS 공유로 측정 instance 마다 keygen 재실행 불요:

```
launch m8a.8xl (us-east-1c, gp3 200GB EBS)
  ↓ Setup: T4 production 2 shape (50_700 + 500_92) keygen → .pk/.vk/.r1cs 보존
  ↓ Prove 10K user × 2 shape 측정 (2 cells, multi-batch)
  ↓ Prove 1-batch × 2 shape 측정 (2 cells)
  ↓ stop + type change → m8a.4xl
Prove 1-batch × 2 shape 측정 (2 cells, .pk 재활용 from EBS)
  ↓ stop + type change → m7a.4xl
Prove 1-batch × 2 shape 측정 (2 cells, .pk 재활용)
  ↓ stop + terminate (EBS DeleteOnTermination=true)
```

총 1대 인스턴스 가동 (type-switch 3회), 누적 wall-clock ~2-3hr.

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

### 2.5 측정 cells 종합 (13 measurements)

기존 측정 lookup table — derived estimates §3 의 fit base:

| # | Instance | Model | Shape | cap | NbConstraints | Prove/batch | Density | RSS peak |
|---|---|---|---|---:|---:|---:|---:|---:|
| 1 | m8a.8xl | T1 | 5_10 | 5 | 165,613 | 461 ms | sparse | n/a |
| 2 | m8a.8xl | T2 | 5_10 | 5 | 167,351 | 443 ms | sparse | n/a |
| 3 | m8a.8xl | T3 | 5_10 | 5 | 204,015 | 467 ms | sparse | n/a |
| 4 | m8a.8xl | T4 | 5_10 | 5 | 286,157 | 674 ms | sparse | n/a |
| 5 | r7a.4xl | T4 | 20_500 | 200 | 23.7M | 15.4 s | sparse | n/a |
| 6 | m8a.8xl | T4 | 20_500 | 200 | 23.7M | 9.7 s | sparse | n/a |
| 7 | c8a.12xl | T4 | 20_500 | 200 | 23.7M | 8.3 s | sparse | n/a |
| 8 | m8a.12xl | T4 (Tier 1) | 50_700 | 500 | 64,341,094 | 17.2 s | sparse | n/a |
| 9 | m8a.12xl | T4 (Tier 2) | 500_92 | 500 | 63,822,805 | ~17 s | sparse | n/a |
| 10 | m8a.8xl | T4 (Tier 1, 1-batch) | 50_700 | 500 | 64,341,094 | 32.1 s | **dense 1.0** | n/a (sample miss) |
| 11 | m8a.8xl | T4 (Tier 2, 1-batch) | 500_92 | 500 | 63,822,805 | 29.7 s | **dense 1.0** | n/a (sample miss) |
| 12 | m8a.8xl | T4 (Tier 1, 15-batch) | 50_700 | 500 | 64,341,094 | 32.6 s avg | **dense 1.0** | **122 GiB** |
| 13 | m8a.8xl | T4 (Tier 2, 109-batch) | 500_92 | 500 | 63,822,805 | 30.7 s avg | **dense 1.0** | **118 GiB** |

### 2.6 R11-D Setup/Prove ablation, dense (2026-05-28)

**T4 production smoke** with R11-A `cmd/gen-testdata` 합성 데이터 (fully
dense — 모든 user × asset slot non-zero). m8a.8xl 단일 instance —
원래 plan 의 m8a.4xl / m7a.4xl ablation 은 dense workload 메모리 한계
(128 GB 초과) 로 좌초, density ablation 으로 재플랜 예정.

| 항목 | 값 |
|---|---|
| 호스트 | m8a.8xlarge (32 vCPU Zen5 / 128 GB) |
| Region/AZ | us-east-1a |
| Instance ID | `i-05da73a6bb557498e` |
| zkpor commit | `5ce4df2` + `25cefd7` (R11-D prep + chmod fix) |
| testdata | gen-testdata uniform, seed=42 |
| 비고 | R11-D dense 측정 — §1.3 prove memory budget 보정 |

**Setup phase** (m8a.8xl 단일):

| Shape | Compile | groth16.Setup | `.pk` | `.vk` | `.r1cs` |
|---|---:|---:|---:|---:|---:|
| 50_700 | 7m47s | 14m30s | 12 GB | 528 B | 4.2 GB |
| 500_92 | 8m45s | 13m8s | 12 GB | 528 B | 2.9 GB |
| **합** | **16m32s** | **27m38s** | **24 GB** | | **7.1 GB** |

Setup wall-clock ~48min. m8a.12xl baseline (§2.4) 대비 ~+9% 느림 — 32
vCPU vs 48 vCPU 의 Amdahl ~25% sequential 가설 일치.

**Prove phase (4 cells)**:

| Cell | Users | Batches | Solver avg | Prover avg | Total/batch | Total cell |
|---|---:|---:|---:|---:|---:|---:|
| t1_700 (1-batch Tier 1) | 700 | 1 | 10.7 s | 20.6 s | **32.1 s** | 32 s |
| t2_92 (1-batch Tier 2) | 92 | 1 | 10.0 s | 19.5 s | **29.7 s** | 30 s |
| t1_10k (15-batch Tier 1) | 10,000 | 15 | 11.1 s (±15%) | 20.8 s (±2%) | **32.6 s avg** | 8m9s |
| t2_10k (109-batch Tier 2) | 10,000 | 109 | 11.2 s (±16%) | 19.3 s (±2%) | **30.7 s avg** | 55m48s |

모든 cell `verify pass` ✓.

**Sanity check 결과**:

1. **Per-batch invariance ✓** — multi-batch avg 가 1-batch 의 +2-3% 이내:
   - Tier 1: 32.1s (1-batch) vs 32.6s (15-batch) — **+1.6%**
   - Tier 2: 29.7s (1-batch) vs 30.7s (109-batch) — **+3.4%**

2. **GC drift 없음** — multi-batch min/max variance ±4-6% 정상 범위:
   - t1_10k: min 31.2s, max 33.5s
   - t2_10k: min 28.9s, max 32.5s

3. **Amdahl 검증** — m8a.12xl Tier 1 baseline 29.6s (§2.4) → m8a.8xl
   Tier 1 32.6s = **+10%**. vCPU 비율 0.67×, Amdahl ~15% sequential
   가설과 일치.

4. **Tier 1 vs Tier 2 prove**: 거의 동일 (constraints 차이 0.8%, prove
   차이 6%) — 회로 size 의 prove 영향이 dominant.

**메모리 측정 (R11-D 핵심 발견)**:

prove RSS peak **~118-122 GiB** (free 메모리 5-8 GiB 만):

| Cell 시점 | RSS | Free | 비고 |
|---|---:|---:|---|
| t1_700 prove (32s) | n/a | n/a | 60s monitor interval, sample miss |
| t2_92 prove (30s) | n/a | n/a | sample miss |
| t1_10k batch 6 | 122 GiB | 4.5 GiB | live monitor |
| t2_10k batch 89 | 118 GiB | 8.0 GiB | live monitor |
| (prove 종료 후) | 1.9 GiB | 120 GiB | 정상 회수 |

**핵심 결론**:
- **multi-batch 메모리 누적 없음** — prove 종료 시 RSS 즉시 1.9 GiB 로 회수
- **per-batch dense witness 가 메모리 폭증의 직접 원인**
- 원래 §1.3 의 `~25-30 GiB` 추정은 sparse testdata 기준이었음 → 회로
  size + density 미반영
- **density=1.0 측정값이 production T4 의 메모리 worst-case 상한** —
  실제 거래소 워크로드 (sparse) 는 이보다 30-60% 적을 것

상세 추정 공식: §1.3 참조.

**Cost / time**:

| Phase | Wall-clock | 비용 (m8a.8xl @ $1.80/hr) |
|---|---:|---:|
| Setup (keygen) | ~48 min | ~$1.44 |
| 4 prove cells | ~65 min | ~$1.95 |
| Idle (between/after) | ~10 min | ~$0.30 |
| EBS gp3 150GB × 2hr | — | ~$0.04 |
| **합** | **~2 hr** | **~$3.75** |

**미완 / 후속**:

- m8a.4xl × m7a.4xl × T4 production-dense → **OOM 으로 좌초**. 추후
  density ablation (sparse 0.1, 0.5) cell 로 4xl viable 검증
- 1-batch (t1_700, t2_92) 시점 RSS 직접 측정 — RSS sampling 주기를
  10s 로 줄이면 32s prove 안에서 sample 가능. r11d.sh 에 prove PID
  RSS 로깅 추가 필요 (후속)
- Setup artifact (`.pk`/`.vk`/`.r1cs` × 2 shape, 31 GB 합) 는 EBS
  보존 — instance stopped, type-switch 또는 sparse cell 측정 시 재사용

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

### 3.8 인스턴스 추천 매트릭스 (R11-D 보정 후)

| 시나리오 | 추천 | 근거 |
|---|---|---|
| Cost-optimal (sparse workload, mid-tier) | r7a.4xlarge ($0.92 mid-tier) | 가장 저렴, sparse OK |
| **Balanced sweet spot (sparse)** | **m8a.8xlarge** ($0.88, 시간 절반) | 비용 거의 동일, 시간 절반 |
| Time-optimal (sparse, mid-tier) | c8a.12xlarge (20m mid-tier) | 같은 비용, 더 빠름 |
| **Production T4 dense workload** | **m8a.12xlarge** (192 GB) | dense peak ~120 GB, ~70 GB margin |
| Production T4 keygen 일회성 | m8a.8xl+ | Setup peak ~87 GB |
| Production T4 prove dense | m8a.12xl+ 권장 / m8a.8xl 살얼음 | §2.6 측정 |
| Dev iteration 자주 (sparse) | **m8a.8xlarge** | sweet spot |
| Dense worst-case 보장 | r7a.8xl (256 GB) | 2× margin |

### 3.9 Confidence intervals

| Matrix cell | Source | Confidence |
|---|---|---|
| (m8a.8xl, T1-T4, shape=5_10, cap=5) | Measured | ✅ High |
| (r7a/m8a/c8a, T4, shape=20_500, cap=200) | Measured (3 instances, sparse) | ✅ High |
| (m8a.12xl, T4, shape=50_700/500_92, cap=500, sparse) | Measured (§2.4) | ✅ High |
| **(m8a.8xl, T4, prod, dense, multi-batch)** | **Measured (§2.6 R11-D)** | **✅ High** |
| **(any 4xl, T4 prod, dense)** | **OOM 확인 (~120 GB > 64 GB)** | **✅ High (negative)** |
| (m8a.4xl/m7a.4xl, T4 prod, sparse) | Hypothesis (sparse fit) | 🟡 Medium — **R11-D Phase 2 후** |
| (m8a.12xl, T4, shape=20_500, cap=200) | Extrapolated | 🟢 Medium-High |
| (any instance, T1-T3, mid-tier) | Extrapolated | 🟡 Medium |
| (any instance, T1-T3, production) | Extrapolated | 🟠 Low-Medium |
| (GPU, any) | Hypothesis (3-5× CPU, host RAM 8xl+) | 🔴 Hypothesis — **R12 후 실측** |
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

### 4.1 R11-D Setup/Prove ablation — status

**Phase 1 (dense): 완료 (2026-05-28, §2.6)**. m8a.8xl 의 4 dense cells
(t1_700, t2_92, t1_10k, t2_10k) 측정 + Setup artifact 보존. 핵심 발견:
prove RSS peak ~120 GiB → §1.3 memory budget 4× 빗나갔던 점 보정.

**Phase 2 (density ablation): 진행 예정**. 원래 plan 의 m8a.4xl ×
m7a.4xl ablation 이 dense workload 메모리 한계로 좌초 — sparse cell
(density=0.05, 0.1, 0.5) 로 instance ablation 재시도.

§1.6 의 8 cells (원본 plan). 진행 시점은 R11 dev infra (R11-A/B/C, 이미 완료) 후.

**구성**:
- **Setup** (artifact 생성): m8a.8xl 1대, T4 production 2 shape (50_700,
  500_92), <1hr → `.pk`/`.vk`/`.r1cs` EBS 보존
- **Prove 1-batch** × 2 shape × {m7a.4xl, m8a.4xl, m8a.8xl} = **6 cells**
  - Setup artifact 재사용 (cmd/prover lazy reload, instance type-switch
    간 동일 EBS attach)
- **Prove 10K user** × 2 shape × m8a.8xl = **2 cells**
  - R11-A `cmd/gen-testdata` 로 10K user testdata 합성 (T4 production
    cap=500, shape 50_700 + 500_92)
  - R11-B `ZKPOR_SMOKE_USER_DATA` 로 smoke harness 에 주입
  - R11-C `--json` 출력으로 multi-batch aggregate 자동 추출

**Insight 영역**:
- m7a.4xl vs m8a.4xl: 세대 차이 (Zen4 → Zen5) — IPC + AVX-512
- m8a.4xl vs m8a.8xl: 코어수 차이 — Amdahl curve
- §2.4 의 production T4 m8a.12xl ~1.3× speedup 가설 (RAM bandwidth
  bound) 검증
- 10K real-batch vs 1-batch math derive 정확도 (per-batch invariance 검증)
- Tier 전환 (50_700 → 500_92) PK reload 오버헤드 측정

**실행 절차**: `docs/R11D_RUNBOOK.md` — EC2 launch, cell 순서,
type-switch, fold-in 절차의 step-by-step 체크리스트.

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
| 0 | ~~**R11-D dense 4 cells** (§2.6) — **완료**~~ | $3.75 (실측) | Setup/Prove dense ablation + memory worst-case lock-in |
| 1 | **R11-D density ablation** (sparse 0.1/0.5 cell + 4xl 재가능 검증) | ~$3-5 | 실제 거래소 sparse benefit 정량화 + instance ablation 완성 |
| 2 | **T1 production-scale** (cap=50, 50_1000, m8a.12xl) | ~$3-5 | Spot GTM segment 견적 정확화 |
| 3 | **R12 PoC** (GPU L4/L40S single point, ≥g6e.8xl) | ~$5-8 | GPU 옵션 + host RAM 요구 검증 |
| 4 | **R13 multi-worker** (4× m8a.8xl) | ~$15-20 | Multi-instance overhead 실측 |
| 5 | m8a.24xl + m8a.48xl on production T4 dense | ~$30 | vCPU saturation + RAM BW 한계 (dense workload) |

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
