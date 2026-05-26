#!/usr/bin/env bash
# Install Docker + Go + git on the EC2 host. Idempotent — re-running is
# safe. Designed for Ubuntu 22.04 LTS (the default Canonical AMI on
# m6i instances); for Amazon Linux 2023 swap apt-get for dnf.
#
# Usage: scripts/ec2/bootstrap.sh

source "$(dirname "$0")/_lib.sh"

GO_VERSION="1.22.7"  # match zkpor/go.mod toolchain pin closely

log "bootstrapping $EC2_HOST (Docker + Go $GO_VERSION + git)"

ec2_ssh bash -s <<EOF
set -euo pipefail

# Detect package manager
if command -v apt-get >/dev/null 2>&1; then
  PKG_INSTALL="sudo apt-get install -y"
  sudo apt-get update -qq
elif command -v dnf >/dev/null 2>&1; then
  PKG_INSTALL="sudo dnf install -y"
else
  echo "unknown package manager (need apt-get or dnf)" >&2
  exit 1
fi

# git
if ! command -v git >/dev/null 2>&1; then
  \$PKG_INSTALL git
fi

# Docker
if ! command -v docker >/dev/null 2>&1; then
  echo "installing docker"
  \$PKG_INSTALL docker.io || \$PKG_INSTALL docker
  sudo systemctl enable --now docker
  # add ubuntu/ec2-user to docker group so we don't need sudo
  sudo usermod -aG docker "\$(whoami)"
  echo "NOTE: log out + back in (or 'newgrp docker') for the group change to apply"
fi

# Docker Compose plugin (v2). Apt's docker.io may or may not ship it.
if ! docker compose version >/dev/null 2>&1; then
  echo "installing docker compose plugin"
  if command -v apt-get >/dev/null 2>&1; then
    \$PKG_INSTALL docker-compose-plugin || true
  fi
  # Fallback: standalone binary
  if ! docker compose version >/dev/null 2>&1; then
    sudo mkdir -p /usr/local/lib/docker/cli-plugins
    sudo curl -sSL \
      "https://github.com/docker/compose/releases/latest/download/docker-compose-\$(uname -s)-\$(uname -m)" \
      -o /usr/local/lib/docker/cli-plugins/docker-compose
    sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
  fi
fi

# Go
GO_INSTALLED_VERSION=""
if command -v go >/dev/null 2>&1; then
  GO_INSTALLED_VERSION="\$(go version | awk '{print \$3}' | sed 's/^go//')"
fi
if [ "\$GO_INSTALLED_VERSION" != "$GO_VERSION" ]; then
  echo "installing go $GO_VERSION (current: \$GO_INSTALLED_VERSION)"
  ARCH="\$(uname -m)"
  case "\$ARCH" in
    x86_64) GO_ARCH=amd64 ;;
    aarch64) GO_ARCH=arm64 ;;
    *) echo "unsupported arch \$ARCH" >&2; exit 1 ;;
  esac
  cd /tmp
  curl -sSL "https://go.dev/dl/go${GO_VERSION}.linux-\${GO_ARCH}.tar.gz" -o go.tgz
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf go.tgz
  rm go.tgz
  if ! grep -q "/usr/local/go/bin" "\$HOME/.profile" 2>/dev/null; then
    echo 'export PATH=\$PATH:/usr/local/go/bin' >> "\$HOME/.profile"
  fi
fi

echo "bootstrap done."
echo "  docker:  \$(docker --version 2>/dev/null || echo MISSING)"
echo "  compose: \$(docker compose version 2>/dev/null || echo MISSING)"
echo "  go:      \$(/usr/local/go/bin/go version 2>/dev/null || echo MISSING)"
EOF

log "bootstrap complete — if docker group was added, re-ssh (or run 'newgrp docker') before next step"
