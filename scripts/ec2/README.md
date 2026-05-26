# EC2 원격 테스트 헬퍼

`scripts/ec2/` 는 로컬 ↔ EC2 인스턴스 동기화 + 원격 smoke 실행을 위한
얇은 rsync/ssh 래퍼 셋. 로컬 macOS 에서 production capacity (=500)
smoke 를 돌리기엔 CPU/RAM 이 빠듯하지만 EC2 (m6i.4xlarge 권장) 에서는
충분 — 이 스크립트들이 그 흐름을 1줄로 만든다.

## 권장 인스턴스 사양

| Instance | Resources | 비고 |
| --- | --- | --- |
| **m6i.4xlarge** | 16 vCPU / 64 GB | ★ 추천. keygen Setup 피크 (~25GB) + prover .pk 12GB 모두 헤드룸 있음 |
| m6i.2xlarge | 8 vCPU / 32 GB | 동작은 하나 keygen Setup 단계에서 swap 가능성. 시간이 더 듦 |
| m6i.8xlarge | 32 vCPU / 128 GB | 두 shape 병렬 keygen 시에만 의미 있음 |

- AMI: Ubuntu 22.04 LTS 또는 Amazon Linux 2023
- EBS: gp3 **150~200 GB** (.artifacts ~50GB + Docker images + Go cache)
- 보안 그룹: SSH(22) 만. MySQL(3306) 은 localhost only (smoke 가 컨테이너 내부).

## 사용 흐름

### 0. 설정 (한 번)

`scripts/ec2/.env` 파일을 만들어 다음 값을 채운다 (gitignore 됨):

```bash
EC2_HOST=ubuntu@xx.xx.xx.xx          # ssh 대상
EC2_KEY=~/.ssh/zkpor-ec2.pem         # 키 파일 (선택; ssh-agent 가 들고 있으면 빈 값)
EC2_REMOTE_DIR=~/zkmerkle-proof-of-solvency
```

### 1. 인스턴스 부트스트랩 (한 번)

```bash
# 로컬에서:
./scripts/ec2/bootstrap.sh
```

→ EC2 에 ssh 해서 Docker + Go (toolchain 일치) + git 설치. apt cache 갱신.

### 2. 코드 sync (반복)

```bash
# 로컬 변경을 EC2 로 푸시 (delete-after, .git/.artifacts/.pk 등 제외)
./scripts/ec2/sync.sh
```

→ `zkmerkle-proof-of-solvency/` 전체를 rsync. 첫 실행 ~1분, 이후 증분 ~수 초.

### 3. 원격 smoke 실행

```bash
# 기본: production capacity (500) + production shape
./scripts/ec2/smoke.sh

# tiny smoke (로컬과 동일):
./scripts/ec2/smoke.sh tiny
```

→ EC2 에서 `./scripts/smoke.sh` 를 실행. 출력은 로컬 터미널로 스트림.

### 4. artifact fetch (선택)

keygen 출력 (.pk/.vk/.r1cs) 을 로컬로 가져와 캐시:

```bash
./scripts/ec2/fetch.sh
```

→ EC2 `.artifacts/` → 로컬 `.artifacts/`. 파일 큼 (~수 GB), 시간 소요.
로컬에서 이후 prover/verifier 단독 실행 시 유용.

### 5. 정리

```bash
./scripts/ec2/down.sh   # 원격 docker compose down -v (DB 볼륨 제거)
```

EC2 인스턴스 자체 종료는 AWS console 또는 `aws ec2 terminate-instances`
로 별도 처리 — 비용 관리 책임 분리.

## 주의

- `.env` 는 절대 commit 하지 말 것 (이미 gitignore).
- production capacity keygen 은 ~30분~수 시간 (shape 별로 다름). spot
  instance 시 interruption 시 처음부터 다시. dedicated on-demand 권장.
- EBS 용량 부족 시 `df -h` 로 모니터링. .artifacts 가 가장 큰 소비처.
