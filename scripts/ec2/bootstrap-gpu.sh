#!/usr/bin/env bash
# Install the GPU prove toolchain on the EC2 host for the `-tags icicle`
# GPU prover (PRODUCTION_ROADMAP R13-A/B/C). VERIFIED PATH (2026-06):
#
#   Launch the GPU box from a Deep Learning AMI (NVIDIA driver + CUDA
#   PREINSTALLED), NOT by self-installing the driver. The driver-build
#   route on a plain Amazon Linux 2023 box FAILS: AL2023 g6 boxes run a
#   6.18 kernel but the NVIDIA DKMS / kernel-modules-extra packages only
#   exist for the 6.1.x line, so the cuda-repo driver build dies on a
#   kernel-version conflict. The DLAMI ships a driver already built for its
#   kernel, sidestepping all of that.
#
#   DLAMI used (AL2023, x86_64, driver+CUDA preinstalled; ec2-user):
#     aws ssm get-parameter --region us-east-1 \
#       --name /aws/service/deeplearning/ami/x86_64/base-oss-nvidia-driver-gpu-amazon-linux-2023/latest/ami-id \
#       --query Parameter.Value --output text
#   (2026-06: ami-0f6660dbc23015eac). The DLAMI carries MULTIPLE CUDA
#   versions under /usr/local/cuda-* — this script builds Icicle v3.2.2
#   against CUDA 12.8 (icicle-gnark/v3 v3.2.2 is a CUDA-12 release; the
#   DLAMI default /usr/local/cuda may be 13.x, which Icicle 3.2.2 does not
#   build against).
#
# So the only thing this script does on a DLAMI box is build + install the
# Icicle v3 native CUDA libraries and set the runtime env. Run it after
# bootstrap.sh (Go) + sync.sh (code):
#
#   scripts/ec2/bootstrap-gpu.sh            # build Icicle v3 libs (CUDA 12.8)
#
# It is idempotent. Then the GPU prover is built on the box with:
#   CGO_ENABLED=1 go build -tags icicle -o /tmp/prover-gpu ./cmd/prover/
# (R13-A also requires the gnark fork vendored at ./gnark-fork with the
# Icicle v3 bundle applied — see docs/R13_GPU_RUNBOOK.md.)

set -euo pipefail
source "$(dirname "$0")/_lib.sh"

ICICLE_VERSION="v3.2.2"
CUDA_FOR_ICICLE="/usr/local/cuda-12.8"

log "GPU toolchain on $EC2_HOST — Icicle $ICICLE_VERSION native libs (CUDA 12.8)"
ec2_ssh bash -s <<EOF
set -euo pipefail

echo "--- GPU check (DLAMI driver must be preinstalled) ---"
if ! nvidia-smi --query-gpu=gpu_name,memory.total,driver_version --format=csv,noheader; then
  echo "ERROR: nvidia-smi failed. Launch the box from a Deep Learning AMI" >&2
  echo "       (driver preinstalled). Self-installing the driver on a plain" >&2
  echo "       AL2023 g6 box fails on the 6.18-kernel DKMS conflict." >&2
  exit 1
fi

if [ ! -d "$CUDA_FOR_ICICLE" ]; then
  echo "ERROR: $CUDA_FOR_ICICLE not found. Icicle $ICICLE_VERSION needs CUDA 12.x;" >&2
  echo "       the DLAMI usually ships it under /usr/local/cuda-12.8." >&2
  echo "       Available: \$(ls -d /usr/local/cuda-* 2>/dev/null | tr '\n' ' ')" >&2
  exit 1
fi

export CUDA_HOME="$CUDA_FOR_ICICLE"
export PATH="$CUDA_FOR_ICICLE/bin:\$PATH"
export CUDACXX="$CUDA_FOR_ICICLE/bin/nvcc"
export ICICLE_BACKEND_INSTALL_DIR=/usr/local/lib/backend
echo "nvcc: \$(nvcc --version | grep release)"

# Persist runtime env (idempotent).
if ! grep -q 'ICICLE_BACKEND_INSTALL_DIR' "\$HOME/.bashrc" 2>/dev/null; then
  {
    echo "export PATH=$CUDA_FOR_ICICLE/bin:\\\$PATH"
    echo "export LD_LIBRARY_PATH=/usr/local/lib:$CUDA_FOR_ICICLE/lib64:\\\$LD_LIBRARY_PATH"
    echo 'export ICICLE_BACKEND_INSTALL_DIR=/usr/local/lib/backend'
  } >> "\$HOME/.bashrc"
fi

echo "--- Building Icicle CUDA backend ($ICICLE_VERSION, curve bn254, CUDA 12.8) ---"
if [ ! -d "\$HOME/icicle-gnark" ]; then
  git clone --depth 1 --branch "$ICICLE_VERSION" https://github.com/ingonyama-zk/icicle-gnark.git "\$HOME/icicle-gnark"
fi
cd "\$HOME/icicle-gnark/wrappers/golang"
chmod +x build.sh
# build.sh runs 'cmake --build build --target install' → writes /usr/local/lib → needs sudo.
# Pass the CUDA 12.8 toolchain explicitly through sudo's clean env.
sudo env PATH="\$PATH" CUDACXX="\$CUDACXX" CUDA_HOME="\$CUDA_HOME" ./build.sh -curve=bn254
sudo ldconfig

echo "--- Installed Icicle libraries ---"
ls -1 /usr/local/lib/libicicle_* 2>/dev/null || { echo "ERROR: libicicle_* not installed" >&2; exit 1; }
ls -1 /usr/local/lib/backend/*/cuda/*.so 2>/dev/null || echo "WARNING: cuda backend libs not found under /usr/local/lib/backend"

echo ""
echo "=== GPU toolchain ready ==="
echo "Next (R13-A/B/C): vendor gnark fork (./gnark-fork + Icicle v3 bundle),"
echo "  CGO_ENABLED=1 go build -tags icicle -o /tmp/prover-gpu ./cmd/prover/,"
echo "  then run the prover with LD_LIBRARY_PATH=/usr/local/lib:$CUDA_FOR_ICICLE/lib64"
echo "  ICICLE_BACKEND_INSTALL_DIR=/usr/local/lib/backend. See docs/R13_GPU_RUNBOOK.md."
EOF
log "GPU toolchain installed — proceed with the gnark-fork vendor + -tags icicle build (docs/R13_GPU_RUNBOOK.md)"
