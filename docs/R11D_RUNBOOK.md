# R11-D Runbook — Setup/Prove ablation 측정

`docs/BENCHMARK.md` §4.1 의 8-cell plan 실행 절차. 본 문서는 EC2 launch
부터 cell 별 측정, 결과 fold-in 까지 단계별 체크리스트.

목표: T4 production (cap=500, shape=50_700+500_92) 의 **Setup 한 번 →
Prove ablation 6 cell + 10K multi-batch sanity 2 cell** 측정. 예산
~$8-10, wall-clock ~2-3hr.

## 1. EC2 launch 사전 결정

| 항목 | 값 |
|---|---|
| Region 우선순위 | us-east-2 (Ohio) → us-east-1 fallback (AZ 가용성에 따라) |
| 초기 인스턴스 | **m8a.8xlarge** (Setup + cells 5,6,7,8 처리) |
| AMI | Amazon Linux 2023 (`al2023-ami-*-x86_64`) |
| EBS | gp3 **150 GB**, `DeleteOnTermination=true` |
| Security group | SSH(22) 만 허용 |
| Key pair | 기존 dev key 재활용 (`ue1-dev` 또는 region 별 신규) |
| Cost ceiling | $15 CloudWatch alarm (안전 margin) |

**Fallback**: 선택 AZ 가 InsufficientInstanceCapacity 반환 시 다른 AZ
로 즉시 재시도. 동일 region 안에서만 EBS 재사용 가능 (region 간 이전
시 snapshot 필요 → R11-D 한도 초과).

## 2. Cell 정의

`scripts/ec2/r11d.sh <cell>` 가 모든 cell parameter (testdata gen +
shape override + asset cap) 를 enforce. 사용자는 cell name 만 지정.

| Cell | Shape | Users | AssetCount | Batches | 측정 목적 |
|---|---|---:|---:|---:|---|
| `setup` | 50_700,500_92 | 700 | 0 (default) | — | initial keygen (`.pk`/`.vk`/`.r1cs`) |
| `t1_700` | 50_700 | 700 | 50 | 1 | Tier 1 isolation, 1-batch prove |
| `t2_92` | 500_92 | 92 | 500 | 1 | Tier 2 isolation, 1-batch prove |
| `t1_10k` | 50_700 | 10000 | 50 | 15 | Tier 1 multi-batch sanity |
| `t2_10k` | 500_92 | 10000 | 500 | 109 | Tier 2 multi-batch sanity |

**핵심 invariant**: `--asset-count` (per-user non-empty asset count) 가
shape 의 `asset_count_tier` 와 일치해야 라우팅 일관성 유지. r11d.sh 가
강제.

## 3. 측정 순서 (type-switch 최소화)

instance type-switch 가 cell 보다 비싸므로 instance 별로 묶어 처리:

```
m8a.8xlarge (Setup + 4 cells)
  setup       — keygen .pk × 2 shape (~1hr)
  t1_700      — 1-batch Tier 1 (~수 분)
  t2_92       — 1-batch Tier 2 (~수 분)
  t1_10k      — Tier 1 multi-batch (15 batches × ~10s = ~3min)
  t2_10k      — Tier 2 multi-batch (109 batches × ~17s = ~30min)
  ↓ switch_type.sh m8a.4xlarge
m8a.4xlarge (2 cells)
  t1_700      — 1-batch Tier 1
  t2_92       — 1-batch Tier 2
  ↓ switch_type.sh m7a.4xlarge
m7a.4xlarge (2 cells)
  t1_700      — 1-batch Tier 1
  t2_92       — 1-batch Tier 2
  ↓ aws ec2 terminate-instances
```

총 **8 cell × 1회씩 + setup 1회 = 9회 실행**. 모든 결과는
`.artifacts/reports/R11D_<instance_tag>_<cell>/run_<ts>.{log,json,meta.json}`
으로 누적.

## 4. 절차

### 4.1 초기 launch (m8a.8xlarge)

```bash
# 1. AWS console 또는 CLI 로 instance launch (us-east-2 권장):
aws ec2 run-instances --region us-east-2 \
  --image-id <al2023-ami-id> --instance-type m8a.8xlarge \
  --key-name ue2-dev --security-group-ids <sg-id> \
  --block-device-mappings 'DeviceName=/dev/xvda,Ebs={VolumeSize=150,VolumeType=gp3,DeleteOnTermination=true}' \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=zkpor-r11d}]'

# 2. .env 작성 (instance ID + 새 IP 포함):
cat > scripts/ec2/.env <<EOF
EC2_HOST=ec2-user@<public-ip>
EC2_KEY=~/path/to/ue2-dev.pem
EC2_REMOTE_DIR=/home/ec2-user/zkmerkle-proof-of-solvency
EC2_INSTANCE_ID=i-xxxxxxxx
AWS_REGION=us-east-2
EOF
chmod 600 ~/path/to/ue2-dev.pem

# 3. bootstrap:
./scripts/ec2/bootstrap.sh
./scripts/ec2/sync.sh
```

### 4.2 Setup phase (m8a.8xlarge)

```bash
# 1. setup cell — keygen 2 shape × 1회
ssh ec2-user@<ip> "cd zkmerkle-proof-of-solvency/zkpor && \
  INSTANCE_TAG=m8a.8xl scripts/ec2/r11d.sh setup"
```

예상 wall-clock: keygen 50_700 (~25min) + keygen 500_92 (~25min) +
짧은 smoke validation = ~50-60min. `.artifacts/zkpor.*.pk` 24GB × 2
보존 — 후속 prove cell 에서 lazy reload.

### 4.3 Prove cells on m8a.8xlarge (4 cells)

```bash
ssh ec2-user@<ip> "cd zkmerkle-proof-of-solvency/zkpor && \
  INSTANCE_TAG=m8a.8xl scripts/ec2/r11d.sh t1_700  && \
  INSTANCE_TAG=m8a.8xl scripts/ec2/r11d.sh t2_92   && \
  INSTANCE_TAG=m8a.8xl scripts/ec2/r11d.sh t1_10k  && \
  INSTANCE_TAG=m8a.8xl scripts/ec2/r11d.sh t2_10k"
```

### 4.4 Switch to m8a.4xlarge + 2 cells

```bash
./scripts/ec2/switch_type.sh m8a.4xlarge
./scripts/ec2/sync.sh
ssh ec2-user@<new-ip> "cd zkmerkle-proof-of-solvency/zkpor && \
  INSTANCE_TAG=m8a.4xl scripts/ec2/r11d.sh t1_700 && \
  INSTANCE_TAG=m8a.4xl scripts/ec2/r11d.sh t2_92"
```

### 4.5 Switch to m7a.4xlarge + 2 cells

```bash
./scripts/ec2/switch_type.sh m7a.4xlarge
./scripts/ec2/sync.sh
ssh ec2-user@<new-ip> "cd zkmerkle-proof-of-solvency/zkpor && \
  INSTANCE_TAG=m7a.4xl scripts/ec2/r11d.sh t1_700 && \
  INSTANCE_TAG=m7a.4xl scripts/ec2/r11d.sh t2_92"
```

### 4.6 결과 회수

```bash
./scripts/ec2/fetch.sh  # .artifacts/reports/ 동기화
ls -1 .artifacts/reports/R11D_*/
```

### 4.7 정리

```bash
./scripts/ec2/down.sh
aws ec2 terminate-instances --region us-east-2 --instance-ids $EC2_INSTANCE_ID
```

EBS `DeleteOnTermination=true` 이므로 volume 자동 회수. 비용 leak 없음.

## 5. 측정 결과 fold-in

각 cell 의 `.json` 을 `docs/BENCHMARK.md §2` 의 새 measurement 섹션
(`### 2.6 R11-D Setup/Prove ablation`) 으로 합치고, §3 의 fit
(NbConstraints → prove, instance speedup) 을 새 data 로 업데이트.

자동화 helper (현재 미작성, 필요 시 후속):
- `.artifacts/reports/R11D_*/run_*.json` → markdown table compose
- Per-batch prove time variance 분석 (cell *_10k vs *_1batch 비교)

## 6. 예상 비용 / 시간

| 단계 | Wall-clock | Instance × 시간 | 단가 ($/hr) | 비용 |
|---|---|---|---:|---:|
| Launch + bootstrap | ~10min | m8a.8xl × 0.17hr | 1.80 | $0.31 |
| Setup | ~1hr | m8a.8xl × 1hr | 1.80 | $1.80 |
| 4 cells on m8a.8xl | ~45min | m8a.8xl × 0.75hr | 1.80 | $1.35 |
| type-switch + 2 cells on m8a.4xl | ~20min | m8a.4xl × 0.33hr | 0.92 | $0.30 |
| type-switch + 2 cells on m7a.4xl | ~20min | m7a.4xl × 0.33hr | 0.92 | $0.30 |
| EBS (gp3 150GB × 3hr) | — | — | 0.013/GB-month | ~$0.03 |
| 오버헤드 / 재시도 여유 | — | — | — | ~$2 |
| **합** | **~2.5-3hr** | | | **~$6-8** |

(BENCHMARK §1.6 의 $8-10 추정 안에 들어옴)

## 7. 트러블슈팅

- **InsufficientInstanceCapacity** (특정 AZ): 다른 AZ 에서 launch 재시도
  (us-east-2a/b/c 순회). EBS 함께 신규 생성.
- **smoke.sh keygen artifact mismatch**: `.artifacts/zkpor.*.r1cs` 가
  현재 코드와 다른 git commit 의 회로일 때 `groth16.Prove` 가 mismatch.
  → 해당 `.pk/.vk/.r1cs` 삭제 후 r11d.sh setup 재실행
- **type-switch 후 SSH timeout**: 새 public IP 가 .env 갱신됐는데 SSH
  fingerprint 미신규 → `ssh-keygen -R <new-ip>` + 재시도
- **`.pk` lazy reload 시 Tier 1↔Tier 2 전환 비용**: r11d.sh 의 cell
  순서 (`t1_*` → `t2_*`) 가 항상 Tier 1 → Tier 2 reload 1회 유발. 측정
  시 첫 batch 의 setup overhead 별도 계산 필요.

## 8. 후속

R11-D closure → `PRODUCTION_ROADMAP.md §Stage R12` (GPU PoC) 진입.
