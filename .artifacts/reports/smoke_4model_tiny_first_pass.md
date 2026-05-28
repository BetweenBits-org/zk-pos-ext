# zkpor smoke run — `smoke_20260527T084626Z`

본 레포트는 `scripts/smoke.sh` (또는 `scripts/ec2/smoke.sh`)의 end-to-end
PoR 파이프라인 1회 실행에 대한 종합 측정 + 검증 결과를 기록한다. 양식은
`docs/reports/SMOKE_TEMPLATE.md`. 자동 채움 helper:
`scripts/extract_smoke_metrics.sh <log>`.

R6.5 baseline `(5,10,2)` 측정과 형식을 맞춰 차이가 한눈에 보이도록 표
구조를 잡았다.

## 메타데이터

| 항목 | 값 |
|---|---|
| Run ID | `smoke_20260527T084626Z` |
| 실행 시각 (UTC) | `2026-05-27T08:46:26Z` |
| 호스트 | `13.223.93.23` (`r7a.4xlarge`, 16 vCPU AMD Genoa / 128 GB RAM) |
| OS / 커널 | Amazon Linux 2023, Linux 6.18.30 x86_64 |
| Go 버전 | go1.25.5 (host: go1.25.5 darwin/arm64) |
| gnark 버전 | bnb-chain/gnark v0.10.1-0.20240910145009-4b5261061f04 (fork) |
| zkpor commit | `53aaa72` |
| 실행자 | `betweenbits` |
| 비고 | Phase 4 closure smoke. 4 fix layers commit chain `beda223` → `1badfc4` → `c19d455` → `53aaa72` 적용 후 첫 통과 |

## 파라미터

| 항목 | 값 |
|---|---|
| AssetCapacity | `5` |
| BatchShape (override) | `5_10` |
| 대상 profile(s) | `t1_reference, t2_reference, t3_reference, t4_reference` |
| testdata accounts | `10` (real) + padding = `500 (10 real + 490 padding) — shape dependent` |
| Batch 수 (per model) | `1 per model` |

## 결과 요약

| Model | Profile | PASS/FAIL | NbConstraints | keygen | prove (per batch) | verify | userproof | total |
|---|---|---|---:|---:|---:|---:|---:|---:|
| T1 | `t1_reference` | `PASS` | `165613` | `compile 1.477s + setup 3.765s` | `461  ms` | `1  ms` | `10 rows` | `see log` |
| T2 | `t2_reference` | `PASS` | `167351` | `compile 1.585929305s + setup 3.941078812s` | `443  ms` | `1  ms` | `10 rows` | `see log` |
| T3 | `t3_reference` | `PASS` | `204015` | `compile 2.100400846s + setup 5.085696112s` | `467  ms` | `1  ms` | `10 rows` | `see log` |
| T4 | `t4_reference` | `PASS` | `286157` | `compile 3.165480759s + setup 7.943209432s` | `674  ms` | `1  ms` | `10 rows` | `see log` |

## R6.5 baseline 대비

| Model | R6.5 `(5,10,2)` | 이번 `(shape, cap)` | 배수 |
|---|---:|---:|---:|
| T1 | 38,149 (raw-sum 회로) | `165613` (scaled-sum 회로) | `4.34x` (regression baseline 갱신 — 회로 변경) |
| T2 | 48,886 | `167351` | `3.42x` |
| T3 | 274,650 | `204015` | `0.74x` |
| T4 | 723,790 | `286157` | `0.40x` |

## Artifact 크기

| Model | `.pk` | `.vk` | `.r1cs` |
|---|---:|---:|---:|
| T1 | `28.9 MiB` (30,355,015 B) | `528 B` | `20.2 MiB` (21,152,938 B) |
| T2 | `29.5 MiB` (30,967,093 B) | `528 B` | `20.5 MiB` (21,509,795 B) |
| T3 | `35.5 MiB` (37,236,329 B) | `528 B` | `22.2 MiB` (23,253,633 B) |
| T4 | `56.7 MiB` (59,450,853 B) | `528 B` | `25.3 MiB` (26,497,210 B) |
| **합계** | **~150 MiB** (158,009,290 B) | 2,112 B | **~88 MiB** (92,413,576 B) |

## Per-model 상세

### T1 (`t1_simple_margin`)

- Profile: `profile/t1_reference/t1_reference.toml`
- Shape: `5_10` (userAssetCounts × allAssetCounts × batchCounts)
- NbConstraints: `165,613` (post-fix scaled-sum 회로; pre-fix raw-sum 회로는 38,149)
- r1cs compile: `1.477s` / groth16.Setup: `3.765s`
- Witness: `1` batch(es), account tree root `1897b35464686324d164bf37a214966ff77b3d6a5907b98fd0ee68890fb02849`
- Prover: `1 (testdata 1 batch)` proofs, prove `461  ms` / verify `1  ms` (per batch)
- Verifier batch: `PASS`, final CEX commitment `PASS`
- Userproof: `10` rows written
- Verifier -user: `PASS` (account index `0`)
- Notes: Phase 4 차단됐던 fail의 fixture. T1 회로 price scaling fix (c19d455) + sparse user.Assets fix (1badfc4) + cex zero-init fix (53aaa72) 세 layer 모두 통과.

### T2 (`t2_static_haircut_margin`)

- Profile: `profile/t2_reference/t2_reference.toml`
- Shape: `5_10`
- NbConstraints: `167351`
- r1cs compile: `1.585929305s` / groth16.Setup: `3.941078812s`
- Witness: `1` batch(es), account tree root `1f9e31980698882865de930cb6d35b4949d3b1f033182a2932c55892fc8535c2`
- Prover: `1 (testdata 1 batch)` proofs, prove `443  ms` / verify `1  ms` (per batch)
- Verifier batch: `PASS`, final CEX commitment `PASS`
- Userproof: `10` rows written
- Verifier -user: `PASS` (account index `0`)
- Notes: `(auto-extracted; manual review recommended)`

### T3 (`t3_tiered_haircut_margin_1pool`)

- Profile: `profile/t3_reference/t3_reference.toml`
- Shape: `5_10`
- NbConstraints: `204015`
- r1cs compile: `2.100400846s` / groth16.Setup: `5.085696112s`
- Witness: `1` batch(es), account tree root `1f9e31980698882865de930cb6d35b4949d3b1f033182a2932c55892fc8535c2`
- Prover: `1 (testdata 1 batch)` proofs, prove `467  ms` / verify `1  ms` (per batch)
- Verifier batch: `PASS`, final CEX commitment `PASS`
- Userproof: `10` rows written
- Verifier -user: `PASS` (account index `0`)
- Notes: `(auto-extracted; manual review recommended)`

### T4 (`t4_tiered_haircut_margin_3pool`)

- Profile: `profile/t4_reference/t4_reference.toml`
- Shape: `5_10`
- NbConstraints: `286157`
- r1cs compile: `3.165480759s` / groth16.Setup: `7.943209432s`
- Witness: `1` batch(es), account tree root `0f6b997313d2884993f2a943e46f6b495d6378780c4a792df58f5b4fae007dff`
- Prover: `1 (testdata 1 batch)` proofs, prove `674  ms` / verify `1  ms` (per batch)
- Verifier batch: `PASS`, final CEX commitment `PASS`
- Userproof: `10` rows written
- Verifier -user: `PASS` (account index `0`)
- Notes: `(auto-extracted; manual review recommended)`

## 관찰 / 이슈 / 후속

**통과**:
- panic 발생: 0건
- verifier verify failed: 0건
- 4 model × end-to-end (keygen → witness → prover → verifier(batch) → userproof → verifier(-user)) 풀 파이프라인 통과

**발견 + 수정한 회귀 (4 layer)**:
1. `beda223` smoke.sh `clear_db_state` 잘못된 테이블 이름 (`batch_witness` → `witness`)
2. `1badfc4` `SetBatchCreateUserCircuitWitness` (4 model 공통) — sparse user.Assets에서 nil + wrong-slot indexing
3. `c19d455` T1 회로 `totalUserEquity` raw-sum (T2/T3/T4는 이미 scaled — T1만 outlier)
4. `53aaa72` `RunWitness` (4 model 공통) — published cex_assets를 BeforeCex로 사용 (should zero-init)

**latent 원인**: R3 step 4 시점 legacy raw CSV path는 데이터가 dense하고 cex_assets_info semantics가 묘하게 매치해서 우연히 통과. R9 standard CSV가 published-final contract를 명시화하면서 4 layer 모두 노출.

**후속 권장**:
- T2/T3 prover_e2e_test 추가 (T1과 동일 패턴) — regression safety net
- Mid-tier shape (cap=50, shape=20_500)로 재시도 — 이번 fix로 통과 예상
- Production T4 (cap=500) re-keygen 후 smoke — R3 step 4 baseline 재확인
- HANDOFF.md + R6.5 baseline 문서 갱신 (T1 NbConstraints 38,149 → 165,613)
- 환경: r7a.4xlarge, AL2023, gnark fork — keygen artifacts cache-hit 시 시간 단축

## 비교 reference

이전 측정 또는 baseline이 있다면 링크.

- R6.5 baseline (capacity=10, shape=5_10_2): `docs/04-solvency-models.md §R6.5` 또는 `HANDOFF.md §Current State`
- Phase 3a keygen validation (capacity=5, shape=5_10): `.artifacts/keygen_validation.log` (이번 세션)
- Production T4 (capacity=500, shape=50_700+500_92): `scripts/ec2/README.md §현재 dev 인스턴스`

## Sanity check

다음 invariant이 깨지지 않았는지 확인.

- [x] 모든 model의 `verify pass!!!` (verifier -user) 통과
- [x] 모든 model의 `All proofs verify passed!!!` (verifier batch) 통과
- [x] account tree root이 witness → prover → verifier → userproof 단계에서 일치 (log root 매치)
- [ ] T4 회로의 R1CS sha256이 R6.5 baseline과 일치 (capacity/shape 동일 시) — setup_test에서 별도 lock
- [x] `.pk`/`.vk`/`.r1cs` 파일이 stem `zkpor.<model>.<tier>_<users>` 형식
- [x] DB row 누수 없음 (smoke.sh clear_db_state 작동 확인)
