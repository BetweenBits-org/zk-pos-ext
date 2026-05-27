# EC2 원격 테스트 헬퍼

`scripts/ec2/` 는 로컬 ↔ EC2 인스턴스 동기화 + 원격 smoke 실행을 위한
얇은 rsync/ssh 래퍼 셋. 로컬 macOS 에서 production capacity (=500)
smoke 를 돌리기엔 CPU/RAM 이 빠듯하지만 EC2 (m7a.4xlarge / m7i.4xlarge
권장) 에서는 충분 — 이 스크립트들이 그 흐름을 1줄로 만든다.

## 권장 인스턴스 사양

V1 (binance reference, capacity=500) production-shape keygen 의 **실측
peak RAM 87GB** — m-family 4xlarge (64GB) 는 OOM. r-family 또는 m-
family 8xlarge 부터 안전.

| Instance | Resources | 비고 |
| --- | --- | --- |
| **r7a.4xlarge** | 16 vCPU AMD Genoa 3.7GHz / **128 GB** | ★ 검증됨. 2026-05-27 production-shape smoke 통과 |
| r7i.4xlarge | 16 vCPU Intel SPR 3.2GHz / 128 GB | $1.06/hr (us-east-1, r7a 대비 ~13% 저렴). 동일 RAM 안전, AMD 대비 5-15% prove time 가능성 |
| m7a.8xlarge | 32 / 128 GB | Setup multi-thread 활용 시 시간 단축. $1.85/hr (r7a 의 ~1.5x). prove SLA 비교 측정 후보 |
| m7i.8xlarge | 32 / 128 GB | Intel 32 vCPU. $1.61/hr |
| ~~m7a.4xlarge / m7i.4xlarge~~ (4xlarge, 64GB) | — | **production capacity=500 에서는 OOM** (peak 87GB 측정). tiny smoke 만 가능 |

비추: c-family (1:2 RAM, OOM risk), Graviton/m7g (gnark amd64
assembly fast-path 부재로 30%+ 느림), 4xlarge (64GB — OOM 확정).

- **AMI**: Amazon Linux 2023 (`al2023-ami-*-x86_64`) 또는 Ubuntu 22.04 LTS.
  bootstrap.sh 가 dnf / apt-get 둘 다 detect.
- **EBS**: gp3 **150 GB**. `.artifacts/` production .pk **24GB × 2 shape**
  (50_700 + 500_92) + .r1cs ~7GB + Docker images + Go module cache
  ≈ **실측 55GB 사용** (97GB 여유).
- **보안 그룹**: SSH(22) 만. MySQL(3306) 은 컨테이너 localhost only.

## 현재 dev 인스턴스 (참조)

| 항목 | 값 |
|---|---|
| Name | `zk-por-dev` |
| InstanceId | `i-0f4b93a48a192dbac` |
| Type | `r7a.4xlarge` (m7a.4xlarge 에서 resize, 2026-05-27 OOM 후) |
| Region / AZ | `us-east-1` / `us-east-1c` |
| AMI | Amazon Linux 2023 (kernel 6.18) |
| Public IP | **stop/start 마다 변경됨**. `aws ec2 describe-instances` 로 확인 후 `.env` 갱신 |
| SSH user | `ec2-user` |
| KeyName | `ue1-dev` |
| EBS root | gp3 150GB, 3000 IOPS, 125 MiB/s |

`.env` 가 이 값들을 기본으로 가지면 바로 동작. 인스턴스 교체/restart 시
위 표 + `.env` + `~/.ssh/known_hosts` (옛 IP 라인) 같이 갱신.

### stop / start 절차 (비용 절감)

dev 작업 중간에 idle 이면 stop 으로 EC2 compute 과금 0 (EBS storage
~$12/month 만 남음, `.artifacts/` 24GB×2 .pk 보존):

```bash
# stop
aws ec2 stop-instances --region us-east-1 --instance-ids i-0f4b93a48a192dbac

# 다음 세션 — start + 새 public IP 받기
aws ec2 start-instances --region us-east-1 --instance-ids i-0f4b93a48a192dbac
aws ec2 wait instance-running --region us-east-1 --instance-ids i-0f4b93a48a192dbac
NEW_IP=$(aws ec2 describe-instances --region us-east-1 --instance-ids i-0f4b93a48a192dbac \
  --query 'Reservations[0].Instances[0].PublicIpAddress' --output text)
echo "$NEW_IP"
# → .env 의 EC2_HOST 갱신 + ssh-keygen -R <old-ip>
```

Elastic IP 할당하면 stop/start 시 IP 고정 — 단 unassociated 상태에서
시간당 과금 ($0.005/hr ≈ $3.6/month). dev 단계에선 매번 갱신이 더 저렴.

## 사용 흐름

### 0. 설정 (한 번)

`scripts/ec2/.env` 가 이미 위 dev 인스턴스 default 로 작성되어 있다
(gitignore 됨). 다른 host / 다른 key 면 그 파일을 직접 수정:

```bash
EC2_HOST=ec2-user@3.237.174.138
EC2_KEY=/Users/betweenbits/Documents/keypairs/ue1-dev.pem
EC2_REMOTE_DIR=/home/ec2-user/zkmerkle-proof-of-solvency
```

비공개 키 권한 600 으로 (ssh 가 거부함):
`chmod 600 "$EC2_KEY"`.

### 1. 인스턴스 부트스트랩 (한 번)

```bash
./scripts/ec2/bootstrap.sh
```

→ EC2 에 ssh 해서 Docker + Go 1.23.1 (parent go.mod toolchain pin) + git
설치. dnf (AL2023) 또는 apt-get (Ubuntu) 자동 detect. docker group
멤버십 추가 시 노트 출력 — 한 번 logout/login 또는 `newgrp docker`.

### 2. 코드 sync (반복)

```bash
./scripts/ec2/sync.sh
```

→ `zkmerkle-proof-of-solvency/` 전체를 rsync (parent + zkpor 둘 다).
첫 실행 ~1분, 이후 증분 ~수 초. 제외 패턴: `.git/`, `.artifacts/`,
`*.pk`/`*.vk`/`*.r1cs`, IDE config, 서비스 config.json (smoke 가 매번
재생성하므로 stale 방지).

### 3. 원격 smoke 실행

```bash
# 기본: production capacity (500) + shapes 50_700,500_92
./scripts/ec2/smoke.sh

# tiny smoke (로컬과 동일):
./scripts/ec2/smoke.sh tiny
```

→ EC2 에서 `./zkpor/scripts/smoke.sh` 실행. R8 wiring 후의 smoke.sh
는 내부에서 `-profile profile/binance/binance.toml` 을 자동으로
모든 service 에 전달. `ZKPOR_BATCH_SHAPE_OVERRIDE` 와
`ZKPOR_SMOKE_ASSET_CAPACITY` 만 env 로 넘기면 됨.

production mode 실측 (2026-05-27, r7a.4xlarge, capacity=500,
sample data 170 valid accounts → 17 batches × 10 users):

| 단계 | 실측 시간 | 비고 |
|---|---:|---|
| keygen 50_700 compile | **11m59s** | 64,341,094 constraints, single-threaded |
| keygen 50_700 groth16.Setup | **31m17s** | multi-threaded (load avg ~16, all vCPU) |
| keygen 50_700 .pk write | ~2분 | 24GB / gp3 125 MiB/s baseline |
| keygen 500_92 compile | **13m25s** | 63,822,805 constraints |
| keygen 500_92 groth16.Setup | **29m31s** | RAM peak 87GB (m7a 4xlarge OOM) |
| keygen 500_92 .pk write | ~2분 | 24GB |
| witness (sample) | < 1분 | 170 accounts, account tree root 결정 |
| prover (all 17 batches) | ~2-3분 (집계) | 각 batch 의 R1CS+pk+vk lazy load + Prove+Verify |
| verifier batch (17 proofs) | < 30초 | worker pool 16 vCPU |
| userproof (170 rows) | < 1분 | tree 재구축 + per-account proof |
| verifier -user | 즉시 | 단일 leaf 재계산 |
| **총** | **~1h 50m** | r7a.4xlarge, sample-data 기준 |

artifact 크기 (실측):
- `zkpor.t4_tiered_haircut_margin_3pool.50_700.pk` = **25,199,017,335 bytes (24GB)**
- `zkpor.t4_tiered_haircut_margin_3pool.50_700.r1cs` = 4,167,684,563 bytes (3.9GB)
- `zkpor.t4_tiered_haircut_margin_3pool.50_700.vk` = 528 bytes
- `zkpor.t4_tiered_haircut_margin_3pool.500_92.pk` = ~24GB (동일 규모)
- `zkpor.t4_tiered_haircut_margin_3pool.500_92.r1cs` = ~2.8GB
- 합계: `.artifacts/` 약 **55GB** (150GB 디스크 의 37%)

production capacity keygen 의 진짜 RAM peak 는 ~87GB — Setup 의 toxic-
waste expansion 단계. r7a.4xlarge (128GB) 안전 영역.

### 4. artifact fetch (선택)

```bash
./scripts/ec2/fetch.sh
```

→ EC2 `.artifacts/` → 로컬 `.artifacts/`. production `.pk` 는 GB
단위라 시간 소요. 로컬에서 prover/verifier 단독 분석 시 유용.

### 5. 정리

```bash
./scripts/ec2/down.sh
```

→ 원격 docker compose down -v (MySQL 볼륨 제거). EC2 인스턴스 자체
종료는 AWS console 또는 `aws ec2 stop-instances --instance-ids
i-0f4b93a48a192dbac --region us-east-1` (stop 은 EBS 만 과금, terminate
는 영구).

## 주의

- `.env` 는 gitignore 되어 있으나 keypair 절대 경로가 들어가므로 다른
  머신으로 옮길 때 .env 도 같이 옮기거나 환경별로 작성.
- **RAM bound**: production capacity=500 의 Setup peak 87GB. m-family
  4xlarge (64GB) 는 명백한 OOM. r-family 또는 m-family 8xlarge 이상
  필수.
- production capacity keygen 은 단발 분 단위 (~1시간) — spot interruption
  시 처음부터 재시작. dev 인스턴스는 on-demand 권장.
- EBS 용량은 `df -h` 로 모니터링. `.artifacts/` 가 가장 큰 소비처
  (실측 55GB / 150GB).
- R8 wiring 후 모든 cmd 가 `-profile profile/binance/binance.toml`
  필수. 다른 customer 로 smoke 변형 시 smoke.sh 안의 그 경로 + 이
  README 의 mode 표를 같이 변경.
- public IP 는 stop/start 마다 바뀜. `.env` + `known_hosts` 같이 갱신
  (위 stop/start 절차 참고).

## 알려진 fix

- `commit d59654e` — smoke.sh `ensure_keys()` 의 boolean 표현 +
  `.artifacts/` mkdir 누락 fix. 로컬에서 cache 잔존 때문에 가려졌던
  bug, fresh EC2 host 에서 surface. fix 후 production smoke 정상 통과.
