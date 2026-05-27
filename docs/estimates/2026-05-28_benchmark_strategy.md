# zkpor benchmark strategy (Setup vs Prove 분리 방법론)

zkpor 의 benchmark 측정 방법론 + ablation 디자인 + minimum viable plan.
실제 측정값은 `docs/reports/`, 종합 견적은 `docs/estimates/2026-05-28_
benchmark_matrix.md`. 본 문서는 **방법론** (어떻게 측정해야 하는지) —
R11 closure 후 실제 측정의 reference.

## 1. 핵심 인사이트: 4-axis → 2-phase 환원

당초 4 axis (instance, model, tier shape, user count) 완전 조합 시 ~500
cells — 비효율. 실제로는 **Setup vs Prove 두 phase 만 별도 측정** 하면
대부분의 GTM/운영 질문이 답해진다:

| Phase | 특성 | 측정 빈도 | 자원 |
|---|---|---|---|
| **Setup (keygen)** | 1회성 (per `<model, shape, capacity>` tuple) | 인스턴스 선택 일회 결정 후 평생 cache hit | RAM 집중 (T4 production peak 87GB) |
| **Prove (groth16.Prove)** | 반복적 (snapshot 마다 batches × N) | 운영 SLA 핵심 | vCPU/AVX-512 집중, RAM 적음 (~25-30GB) |

→ Setup 과 Prove 가 **자원 프로파일 다름** + **운영 빈도 다름** + **인스턴스
요구 다름**. 따라서 측정 + instance 선택 모두 분리해서 다룸.

## 2. Setup phase — instance OOM 경계만 lock

Setup 은 cache hit 되면 추가 비용 0 (peer file system 또는 EBS volume 에
.pk 저장 후 평생 재사용). 즉 instance 선택 가이드만 정확하면 됨.

### 2.1 측정 cells

각 `<model, shape, capacity>` production tuple 당 2 cells:

| 측정 | 목적 |
|---|---|
| **OOM 측정** (RAM 부족 instance) | 어디서 fail 하는지 + peak RAM 확인 → OOM 경계 lock |
| **OK 측정** (RAM 안전 instance) | 통과 시간 + actual peak → 운영 instance 가이드 |

### 2.2 예: T4 production (cap=500, shape 50_700+500_92)

| Instance | RAM | 예상 | 측정 목적 |
|---|---:|---|---|
| m8a.4xl | 64 GB | **OOM** (peak 87GB > 64GB) | fail point + peak 직전 메모리 dump |
| m8a.8xl | 128 GB | 통과 | setup 시간 + actual peak baseline |

운영 가이드 결과: **"T4 production keygen은 128GB+ instance"** — 추가 측정
없이 결정. 다른 instance 종류는 비용/속도 trade-off만 (선택). 한 번
측정 후 평생.

### 2.3 Setup 측정 추가 안 하는 영역

- Setup 시간 자체 — Amdahl curve 가 prove와 유사하므로 mid-tier 3-way
  비교 (이미 측정) 의 fit 으로 derive.
- Mid-tier (cap=200 이하) setup — peak RAM 30GB 이하라 64GB instance 모두
  OK. OOM 경계 의미 없음.
- Cross-instance 비교 — 1회성 cost 라 ROI 낮음. mid-tier setup time 으로
  scaling 추정.

## 3. Prove phase — 핵심 측정 영역

`cmd/prover/main.go:106-122` 의 **PK in-memory + multi-batch lazy reload**
구조 확인:

```go
var params snarkParams                // outer scope, persists
for {
    row, err := witnessStore.ClaimOldestByStatus(...)
    proveOne(row, &params, plan, ...)  // params 재사용
}
// loadSnarkParams 안:
//   if params.tier == targetTier && params.r1cs != nil { return nil }
//   else read .r1cs / .pk / .vk from disk
```

→ PK + R1CS 한 번 메모리 적재 후 multiple batches process. Tier 변경 시에만
reload (T4 production 시 Tier 1 ↔ Tier 2 전환 1회).

### 3.1 Prove 메모리 budget (T4 production 기준)

| 항목 | 크기 |
|---|---:|
| `.pk` 적재 | ~12 GiB |
| `.r1cs` 적재 | ~3-4 GiB |
| groth16.Prove intermediate (MSM working set) | ~5-10 GiB |
| per-batch witness | ~수십 MB |
| **Prove peak RAM** | **~25-30 GiB** |

→ **m8a.4xl (64GB) 도 prove 가능**. Setup 만 다른 instance 에서 (one-time)
한 후 `.pk` 24GB 만 메모리 적재 + working set.

### 3.2 Prove ablation 디자인 (세대차이 + 코어수차이 분리)

**3 instances 만으로 깔끔한 ablation**:

| Pair | 변수 | 측정하는 것 |
|---|---|---|
| **m7a.4xl vs m8a.4xl** | 세대 (Zen4 → Zen5) | IPC + AVX-512 + DDR5 BW |
| **m8a.4xl vs m8a.8xl** | 코어수 (16 → 32 vCPU) | Amdahl curve |
| (m7a.4xl vs m8a.8xl) | 둘 다 | 통합 효과 |

→ 3 instances × 3 shape (T4 mid 20_500 / Tier 1 50_700 / Tier 2 500_92)
= **9 cells** 로 instance × shape matrix 완성.

### 3.3 User count 는 math derive (실측 불요)

`cmd/prover` 의 multi-batch lazy reload → per-batch prove time 은 batches
수에 독립.

```
total_prove_time = batches × per_batch_prove_time
batches = ceil(users / users_per_batch)
```

→ **1-batch 실측 후 batches math 만 곱하면 N users prove time 추정**.

R11-A testdata generator 완성 후 multi-batch sanity (GC/메모리 drift)
만 **1회** 검증하면 linear scaling 가정 lock.

### 3.4 Prove 측정 추가 안 하는 영역

- User count 1k / 10k / 100k 등 실측 — math derive 으로 충분 (per-batch ×
  batches). multi-batch sanity 1 cell 로 충분.
- Multi-model (T1/T2/T3) at production scale — production T4 가 가장 큰
  case. 작은 모델은 NbConstraints 변동으로 fit 가능.
- Cross-instance × all-shapes 완전 매트릭스 — Amdahl fit + IPC factor
  로 derive.

## 4. Minimum viable benchmark plan

총 **11 cells, ~$15-20, ~3-4hr** 측정 (실제 진행은 R11 closure 후):

| 구분 | Cells | 비용 (대략) | 측정 instance |
|---|---:|---:|---|
| Setup OOM | 1 | ~$1 | m8a.4xl (T4 production setup attempt) |
| Setup OK | 1 | ~$4 | m8a.8xl (T4 production setup full) |
| Prove × 3 shapes × 3 instances | 9 | ~$10-15 | m7a.4xl, m8a.4xl, m8a.8xl |
| **합** | **11** | **~$15-20** | |

이걸로 **모든 4-axis cells 의 fit + derive 가능**:
- Instance axis: 3 instances (세대 + 코어 ablation)
- Shape axis: 3 production shapes
- Model axis: NbConstraints 로 환원 (다른 model은 derive)
- User axis: math derive

## 5. Instance type-switch 전략 (single instance, cost 효율)

`.pk` 24GB EBS 공유로 측정 instance 마다 keygen 재실행 불요:

```
launch m8a.4xl (us-east-1c, gp3 150GB EBS)
  ↓ setup attempt → OOM 측정 (T4 production)
  ↓ stop + type change → m8a.8xl
setup OK 측정 + prove 측정 (3 cells)
  ↓ stop + type change → m7a.4xl
prove 측정 (3 cells, .pk 재활용 from EBS)
  ↓ stop + type change → m8a.4xl
prove 측정 (3 cells, .pk 재활용)
  ↓ stop + terminate (EBS DeleteOnTermination=true)
```

### 5.1 단점 + 대안

- **Capacity 변동 시 type-switch fail** 가능 (us-east-1c 의 m8a 12xl+
  처럼 InsufficientInstanceCapacity).
- **대안**: 다른 AZ 에 새 EBS + 인스턴스 launch (production T4 측정 시
  사용한 pattern). bootstrap.sh + sync.sh 재실행 ~5min overhead.

## 6. R12 (GPU) 이후 추가 측정

R12 closure (ICICLE backend) 후:
- **Single cell 측정**: g6.4xl (L4) × T4 mid (20_500)
- GPU multiplier 가설 (3-5× CPU) 실측 검증
- `docs/estimates/2026-05-28_gpu_1m_prove_estimate.md` 의 추정값 lock-in
- Matrix v2 의 GPU column fill

추가 측정 (선택):
- g6e.4xl (L40S, 48GB VRAM) — 더 큰 회로 사양 검증
- Multi-GPU instance (g6.12xl) — instance 단일 다중 GPU contention

## 7. R13 (Multi-instance) 이후 추가 측정

R13 closure (Redis BLPOP queue) 후:
- **4× m8a.8xl multi-worker × T4 production**
- Multi-instance overhead 실측 (queue + state coordination cost)
- Matrix v2 의 multi-instance throughput section fill

추가 측정 (선택):
- 8× multi-worker — vCPU saturation 확인
- GPU multi-instance (4× g6.4xl + Redis) — GTM SLA 견적의 base

## 8. 측정 결과의 보고서 흐름

```
1-batch 실측 → docs/reports/2026-XX-XX_<phase>_<instance>.md (cell 측정 raw)
  ↓ aggregate
docs/estimates/2026-XX-XX_benchmark_matrix_v2.md (4-axis matrix update)
  ↓ derive
docs/estimates/2026-XX-XX_gtm_pricing_grid.md (GTM 가격표 — customer-facing)
```

세 단계 분리:
- `reports/` = raw measurement (per cell)
- `estimates/benchmark_matrix_*.md` = derived 4-axis grid
- `estimates/*gtm*.md` = customer-facing GTM pricing (R11+ 후 작성)

## 9. R11 시작 후 진행 plan

R11 closure 가 우선:

1. **R11-A**: real-scale testdata generator
   - `scripts/gen_testdata.sh` 또는 cmd
   - 입력: model, capacity, target account count, asset distribution
   - 출력: `profile/<>/testdata/scale_<N>/{accounts,cex_assets[,tier_ratios]}.csv`
2. **R11-B**: smoke.sh sample-data 인자화
   - `-snapshot <path>` 또는 env 로 testdata 경로
   - 현재 `profile/<>/testdata/happy/` 하드코딩 제거
3. **R11-C**: extract_smoke_metrics 확장
   - total prove time, per-batch average, userproof per-account
4. **R11-D 측정** = §4 의 11-cell minimum viable plan 실행 (본 문서 reference)

본 strategy 가 R11-D 실행의 가이드 — 측정 시 본 문서 따르며,
결과는 matrix v2 로 보강.

## 10. 향후 변경 시 갱신

본 strategy 는 method 문서 — `cmd/prover` 의 multi-batch reload 구조,
4-axis 환원 정의, ablation 디자인이 바뀌면 갱신.

- 새 instance generation (m9a, c9a 등 — 미래) 추가 시 §3.2 ablation 표 보강
- GPU/multi-instance 측정 후 §6 / §7 의 가설값을 측정값으로 격상
- R11-D 측정 결과로 §4 의 minimum viable plan 정확도 검증 → strategy v2
