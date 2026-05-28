#!/usr/bin/env bash
# Stop the remote docker compose stack and remove the volume. The EC2
# instance itself stays up — terminate via AWS console / CLI if desired.
#
# Usage: scripts/ec2/down.sh

source "$(dirname "$0")/_lib.sh"

log "stopping remote docker compose stack on $EC2_HOST"

ec2_ssh bash <<EOF
set -euo pipefail
cd "$EC2_REMOTE_DIR/zkpor"
docker compose -f scripts/deploy/docker-compose.yml down -v
EOF

log "remote stack stopped"
