#!/usr/bin/env bash
# Switch the EC2 dev instance to a different type. Used in R11-D's
# instance ablation sequence (m8a.8xl → m8a.4xl → m7a.4xl) to reuse the
# same EBS (and any cached `.pk` artifacts) across cells.
#
# Workflow:
#   1. aws ec2 stop-instances + wait for stopped
#   2. modify-instance-attribute --instance-type <new>
#   3. start-instances + wait for running
#   4. describe-instances → new public IP
#   5. rewrite scripts/ec2/.env EC2_HOST=ec2-user@<new-ip>
#   6. ssh-keygen -R <old-ip> (prevent known_hosts conflict)
#
# Usage:
#   scripts/ec2/switch_type.sh m8a.4xlarge
#   scripts/ec2/switch_type.sh m7a.4xlarge
#
# Env / .env requirements:
#   AWS_REGION       — default us-east-1, override via env
#   EC2_INSTANCE_ID  — required, read from .env (added during launch)
#   EC2_HOST         — current ec2-user@<ip>, rewritten by this script
#   EC2_SSH_USER     — default ec2-user

source "$(dirname "$0")/_lib.sh"

NEW_TYPE="${1:?usage: $0 <new-instance-type> (e.g., m8a.4xlarge)}"
REGION="${AWS_REGION:-us-east-1}"
SSH_USER="${EC2_SSH_USER:-ec2-user}"

: "${EC2_INSTANCE_ID:?EC2_INSTANCE_ID not set in .env — add the instance ID line}"

require_aws() {
  command -v aws >/dev/null 2>&1 || {
    echo "missing aws CLI — install + configure credentials" >&2; exit 1;
  }
}
require_aws

log "switching $EC2_INSTANCE_ID → $NEW_TYPE (region=$REGION)"

OLD_HOST="$EC2_HOST"
OLD_IP="${OLD_HOST#*@}"

# Capture pre-switch state for the report.
CURRENT_TYPE="$(aws ec2 describe-instances --region "$REGION" \
  --instance-ids "$EC2_INSTANCE_ID" \
  --query 'Reservations[0].Instances[0].InstanceType' --output text)"
log "current type: $CURRENT_TYPE"

if [ "$CURRENT_TYPE" = "$NEW_TYPE" ]; then
  log "already $NEW_TYPE — nothing to do"
  exit 0
fi

# 1. stop
log "stopping $EC2_INSTANCE_ID"
aws ec2 stop-instances --region "$REGION" --instance-ids "$EC2_INSTANCE_ID" \
  --query 'StoppingInstances[0].CurrentState.Name' --output text
aws ec2 wait instance-stopped --region "$REGION" --instance-ids "$EC2_INSTANCE_ID"
log "stopped"

# 2. modify type
log "modifying instance-type → $NEW_TYPE"
aws ec2 modify-instance-attribute --region "$REGION" \
  --instance-id "$EC2_INSTANCE_ID" \
  --instance-type "{\"Value\": \"$NEW_TYPE\"}"

# 3. start
log "starting"
aws ec2 start-instances --region "$REGION" --instance-ids "$EC2_INSTANCE_ID" \
  --query 'StartingInstances[0].CurrentState.Name' --output text
aws ec2 wait instance-running --region "$REGION" --instance-ids "$EC2_INSTANCE_ID"
log "running"

# 4. fetch new public IP
NEW_IP="$(aws ec2 describe-instances --region "$REGION" \
  --instance-ids "$EC2_INSTANCE_ID" \
  --query 'Reservations[0].Instances[0].PublicIpAddress' --output text)"
if [ -z "$NEW_IP" ] || [ "$NEW_IP" = "None" ]; then
  echo "could not read PublicIpAddress for $EC2_INSTANCE_ID" >&2
  exit 1
fi
NEW_HOST="${SSH_USER}@${NEW_IP}"
log "new public IP: $NEW_IP → EC2_HOST=$NEW_HOST"

# 5. rewrite .env (preserve other lines, swap EC2_HOST=)
ENV_FILE="$ZKPOR_DIR/scripts/ec2/.env"
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT
awk -v new="$NEW_HOST" '
  BEGIN { replaced = 0 }
  /^EC2_HOST=/ { print "EC2_HOST=" new; replaced = 1; next }
  { print }
  END { if (!replaced) print "EC2_HOST=" new }
' "$ENV_FILE" > "$TMP"
mv "$TMP" "$ENV_FILE"
log "rewrote $ENV_FILE EC2_HOST=$NEW_HOST"

# 6. drop stale known_hosts entry for old IP (best-effort)
if [ -n "$OLD_IP" ] && [ -f "$HOME/.ssh/known_hosts" ]; then
  ssh-keygen -R "$OLD_IP" 2>/dev/null || true
  log "removed $OLD_IP from known_hosts"
fi

log "switch complete: $CURRENT_TYPE → $NEW_TYPE (host=$NEW_HOST)"
log "next: scripts/ec2/sync.sh && scripts/ec2/r11d.sh <cell>"
