# EC2 원격 테스트 헬퍼

`scripts/ec2/` 는 로컬 ↔ EC2 인스턴스 동기화 + 원격 smoke 실행을 위한
얇은 rsync/ssh 래퍼 셋. 로컬 macOS 에서 production capacity (=500)
smoke 를 돌리기엔 CPU/RAM 이 빠듯하지만 EC2 (m7a.4xlarge / m7i.4xlarge
권장) 에서는 충분 — 이 스크립트들이 그 흐름을 1줄로 만든다.

## 권장 인스턴스 사양

| Instance | Resources | 비고 |
| --- | --- | --- |
| **m7a.4xlarge** | 16 vCPU AMD Genoa 3.7GHz / 64 GB | ★ 추천. gnark BN254 BMI2 hot-path 에서 Zen 4 우위 |
| m7i.4xlarge | 16 vCPU Intel SPR 3.2GHz / 64 GB | 가격 ~13% 저렴 (us-east-1). 동일 RAM headroom |
| m7a.8xlarge / m7i.8xlarge | 32 / 128 GB | multi-shape concurrent 또는 RAM 압박 발견 시 |
| r7i.4xlarge | 16 / 128 GB | capacity ≥ 1000 또는 multi-worker prover 시점 |

비추: c-family (1:2 RAM, OOM risk), Graviton/m7g (gnark amd64
assembly fast-path 부재로 30%+ 느림).

- **AMI**: Amazon Linux 2023 (`al2023-ami-*-x86_64`) 또는 Ubuntu 22.04 LTS.
  bootstrap.sh 가 dnf / apt-get 둘 다 detect.
- **EBS**: gp3 **150 GB** (zk-por-dev 의 현재 디스크). `.artifacts/`
  production `.pk` 12GB × 2 shape + Docker images + Go cache ≈ 50-70GB.
- **보안 그룹**: SSH(22) 만. MySQL(3306) 은 컨테이너 localhost only.

## 현재 dev 인스턴스 (참조)

| 항목 | 값 |
|---|---|
| Name | `zk-por-dev` |
| InstanceId | `i-0f4b93a48a192dbac` |
| Type | `m7a.4xlarge` |
| Region / AZ | `us-east-1` / `us-east-1c` |
| AMI | Amazon Linux 2023 (kernel 6.18) |
| Public IP | `3.237.174.138` |
| SSH user | `ec2-user` |
| KeyName | `ue1-dev` |

`.env` 가 이 값들을 기본으로 가지면 바로 동작. 인스턴스 교체 시 위
표 + `.env` 를 같이 갱신.

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

production mode 시점 예측 (m7a.4xlarge 기준 추정 — 실측 시 README
업데이트):

| 단계 | 추정 시간 |
|---|---:|
| keygen 50_700 (compile + Setup) | 5-15 분 |
| keygen 500_92 (compile + Setup) | 10-25 분 |
| witness (sample data) | < 1 분 |
| prover (per batch) | 수 초 ~ 수십 초 (SLA measurement target) |
| verifier batch | < 1 분 |
| userproof + verifier -user | < 1 분 |

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
- production capacity keygen 은 단발 분 단위 — spot interruption 시
  처음부터 재시작. dev 인스턴스는 on-demand 권장.
- EBS 용량은 `df -h` 로 모니터링. `.artifacts/` 가 가장 큰 소비처.
- R8 wiring 후 모든 cmd 가 `-profile profile/binance/binance.toml`
  필수. 다른 customer 로 smoke 변형 시 smoke.sh 안의 그 경로 + 이
  README 의 mode 표를 같이 변경.
