# R11-D Runbook — Setup/Prove ablation 측정

`docs/BENCHMARK.md §4.1` 의 R11-D plan 실행 절차. Phase 1 closed,
Phase 2 pending.

## 상태

| Phase | 상태 | 결과 |
|---|---|---|
| **Phase 1** (Setup + 4 dense cells) | ✅ closed (2026-05-28) | BENCHMARK §2.6 |
| **Phase 2** (4 sparse density cells) | ⏳ pending | 이 runbook §3-5 |

**Phase 1 핵심 발견**: T4 production dense prove peak RSS ~120 GiB —
회로 size × density 함수 (§1.3 보정). 4xl-class instance 의 dense
workload 측정 불가 → Phase 2 단일 m8a.8xl × density 축으로 단순화.

## 1. EC2 자산 (현재 stopped)

| 항목 | 값 |
|---|---|
| Instance ID | `i-05da73a6bb557498e` |
| Type | m8a.8xlarge (32 vCPU Zen5 / 128 GB) |
| Region/AZ | us-east-1a |
| EBS | gp3 150 GB, `DeleteOnTermination=true` |
| 보존 artifact | `.pk` × 2 shape (24 GB) + `.vk` + `.r1cs` + Phase 1 testdata |
| Key pair | `ue1-dev` (`~/Documents/keypairs/ue1-dev.pem`) |
| Security group | `sg-042b08f0a129f3832` (SSH 22 open) |
| Restart 비용 | ~2min boot + status-ok 대기 |

**State 갱신 시 주의**: AWS console 또는 `aws ec2 start-instances` 후
public IP 가 매 재시작 마다 바뀜 → `scripts/ec2/.env` 의 `EC2_HOST`
수동 갱신 필요 (agent 권한 막힘).

## 2. Phase 2 cell 정의

`scripts/ec2/r11d.sh <cell>` 가 모든 parameter + RSS sampler enforce.

| Cell | Shape | Users | AssetCount | Density | Batches | 기대 RSS |
|---|---|---:|---:|---:|---:|---:|
| `t1_700_d10` | 50_700 | 700 | 5 | ~10% | 1 | ~30 GiB |
| `t1_700_d50` | 50_700 | 700 | 25 | ~50% | 1 | ~65 GiB |
| `t2_92_d10` | 500_92 | 92 | 50 | ~10% | 1 | ~30 GiB |
| `t2_92_d50` | 500_92 | 92 | 250 | ~50% | 1 | ~65 GiB |

기존 Phase 1 의 `t1_700` / `t2_92` 가 density=100% 의 anchor.

**Cell pairing invariant**: `asset_count` 가 shape 의 `asset_count_tier`
(50 또는 500) 이내여야 라우팅 일관. r11d.sh case 분기가 강제.

## 3. Phase 2 절차

### 3.1 Instance start + IP 갱신

```bash
# 1. Start
aws ec2 start-instances --region us-east-1 --instance-ids i-05da73a6bb557498e
aws ec2 wait instance-running --region us-east-1 --instance-ids i-05da73a6bb557498e
aws ec2 wait instance-status-ok --region us-east-1 --instance-ids i-05da73a6bb557498e

# 2. IP 확인
aws ec2 describe-instances --region us-east-1 \
  --instance-ids i-05da73a6bb557498e \
  --query 'Reservations[0].Instances[0].PublicIpAddress' --output text

# 3. .env 갱신 (사용자 수동 — agent 권한 막힘)
#    scripts/ec2/.env 의 EC2_HOST=ec2-user@<new-ip> 수정
```

### 3.2 코드 sync

```bash
./scripts/ec2/sync.sh
```

(현재 `48d5b5e` 의 r11d.sh + sparse cell 정의 + RSS sampler 가 EC2 측에
적용됨)

### 3.3 4 sparse cells 순차

```bash
ssh -i ~/Documents/keypairs/ue1-dev.pem ec2-user@<ip> '
  cd zkmerkle-proof-of-solvency/zkpor && \
  export INSTANCE_TAG=m8a.8xl && \
  export PATH=$PATH:/usr/local/go/bin && \
  setsid nohup bash -c "
    set -e
    ./scripts/ec2/r11d.sh t1_700_d10
    ./scripts/ec2/r11d.sh t1_700_d50
    ./scripts/ec2/r11d.sh t2_92_d10
    ./scripts/ec2/r11d.sh t2_92_d50
    date -u > /home/ec2-user/r11d_phase2_done.flag
  " > /home/ec2-user/r11d_phase2.out 2>&1 </dev/null &
  echo \"[launched] pid=$!\"; exit 0
'
```

cell 당 ~5min × 4 = ~20-30min wall-clock.

### 3.4 Monitor

Phase 1 의 polling monitor 패턴 — `/home/ec2-user/r11d_phase2_done.flag`
파일 등장까지 polling, 도중 `/home/ec2-user/r11d_phase2.out` tail 로
진행 추적.

### 3.5 Fetch + stop

```bash
rsync -avz -e "ssh -i ~/Documents/keypairs/ue1-dev.pem" \
  ec2-user@<ip>:/home/ec2-user/zkmerkle-proof-of-solvency/zkpor/.artifacts/reports/ \
  .artifacts/reports/

aws ec2 stop-instances --region us-east-1 --instance-ids i-05da73a6bb557498e
```

### 3.6 Fold-in to BENCHMARK §2.7

Phase 2 결과는 새 `### 2.7 R11-D Phase 2 density ablation` 섹션 추가
+ §3.x fit 갱신.

## 4. RSS sampler 사용

r11d.sh 가 자동 sample, 종료 시 log 에 summary 출력:

```
[r11d] prover RSS samples: n=15 peak=68000MB avg=65000MB min=51000MB peak_GiB=66.4
```

Raw data: `.artifacts/reports/R11D_<tag>_<cell>/run_<ts>.mem.tsv`
(TSV: `ts_utc pid rss_kb vsz_kb`).

## 5. 비용 / 시간 예상

| 항목 | 예상 |
|---|---:|
| Wall-clock | ~50min (boot 2min + 4 cells ~30min + idle/fetch ~18min) |
| Instance cost | $1.50 (~50min × $1.80/hr) |
| EBS gp3 (already created) | 무시 |
| **합** | **~$1.5** |

## 6. 트러블슈팅

- **Public IP 변경 후 SSH fingerprint 충돌**: `ssh-keygen -R <new-ip>`
  또는 SSH 옵션 `-o StrictHostKeyChecking=accept-new -o
  UserKnownHostsFile=/tmp/...` 사용 (RUNBOOK 의 orchestration script
  패턴 참고).
- **r11d.sh `extract_smoke_metrics.sh: Permission denied`**: `25cefd7`
  에서 chmod +x 적용됨. 새 EC2 에서 재발 시 `ssh ... 'cd ... && chmod
  +x scripts/extract_smoke_metrics.sh scripts/ec2/*.sh'`.
- **prover OOM**: density 10% 도 64 GiB 한계 (m8a.4xl) 넘으면 (가설
  반대) m8a.8xl 도 살얼음일 수 있음. Phase 2 cell t1_700_d10 의 RSS
  sample 이 65 GiB 초과 시 RSS scaling 가설 재검증 필요.
- **RSS sampler 가 sample 0개**: prover process 찾는 패턴 (`exe/prover`)
  과 실제 binary 명 mismatch. `ps aux | grep prover` 로 검증 후 r11d.sh
  의 `pgrep -f` 패턴 조정.

## 7. R11-D closure 후 다음

R11-D 종료 → `PRODUCTION_ROADMAP.md §Stage R12` (GPU PoC). entry
instance 는 host RAM ≥128 GB (Phase 1 발견에서 도출) — g6e.8xl ($3.2/hr)
또는 g6.12xl ($2.5/hr) 후보.
