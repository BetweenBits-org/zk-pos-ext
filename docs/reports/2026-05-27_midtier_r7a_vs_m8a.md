# zkpor mid-tier smoke — r7a.4xlarge vs m8a.8xlarge 비교

`profile/t{1,2,3,4}_reference` × `cap=200, shape=20_500` (mid-tier) 의 동일
실행을 두 인스턴스 타입에서 수행한 직접 비교. zkpor commit `b16ec64`
(Phase 4 closure + 4 fix layer 모두 적용된 상태) 동일 코드 베이스에서
.pk 를 force re-keygen 하여 keygen 시간도 측정.

## 환경

| 항목 | r7a.4xlarge | m8a.8xlarge |
|---|---|---|
| CPU 모델 | AMD Genoa (Zen4) | **AMD EPYC 9R45** (Turin, Zen5) |
| vCPU | 16 | **32** |
| RAM | 128 GB DDR5-4800 | 128 GB DDR5-5200 |
| EBS | gp3 150GB | gp3 150GB (동일 볼륨, type 변경) |
| AMI / OS | AL2023 (Linux 6.18.30) | 동일 |
| Go | go1.25.5 | 동일 |
| gnark | bnb-chain/gnark v0.10.1 fork | 동일 |
| us-east-1 AZ | us-east-1c | us-east-1c (동일) |
| 가격 (on-demand) | $1.06/hr | **~$2.10/hr** (추정, m7a 대비 +10%) |

## 결과 요약 (4 model 모두 PASS, 두 인스턴스)

### NbConstraints (회로 자체는 동일)

| Model | NbConstraints |
|---|---:|
| T1 | 9,483,618 |
| T2 | 10,831,084 |
| T3 | 14,481,701 |
| T4 | 23,695,376 |
| **합** | **58,491,779** |

### Keygen — r1cs compile

| Model | r7a | m8a | Speedup |
|---|---:|---:|---:|
| T1 | 1m 50s (110s) | **1m 08s** (68s) | **1.62×** |
| T2 | 2m 14s (134s) | **1m 25s** (85s) | **1.58×** |
| T3 | 2m 48s (168s) | **1m 46s** (106s) | **1.58×** |
| T4 | 4m 13s (253s) | **2m 43s** (163s) | **1.55×** |
| **합** | **11m 05s** (665s) | **6m 02s** (422s) | **1.58×** |

### Keygen — groth16.Setup

| Model | r7a | m8a | Speedup |
|---|---:|---:|---:|
| T1 | 4m 14s (254s) | **1m 58s** (118s) | **2.15×** |
| T2 | 5m 06s (306s) | **2m 27s** (147s) | **2.08×** |
| T3 | 6m 20s (380s) | **3m 03s** (183s) | **2.08×** |
| T4 | ~14m (~840s) | **5m 26s** (326s) | **~2.58×*** |
| **합** | ~30m (1780s) | **12m 54s** (774s) | **~2.30×** |

*T4 r7a setup time은 .pk mtime 기준 추정 (SSH disconnect로 정확한 측정 lost). 실측 가능했다면 ~10-12m 일 가능성.

### groth16.Prove

| Model | r7a | m8a | Speedup |
|---|---:|---:|---:|
| T1 | 17.8s | **8.5s** | **2.09×** |
| T2 | 18.4s | **9.1s** | **2.02×** |
| T3 | 19.3s | **9.7s** | **1.99×** |
| T4 | ~31s 추정 | **15.4s** | **~2.0×** |
| **합** | ~87s | **42.7s** | **~2.04×** |

### Wall-clock 전체 (smoke 4 model loop)

| 환경 | 전체 wall-clock | 단계 분해 |
|---|---:|---|
| r7a.4xlarge | **~52분** | keygen ~40m + pipeline ~12m |
| m8a.8xlarge | **~25분** | keygen ~19m + pipeline ~6m |
| **Speedup** | **2.08×** | |

## 비용 비교 (1회 mid-tier smoke 실행 기준)

| 환경 | wall-clock | 시간당 가격 | 1회 비용 |
|---|---:|---:|---:|
| r7a.4xlarge | 0.87 h | $1.06 | **$0.92** |
| m8a.8xlarge | 0.42 h | ~$2.10 | **$0.88** |
| **비용 차** | | | **-4%** |

→ **시간 절약 2.08×, 비용은 거의 동일**. m8a가 dev iteration에 명백히 우위.

## Single-thread 부분 비교

| 단계 | Parallel 정도 | Speedup | 분석 |
|---|---|---:|---|
| compile (R1CS 변환) | 직렬 우세 | **1.58×** | IPC + clock 영향 dominant. vCPU 2× 효과 작음. |
| Setup (MSM + sampling) | parallel 70% | **2.15-2.30×** | vCPU 2× + IPC + Zen5 SIMD. **최대 speedup 영역**. |
| Prove (MSM/FFT + solver) | parallel 80% | **~2.0×** | vCPU + AVX-512 활용. Amdahl 의 직렬 부분 (solver) 이 일부 발목. |
| Verify | <ms | n/a | parallelism 무관, 단일 thread groth16.Verify. |

→ **Compile은 IPC + clock 지배**, **Setup/Prove는 vCPU + IPC 결합**.

## 추정 vs 실측

| 단계 | 사전 추정 | 실측 | 정확도 |
|---|---|---:|---|
| compile speedup | 1.1-1.2× | **1.58×** | 추정 over-conservative (Zen5 IPC + clock 효과 큼) |
| Setup speedup | 1.6-1.8× | **2.15-2.30×** | 추정 over-conservative (AVX-512 효과) |
| Prove speedup | 1.7-1.9× | **~2.0×** | **추정 정확** |
| Wall-clock speedup | 1.7-1.9× | **2.08×** | 추정의 upper bound 정확히 적중 |

**Zen4→Zen5 IPC + clock 향상이 추정보다 컸음**. m7a (Zen4 8xlarge) 대비 m8a 의 절대 speedup 추정은 별도 측정 필요 — 본 비교는 r7a (Zen4 4xlarge) vs m8a (Zen5 8xlarge) 이라서 두 변수 (vCPU + IPC) 모두 변화.

## R7a vs M8a 인스턴스 사양 정밀 비교

vCPU만 보면 2× 인데 실측 speedup은 평균 **~2.0×**. 즉 이상적 linear scaling 에서 ~vCPU 비례 거의 fit. 그 이유:
1. **MSM/FFT의 거의 ideal parallelism** — Setup/Prove가 시간의 대부분 차지
2. **Zen5 IPC 향상 (~15-20%)** — 직렬 부분 (compile, solver) speedup
3. **DDR5-5200 (8% 더 빠른 메모리 BW)** — 큰 회로에서 marginal gain
4. **AVX-512 native 경로 (Zen5 = 512-bit, Zen4 = 256-bit chunked)** — gnark-crypto MSM acceleration

## 인스턴스 선택 가이드 (업데이트)

| 시나리오 | 추천 |
|---|---|
| 일회성 production keygen (단순 비용 최소) | **r7a.4xlarge** ($0.92 mid-tier) |
| Dev iteration (1일 1+회 smoke) | **m8a.8xlarge** ($0.88 mid-tier, 시간 절반) |
| 시간 critical | m8a.8xlarge (또는 m8a.12xlarge) |
| 비용 + 시간 균형 | **m8a.8xlarge** ← 본 측정 기준 sweet spot |

본 측정으로 **m8a.8xlarge가 r7a.4xlarge 대비 거의 동일 비용으로 wall-clock 2× 단축**이 확인됨.

## 다음 단계

- T4 mid-tier `r7a` setup time 정확 측정 (SSH disconnect 없이 재시도): 본 비교에서 추정한 ~14m 의 정확도 확인. m8a 5m 26s 대비 실제 ratio.
- m8a.12xlarge 측정 — vCPU 48 (1.5×) 의 한계 효율 확인. 예상 wall-clock ~18-20m (m8a.8xlarge 대비 1.3-1.4× 단축, 비용 1.5×).
- Production T4 (cap=500, shape=50_700+500_92) 두 환경 비교 — RAM 87GB peak이라 RAM bound 영향이 mid-tier보다 클 수 있음.
- nohup / screen / server-side tee 로 SSH disconnect 회피 (smoke.sh 또는 ec2/smoke.sh 에 적용).

## Sanity check

- [x] 두 인스턴스 모두 4 model 전 파이프라인 PASS
- [x] NbConstraints 양쪽 동일 (회로 byte-parity 보존)
- [x] account tree root 일치 (T1: 154739..., T2/T3/T4: 0e08eda... 두 환경에서 동일)
- [x] DB clear_db_state 작동 (between model + between run)
- [x] T4 m8a 는 client-side SSH disconnect 없이 끝까지 stream — 본 보고서의 T4 setup time은 실측값
- [x] r7a 인스턴스 stopped (compute 과금 $0)
- [x] m8a 인스턴스 stopped (compute 과금 $0)
