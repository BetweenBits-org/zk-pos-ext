#!/usr/bin/env bash
# Pull EC2 .artifacts/ (keygen output: .pk/.vk/.r1cs + dumped JSON)
# back to the local repo. Useful for caching production-capacity
# keygen artifacts so subsequent local prover/verifier runs don't
# need to re-ceremony.
#
# Usage: scripts/ec2/fetch.sh

source "$(dirname "$0")/_lib.sh"

LOCAL_ARTIFACTS="$ZKPOR_DIR/.artifacts/"
REMOTE_ARTIFACTS="$EC2_REMOTE_DIR/zkpor/.artifacts/"

log "fetching $EC2_HOST:$REMOTE_ARTIFACTS → $LOCAL_ARTIFACTS"
log "(no --delete — local kept on conflict; remove .artifacts/ first to force refresh)"

mkdir -p "$LOCAL_ARTIFACTS"

rsync -avh --progress \
  -e "$(ec2_rsync_e)" \
  "$EC2_HOST:$REMOTE_ARTIFACTS" \
  "$LOCAL_ARTIFACTS"

log "fetch complete"
