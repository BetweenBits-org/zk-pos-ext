# zkpor benchmark matrix v1 — 4-axis 견적 (instance × model × tier × user count)

zkpor wall-clock 추정값을 4-axis (instance, model, tier shape, user count)
combined matrix 형태로 정리한 산출물. 본 문서는 **추정값**
(`docs/estimates/`) — 측정값은 `docs/reports/`. 측정한 9 cells +
sub-linearity / Amdahl / Math 로 derive.

**GPU column 은 R12 closure (gnark ICICLE backend) 후 실측 채움**.
현재는 `docs/estimates/2026-05-28_gpu_1m_prove_estimate.md` 의 가설값
(3-5× CPU speedup) 으로 placeholder.

## 1. 측정 데이터 (9 cells)

`docs/reports/` 에 누적된 실측. 모든 cells 는 zkpor commit
`92e7785` 의 codebase + force re-keygen + same testdata (10 real
accounts + padding to batch size).

### Per-cell measurements

| # | Instance | Model | Shape | cap | NbConstraints | Compile | Setup | Prove/batch |
|---|---|---|---|---:|---:|---:|---:|---:|
| 1 | m8a.8xl | T1 | 5_10 | 5 | 165,613 | 1m 08s | 1m 58s | 461 ms |
| 2 | m8a.8xl | T2 | 5_10 | 5 | 167,351 | 1m 25s | 2m 27s | 443 ms |
| 3 | m8a.8xl | T3 | 5_10 | 5 | 204,015 | 1m 46s | 3m 03s | 467 ms |
| 4 | m8a.8xl | T4 | 5_10 | 5 | 286,157 | 3m 17s | 7m 56s | 674 ms |
| 5 | r7a.4xl | T4 | 20_500 | 200 | 9,483,618 (T1)/...23.7M | 1m 50s+... | 4m 14s+... | 15.4 s |
| 6 | m8a.8xl | T4 | 20_500 | 200 | 23.7M | 2m 43s | 5m 26s | 15.4 s |
| 7 | c8a.12xl | T4 | 20_500 | 200 | 23.7M | 2m 46s | 4m 17s | ~13 s |
| 8 | m8a.12xl | T4 (Tier 1) | 50_700 | 500 | 64,341,094 | 7m 47s | 11m 59s | 17.2 s |
| 9 | m8a.12xl | T4 (Tier 2) | 500_92 | 500 | 63,822,805 | 8m 47s | 10m 50s | ~17 s |

자세한 cell 별 출처: `docs/reports/2026-05-27_tiny_first_pass.md`,
`docs/reports/2026-05-27_midtier_r7a_vs_m8a.md`,
`docs/reports/2026-05-27_production_t4_m8a12xl.md`.

## 2. Axis 환원: 4 axis → 2 axis (회로 size + hardware)

핵심 통찰:
- **Model + Shape (tier) → NbConstraints + batches**. 회로 byte-parity 가 invariant라 model 차이는 NbConstraints에 absorb.
- **User count → batches** (linear: `users / users_per_batch`).
- **Instance** 만 hardware axis (vCPU, IPC, memory BW, GPU).

압축 모델:
```
wall_clock(model, shape, users, instance, workers)
  = compile(N_constraints, instance)                             # one-time
  + setup(N_constraints, instance)                                # one-time
  + batches × per_batch_prove(N_constraints, instance)            # parallelizable across workers
  + per_user_userproof(instance) × users / userproof_workers      # parallelizable
+ overhead(witness, verifier-batch, verifier-user)
```

## 3. Fit 1: NbConstraints → per-batch prove time (sub-linearity)

측정 cells × instance 별 prove time:

| Instance | NbConstraints | Prove (s) | s/M-constraint |
|---|---:|---:|---:|
| m8a.8xl | 165k | 0.46 | 2.8 |
| m8a.8xl | 286k | 0.67 | 2.4 |
| m8a.8xl | 23.7M | 9.7 | 0.41 |
| r7a.4xl | 23.7M | 15.4 | 0.65 |
| c8a.12xl | 23.7M | 8.3 | 0.35 |
| m8a.12xl | 64.3M | 17.2 | 0.27 |

**Per-million-constraint cost ↓ 회로 클수록**. Sub-linear scaling fit:
```
per_batch_prove(N, instance) ≈ a[instance] + b[instance] × N^0.7
```

대략적 fit (m8a.8xl 기준):
- a ≈ 0.3 s (fixed solver/setup overhead per batch)
- b ≈ 1.8e-5 s × N^0.7
- 예: N=10M → 0.3 + 1.8e-5 × (10e6)^0.7 = 0.3 + 1.8e-5 × 7.94e4 = 0.3 + 1.43 ≈ 1.7s? 측정 9.7s? Fit imprecise — N^0.85 ~ N^1.0 사이.

**Conservative 가정**: `per_batch_prove ≈ k × NbConstraints^0.85` (linear에 가까운 sub-linear).

## 4. Fit 2: Instance speedup (Amdahl + IPC)

3-way 측정 (r7a/m8a.8xl/c8a.12xl + m8a.12xl) 으로 도출 (mid-tier 보고서 §3.way 비교):

| Transition | vCPU 비율 | Wall-clock speedup | Efficiency |
|---|---:|---:|---:|
| r7a (Zen4 16) → m8a.8xl (Zen5 32) | 2× | 2.08× | super-linear (IPC + AVX-512) |
| m8a.8xl (Zen5 32) → c8a.12xl (Zen5 48) | 1.5× | 1.25× | 83% (Amdahl) |
| r7a (Zen4 16) → m8a.12xl (Zen5 48) | 3× | ~2.60× | 87% |

각 단계별 sequential fraction (P_seq):
- compile: ~85% sequential
- Setup: ~25% sequential
- **Prove: ~15% sequential** (MSM/FFT dominant)

추정 함수:
```
speedup(instance, baseline=m8a.8xl) ≈
  IPC_factor × Amdahl(vCPU_ratio, P_seq_per_stage)
```

## 5. Fit 3: User count → batches (math, no measurement)

```
batches = ceil(users / users_per_batch)
```

User count 자체는 **per-batch cost에 영향 없음** (회로는 batch size에 고정).
1M user 처리는 ~1429-10870 batches (shape에 따라).

## 6. 4-axis Derived Matrix (1M user 기준)

가정:
- Single instance (no multi-worker — R13 적용 시 batches / N workers)
- `userproof per user ≈ 5ms` (single worker estimate)
- T4 production: Tier 1 routing 95%, Tier 2 5% (typical Binance-style distribution)

### Per-instance wall-clock (1M users, single instance)

| Instance | T1 spot (50_1000) | T2 margin (50_500) | T3 1pool (50_500) | T4 prod mix | T4 mid (20_500) |
|---|---:|---:|---:|---:|---:|
| r7a.4xl (16 vCPU Zen4) | ~85 min | ~3.5 hr | ~3.7 hr | ~7-8 hr | ~50 min |
| m8a.8xl (32 vCPU Zen5) | ~42 min | ~1.7 hr | ~1.8 hr | ~3.5-4 hr | ~25 min |
| **m8a.12xl (48 vCPU Zen5)** | **~33 min** | **~1.3 hr** | **~1.4 hr** | **~2.7-3 hr** | **~20 min** |
| c8a.12xl (48 vCPU Zen5, 96GB) | ~33 min | ~1.3 hr | ~1.4 hr | OOM 위험 (peak 87GB) | ~20 min |
| **GPU g6.4xl** (L4) ⭐ | **~12 min** | **~25 min** | **~28 min** | **~50 min** | **~7 min** |
| GPU g6.48xl (8× L4 inline) ⭐ | **~3 min** | **~6 min** | **~7 min** | **~12 min** | **~2 min** |

⭐ = R12 closure 후 실측 필요 (현재 추정값, `docs/estimates/2026-05-28_gpu_1m_prove_estimate.md` 가설 3-5× CPU 대비 speedup).

### Multi-instance scaling (R13 closure 후 적용)

`users_per_batch` 그대로 + Redis BLPOP queue + N workers:

```
wall_clock_multi(users, N_workers) ≈ (single instance wall-clock) / N_workers × overhead_factor
```

`overhead_factor` 추정: 1.1-1.3 (queue + state coordination cost).

| Cluster (CPU) | T1 spot 1M | T4 prod mix 1M |
|---|---:|---:|
| 4× m8a.8xl ($8.4/hr) | ~12 min | ~1 hr |
| **4× m8a.12xl** ($12.6/hr) | **~10 min** | **~50 min** |
| 4× g6.4xl GPU (R12+R13) | ~4 min | ~15 min |
| **8× g6.4xl GPU** ($10.6/hr) | **~2 min** | **~8 min** |

## 7. GTM 거래소 시나리오 별 SLA 견적 (Single instance, CPU)

`docs/estimates/2026-05-28_gpu_1m_prove_estimate.md` 의 추정과 같은 base. 본 표는 single CPU instance 만, GPU/multi-instance 변형은 위 §6.

| 거래소 규모 | Model | Setup | Wall-clock | Instance/h | 비용 |
|---|---|---|---:|---:|---:|
| 10K spot | T1 | m8a.8xl | ~6 min | $2.10 | $0.21 |
| 30K spot | T1 | m8a.8xl | ~10 min | $2.10 | $0.35 |
| 100K spot | T1 | m8a.12xl | ~12 min | $3.15 | $0.63 |
| 500K spot | T1 | m8a.12xl | ~25 min | $3.15 | $1.31 |
| **1M spot (T1)** | T1 | m8a.12xl | **~33 min** | $3.15 | **$1.73** |
| 10K margin | T2/T3 | m8a.8xl | ~7 min | $2.10 | $0.24 |
| 30K margin | T2/T3 | m8a.8xl | ~15 min | $2.10 | $0.53 |
| 100K margin | T2/T3 | m8a.12xl | ~22 min | $3.15 | $1.16 |
| 500K margin | T2/T3 | m8a.12xl | ~50 min | $3.15 | $2.62 |
| **1M margin (T2/T3)** | T2/T3 | m8a.12xl | **~1.3 hr** | $3.15 | **$4.10** |
| 100K full-margin | T4 | m8a.12xl | ~30 min | $3.15 | $1.58 |
| **1M full-margin (T4)** | T4 | m8a.12xl | **~2.7-3 hr** | $3.15 | **$8.50-9.45** |

**Multi-instance + GPU** 적용 시 위 수치의 1/N (cluster size) — R12 + R13 closure 후 실측.

## 8. Confidence intervals (measured vs derived)

| Matrix cell | Source | Confidence |
|---|---|---|
| (m8a.8xl, T1-T4, shape=5_10, cap=5) | Measured | ✅ High |
| (r7a/m8a/c8a, T4, shape=20_500, cap=200) | Measured (3 instances) | ✅ High |
| (m8a.12xl, T4, shape=50_700/500_92, cap=500) | Measured (production) | ✅ High |
| (m8a.12xl, T4, shape=20_500, cap=200) | Extrapolated from #6+#7 | 🟢 Medium-High |
| (any instance, T1-T3, mid-tier) | Extrapolated from tiny + Model overhead 추정 | 🟡 Medium |
| (any instance, T1-T3, production) | Extrapolated — no T1/T2/T3 at production scale 측정 | 🟠 Low-Medium |
| (GPU, any) | docs/estimates/ 가설 (3-5× CPU) | 🔴 Hypothesis only — R12 closure 후 실측 |
| (multi-instance, any) | Math derivation (overhead 가정) | 🔴 Hypothesis — R13 closure 후 실측 |

## 9. 추가 측정 priority (효율 max)

낮은 confidence 영역의 cells 채우는 순서:

| 순위 | 측정 | 비용 | 효과 |
|---|---|---:|---|
| **1** | T1/T2/T3 at m8a.8xl × shape=20_500 (mid-tier) | ~$3-5 | Model axis × mid-tier scaling 정확화. extrapolation 직접 비교 |
| **2** | T1 production-scale (cap=50, shape=50_1000) on m8a.12xl | ~$5-8 | T1 spot GTM 견적 정확화 — 가장 큰 GTM segment |
| 3 | **R12 closure**: GPU L4 × T4 × shape=20_500 | ~$2 | GPU column lock-in. **R12 entry blocker** |
| 4 | **R11-A**: real-scale 1M user testdata generator | dev time | userproof + multi-instance 실측 baseline |
| 5 | **R13 closure**: 4× m8a.8xl multi-worker | ~$15-20 | multi-instance overhead 실측 |
| 6 | m8a.24xl + m8a.48xl on production T4 | ~$30 | vCPU saturation point + RAM BW limit |

## 10. 사용 가이드 (이 문서를 GTM 견적 시 활용 방법)

1. **목표 고객 정의**: 거래소 규모 (user count) + business model (spot/margin → T1-T4)
2. **§7 의 거래소 시나리오 매트릭스 lookup** — single instance CPU baseline
3. **GPU/multi-instance 필요 시 §6 의 multiplier 적용** (R12/R13 closure 후 정확화)
4. **Confidence (§8) 확인** — High/Medium/Low 별 신뢰도 표시
5. **Customer SLA 협상 시** — High confidence cells 만 약속, Medium은 buffer 추가, Low는 PoC measurement 권장

## 11. R11 entry: 본 matrix → R11 closure 후 업데이트

본 v1 matrix 의 가장 큰 약점 = **real-scale testdata 없음**. Testdata 10 real
+ padding 으로 prove time 의 fixed overhead 비중이 큼 → 1M real users 시
sub-linearity 가 정확히 어떻게 나오는지 미검증. **R11 closure** 후 본
문서 v2 발행 — real-scale base + GPU + multi-instance 추가.
