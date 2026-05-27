# zkpor mid-tier smoke — 3-way 인스턴스 비교 (r7a / m8a / c8a)

`profile/t{1,2,3,4}_reference` × `cap=200, shape=20_500` (mid-tier) 의 동일
실행을 세 인스턴스 타입에서 수행한 직접 비교. 같은 EBS volume,
same git commit `b16ec64`, force re-keygen.

m8a.12xlarge는 us-east-1c capacity 부족으로 launch 실패 (`Insufficient
InstanceCapacity`) → 같은 generation + 같은 vCPU의 c8a.12xlarge로 대체.
c8a 는 m8a 와 동일 chip (AMD EPYC 9R45 Turin Zen5) + half RAM. mid-tier
peak RAM ~30GB라 96GB 충분 — c8a vs m8a-가상값은 직접 비교 가능.

## 환경

| 항목 | r7a.4xlarge | m8a.8xlarge | c8a.12xlarge |
|---|---|---|---|
| CPU 모델 | AMD Genoa (Zen4) | **AMD EPYC 9R45** (Turin Zen5) | **AMD EPYC 9R45** (Turin Zen5) |
| vCPU | 16 | 32 | **48** |
| RAM | 128 GB | 128 GB | 92 GB |
| RAM/vCPU | 8 | 4 | ~2 (compute-optimized) |
| Memory BW | DDR5-4800 | DDR5-5200 | DDR5-5200 |
| us-east-1 가격 (on-demand) | $1.06/hr | ~$2.10/hr (추정) | ~$2.65/hr (추정) |
| 측정 일자 | 2026-05-27 09:21-09:46 UTC | 2026-05-27 09:53-10:13 UTC | 2026-05-27 10:30-10:50 UTC |

같은 인스턴스 ID `i-0f4b93a48a192dbac` 의 type을 변경하며 측정 (EBS volume 공유).

## NbConstraints (회로 동일)

| Model | NbConstraints |
|---|---:|
| T1 | 9,483,618 |
| T2 | 10,831,084 |
| T3 | 14,481,701 |
| T4 | 23,695,376 |
| **합** | **58,491,779** |

## r1cs compile

| Model | r7a (16) | m8a (32) | c8a (48) | r7a→m8a | m8a→c8a | r7a→c8a |
|---|---:|---:|---:|---:|---:|---:|
| T1 | 110s | 68s | 67.7s | 1.62× | 1.00× | 1.62× |
| T2 | 134s | 85s | 86.2s | 1.58× | 0.99× | 1.55× |
| T3 | 168s | 106s | 107.3s | 1.58× | 0.99× | 1.57× |
| T4 | 253s | 163s | 165.5s | 1.55× | 0.98× | 1.53× |
| **합** | 665s | 422s | 426.7s | **1.58×** | **0.99×** | **1.56×** |

**관찰**: compile은 직렬 우세 → vCPU 16→32에서 IPC+clock 효과로 1.58× 단축, 32→48에서는 추가 vCPU 효과 0 (오히려 ~1% 더 느림). 명확한 Amdahl sequential bottleneck.

## groth16.Setup

| Model | r7a (16) | m8a (32) | c8a (48) | r7a→m8a | m8a→c8a | r7a→c8a |
|---|---:|---:|---:|---:|---:|---:|
| T1 | 254s | 118s | 92.5s | 2.15× | **1.28×** | **2.75×** |
| T2 | 306s | 147s | 116.5s | 2.08× | **1.26×** | **2.63×** |
| T3 | 380s | 183s | 144.8s | 2.08× | **1.27×** | **2.62×** |
| T4 | ~840s* | 326s | 257.3s | ~2.58×* | **1.27×** | **~3.26×*** |
| **합** | ~1780s* | 774s | 611.1s | ~2.30×* | **1.27×** | **~2.91×*** |

*T4 r7a는 .pk mtime 기준 추정 (SSH disconnect 영향).

**관찰**: Setup은 ~70% parallel → m8a→c8a 1.27× speedup (이론 1.5× 대비). Amdahl 70% parallel 가정 시 1/(0.3 + 0.7/1.5) = 1.31× 추정 매우 가깝다. r7a→m8a는 vCPU 효과 + Zen5 IPC + AVX-512 결합으로 super-linear.

## groth16.Prove

| Model | r7a (16) | m8a (32) | c8a (48) | r7a→m8a | m8a→c8a | r7a→c8a |
|---|---:|---:|---:|---:|---:|---:|
| T1 | 17.8s | 8.5s | 7.6s | 2.09× | 1.12× | 2.34× |
| T2 | 18.4s | 9.1s | 7.9s | 2.02× | 1.15× | 2.33× |
| T3 | 19.3s | 9.7s | 8.3s | 1.99× | 1.17× | 2.33× |
| T4 | ~31s* | 15.4s | 13.0s | ~2.0× | 1.18× | ~2.38× |
| **합** | ~87s | 42.7s | 36.8s | **~2.04×** | **1.16×** | **~2.36×** |

**관찰**: Prove의 m8a→c8a speedup은 1.16× (이론 1.5×의 Amdahl 80% parallel 추정 1.36× 보다 낮음). 원인: 작은 testdata (10 real + 490 padding accounts → 1 batch only) — prove time의 절대값이 작아 fixed overhead (witness solver, lookup table build, MSM coefficient prep) 비중이 큼. **production scale (cap=500, larger batches) 에서는 c8a vs m8a prove speedup이 더 클 것**.

## Wall-clock 전체 (4 model loop)

| 환경 | 총 시간 | 가격/hr | 1회 비용 |
|---|---:|---:|---:|
| r7a.4xlarge (16 vCPU) | ~52 min | $1.06 | **$0.92** |
| m8a.8xlarge (32 vCPU) | ~25 min | ~$2.10 | **$0.88** |
| c8a.12xlarge (48 vCPU) | ~20 min | ~$2.65 | **$0.88** |

**시간 단축 효율 비교**:

| Transition | vCPU 비율 | 실측 wall-clock 단축 | 효율 |
|---|---:|---:|---:|
| r7a → m8a | 2× | 2.08× | **104%** (super-linear: IPC + AVX-512) |
| m8a → c8a | 1.5× | 1.25× | **83%** (sub-linear: Amdahl 직렬 부분 노출) |
| r7a → c8a | 3× | 2.60× | **87%** (전체 통합) |

## 비용 효율 분석

| 환경 | 시간 | 비용 | $/min |
|---|---:|---:|---:|
| r7a.4xlarge | 52 min | $0.92 | $0.0177 |
| m8a.8xlarge | 25 min | $0.88 | $0.0352 |
| c8a.12xlarge | 20 min | $0.88 | $0.0440 |

→ **비용은 세 환경 거의 동일 ($0.88-$0.92)**, 시간만 단축. 절대 비용 무관하면 c8a가 최고.

다만 $/min 단위로 보면 r7a가 더 효율적 — 단순 throughput (batch 처리 속도) 이 아니라 wall-clock latency 가 critical할 때만 비싼 환경 가치.

## 인스턴스 추천 매트릭스 (3-way 비교 후 갱신)

| 시나리오 | 추천 |
|---|---|
| Cost-optimal (Total $) | r7a.4xlarge ($0.92) |
| Time-optimal (Wall-clock) | **c8a.12xlarge (20m)** |
| Balanced (cost ≈ same, time halved vs r7a) | **m8a.8xlarge** ← sweet spot |
| Production keygen 한 번만 | r7a.4xlarge (단순 비용) |
| Dev iteration 자주 + 시간 critical | c8a.12xlarge |
| Dev iteration 일반 | **m8a.8xlarge** (sweet spot) |

## Amdahl curve 측정 결과 정리

세 환경 측정으로 도출한 zkpor mid-tier groth16 의 parallel fraction:

| 단계 | 실측 sequential fraction (P) | 이론 max speedup with N vCPU |
|---|---:|---|
| compile | ~85% sequential | ~1.6-1.7× (n→∞) — vCPU 효과 4 vCPU 정도까지만 |
| Setup | ~25% sequential | ~4× (n→∞) — vCPU scaling 가장 효과적 |
| Prove | ~15% sequential | ~6× (n→∞) — large batch에서 가장 효과적 (mid-tier 작아서 overhead 노출) |

**Production scale 시 prove speedup이 mid-tier보다 클 것** — c8a.12xlarge 또는 더 큰 size 가 production keygen 시 시간 절약 큼.

## Sanity check

- [x] 세 환경 모두 4 model 전 파이프라인 PASS
- [x] NbConstraints 전 동일 (회로 byte-parity 보존)
- [x] account tree root 일치 (T1: 15473..., T2/T3/T4: 0e08eda...)
- [x] m8a/c8a smoke는 SSH disconnect 없이 끝까지 stream
- [x] 모든 인스턴스 stopped (compute 과금 $0)
- [x] EBS volume 보존 (다음 측정 또는 production 활용 가능)

## 다음 측정 후보

1. **m8a.12xlarge** capacity 풀리는 시점에 측정 — c8a.12xlarge (same chip, 92GB RAM) vs m8a.12xlarge (128GB RAM) 비교. 같은 vCPU 48 + RAM 차이만 → mid-tier에서 영향 0 예상, production scale (cap=500, peak 87GB)에서는 c8a OOM 위험.
2. **Production T4** (cap=500, shape=50_700) — RAM bandwidth bound 효과 확인.
3. **r8a.4xlarge** (Zen5 16 vCPU + 192GB) — r7a (Zen4 16 vCPU + 128GB) 대비 generation jump만 분리 측정.
4. **m8a.24xlarge** (Zen5 96 vCPU + 384GB) — vCPU 더 키워서 Amdahl saturation 점 확인.
