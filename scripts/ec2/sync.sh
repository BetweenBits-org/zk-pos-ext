#!/usr/bin/env bash
# Push local zkmerkle-proof-of-solvency/ (parent repo, including zkpor/
# AND src/sampledata) to EC2. --delete keeps the remote in sync but
# excludes are tuned so artifacts and local state don't get clobbered.
#
# Usage: scripts/ec2/sync.sh

source "$(dirname "$0")/_lib.sh"

log "syncing $PARENT_REPO_DIR/ → $EC2_HOST:$EC2_REMOTE_DIR/"
log "excludes: .git, .artifacts, *.pk/.vk/.r1cs, Docker volumes, IDE"

rsync -avh --progress \
  --delete \
  --exclude='.git/' \
  --exclude='zkpor/.git/' \
  --exclude='zkpor/.artifacts/' \
  --exclude='*.pk' \
  --exclude='*.vk' \
  --exclude='*.r1cs' \
  --exclude='*.proof' \
  --exclude='node_modules/' \
  --exclude='.idea/' \
  --exclude='.vscode/' \
  --exclude='.DS_Store' \
  --exclude='zkpor/cmd/*/config/config.json' \
  --exclude='zkpor/cmd/verifier/config/user_config.json' \
  --exclude='zkpor/scripts/ec2/.env' \
  -e "$(ec2_rsync_e)" \
  "$PARENT_REPO_DIR/" \
  "$EC2_HOST:$EC2_REMOTE_DIR/"

log "sync complete"
