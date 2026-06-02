# R13 GPU Prove — Runbook & Verified Result

PRODUCTION_ROADMAP Stage **R13 (Prove-path 가속 & 스케일)**. This is the
end-to-end procedure that was actually run (2026-06) to port the GPU prove
path onto zkpor and validate it on a real GPU box, plus the measured result.

GPU acceleration is **R&D-done elsewhere**: the sibling `bb-por` track built
the Icicle v3 gnark-fork backend and benchmarked **2.3× at 64M constraints
(L40S)**. zkpor's R13 is a *port*, and this runbook is its validation.

## TL;DR result

| | nbConstraints | acceleration | prove (gnark) | prove (wall) | verify |
|---|---:|---|---:|---:|---|
| CPU | 174,463 | none | 499 ms | 655 ms | pass |
| **GPU (icicle)** | 174,463 | **icicle** | 1437 ms | 1643 ms | **pass** |

- ✅ **zkpor proves on GPU and the proof verifies** (`acceleration=icicle`,
  `ICICLE backend loaded`). The whole port — vendored Icicle v3 gnark fork,
  `-tags icicle` build, native CUDA libs — works end-to-end.
- ⚠️ At this **tiny** circuit (174k constraints) the GPU is **slower** than
  CPU — GPU setup/transfer overhead dominates small circuits. Speedup only
  appears at scale (bb-por: 64M → 2.3×). A tiny smoke validates *correctness*,
  not *speedup*.
- ✅ **G1 byte-equivalence preserved**: vendoring the v3 fork (base commit
  `4b5261061f04`, the same commit zkpor's go.mod already pins) leaves the
  T1 R1CS hash unchanged — the trusted-setup contract is intact.

## Hard-won environment facts

- **Do NOT self-install the NVIDIA driver on a plain AL2023 g6 box.** Those
  boxes run a 6.18 kernel; the cuda-repo NVIDIA DKMS + `kernel-modules-extra`
  only exist for the 6.1.x line → kernel-version conflict, driver build dies.
  Launch from a **Deep Learning AMI** (driver + CUDA preinstalled) instead.
- **Build Icicle against CUDA 12.8, not the DLAMI default.** The DLAMI ships
  several CUDA versions (`/usr/local/cuda-12.8 … 13.2`); the default
  `/usr/local/cuda` may be 13.x, which Icicle v3.2.2 does not build against.
- **First GPU prove can crash (cold start).** The very first `RunOnDevice`
  invocation crashed once (icicle async device init); the immediate re-run
  succeeded. Harden device init before production (bb-por's channel-sync note).

## Procedure

```bash
# 0. GPU instance from a DLAMI (driver+CUDA preinstalled). AMI via SSM:
aws ssm get-parameter --region us-east-1 \
  --name /aws/service/deeplearning/ami/x86_64/base-oss-nvidia-driver-gpu-amazon-linux-2023/latest/ami-id \
  --query Parameter.Value --output text          # 2026-06: ami-0f6660dbc23015eac
aws ec2 run-instances --region us-east-1 --image-id <ami> --instance-type g6.4xlarge \
  --key-name ue1-dev --subnet-id <subnet> --security-group-ids <sg> \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=zk-por-gpu-dlami}]'
# update scripts/ec2/.env (EC2_INSTANCE_ID + EC2_HOST=ec2-user@<ip>)

# 1. Go + code
scripts/ec2/bootstrap.sh        # Go 1.23.1 (driver/CUDA already there)
scripts/ec2/sync.sh             # rsync source

# 2. Icicle v3 native CUDA libs (CUDA 12.8)
scripts/ec2/bootstrap-gpu.sh

# 3. R13-A: vendor the gnark fork + apply the Icicle v3 bundle (on the box).
#    Base commit 4b5261061f04 == zkpor go.mod's pinned gnark → bundle applies.
#    The bundle lives in bb-por (apply_optimize_gnark_pr/gnark-fork-icicle-v3.bundle);
#    scp it to the box first.
cd <remote>/zkpor
git clone https://github.com/bnb-chain/gnark.git gnark-fork
cd gnark-fork && git checkout 4b5261061f04 && git checkout -B perf/bb-por-optimizations
git fetch ~/gnark-fork-icicle-v3.bundle perf/bb-por-optimizations && git reset --hard FETCH_HEAD
cd .. && go mod edit -replace github.com/consensys/gnark=./gnark-fork
go get github.com/ingonyama-zk/icicle-gnark/v3@v3.2.2

# 4. CPU build + G1 byte-equivalence (must pass — fork swap is contract-safe)
go build ./cmd/...
go test ./core/solvency/t1_simple_margin/circuit/...     # R1CS hash assertion

# 5. GPU build (R13-B seam wires backend.WithIcicleAcceleration via core/host.ProverOptions)
CGO_ENABLED=1 go build -tags icicle -o /tmp/prover-gpu ./cmd/prover/
ldd /tmp/prover-gpu | grep icicle                        # libicicle_*.so linked

# 6. GPU prove smoke (reuse the smoke harness for keygen+witness, then GPU prove)
PROFILE=profile/t1_reference/t1_reference.toml CAPACITY=50 SHAPE=5_10 \
  scripts/ec2/smoke.sh custom                            # CPU pipeline + keys + witness
# reset queue + run the GPU binary:
docker exec zkpor-smoke-mysql mysql -uzkpor -p'zkpor@123' zkpor \
  -e "DELETE FROM proof; UPDATE witness SET status=0;"
cd cmd/prover && LD_LIBRARY_PATH=/usr/local/lib:/usr/local/cuda-12.8/lib64 \
  ICICLE_BACKEND_INSTALL_DIR=/usr/local/lib/backend ZKPOR_BATCH_SHAPE_OVERRIDE=5_10 \
  /tmp/prover-gpu -profile <t1.toml> -keys-dir <repo>/.artifacts
# → "acceleration=icicle", "proof verify success"

# 7. Stop the box (halt billing)
aws ec2 stop-instances --region us-east-1 --instance-ids <id>
```

## Permanent-integration decision (still open)

R13-A here is **box-local** (the fork vendor + go.mod edits live only on the
GPU box, not committed to zkpor). Permanent integration has an open choice,
to decide before R13 ships:

- **(a) vendor `./gnark-fork` into the repo** (bb-por's way) — repo bloat
  (the gnark fork is large), but self-contained.
- **(b) push a BetweenBits gnark fork** (base + Icicle v3 bundle) and point
  go.mod's `replace` at it — keeps the app repo clean.

The on/off seam (R13-B, `core/host.ProverOptions` + `-tags icicle`) and the
multi-instance claim-safety (R13-D) are already committed to zkpor.
