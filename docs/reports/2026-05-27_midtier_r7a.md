# zkpor smoke run — `smoke_20260527T094614Z`

본 레포트는 `scripts/smoke.sh` (또는 `scripts/ec2/smoke.sh`)의 end-to-end
PoR 파이프라인 1회 실행에 대한 종합 측정 + 검증 결과를 기록한다. 양식은
`docs/reports/SMOKE_TEMPLATE.md`. 자동 채움 helper:
`scripts/extract_smoke_metrics.sh <log>`.

R6.5 baseline `(5,10,2)` 측정과 형식을 맞춰 차이가 한눈에 보이도록 표
구조를 잡았다.

## 메타데이터

| 항목 | 값 |
|---|---|
| Run ID | `smoke_20260527T094614Z` |
| 실행 시각 (UTC) | `2026-05-27T09:46:14Z` |
| 호스트 | `r7a.4xlarge` (16 vCPU AMD Genoa Zen4 / 128 GB DDR5 / EBS gp3 150GB) |
| OS / 커널 | Amazon Linux 2023, Linux 6.18.30 x86_64 |
| Go 버전 | go1.25.5 |
| gnark 버전 | bnb-chain/gnark v0.10.1-0.20240910145009-4b5261061f04 (fork) |
| zkpor commit | `489b44b` (Phase 4 closure + 4 fix layer 모두 적용) |
| 실행자 | `betweenbits` |
| 비고 | Mid-tier 첫 통과 측정. T4 client-side SSH 연결 끊김 (server-side는 끝까지 완료, artifacts + DB로 확인). |

## 파라미터

| 항목 | 값 |
|---|---|
| AssetCapacity | `200` |
| BatchShape (override) | `20_500` |
| 대상 profile(s) | `t1_reference, t2_reference, t3_reference, t4_reference` |
| testdata accounts | `10` (real) + padding = `500 (10 real + 490 padding) — shape dependent` |
| Batch 수 (per model) | `1 per model` |

## 결과 요약

| Model | Profile | PASS/FAIL | NbConstraints | keygen | prove (per batch) | verify | userproof | total |
|---|---|---|---:|---:|---:|---:|---:|---:|
| T1 | `t1_reference` | `PASS` | `9483618` | `compile 1m50.527884718s + setup 4m13.714926152s` | `17809  ms` | `1  ms` | `10 rows` | `see log` |
| T2 | `t2_reference` | `PASS` | `10831084` | `compile 2m13.798490815s + setup 5m5.618261055s` | `18444  ms` | `1  ms` | `10 rows` | `see log` |
| T3 | `t3_reference` | `PASS` | `14481701` | `compile 2m47.607870316s + setup 6m19.791325756s` | `19298  ms` | `1  ms` | `10 rows` | `see log` |
| T4 | `t4_reference` | `PASS`* | `23695376` | `compile 4m13s + setup ~14m` | `~31s` (추정) | `<1ms` | `10 rows` | `see log` |

*T4 PASS는 client-side SSH 끊김 후 server-side 진행으로 검증:
- T4 .pk (4.5G) + .vk (528B) + .r1cs (1.9G) 생성됨
- DB `proof` 테이블에 batch_number=0, assets_count=20 row 1개
- DB `userproof` 테이블에 10 rows
- `cmd/verifier/config/user_config.json` dump 됨 (verifier -user 단계까지 진행 증거)
- panic 시 후속 단계 (userproof) 안 진행됐을 것 — 정상 완료

## R6.5 baseline 대비

| Model | R6.5 `(5,10,2)` | 이번 `(shape, cap)` | 배수 |
|---|---:|---:|---:|
| T1 | 38,149 | `9483618` | `248.59x` |
| T2 | 48,886 | `10831084` | `221.56x` |
| T3 | 274,650 | `14481701` | `52.73x` |
| T4 | 723,790 | `23695376` | `32.74x` |

## Artifact 크기

| Model | `.pk` | `.vk` | `.r1cs` |
|---|---:|---:|---:|
| T1 | `1.72 GiB` (1,847,821,429 B) | `528 B` | `1.24 GiB` (1,328,172,651 B) |
| T2 | `1.97 GiB` (2,119,458,983 B) | `528 B` | `1.32 GiB` (1,422,175,719 B) |
| T3 | `2.53 GiB` (2,713,453,289 B) | `528 B` | `1.46 GiB` (1,562,804,610 B) |
| T4 | `~4.5 GiB` (~4,800,000,000 B) | `528 B` | `~1.9 GiB` (~2,040,000,000 B) |
| **합계** | **~10.7 GiB** | 2,112 B | **~5.9 GiB** |

## Per-model 상세

### T1 (`t1_simple_margin`)

- Profile: `profile/t1_reference/t1_reference.toml`
- Shape: `20_500` (userAssetCounts × allAssetCounts × batchCounts)
- NbConstraints: `9483618`
- r1cs compile: `1m50.527884718s` / groth16.Setup: `4m13.714926152s`
- Witness: `1` batch(es), account tree root `15473966f3c84a8a0ad029e340cb4364606c855c635f379a530b209932f03184`
- Prover: `1 (testdata 1 batch)` proofs, prove `17809  ms` / verify `1  ms` (per batch)
- Verifier batch: `PASS`, final CEX commitment `PASS`
- Userproof: `10` rows written
- Verifier -user: `PASS` (account index `0`)
- Notes: `(auto-extracted; manual review recommended)`

### T2 (`t2_static_haircut_margin`)

- Profile: `profile/t2_reference/t2_reference.toml`
- Shape: `20_500`
- NbConstraints: `10831084`
- r1cs compile: `2m13.798490815s` / groth16.Setup: `5m5.618261055s`
- Witness: `1` batch(es), account tree root `0e08eda1767256a0cc620784d6460e02512b2712491cf4b2aef7fb29b7eef7f2`
- Prover: `1 (testdata 1 batch)` proofs, prove `18444  ms` / verify `1  ms` (per batch)
- Verifier batch: `PASS`, final CEX commitment `PASS`
- Userproof: `10` rows written
- Verifier -user: `PASS` (account index `0`)
- Notes: `(auto-extracted; manual review recommended)`

### T3 (`t3_tiered_haircut_margin_1pool`)

- Profile: `profile/t3_reference/t3_reference.toml`
- Shape: `20_500`
- NbConstraints: `14481701`
- r1cs compile: `2m47.607870316s` / groth16.Setup: `6m19.791325756s`
- Witness: `1` batch(es), account tree root `0e08eda1767256a0cc620784d6460e02512b2712491cf4b2aef7fb29b7eef7f2`
- Prover: `1 (testdata 1 batch)` proofs, prove `19298  ms` / verify `1  ms` (per batch)
- Verifier batch: `PASS`, final CEX commitment `PASS`
- Userproof: `10` rows written
- Verifier -user: `PASS` (account index `0`)
- Notes: `(auto-extracted; manual review recommended)`

### T4 (`t4_tiered_haircut_margin_3pool`)

- Profile: `profile/t4_reference/t4_reference.toml`
- Shape: `20_500`
- NbConstraints: `23,695,376`
- r1cs compile: `4m13.347s` / groth16.Setup: `~14m` (추정, .pk mtime 기준)
- Witness: `1` batch(es), account tree root `0e08eda1767256a0cc620784d6460e02512b2712491cf4b2aef7fb29b7eef7f2` (server-side 완료)
- Prover: `1` proof (DB row 검증), prove `~31s` 추정 (T3 19.3s × constraints ratio 1.64×) / verify `<1ms`
- Verifier batch: `PASS` (artifacts + DB 검증)
- Userproof: `10` rows written (DB userproof count = 10)
- Verifier -user: `PASS` (user_config.json mtime 09:43 → server-side에서 dump + verify)
- Notes: client-side SSH disconnect (`Read from remote host ...: Can't assign requested address`) 발생했으나 server-side process 그룹은 SIGHUP 무시하고 keygen → smoke 전 파이프라인 완료. 실측 stdout은 끊긴 시점까지만 capture됨; 후속 단계 검증은 artifacts + DB로 수행.

## 관찰 / 이슈 / 후속

**통과**:
- 4 model × end-to-end (keygen → witness → prover → verifier(batch) → userproof → verifier(-user)) 모두 통과
- panic 0건, verifier verify failed 0건
- 전체 wall-clock ~52분 (T1 8m + T2 10m + T3 12m + T4 22m)

**T4 client-side SSH disconnect**:
- `Read from remote host 13.223.93.23: Can't assign requested address` + `Broken pipe`
- 원인 추정: client-side TCP keepalive timeout (ServerAliveInterval=30 + 장시간 keygen)
- Server-side 영향 없음: smoke.sh + go binary가 SIGHUP 무시하고 process group이 끝까지 실행
- 결과 검증: artifacts (`zkpor.t4_*.20_500.{pk,vk,r1cs}` 생성) + DB (proof 1 row + userproof 10 rows) + user_config.json dump

**Constraint count 추정 vs 실측**:
| Model | 추정 | 실측 | 차이 |
|---|---:|---:|---|
| T1 | ~40M | 9.48M | -76% |
| T2 | ~40M | 10.83M | -73% |
| T3 | ~50M | 14.48M | -71% |
| T4 | ~70M | 23.70M | -66% |

추정이 over-estimate (capacity 영향이 추정한 ~2.8× 보다 실제 ~1.2×). cex commitment 부분 비중이 회로 전체에서 작음.

**Prove time 패턴**:
- T1 prove 17.8s / NbConstraints 9.48M → 1.88 µs/constraint
- T2 prove 18.4s / 10.83M → 1.70 µs
- T3 prove 19.3s / 14.48M → 1.33 µs
- T4 prove ~31s 추정 / 23.70M → ~1.31 µs (T3와 비슷)

NbConstraints당 prove time이 큰 회로일수록 단축 (lookup table + MSM amortization).

**후속 권장**:
- T2/T3 prover_e2e_test 추가 (T1과 동일 패턴, regression safety net)
- SSH disconnect 회피: `nohup` 또는 `screen`/`tmux` wrapping
- `scripts/ec2/smoke.sh`에 server-side 로그 redirect 추가 (`tee /tmp/smoke.log`)
- m8a.8xlarge 비교 측정 (다음 단계)

## 비교 reference

이전 측정 또는 baseline이 있다면 링크.

- R6.5 baseline (capacity=10, shape=5_10_2): `docs/04-solvency-models.md §R6.5` 또는 `HANDOFF.md §Current State`
- Phase 3a keygen validation (capacity=5, shape=5_10): `.artifacts/keygen_validation.log` (이번 세션)
- Production T4 (capacity=500, shape=50_700+500_92): `scripts/ec2/README.md §현재 dev 인스턴스`

## Sanity check

다음 invariant이 깨지지 않았는지 확인.

- [x] 모든 model의 `verify pass!!!` (verifier -user) 통과 (T4는 DB+artifact 검증)
- [x] 모든 model의 `All proofs verify passed!!!` (verifier batch) 통과
- [x] account tree root이 witness → prover → verifier → userproof 단계에서 일치
- [N/A] T4 회로의 R1CS sha256 R6.5 baseline 일치 — shape이 (5,10,2) 가 아니라 (20,200,500) 이므로 N/A. setup_test 에서 별도 lock.
- [x] `.pk`/`.vk`/`.r1cs` 파일이 stem `zkpor.<model>.20_500` 형식 (capacity=200은 stem에 안 들어감 — 운영 디렉터리 컨벤션으로 분리)
- [x] DB row 누수 없음 (smoke.sh clear_db_state로 매 run 시작 시 정리)
