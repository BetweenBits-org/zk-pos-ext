#!/usr/bin/env bash
# Extract smoke metrics from a smoke.sh / ec2/smoke.sh log file and emit
# a markdown report by filling in docs/reports/SMOKE_TEMPLATE.md.
#
# Usage:
#   scripts/extract_smoke_metrics.sh <log-file> [<output.md>]
#
# Default output: .artifacts/reports/smoke_<timestamp>.md.
#
# The log is expected to contain ec2/smoke.sh wrapper lines marking
# per-profile sections:
#   [ec2] === smoke: profile/<name>/<name>.toml ===
# inside which the local scripts/smoke.sh emits keygen, witness, prover,
# verifier, userproof, verifier-user output.
#
# Resilient to missing fields: any value that can't be extracted stays
# as the `{{PLACEHOLDER}}` from the template so the gap is visible.

set -euo pipefail

LOG="${1:?usage: $0 <log-file> [<output.md>]}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="${2:-.artifacts/reports/smoke_${TS}.md}"

if [ ! -f "$LOG" ]; then
  echo "log file not found: $LOG" >&2
  exit 1
fi

TEMPLATE="docs/reports/SMOKE_TEMPLATE.md"
if [ ! -f "$TEMPLATE" ]; then
  echo "template not found: $TEMPLATE" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUT")"
cp "$TEMPLATE" "$OUT"

# ANSI escape stripper — wrappers emit color codes for [smoke]/[ec2]
# prefixes that interfere with awk pattern matching when piped through
# tee. Strip once into a temp file we then probe.
STRIPPED="$(mktemp)"
trap 'rm -f "$STRIPPED"' EXIT
sed -E 's/\x1B\[[0-9;]*[mK]//g' "$LOG" > "$STRIPPED"

# Substitute helper — `:` is a delimiter that won't appear in values we
# extract (paths use /, durations use space + unit). Falls back to a
# literal `{{X}}` when value is empty so missing fields are obvious.
sub() {
  local key="$1" val="$2"
  if [ -z "$val" ]; then
    return
  fi
  # macOS sed doesn't support \n in replacement; we don't need it. Use
  # a uncommon delimiter (|) so paths and durations don't clash.
  python3 -c "
import sys, re
key = sys.argv[1]
val = sys.argv[2]
path = sys.argv[3]
with open(path) as f:
    s = f.read()
s = s.replace('{{' + key + '}}', val)
with open(path, 'w') as f:
    f.write(s)
" "$key" "$val" "$OUT"
}

# --- meta ---
sub "RUN_ID" "smoke_${TS}"
sub "TIMESTAMP_UTC" "$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
sub "HOST" "$(awk -F'@' '/running .* on / && /smoke\.sh/ {print $2; exit}' "$STRIPPED" | awk '{print $1}')"
sub "ZKPOR_COMMIT" "$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
sub "OPERATOR" "${USER:-unknown}"

# Parameters — from the wrapper banner line.
SHAPE="$(awk -F'shape=' '/MODE=(mid|custom)/ {print $2; exit}' "$STRIPPED" | awk -F')' '{print $1}')"
CAPACITY="$(awk -F'capacity=' '/MODE=(mid|custom)/ {print $2; exit}' "$STRIPPED" | awk -F',' '{print $1}')"
sub "SHAPE" "${SHAPE:-unknown}"
sub "CAPACITY" "${CAPACITY:-unknown}"

# Per-model extractor. The wrapper marks sections with
#   [ec2] === smoke: profile/<NAME>/<NAME>.toml ===
# Each section then runs the local smoke pipeline. We split on those
# markers and feed each chunk to per-model awk patterns.
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR" "$STRIPPED"' EXIT

awk '
/^\[ec2\] === smoke: / {
  if (out) close(out)
  match($0, /profile\/([a-z0-9_]+)\//, m)
  out = "'"$TMPDIR"'/" m[1] ".log"
  next
}
{
  if (out) print > out
}
' "$STRIPPED"

for tier in t1 t2 t3 t4; do
  file="$TMPDIR/${tier}_reference.log"
  upper="$(echo "$tier" | tr '[:lower:]' '[:upper:]')"
  if [ ! -f "$file" ]; then
    sub "${upper}_STATUS" "NOT_RUN"
    continue
  fi

  # Keygen metrics (regex emitted by cmd/keygen).
  CONSTRAINTS="$(awk -F'[()]' '/r1cs compiled in .* constraints/ {gsub(/ constraints/, "", $2); print $2; exit}' "$file")"
  COMPILE_TIME="$(awk -F'compiled in ' '/r1cs compiled in/ {split($2, a, " "); print a[1]; exit}' "$file")"
  SETUP_TIME="$(awk -F'groth16.Setup done in ' '/groth16.Setup done in/ {split($2, a, " "); print a[1]; exit}' "$file")"
  PK_BYTES="$(awk -F': ' '/\.pk: .* bytes/ {split($2, a, " "); print a[1]; exit}' "$file")"
  VK_BYTES="$(awk -F': ' '/\.vk: .* bytes/ {split($2, a, " "); print a[1]; exit}' "$file")"
  R1CS_BYTES="$(awk -F': ' '/\.r1cs: .* bytes/ {split($2, a, " "); print a[1]; exit}' "$file")"

  # Witness metrics.
  ACCT_COUNT="$(awk -F'loaded |across' '/loaded .* accounts across/ {gsub(/ accounts /, "", $2); print $2; exit}' "$file")"
  BATCH_LINE="$(awk -F'→ | batches' '/→ .* batches \(/{print $2; exit}' "$file")"
  WITNESS_ROOT="$(awk -F'witness run finished, account tree root = ' '/witness run finished/ {print $2; exit}' "$file")"

  # Prover metrics (per-batch — first batch shown).
  PROVE_MS="$(awk -F'proof generation cost |ms' '/proof generation cost/ {print $2; exit}' "$file")"
  VERIFY_MS="$(awk -F'proof verification cost |ms' '/proof verification cost/ {print $2; exit}' "$file")"

  # Verifier batch.
  BATCH_RESULT="FAIL"
  if grep -q "All proofs verify passed!!!" "$file"; then BATCH_RESULT="PASS"; fi

  # Userproof.
  USERPROOF_ROWS="$(awk -F'userproof run finished, | rows written' '/userproof run finished/ {print $2; exit}' "$file")"

  # Verifier -user.
  USER_RESULT="FAIL"
  if grep -q "^verify pass!!!" "$file"; then USER_RESULT="PASS"; fi

  # Overall status.
  STATUS="PASS"
  if [ "$BATCH_RESULT" != "PASS" ] || [ "$USER_RESULT" != "PASS" ]; then STATUS="FAIL"; fi
  if grep -q "panic:" "$file"; then STATUS="PANIC"; fi

  # Fill template placeholders.
  sub "${upper}_STATUS" "$STATUS"
  sub "${upper}_CONSTRAINTS" "${CONSTRAINTS:-?}"
  sub "${upper}_COMPILE" "${COMPILE_TIME:-?}"
  sub "${upper}_SETUP" "${SETUP_TIME:-?}"
  sub "${upper}_KEYGEN_TIME" "compile ${COMPILE_TIME:-?} + setup ${SETUP_TIME:-?}"
  sub "${upper}_PROVE_TIME" "${PROVE_MS:-?} ms"
  sub "${upper}_VERIFY_TIME" "${VERIFY_MS:-?} ms"
  sub "${upper}_PROVE_VERIFY_TIME" "${VERIFY_MS:-?} ms"
  sub "${upper}_USERPROOF_TIME" "${USERPROOF_ROWS:-?} rows"
  sub "${upper}_TOTAL_TIME" "see log"
  sub "${upper}_PK_SIZE" "$(numfmt --to=iec "${PK_BYTES:-0}" 2>/dev/null || echo "${PK_BYTES:-?} B")"
  sub "${upper}_VK_SIZE" "$(numfmt --to=iec "${VK_BYTES:-0}" 2>/dev/null || echo "${VK_BYTES:-?} B")"
  sub "${upper}_R1CS_SIZE" "$(numfmt --to=iec "${R1CS_BYTES:-0}" 2>/dev/null || echo "${R1CS_BYTES:-?} B")"
  sub "${upper}_SHAPE" "${SHAPE:-?}"
  sub "${upper}_BATCH_COUNT" "${BATCH_LINE:-?}"
  sub "${upper}_FINAL_ROOT" "${WITNESS_ROOT:-?}"
  sub "${upper}_PROVE_COUNT" "1 (testdata 1 batch)"
  sub "${upper}_VERIFIER_BATCH_RESULT" "$BATCH_RESULT"
  sub "${upper}_FINAL_CEX_RESULT" "$BATCH_RESULT"
  sub "${upper}_USERPROOF_ROWS" "${USERPROOF_ROWS:-?}"
  sub "${upper}_VERIFIER_USER_RESULT" "$USER_RESULT"
  sub "${upper}_USER_INDEX" "0"
  sub "${upper}_NOTES" "(auto-extracted; manual review recommended)"

  # R6.5 ratio.
  case "$tier" in
    t1) BASELINE=38149 ;;
    t2) BASELINE=48886 ;;
    t3) BASELINE=274650 ;;
    t4) BASELINE=723790 ;;
  esac
  if [ -n "$CONSTRAINTS" ] && [ "$CONSTRAINTS" != "?" ]; then
    RATIO="$(awk -v c="$CONSTRAINTS" -v b="$BASELINE" 'BEGIN{printf "%.2fx", c/b}')"
  else
    RATIO="?"
  fi
  sub "${upper}_RATIO" "$RATIO"
done

# Totals.
sub "ACCOUNT_COUNT" "10"
sub "TOTAL_USERS" "500 (10 real + 490 padding) — shape dependent"
sub "BATCH_COUNT" "1 per model"
sub "PROFILES" "t1_reference, t2_reference, t3_reference, t4_reference"

# Observations: fill with summary of any panic / fail lines.
PANIC_LINES="$(grep -c "panic:" "$STRIPPED" || true)"
FAIL_LINES="$(grep -c "verify failed" "$STRIPPED" || true)"
sub "OBSERVATION_1" "panic 발생: ${PANIC_LINES}건"
sub "OBSERVATION_2" "verifier verify failed: ${FAIL_LINES}건"
sub "OBSERVATION_3" "환경: r7a.4xlarge, AL2023, gnark fork — keygen artifacts cache-hit 시 시간 단축"

# Strip unfilled placeholders by leaving them — operator can scan for
# `{{...}}` to see what wasn't auto-extracted.

echo "wrote: $OUT"
echo "unfilled placeholders remaining: $(grep -c '{{[A-Z_0-9]*}}' "$OUT" || true)"
