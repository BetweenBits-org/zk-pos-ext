#!/usr/bin/env bash
# R11-D per-cell measurement wrapper. Invokes (on EC2):
#   1. gen-testdata at the cell's asset-count to force tier routing
#   2. scripts/smoke.sh with single-shape ZKPOR_BATCH_SHAPE_OVERRIDE +
#      generated testdata dir + production asset_capacity
#   3. scripts/extract_smoke_metrics.sh --json
#   4. saves labeled .json + raw log to .artifacts/reports/R11D_<cell>/
#
# Cell contract (Tier-shape pairing must match testdata's non-empty count):
#   cell=t1_700   : shape=50_700,  users=700,   asset_count=50  → Tier 1 isolation, 1 batch
#   cell=t2_92    : shape=500_92,  users=92,    asset_count=500 → Tier 2 isolation, 1 batch
#   cell=t1_10k   : shape=50_700,  users=10000, asset_count=50  → Tier 1 multi-batch (≈15 batches)
#   cell=t2_10k   : shape=500_92,  users=10000, asset_count=500 → Tier 2 multi-batch (≈109 batches)
#
# Profile is always T4 production (profile/t4_reference/t4_reference.toml,
# asset_capacity=500). The .pk used must be a production keygen artifact
# at asset_capacity=500 — Setup is a separate phase (cell=setup).
#
# Usage (run from zkpor/ on EC2 — NOT from the local-side ec2 wrapper):
#   scripts/ec2/r11d.sh setup                # initial keygen (cap=500, both shapes)
#   scripts/ec2/r11d.sh t1_700               # cell 1-batch Tier 1
#   scripts/ec2/r11d.sh t2_92                # cell 1-batch Tier 2
#   scripts/ec2/r11d.sh t1_10k               # cell 10K Tier 1
#   scripts/ec2/r11d.sh t2_10k               # cell 10K Tier 2
#
# Env overrides:
#   INSTANCE_TAG   label written into cell report path (e.g., m8a.8xl)
#                  default: $(uname -n) — typically "ip-10-x-x-x"

set -euo pipefail

cd "$(dirname "$0")/../.."

CELL="${1:?usage: $0 <setup|t1_700|t2_92|t1_10k|t2_10k>}"
INSTANCE_TAG="${INSTANCE_TAG:-$(uname -n)}"
PROFILE="profile/t4_reference/t4_reference.toml"
ASSET_CAPACITY=500

REPORT_ROOT=".artifacts/reports/R11D_${INSTANCE_TAG}_${CELL}"
mkdir -p "$REPORT_ROOT"

# Resolve cell parameters.
case "$CELL" in
  setup)
    SHAPE="50_700,500_92"
    USERS=700                  # padded to fill both shapes deterministically
    ASSET_COUNT=0              # default → catalog * cap, irrelevant for setup
    DATA_LABEL="bootstrap"
    ;;
  t1_700)   SHAPE="50_700"  ; USERS=700   ; ASSET_COUNT=50  ; DATA_LABEL="t1_700"  ;;
  t2_92)    SHAPE="500_92"  ; USERS=92    ; ASSET_COUNT=500 ; DATA_LABEL="t2_92"   ;;
  t1_10k)   SHAPE="50_700"  ; USERS=10000 ; ASSET_COUNT=50  ; DATA_LABEL="t1_10k"  ;;
  t2_10k)   SHAPE="500_92"  ; USERS=10000 ; ASSET_COUNT=500 ; DATA_LABEL="t2_10k"  ;;
  *)
    echo "unknown cell '$CELL' — expected setup|t1_700|t2_92|t1_10k|t2_10k" >&2
    exit 1
    ;;
esac

# Cell metadata header for the run log.
RUN_TS="$(date -u +%Y%m%dT%H%M%SZ)"
LOG="$REPORT_ROOT/run_${RUN_TS}.log"
META="$REPORT_ROOT/run_${RUN_TS}.meta.json"
JSON_OUT="$REPORT_ROOT/run_${RUN_TS}.json"

cat > "$META" <<EOF
{
  "cell": "$CELL",
  "instance_tag": "$INSTANCE_TAG",
  "shape_override": "$SHAPE",
  "users": $USERS,
  "asset_count": $ASSET_COUNT,
  "asset_capacity": $ASSET_CAPACITY,
  "profile": "$PROFILE",
  "run_ts_utc": "$RUN_TS",
  "git_commit": "$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
}
EOF

log() { printf "\033[1;34m[r11d:%s]\033[0m %s\n" "$CELL" "$*"; }

log "cell=$CELL shape=$SHAPE users=$USERS asset_count=$ASSET_COUNT cap=$ASSET_CAPACITY"
log "report=$REPORT_ROOT"

# 1. gen-testdata — skip for setup cell (no data needed for keygen).
TESTDATA_DIR=""
if [ "$CELL" != "setup" ]; then
  TESTDATA_DIR=".artifacts/testdata/${DATA_LABEL}"
  if [ ! -f "$TESTDATA_DIR/accounts.csv" ]; then
    log "generating testdata → $TESTDATA_DIR"
    go run ./cmd/gen-testdata \
      -profile "$PROFILE" \
      -users "$USERS" \
      -asset-capacity "$ASSET_CAPACITY" \
      -asset-count "$ASSET_COUNT" \
      -out "$TESTDATA_DIR" \
      -seed 42 2>&1 | tee -a "$LOG"
  else
    log "testdata already present at $TESTDATA_DIR (skipping gen)"
  fi
fi

# 2. Run smoke. For setup, the testdata/happy/ default is enough — keygen
#    only reads the profile to determine circuit dim, not the data.
log "running smoke (output → $LOG)"
if [ -n "$TESTDATA_DIR" ]; then
  ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE" \
  ZKPOR_SMOKE_ASSET_CAPACITY="$ASSET_CAPACITY" \
  ZKPOR_SMOKE_USER_DATA="$TESTDATA_DIR" \
    ./scripts/smoke.sh "$PROFILE" 2>&1 | tee -a "$LOG"
else
  ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE" \
  ZKPOR_SMOKE_ASSET_CAPACITY="$ASSET_CAPACITY" \
    ./scripts/smoke.sh "$PROFILE" 2>&1 | tee -a "$LOG"
fi

# 3. Extract metrics → json.
log "extracting metrics → $JSON_OUT"
./scripts/extract_smoke_metrics.sh "$LOG" --json "$JSON_OUT" 2>&1 | tee -a "$LOG"

log "cell $CELL done. artifacts: $REPORT_ROOT"
log "  meta : $META"
log "  log  : $LOG"
log "  json : $JSON_OUT"
