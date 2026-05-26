#!/usr/bin/env bash
# zkpor end-to-end smoke — R3 step 4 exit-criteria gate.
#
# Runs the full pipeline against the legacy sample data:
#   keygen → witness → prover → verifier(batch) → userproof → verifier(-user)
# at AssetCapacity=5, BatchShape=5_10. ~21 seconds keygen (one-time, cached
# in .artifacts/), then the prove/verify chain is <1 min on a laptop.
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

ARTIFACTS=".artifacts"
KEYGEN_STEM="$ARTIFACTS/zkpor.tier_3bucket.5_10"
ASSET_CAPACITY=5
SHAPE_OVERRIDE="5_10"
SAMPLE_DATA="$(pwd)/../src/sampledata"

DSN="zkpor:zkpor@123@tcp(127.0.0.1:3306)/zkpor?parseTime=true"

STAGE_PREFIX="\033[1;34m[smoke]\033[0m"
log() { printf "%b %s\n" "$STAGE_PREFIX" "$*"; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing required command: $1" >&2; exit 1; }
}
require_cmd docker
require_cmd go

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

# 2. Keygen — skip if artifacts already exist.
ensure_keys() {
  if [ -f "$KEYGEN_STEM.pk" ] && [ -f "$KEYGEN_STEM.vk" ] && [ -f "$KEYGEN_STEM.r1cs" ]; then
    log "keygen artifacts already present at $KEYGEN_STEM.*"
    return 0
  fi
  log "running keygen (asset-capacity=$ASSET_CAPACITY, shape=$SHAPE_OVERRIDE)"
  ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE_OVERRIDE" \
    go run ./cmd/keygen -asset-capacity "$ASSET_CAPACITY" -out "$ARTIFACTS/"
}

# 3-7. Run each service from its own cwd so its hard-coded
# `config/config.json` path resolves correctly. Each service writes a
# config.json fresh at the start of the smoke; if you want to inspect
# the configs after a run, look at cmd/<svc>/config/config.json.
write_service_configs() {
  local artifacts_abs
  artifacts_abs="$(cd "$ARTIFACTS" && pwd)"

  cat > cmd/witness/config/config.json <<EOF
{
  "MysqlDataSource": "$DSN",
  "UserDataFile": "$SAMPLE_DATA",
  "DbSuffix": "",
  "AssetCapacity": $ASSET_CAPACITY,
  "TreeDB": { "Driver": "memory", "Option": { "Addr": "" } }
}
EOF

  cat > cmd/prover/config/config.json <<EOF
{
  "MysqlDataSource": "$DSN",
  "DbSuffix": "",
  "ZkKeyName": ["$artifacts_abs/zkpor.tier_3bucket.5_10"],
  "AssetsCountTiers": [$ASSET_CAPACITY]
}
EOF

  cat > cmd/userproof/config/config.json <<EOF
{
  "MysqlDataSource": "$DSN",
  "UserDataFile": "$SAMPLE_DATA",
  "DbSuffix": "",
  "AssetCapacity": $ASSET_CAPACITY,
  "TreeDB": { "Driver": "memory", "Option": { "Addr": "" } }
}
EOF
}

run_witness() {
  log "running witness (snapshot → BatchWitness rows)"
  ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE_OVERRIDE" \
    bash -c 'cd cmd/witness && go run . -dump-final-cex ../../.artifacts/final_cex_assets.json'
}

run_prover() {
  log "running prover (DB-poll → groth16.Prove+Verify → Proof rows)"
  ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE_OVERRIDE" \
    bash -c 'cd cmd/prover && go run .'
}

write_verifier_config() {
  local artifacts_abs
  artifacts_abs="$(cd "$ARTIFACTS" && pwd)"
  local cex_json
  cex_json="$(cat "$ARTIFACTS/final_cex_assets.json")"
  cat > cmd/verifier/config/config.json <<EOF
{
  "MysqlDataSource": "$DSN",
  "DbSuffix": "",
  "ZkKeyName": ["$artifacts_abs/zkpor.tier_3bucket.5_10"],
  "AssetsCountTiers": [$ASSET_CAPACITY],
  "AssetCapacity": $ASSET_CAPACITY,
  "CexAssetsInfo": $cex_json
}
EOF
}

run_verifier_batch() {
  log "running verifier (batch — DB direct read)"
  ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE_OVERRIDE" \
    bash -c 'cd cmd/verifier && go run .'
}

run_userproof() {
  log "running userproof (per-account Merkle proofs → UserProof rows + dump user_config[0])"
  ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE_OVERRIDE" \
    bash -c 'cd cmd/userproof && go run . -dump-user-index 0 -dump-user-path ../verifier/config/user_config.json'
}

run_verifier_user() {
  log "running verifier -user (single account inclusion)"
  ZKPOR_BATCH_SHAPE_OVERRIDE="$SHAPE_OVERRIDE" \
    bash -c 'cd cmd/verifier && go run . -user'
}

main() {
  ensure_mysql
  ensure_keys
  write_service_configs
  run_witness
  run_prover
  write_verifier_config
  run_verifier_batch
  run_userproof
  run_verifier_user
  log "smoke passed"
}

main "$@"
