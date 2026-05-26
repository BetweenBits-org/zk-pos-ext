# Shared helpers for scripts/ec2/*.sh. Source via:
#   source "$(dirname "$0")/_lib.sh"
# (not intended to run directly).

set -euo pipefail

# Resolve the parent repo root (zkmerkle-proof-of-solvency/) from this file's
# location: scripts/ec2/_lib.sh is two levels down inside zkpor/.
ZKPOR_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PARENT_REPO_DIR="$(cd "$ZKPOR_DIR/.." && pwd)"

ENV_FILE="$ZKPOR_DIR/scripts/ec2/.env"
if [ ! -f "$ENV_FILE" ]; then
  echo "missing $ENV_FILE — copy from .env.example and fill in your values" >&2
  exit 1
fi
# shellcheck disable=SC1090
source "$ENV_FILE"

: "${EC2_HOST:?EC2_HOST not set in .env}"
: "${EC2_REMOTE_DIR:?EC2_REMOTE_DIR not set in .env}"

# SSH key opt — empty if ssh-agent has the key, else -i <path>.
SSH_KEY_OPT=()
if [ -n "${EC2_KEY:-}" ]; then
  # Expand ~ since .env supplies a string.
  SSH_KEY_PATH="${EC2_KEY/#\~/$HOME}"
  if [ ! -f "$SSH_KEY_PATH" ]; then
    echo "EC2_KEY=$SSH_KEY_PATH not found" >&2
    exit 1
  fi
  SSH_KEY_OPT=(-i "$SSH_KEY_PATH")
fi

# ssh / rsync wrappers — the SSH_KEY_OPT splice is handled by callers so
# rsync can pass it through -e.
ec2_ssh() {
  ssh "${SSH_KEY_OPT[@]}" -o ServerAliveInterval=30 "$EC2_HOST" "$@"
}

ec2_rsync_e() {
  # Build the -e flag rsync passes to ssh. Quoting matters here.
  if [ -n "${EC2_KEY:-}" ]; then
    echo "ssh -i $SSH_KEY_PATH -o ServerAliveInterval=30"
  else
    echo "ssh -o ServerAliveInterval=30"
  fi
}

log() {
  printf "\033[1;34m[ec2]\033[0m %s\n" "$*"
}
