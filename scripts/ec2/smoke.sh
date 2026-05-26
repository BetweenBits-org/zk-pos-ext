#!/usr/bin/env bash
# Run the smoke harness on EC2. Streams output to the local terminal.
# Argument:
#   tiny         — same as local smoke (capacity=5, shape=5_10)
#   (default)    — production capacity (500) + production shape 50_700
#                  via the binance.NewBatchShape default (no env var)
#
# Usage:
#   scripts/ec2/smoke.sh             # production
#   scripts/ec2/smoke.sh tiny        # tiny

source "$(dirname "$0")/_lib.sh"

MODE="${1:-prod}"

case "$MODE" in
  tiny)
    SHAPE='5_10'
    CAPACITY=5
    log "MODE=tiny (capacity=5, shape=5_10)"
    ;;
  prod)
    SHAPE='50_700,500_92'
    CAPACITY=500
    log "MODE=prod (capacity=500, shapes 50_700 + 500_92)"
    log "NOTE: production keygen is multi-shape + multi-GB. expect ~30min-1h on m6i.4xlarge."
    ;;
  *)
    echo "unknown mode '$MODE' — expected 'tiny' or 'prod'" >&2
    exit 1
    ;;
esac

log "running ./scripts/smoke.sh on $EC2_HOST"

ec2_ssh bash <<EOF
set -euo pipefail
cd "$EC2_REMOTE_DIR/zkpor"
export PATH=\$PATH:/usr/local/go/bin
export ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE"
export ZKPOR_SMOKE_ASSET_CAPACITY="$CAPACITY"
./scripts/smoke.sh
EOF

log "remote smoke complete"
