#!/usr/bin/env bash
# zkpor end-to-end smoke — runs the full pipeline against the
# selected profile's testdata/happy/ canonical CSV:
#   keygen → witness → prover → verifier(batch) → userproof → verifier(-user)
# at AssetCapacity=5, BatchShape=5_10. ~21 seconds keygen (one-time, cached
# in .artifacts/), then the prove/verify chain is <1 min on a laptop.
#
# Phase 4 (R10+1): the script is profile-driven. The first positional
# argument selects the profile.toml; the model is derived from the
# toml's [profile].model field. Each profile's own testdata/happy/
# supplies the canonical accounts.csv / cex_assets.csv / tier_ratios.csv
# the standard CSV connector consumes.
#
# Usage:
#   scripts/smoke.sh                              # defaults to t4_reference
#   scripts/smoke.sh profile/t1_reference/t1_reference.toml   # T1
#   scripts/smoke.sh profile/t2_reference/t2_reference.toml   # T2
#   scripts/smoke.sh profile/t3_reference/t3_reference.toml   # T3
#   scripts/smoke.sh profile/t4_reference/t4_reference.toml   # T4
#
# Env overrides (defaults are tiny smoke):
#   ZKPOR_BATCH_SHAPE_OVERRIDE   default 5_10
#   ZKPOR_SMOKE_ASSET_CAPACITY   default 5
#
# Prerequisites:
#   - Docker (for the MySQL fixture)
#   - Go ≥ the toolchain pin in zkpor/go.mod
#   - This script is meant to run from the zkpor/ directory.
#
# Exit codes are propagated: any failing stage aborts the run and the
# script returns non-zero.

set -euo pipefail

cd "$(dirname "$0")/.."

PROFILE_PATH="${1:-profile/t4_reference/t4_reference.toml}"
if [ ! -f "$PROFILE_PATH" ]; then
  echo "profile not found: $PROFILE_PATH" >&2
  exit 1
fi

# Derive the testdata dir + model from the profile.
PROFILE_DIR="$(cd "$(dirname "$PROFILE_PATH")" && pwd)"
USER_DATA_DIR="$PROFILE_DIR/testdata/happy"
if [ ! -d "$USER_DATA_DIR" ]; then
  echo "user-data-dir not found: $USER_DATA_DIR" >&2
  exit 1
fi

# Extract the solvency model from [profile].model. The toml schema
# guarantees a single quoted string on that line.
MODEL="$(awk -F'"' '/^[[:space:]]*model[[:space:]]*=/{print $2; exit}' "$PROFILE_PATH")"
if [ -z "$MODEL" ]; then
  echo "could not parse [profile].model from $PROFILE_PATH" >&2
  exit 1
fi

ARTIFACTS=".artifacts"
SHAPE_OVERRIDE="${ZKPOR_BATCH_SHAPE_OVERRIDE:-5_10}"
ASSET_CAPACITY="${ZKPOR_SMOKE_ASSET_CAPACITY:-5}"

# Compose the artifact stems from MODEL + SHAPE_OVERRIDE. The keygen
# CLI writes one .pk/.vk/.r1cs triplet per shape under .artifacts/.
shape_stems() {
  echo "$SHAPE_OVERRIDE" | tr ',' '\n' | awk -F_ -v model="$MODEL" '{print "zkpor." model "." $1 "_" $2}'
}

DSN="zkpor:zkpor@123@tcp(127.0.0.1:3306)/zkpor?parseTime=true"

STAGE_PREFIX="\033[1;34m[smoke]\033[0m"
log() { printf "%b %s\n" "$STAGE_PREFIX" "$*"; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing required command: $1" >&2; exit 1; }
}
require_cmd docker
require_cmd go

log "profile=$PROFILE_PATH model=$MODEL data=$USER_DATA_DIR shape=$SHAPE_OVERRIDE cap=$ASSET_CAPACITY"

# 1. MySQL fixture — start if not already up, wait for healthcheck.
ensure_mysql() {
  if docker ps --filter "name=zkpor-smoke-mysql" --format '{{.Names}}' | grep -q zkpor-smoke-mysql; then
    log "mysql container already running"
  else
    log "starting mysql container"
    docker compose -f deploy/docker-compose.yml up -d
  fi
  log "waiting for mysql healthcheck"
  for _ in $(seq 1 40); do
    status=$(docker inspect -f '{{.State.Health.Status}}' zkpor-smoke-mysql 2>/dev/null || echo "starting")
    if [ "$status" = "healthy" ]; then
      log "mysql healthy"
      return 0
    fi
    sleep 1
  done
  echo "mysql did not become healthy within 40s" >&2
  return 1
}

# 2. Keygen — skip if every shape's artifact triplet is already present.
# ARTIFACTS dir is created either way so downstream stages can write
# their dumps (final_cex_assets.json, user_config.json) without
# tripping over a missing parent dir.
ensure_keys() {
  mkdir -p "$ARTIFACTS"
  local missing=0
  while read -r stem; do
    if [ ! -f "$ARTIFACTS/$stem.pk" ] || [ ! -f "$ARTIFACTS/$stem.vk" ] || [ ! -f "$ARTIFACTS/$stem.r1cs" ]; then
      missing=1
    fi
  done < <(shape_stems)
  if [ "$missing" = 0 ]; then
    log "keygen artifacts already present for shape(s): $SHAPE_OVERRIDE"
    return 0
  fi
  log "running keygen (model=$MODEL, asset-capacity=$ASSET_CAPACITY, shape=$SHAPE_OVERRIDE)"
  ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE_OVERRIDE" \
    go run ./cmd/keygen \
      -profile "$PROFILE_PATH" \
      -asset-capacity "$ASSET_CAPACITY" \
      -out "$ARTIFACTS/"
}

# 3-7. Run each service from its own cwd so its hard-coded
# `config/config.json` path resolves correctly. Each service writes a
# config.json fresh at the start of the smoke; if you want to inspect
# the configs after a run, look at cmd/<svc>/config/config.json.
write_service_configs() {
  cat > cmd/witness/config/config.json <<EOF
{
  "MysqlDataSource": "$DSN",
  "DbSuffix": "",
  "TreeDB": { "Driver": "memory", "Option": { "Addr": "" } }
}
EOF

  cat > cmd/prover/config/config.json <<EOF
{
  "MysqlDataSource": "$DSN",
  "DbSuffix": ""
}
EOF

  cat > cmd/userproof/config/config.json <<EOF
{
  "MysqlDataSource": "$DSN",
  "DbSuffix": "",
  "TreeDB": { "Driver": "memory", "Option": { "Addr": "" } }
}
EOF
}

# Each service runs from its own directory; -profile is passed as an
# absolute path so the relative cwd of the service does not matter.
PROFILE_ABS="$(cd "$(dirname "$PROFILE_PATH")" && pwd)/$(basename "$PROFILE_PATH")"

run_witness() {
  log "running witness (snapshot → BatchWitness rows)"
  ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE_OVERRIDE" \
  ZKPOR_SMOKE_USER_DATA="$USER_DATA_DIR" \
  ZKPOR_SMOKE_ASSET_CAPACITY="$ASSET_CAPACITY" \
  ZKPOR_SMOKE_PROFILE="$PROFILE_ABS" \
    bash -c '
      cd cmd/witness && go run . \
        -profile "$ZKPOR_SMOKE_PROFILE" \
        -user-data-dir "$ZKPOR_SMOKE_USER_DATA" \
        -asset-capacity "$ZKPOR_SMOKE_ASSET_CAPACITY" \
        -dump-final-cex ../../.artifacts/final_cex_assets.json
    '
}

run_prover() {
  log "running prover (DB-poll → groth16.Prove+Verify → Proof rows)"
  local artifacts_abs
  artifacts_abs="$(cd "$ARTIFACTS" && pwd)"
  ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE_OVERRIDE" \
  ZKPOR_SMOKE_KEYS_DIR="$artifacts_abs" \
  ZKPOR_SMOKE_PROFILE="$PROFILE_ABS" \
    bash -c '
      cd cmd/prover && go run . \
        -profile "$ZKPOR_SMOKE_PROFILE" \
        -keys-dir "$ZKPOR_SMOKE_KEYS_DIR"
    '
}

write_verifier_config() {
  # verifier derives tiers + .vk stems + capacity from profile.toml +
  # -keys-dir. config.json keeps DB + per-snapshot CexAssetsInfo (raw
  # JSON the model's runner decodes).
  local cex_json
  cex_json="$(cat "$ARTIFACTS/final_cex_assets.json")"
  cat > cmd/verifier/config/config.json <<EOF
{
  "MysqlDataSource": "$DSN",
  "DbSuffix": "",
  "CexAssetsInfo": $cex_json
}
EOF
}

run_verifier_batch() {
  log "running verifier (batch — DB direct read)"
  local artifacts_abs
  artifacts_abs="$(cd "$ARTIFACTS" && pwd)"
  ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE_OVERRIDE" \
  ZKPOR_SMOKE_KEYS_DIR="$artifacts_abs" \
  ZKPOR_SMOKE_ASSET_CAPACITY="$ASSET_CAPACITY" \
  ZKPOR_SMOKE_PROFILE="$PROFILE_ABS" \
    bash -c '
      cd cmd/verifier && go run . \
        -profile "$ZKPOR_SMOKE_PROFILE" \
        -keys-dir "$ZKPOR_SMOKE_KEYS_DIR" \
        -asset-capacity "$ZKPOR_SMOKE_ASSET_CAPACITY"
    '
}

run_userproof() {
  log "running userproof (per-account Merkle proofs → UserProof rows + dump user_config[0])"
  ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE_OVERRIDE" \
  ZKPOR_SMOKE_USER_DATA="$USER_DATA_DIR" \
  ZKPOR_SMOKE_ASSET_CAPACITY="$ASSET_CAPACITY" \
  ZKPOR_SMOKE_PROFILE="$PROFILE_ABS" \
    bash -c '
      cd cmd/userproof && go run . \
        -profile "$ZKPOR_SMOKE_PROFILE" \
        -user-data-dir "$ZKPOR_SMOKE_USER_DATA" \
        -asset-capacity "$ZKPOR_SMOKE_ASSET_CAPACITY" \
        -dump-user-index 0 \
        -dump-user-path ../verifier/config/user_config.json
    '
}

run_verifier_user() {
  log "running verifier -user (single account inclusion)"
  ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE_OVERRIDE" \
  ZKPOR_SMOKE_ASSET_CAPACITY="$ASSET_CAPACITY" \
  ZKPOR_SMOKE_PROFILE="$PROFILE_ABS" \
    bash -c '
      cd cmd/verifier && go run . \
        -profile "$ZKPOR_SMOKE_PROFILE" \
        -asset-capacity "$ZKPOR_SMOKE_ASSET_CAPACITY" \
        -user
    '
}

# Clear DB rows between smoke runs against different profiles so the
# verifier doesn't see leftover proofs from a previous model. Idempotent:
# tables may not exist on a fresh run.
clear_db_state() {
  log "clearing prior smoke DB state (idempotent)"
  docker exec zkpor-smoke-mysql mysql -uzkpor -pzkpor@123 -e "
    DROP TABLE IF EXISTS zkpor.proof;
    DROP TABLE IF EXISTS zkpor.batch_witness;
    DROP TABLE IF EXISTS zkpor.user_proof;
  " >/dev/null 2>&1 || true
}

main() {
  ensure_mysql
  clear_db_state
  ensure_keys
  write_service_configs
  run_witness
  run_prover
  write_verifier_config
  run_verifier_batch
  run_userproof
  run_verifier_user
  log "smoke passed for $MODEL"
}

main "$@"
