#!/usr/bin/env bash
# Install the GPU prove toolchain on the EC2 host: NVIDIA driver + CUDA
# toolkit + Icicle v3 native CUDA libraries. This is what the `-tags icicle`
# GPU prover (PRODUCTION_ROADMAP R13-A/B/C) needs at link/run time. Parallel
# to bootstrap.sh (Docker + Go) — run this AFTER it, and AFTER switching the
# instance to a GPU type:
#
#   scripts/ec2/switch_type.sh g6e.4xlarge     # L40S 48GB  (~$4.19/hr) — T4-scale
#   scripts/ec2/switch_type.sh g6.4xlarge      # L4  24GB   (cheaper)   — T1/mid
#   scripts/ec2/bootstrap.sh                   # Docker + Go (once)
#   scripts/ec2/bootstrap-gpu.sh install       # driver + CUDA  → then reboot the box
#   scripts/ec2/bootstrap-gpu.sh post-reboot   # Icicle v3 native libs + verify
#
# A reboot is REQUIRED between `install` and `post-reboot` so the NVIDIA
# kernel module loads. Reboot the box with:
#   aws ec2 reboot-instances --region "$AWS_REGION" --instance-ids "$EC2_INSTANCE_ID"
# (wait ~60s, refresh EC2_HOST IP if it changed, then run post-reboot).
#
# OS support: detects dnf (Amazon Linux 2023) vs apt (Ubuntu 24.04). The
# bb-por reference benchmark (2.3x on L40S) ran on Ubuntu 24.04 with
# nvidia-driver-535 + cuda-toolkit-12-8. The AL2023 path uses the amzn2023
# CUDA repo + dnf driver module and is LESS battle-tested — if the AL2023
# driver install fights you, relaunch the box from an Ubuntu 24.04 GPU AMI
# (or an AWS Deep Learning AMI with CUDA preinstalled) and re-run.
#
# Reference: bb-por apply_optimize_gnark_pr/{01_gpu_benchmark_setup.sh,
# ICICLE_V3_UPGRADE_PLAN.md}. The gnark-fork Icicle v3 swap itself (R13-A)
# is a repo change (vendor the fork at commit 4b5261061f04 — the SAME base
# zkpor's go.mod already pins — then apply the bb-por bundle), not part of
# this host-setup script.

set -euo pipefail
source "$(dirname "$0")/_lib.sh"

ICICLE_VERSION="v3.2.2"
PHASE="${1:-}"

usage() {
  echo "usage: $0 <install|post-reboot>" >&2
  echo "  install      — NVIDIA driver + CUDA toolkit (reboot after)" >&2
  echo "  post-reboot   — Icicle $ICICLE_VERSION native libs + GPU verify" >&2
  exit 2
}

case "$PHASE" in
  install) ;;
  post-reboot) ;;
  *) usage ;;
esac

# ── install: NVIDIA driver + CUDA toolkit ───────────────────────────────────
if [ "$PHASE" = "install" ]; then
  log "GPU install on $EC2_HOST — NVIDIA driver + CUDA toolkit"
  ec2_ssh bash -s <<'EOF'
set -euo pipefail

if nvidia-smi >/dev/null 2>&1; then
  echo "nvidia-smi already works — driver present"
  nvidia-smi --query-gpu=gpu_name,memory.total --format=csv,noheader
  echo "skip driver install; proceed to post-reboot"
  exit 0
fi

if command -v apt-get >/dev/null 2>&1; then
  echo "=== Ubuntu path (apt) ==="
  sudo apt-get update -qq
  sudo apt-get install -y build-essential cmake pkg-config wget curl git
  wget -q https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2404/x86_64/cuda-keyring_1.1-1_all.deb -O /tmp/cuda-keyring.deb
  sudo dpkg -i /tmp/cuda-keyring.deb
  sudo apt-get update -qq
  sudo apt-get install -y cuda-toolkit-12-8 nvidia-driver-535
elif command -v dnf >/dev/null 2>&1; then
  echo "=== Amazon Linux 2023 path (dnf) — less-tested, see header note ==="
  sudo dnf install -y kernel-devel-"$(uname -r)" kernel-modules-extra gcc make cmake pkgconfig wget curl git || \
    sudo dnf install -y kernel-devel kernel-modules-extra gcc make cmake pkgconfig wget curl git
  sudo dnf config-manager --add-repo \
    https://developer.download.nvidia.com/compute/cuda/repos/amzn2023/x86_64/cuda-amzn2023.repo
  sudo dnf clean all
  # cuda-toolkit pulls the matching driver (open-dkms) on AL2023.
  sudo dnf -y module install nvidia-driver:latest-dkms || true
  sudo dnf install -y cuda-toolkit-12-8
else
  echo "unknown package manager (need apt-get or dnf)" >&2
  exit 1
fi

echo ""
echo "=== driver + CUDA installed — REBOOT REQUIRED to load the kernel module ==="
echo "from your laptop:"
echo "  aws ec2 reboot-instances --region <region> --instance-ids <id>"
echo "then run: scripts/ec2/bootstrap-gpu.sh post-reboot"
EOF
  log "install phase done — reboot the instance, then run: $0 post-reboot"
  exit 0
fi

# ── post-reboot: Icicle v3 native libs + verify ─────────────────────────────
log "GPU post-reboot on $EC2_HOST — verify driver + build Icicle $ICICLE_VERSION native libs"
ec2_ssh bash -s <<EOF
set -euo pipefail

echo "--- GPU check ---"
nvidia-smi || { echo "ERROR: nvidia-smi failed — driver not loaded. Did you reboot after 'install'?"; exit 1; }
nvidia-smi --query-gpu=gpu_name,memory.total,driver_version --format=csv,noheader

# Persist CUDA + Icicle backend env (idempotent).
if ! grep -q 'ICICLE_BACKEND_INSTALL_DIR' "\$HOME/.bashrc" 2>/dev/null; then
  {
    echo 'export PATH=/usr/local/cuda/bin:\$PATH'
    echo 'export LD_LIBRARY_PATH=/usr/local/cuda/lib64:\$LD_LIBRARY_PATH'
    echo 'export ICICLE_BACKEND_INSTALL_DIR=/usr/local/lib/backend'
  } >> "\$HOME/.bashrc"
fi
export PATH=/usr/local/cuda/bin:\$PATH
export ICICLE_BACKEND_INSTALL_DIR=/usr/local/lib/backend

echo "--- Building Icicle CUDA backend ($ICICLE_VERSION, curve bn254) ---"
if [ ! -d "\$HOME/icicle-gnark" ]; then
  git clone --depth 1 --branch "$ICICLE_VERSION" https://github.com/ingonyama-zk/icicle-gnark.git "\$HOME/icicle-gnark"
fi
cd "\$HOME/icicle-gnark/wrappers/golang"
chmod +x build.sh
# build.sh runs 'cmake --build build --target install' → writes /usr/local/lib → needs sudo.
sudo PATH="\$PATH" ./build.sh -curve=bn254
sudo ldconfig

echo "--- Installed Icicle libraries ---"
ls -1 /usr/local/lib/libicicle_* 2>/dev/null || echo "WARNING: libicicle_* not found in /usr/local/lib"
ls -1 /usr/local/lib/backend/cuda/ 2>/dev/null || echo "WARNING: cuda backend not found in /usr/local/lib/backend"

echo ""
echo "=== GPU toolchain ready ==="
echo "nvcc: \$(nvcc --version 2>&1 | tail -1)"
echo ""
echo "Next (R13-A/B/C):"
echo "  1. Vendor the gnark fork + apply the bb-por Icicle v3 bundle (R13-A)."
echo "  2. Build the GPU prover:  CGO_ENABLED=1 go build -tags icicle ./cmd/prover/"
echo "  3. Run the GPU smoke (R13-C) and compare prove time vs the CPU baseline."
EOF
log "post-reboot done — GPU toolchain installed; proceed with R13-A (fork vendor) + R13-C (GPU smoke)"
