# zkpor production T4 smoke — m8a.12xlarge (Zen5, 48 vCPU, 192GB)

`profile/t4_reference/t4_reference.toml` 의 production batch shapes
`50_700 + 500_92` + `cap=500` end-to-end smoke 측정. R3 step 4 시점
`scripts/ec2/README.md` baseline (r7a.4xlarge ~1시간 keygen) 대비
m8a.12xlarge speedup 측정. 새 인스턴스 fresh bootstrap (us-east-1c
capacity 부족 → us-east-1b의 m8a.12xl으로 launch).

## 메타데이터

| 항목 | 값 |
|---|---|
| Run ID | `smoke_production_t4_m8a12xl_2026-05-27` |
| 실행 시각 (UTC) | 2026-05-27 13:50-14:55 |
| 호스트 | `m8a.12xlarge` (AMD EPYC 9R45 Turin Zen5 / 48 vCPU / **192 GB RAM**) |
| Instance ID | `i-0b251761db7ec9565` (fresh launch, us-east-1b) |
| EBS | gp3 150GB (fresh, DeleteOnTermination=true) |
| OS | Amazon Linux 2023, Linux 6.12.88 x86_64 |
| Go | go1.23.1 (bootstrap.sh installed) |
| gnark | bnb-chain/gnark v0.10.1 fork |
| zkpor commit | `496c492` (Phase 4 closure + 4 fix layers 적용) |
| 비고 | Force re-keygen. R3 step 4 시점 production T4가 r7a.4xlarge에서 ~1시간 걸렸던 것 대비 m8a.12xl speedup 측정. |

## 파라미터

| 항목 | 값 |
|---|---|
| AssetCapacity | **500** |
| BatchShape | **`50_700, 500_92`** (Production 2-tier) |
| 대상 model | t4_tiered_haircut_margin_3pool |
| testdata accounts | 10 real + padding → 1 batch (testdata 한계) |

## 결과 (PASS)

### Tier별 NbConstraints

| Tier | shape | NbConstraints | nbSecret | 비고 |
|---|---|---:|---:|---|
| **Tier 1** | 50_700 | **64,341,094** | 853,604 | 한 user ≤50 자산, batch당 700 user |
| **Tier 2** | 500_92 | **63,822,805** | 1,063,604 | 한 user ≤500 자산, batch당 92 user |
| **합** | | **128,163,899** | | |

Tier 1과 Tier 2의 NbConstraints가 거의 비슷 (~64M each)는 흥미. T4의 shape 곱 50×700=35,000 vs 500×92=46,000 — Tier 2가 user-asset 곱 30% 더 큼이지만 constraints는 유사. asset-dimension Merkle path + tier curve lookup이 batch-dimension보다 cost 크게 잡힘.

### Keygen

| Tier | r1cs compile | groth16.Setup | total |
|---|---:|---:|---:|
| Tier 1 (50_700) | 7m 47s | 11m 59s | **19m 46s** |
| Tier 2 (500_92) | 8m 47s | 10m 50s | **19m 37s** |
| **합** | **16m 34s** | **22m 49s** | **39m 23s** |

### Prove + Verify (per batch)

| 항목 | 값 |
|---|---:|
| proof generation cost | **29.6 s** |
| proof verification cost | <1 ms |

### Artifact 크기

| 파일 | Tier 1 (50_700) | Tier 2 (500_92) | 합 |
|---|---:|---:|---:|
| `.pk` | **12 GiB** | **12 GiB** | **24 GiB** |
| `.r1cs` | 3.9 GiB | 2.8 GiB | 6.7 GiB |
| `.vk` | 528 B | 528 B | 1,056 B |
| **합** | **16 GiB** | **15 GiB** | **~31 GiB** |

.pk 24GB 합 — `scripts/ec2/README.md` 의 "production .pk 24GB × 2 shape" baseline과 정확히 일치.

### Wall-clock 전체

| 단계 | 시간 |
|---|---:|
| Bootstrap (Docker + Go install) | ~5 min |
| Code sync | ~30 s |
| Tier 1 keygen | 19m 46s |
| Tier 2 keygen | 19m 37s |
| Pipeline (witness + prove + verify + userproof) | ~5 min |
| **전체** | **~50 min** (bootstrap 제외 시 ~45 min) |

## R3 step 4 baseline (r7a.4xlarge) 대비 추정 speedup

`scripts/ec2/README.md` 의 r7a.4xlarge production keygen baseline:
- "production keygen은 multi-shape + multi-GB. expect ~30min-1h on m6i.4xlarge"
- "r7a.4xlarge ... 2026-05-27 production-shape smoke 통과"

직접 측정 baseline은 r7a 에서 잡지 않았지만 R3 step 4 closure 기록 기준 **~60min**. 이번 m8a.12xlarge **45min** (bootstrap 제외) → **추정 ~1.3× speedup**.

mid-tier 측정 (3-way 보고서)에서 r7a → c8a.12xl (vCPU 16 → 48) ~2.6× wall-clock speedup. **Production에서 ~1.3× 만 단축되는 이유 추정**:
1. **RAM bandwidth bound** — Production T4 회로 size (~64M constraints/tier)가 cache 한참 넘어 메모리 bottleneck. DDR5 bandwidth가 vCPU scaling을 막음.
2. **MSM working set** — 큰 keygen은 working set이 L3 cache 넘어가서 memory-bound 가까워짐.
3. **Sequential setup phases** — groth16.Setup의 일부 단계가 multi-thread well 안 됨.

본 가설은 r7a.4xlarge 직접 측정 (다음 task) 필요.

## Sanity check

- [x] Production T4 keygen 두 shape 모두 완료
- [x] .pk 24GB (README baseline 일치)
- [x] proof 1 row + userproof 10 rows (DB 검증)
- [x] `verify pass!!!` + `All proofs verify passed!!!` 둘 다 통과
- [x] account tree root 일관성 유지
- [x] SSH disconnect 없이 client-side 전 stream

## 비용

| 항목 | 시간 | 가격 | 비용 |
|---|---:|---:|---:|
| m8a.12xlarge (smoke) | 50 min (0.83 h) | $3.15/hr (추정) | **$2.62** |
| EBS 150GB gp3 (한 번) | <0.1 month | $12/mo | <$0.04 |
| **합** | | | **~$2.66** |

DeleteOnTermination=true이라 terminate 시 EBS도 함께 삭제 → 추가 보관 비용 0.

## 다음 측정 후보

1. **r7a.4xlarge production T4 직접 측정** — R3 step 4 closure 시점 baseline (~60min) 의 정확한 측정. m8a.12xl 대비 speedup 정밀화.
2. **c8a.16xlarge production T4** (capacity 풀리는 시점) — 같은 chip 다른 vCPU, RAM bandwidth bound 가설 검증.
3. **m8a.24xlarge / m8a.48xlarge** — vCPU 96/192으로 Amdahl saturation 정확 측정 + RAM bandwidth 한계 노출.
4. **Production batch with real-scale testdata** (e.g. 100K real accounts) — prove time이 fixed overhead 비중에서 벗어나 가장 정확한 production 측정.

## 인스턴스 처리

`i-0b251761db7ec9565` (m8a.12xlarge, us-east-1b) — 측정 후 stop 또는 terminate 결정.
- **stop**: EBS gp3 150GB ~$12/mo, 추후 동일 인스턴스로 production T4 재사용 가능 (.pk 24GB cache 보존)
- **terminate**: DeleteOnTermination=true 라 EBS도 삭제, $0 비용 보존 안 됨. Snapshot으로 .pk 보존 가능 (~$0.05/GB-month × 24GB = ~$1.2/mo).
