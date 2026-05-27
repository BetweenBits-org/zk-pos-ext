# GPU 가속 1M 거래소 prove 예상 견적 (R12 기획용)

zkpor 의 prove path 에 GPU backend (ICICLE) 를 도입했을 때 1M users
거래소 시나리오의 예상 prove time + 비용 견적표.

본 문서는 **추정값** (`docs/estimates/`) — 실제 측정값과 분리. 추정 베이스:
- CPU 측정: `docs/reports/2026-05-27_production_t4_m8a12xl.md` (Production T4
  Tier 1 64M constraints, prove 17.2s on m8a.12xlarge)
- ICICLE / GPU speedup: 일반적 zk-SNARK GPU 가속 benchmark
  (Ingonyama, Polygon zkEVM, Aleo 보고)

## 기본 가정

| 항목 | 가정 | 근거 |
|---|---|---|
| MSM/FFT GPU speedup | **3-5×** | ICICLE BN254 vs gnark CPU benchmark (저자 측정값) |
| MSM 비중 in prove time | **70-80%** | groth16 prove의 dominant cost |
| Amdahl 비-MSM 부분 | <1.3× speedup | witness solver 등 |
| GPU memory 충분성 | L4 24GB로 64M constraint .pk (12GB) 적재 가능 | margin 12GB |
| Multi-instance Redis BLPOP queue 작동 | R3 step 4 follow-up 완성 가정 | HANDOFF "prover Redis BLPOP 큐 + multi-worker" |
| 도입 작업 | ICICLE backend integration ~1-1.5일 | gnark fork 호환성 PoC + AMI + bootstrap |

## 단계별 GPU 적용 가능성

| 단계 | GPU 적용 | 추정 speedup |
|---|---|---:|
| `frontend.Compile` (R1CS) | ❌ | 1.0× |
| `groth16.Setup` | △ 가능 (일회성이라 critical 안 함) | 2-3× |
| **`groth16.Prove` MSM** | ✅ **dominant** | **5-10×** |
| **`groth16.Prove` FFT** | ✅ | **3-5×** |
| Witness solver | ❌ | 1.0× |
| `groth16.Verify` | △ (<ms) | 무의미 |

→ Prove 전체로는 **3-5×** speedup (Amdahl considering non-MSM portions).

## Per-batch prove time 추정 (m8a.12xl 측정값 → GPU 변환)

| Model + shape | NbConstraints | CPU prove (m8a.12xl 측정) | **L4 GPU 추정** (3-5×) |
|---|---:|---:|---:|
| T1 spot (50_1000) | ~50M | ~10s | **~2-3s** |
| T2/T3 mid-margin (50_500) | ~30-40M | ~12s | **~3-4s** |
| T4 Tier 1 (50_700) | 64.3M (실측) | **17.2s** (실측) | **~4-5s** |
| T4 Tier 2 (500_92) | 63.8M (실측) | ~17s (실측) | **~4-5s** |
| Mid-tier T4 (20_500) | 23.7M | 15.4s (m8a.8xl 측정) | **~3-4s** |

## 1M users prove only — Shape별 batches 수

| Shape | users/batch | 1M users 시 batches |
|---|---:|---:|
| T1 spot (50_1000) | 1,000 | 1,000 |
| T4 Tier 1 (50_700) | 700 | 1,429 |
| T4 Tier 2 (500_92) | 92 | 10,870 |
| Mid (20_500) | 500 | 2,000 |

## 1M users prove 시간 (single L4 GPU, g6.4xlarge)

| Scenario | Batches | per-batch | **Total prove** | Wall-clock |
|---|---:|---:|---:|---|
| T1 spot (50_1000) | 1,000 | 2.5s | 2,500s | **~42 min** |
| T2/T3 margin (50_500) | 2,000 | 3.5s | 7,000s | **~117 min ≈ 2 hr** |
| T4 Tier 1 only (50_700) | 1,429 | 4.5s | 6,430s | **~107 min ≈ 1.8 hr** |
| **T4 production mix** (95% T1 + 5% T2 in tier-1/2) | 1,357 + 544 = 1,901 | 4.5s | 8,555s | **~143 min ≈ 2.4 hr** |
| T4 worst (전체 Tier 2) | 10,870 | 4.5s | 48,915s | ~13.6 hr |

## Multi-GPU scaling (병렬 처리)

Multi-instance Redis BLPOP queue 또는 multi-GPU instance:

| Setup | GPU | T1 1M | T4 mix 1M |
|---|---|---:|---:|
| 1× L4 (g6.4xl, $1.32/hr) | 1 | 42 min | 143 min |
| 4× L4 (g6.12xl, $5.30/hr) | 4 | **~11 min** | **~36 min** |
| 8× L4 (g6.48xl, $13.35/hr) | 8 | **~6 min** | **~18 min** |
| 4× g6.4xl (multi-instance) | 4 | ~11 min | ~36 min |
| **8× g6.4xl (multi-instance)** | 8 | ~6 min | **~18 min** |
| 4× L40S (g6e.12xl, ~$10/hr) | 4 (larger VRAM) | ~9 min | ~28 min |

## 비용 견적 (1M user smoke 1회, prove only)

### T1 spot 거래소 (SEA 중심 GTM)

| Setup | Wall-clock | 비용 |
|---|---:|---:|
| 1× L4 g6.4xl | 42 min | $0.93 |
| **4× L4 g6.12xl** | **~11 min** | **$0.97** |
| 8× L4 g6.48xl | ~6 min | $1.34 |

→ **T1 spot 1M user prove = ~10 min @ ~$1** (multi-GPU)

### T4 margin 거래소 (Binance/OKX class)

| Setup | Wall-clock | 비용 |
|---|---:|---:|
| 1× L4 g6.4xl | 2.4 hr | $3.17 |
| 4× L4 g6.12xl | ~36 min | $3.18 |
| **8× L4 g6.48xl** | **~18 min** | **$4.01** |
| **8× g6.4xl multi-instance** | **~18 min** | **$3.17** |

→ **T4 production 1M user prove = ~20 min @ ~$3-4** (multi-instance recommended for cost efficiency)

## CPU vs GPU 비교 (1M users T4 production mix)

| Setup | Time | 비용 |
|---|---:|---:|
| 1× m8a.8xl (CPU baseline) | ~6-8 hr | $12.6-16.8 |
| 8× m8a.8xl multi-worker (CPU cluster) | ~50 min | $14.0 |
| 8× g6.4xl multi-instance (GPU cluster) | **~18 min** | **$3.17** |

→ **GPU cluster가 CPU cluster 대비 ~3× faster + ~4× cheaper** for 1M T4.

## 시나리오별 추천 SLA + 비용

| 거래소 | Model | Setup | Wall-clock | 비용 |
|---|---|---|---:|---:|
| **Spot small (10K-100K)** | T1 | 1× L4 g6.4xl | ~5 min | <$0.20 |
| **Spot mid (100K-1M)** | T1 | 4× L4 g6.12xl | **~11 min** | **~$1** |
| **Margin mid (100K-1M)** | T2/T3 | 8× L4 g6.48xl 또는 8× g6.4xl | ~20 min | ~$2-3 |
| **Margin large (1M)** | T4 production | 8× g6.4xl multi-instance | **~18 min** | **~$3-4** |
| **Binance scale (100M)** | T4 production | ~80 GPU cluster | ~20 min | **~$300/run** |

## GTM 포지셔닝 implications

본 견적이 정확하다면 zkpor V1+ 의 GTM 차별화 포인트:

1. **"Real-time PoR"** : 1M spot 거래소가 **10분 + $1**에 zk PoR 발행 가능. 일별 PoR 가능.
2. **"Mid-tier 거래소도 일별 PoR"** : Binance/OKX 외 100K-1M 거래소도 분 단위 PoR. 산업 표준 reset.
3. **Binance scale 도 가능** : $300/run × 일별 = 월 $9K. 거래소 매출 대비 무시 수준.

## 도입 시 zkpor 작업 (R12 + R13)

본 견적이 product roadmap 의 **R12 (GPU 코어)** + **R13 (Multi-instance 외부)** 로 분기:

| Stage | 범위 | Owner | 의존성 |
|---|---|---|---|
| **R12** | gnark ICICLE backend wiring 코어 (`cmd/prover` backend option) | core engine | `bnb-chain/gnark` fork ICICLE 호환성 PoC |
| **R13** | Multi-instance prover (Redis BLPOP 큐 + worker config) | infra/ops | R3 step 4 follow-up "prover Redis BLPOP" 항목 closure |

## 주의 사항 / 추정값 한계

1. **MSM speedup 3-5× 가정 — 실측 미검증**. ICICLE BN254 + bnb-chain/gnark fork 호환성 PoC 가 R12 entry blocker.
2. **GPU memory 24GB로 .pk 12GB 적재 + intermediate working set** — 64M constraint 회로는 OK, 더 큰 (~200M+) 회로는 L40S (48GB) 필요할 수도.
3. **Real production testdata 미사용** — testdata 10 user → real 1M user 시 prove time이 fixed overhead가 묽혀져 GPU efficiency↑ 가능 (sub-linear 더 강함).
4. **Multi-instance overhead** — Redis BLPOP 큐 + state coordination 비용 무시 (작음, 추정).
5. **AWS GPU instance 가격 변동** — Spot pricing 시 30-50% 절감 (dev/staging).
6. **Witness 단계 GPU 비활용** — witness 도 N batches × 비용. GPU prove와 동시 진행 가능 (R13 multi-instance에서).
7. **Userproof per-user Merkle path** — 1M user × ~5ms = 5000s ≈ 83min single instance. Multi-worker 필요. **R13 항목 일부**.

## 후속 측정 단계

1. **R12 PoC**: `bnb-chain/gnark` v0.10.1 fork의 `groth16.Prove(..., backend.WithIcicleAcceleration())` 작동 검증. 1 batch CPU vs GPU 직접 비교.
2. **R11**: 실제 1M user 합성 testdata 생성 → CPU 환경에서 다양한 shape prove time 직접 측정. 본 견적의 실측 base.
3. **GPU smoke**: R12 closure 후 같은 1M testdata로 GPU L4/L40S 실측.
4. **Multi-instance smoke (R13)**: 4-8 worker cluster 에서 throughput 측정.
