#!/usr/bin/env bash
# Run the smoke harness on EC2. Streams output to the local terminal.
#
# Modes (positional $1):
#   tiny           — capacity=5, shape=5_10. T4 reference profile only.
#   prod           — capacity=500, shape=50_700,500_92. T4 production.
#   mid            — mid-tier, capacity + shape from env or defaults
#                    (CAPACITY=50, SHAPE=20_500). 4 model loop unless
#                    PROFILE is set.
#   custom         — fully env-driven: PROFILE + CAPACITY + SHAPE.
#
# Env overrides (for `mid` and `custom`):
#   PROFILE       — path to profile.toml (e.g. profile/t1_reference/t1_reference.toml).
#                   In `mid` mode, omitting PROFILE runs all 4 reference
#                   profiles in sequence (T1 → T2 → T3 → T4).
#   CAPACITY      — asset capacity override (default 50 for mid).
#   SHAPE         — batch shape override (default 20_500 for mid).
#
# Usage:
#   scripts/ec2/smoke.sh                            # defaults to prod (legacy)
#   scripts/ec2/smoke.sh tiny                       # tiny T4
#   scripts/ec2/smoke.sh prod                       # production T4
#   scripts/ec2/smoke.sh mid                        # 4 model x (cap=50, shape=20_500)
#   PROFILE=profile/t1_reference/t1_reference.toml scripts/ec2/smoke.sh mid
#   PROFILE=profile/t3_reference/t3_reference.toml CAPACITY=30 SHAPE=10_200 \
#       scripts/ec2/smoke.sh custom

source "$(dirname "$0")/_lib.sh"

MODE="${1:-prod}"

# Default profile set for `mid` 4 model loop.
ALL_PROFILES=(
  "profile/t1_reference/t1_reference.toml"
  "profile/t2_reference/t2_reference.toml"
  "profile/t3_reference/t3_reference.toml"
  "profile/t4_reference/t4_reference.toml"
)

case "$MODE" in
  tiny)
    PROFILES=("profile/t4_reference/t4_reference.toml")
    SHAPE='5_10'
    CAPACITY=5
    log "MODE=tiny (profile=t4_reference, capacity=5, shape=5_10)"
    ;;
  prod)
    PROFILES=("profile/t4_reference/t4_reference.toml")
    SHAPE='50_700,500_92'
    CAPACITY=500
    log "MODE=prod (profile=t4_reference, capacity=500, shapes 50_700 + 500_92)"
    log "NOTE: production keygen is multi-shape + multi-GB. expect ~30min-1h on r7a.4xlarge."
    ;;
  mid)
    SHAPE="${SHAPE:-20_500}"
    CAPACITY="${CAPACITY:-50}"
    if [ -n "${PROFILE:-}" ]; then
      PROFILES=("$PROFILE")
      log "MODE=mid (single profile=$PROFILE, capacity=$CAPACITY, shape=$SHAPE)"
    else
      PROFILES=("${ALL_PROFILES[@]}")
      log "MODE=mid 4-model loop (capacity=$CAPACITY, shape=$SHAPE)"
      log "  profiles: ${ALL_PROFILES[*]}"
    fi
    log "NOTE: mid-tier 4 model expects ~330-470M constraints total,"
    log "      keygen ~50-80min + pipeline ~1.5h on r7a.4xlarge."
    ;;
  custom)
    : "${PROFILE:?PROFILE env var required for custom mode}"
    : "${CAPACITY:?CAPACITY env var required for custom mode}"
    : "${SHAPE:?SHAPE env var required for custom mode}"
    PROFILES=("$PROFILE")
    log "MODE=custom (profile=$PROFILE, capacity=$CAPACITY, shape=$SHAPE)"
    ;;
  *)
    echo "unknown mode '$MODE' — expected 'tiny' | 'prod' | 'mid' | 'custom'" >&2
    exit 1
    ;;
esac

log "running ./scripts/smoke.sh on $EC2_HOST for ${#PROFILES[@]} profile(s)"

for profile_path in "${PROFILES[@]}"; do
  log "=== smoke: $profile_path ==="
  # shellcheck disable=SC2087
  ec2_ssh bash <<EOF
set -euo pipefail
cd "$EC2_REMOTE_DIR/zkpor"
export PATH=\$PATH:/usr/local/go/bin
export ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE"
export ZKPOR_SMOKE_ASSET_CAPACITY="$CAPACITY"
./scripts/smoke.sh "$profile_path"
EOF
done

log "remote smoke complete"
