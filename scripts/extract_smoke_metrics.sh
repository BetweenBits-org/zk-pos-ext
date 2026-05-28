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

# Usage:
#   extract_smoke_metrics.sh <log-file> [<output.md>] [--json <output.json>]
#
# --json optionally writes a machine-readable file with per-model
# metrics — handy for benchmark matrix automation (R11+).
JSON_OUT=""
POSITIONAL=()
while [ "$#" -gt 0 ]; do
  case "$1" in
    --json)
      JSON_OUT="${2:?--json requires a path argument}"; shift 2 ;;
    *)
      POSITIONAL+=("$1"); shift ;;
  esac
done
set -- "${POSITIONAL[@]+"${POSITIONAL[@]}"}"

LOG="${1:?usage: $0 <log-file> [<output.md>] [--json <path>]}"
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

python3 - "$STRIPPED" "$TMPDIR" <<'PY'
import re
import sys
log, outdir = sys.argv[1], sys.argv[2]
section_re = re.compile(r"^\[ec2\] === smoke: profile/([a-z0-9_]+)/")
out = None
with open(log) as f:
    for line in f:
        m = section_re.search(line)
        if m:
            if out is not None:
                out.close()
            out = open(f"{outdir}/{m.group(1)}.log", "w")
            continue
        if out is not None:
            out.write(line)
if out is not None:
    out.close()
PY

# JSON entries (one per processed model) — assembled at the end into
# the optional <output.json>.
JSON_ENTRIES=()

for tier in t1 t2 t3 t4; do
  file="$TMPDIR/${tier}_reference.log"
  upper="$(echo "$tier" | tr '[:lower:]' '[:upper:]')"
  if [ ! -f "$file" ]; then
    sub "${upper}_STATUS" "NOT_RUN"
    if [ -n "$JSON_OUT" ]; then
      JSON_ENTRIES+=("$(printf '{"model":"%s","status":"NOT_RUN"}' "$upper")")
    fi
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

  # Prover metrics — both first-batch (legacy) and multi-batch
  # aggregate (R11-C). First-batch is kept for placeholder backwards
  # compat with v1 template; aggregate is the R11 multi-batch view.
  PROVE_MS="$(awk -F'proof generation cost |ms' '/proof generation cost/ {print $2; exit}' "$file")"
  VERIFY_MS="$(awk -F'proof verification cost |ms' '/proof verification cost/ {print $2; exit}' "$file")"

  # Aggregate prove time across every batch in this model run.
  # awk emits "sum count min max" (all in ms, integer).
  PROVE_AGG="$(awk '/proof generation cost/ {
      ms = $4 + 0
      sum += ms; n++
      if (min == 0 || ms < min) min = ms
      if (ms > max) max = ms
    } END {
      if (n == 0) print "0 0 0 0"
      else        print sum, n, min, max
    }' "$file")"
  PROVE_TOTAL_MS="$(echo "$PROVE_AGG" | awk '{print $1}')"
  PROVE_BATCHES="$(echo "$PROVE_AGG" | awk '{print $2}')"
  PROVE_MIN_MS="$(echo "$PROVE_AGG" | awk '{print $3}')"
  PROVE_MAX_MS="$(echo "$PROVE_AGG" | awk '{print $4}')"
  PROVE_AVG_MS="$(echo "$PROVE_AGG" | awk '{
      if ($2 > 0) printf "%.1f", $1 / $2; else print "0"
    }')"

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

  # Multi-batch aggregate (R11-C). When the testdata produces only one
  # batch these match the per-batch values above; with R11-A real-scale
  # testdata they diverge meaningfully.
  sub "${upper}_PROVE_BATCH_COUNT" "${PROVE_BATCHES:-0}"
  sub "${upper}_PROVE_TOTAL_MS" "${PROVE_TOTAL_MS:-0}"
  sub "${upper}_PROVE_AVG_MS" "${PROVE_AVG_MS:-0}"
  sub "${upper}_PROVE_MIN_MS" "${PROVE_MIN_MS:-0}"
  sub "${upper}_PROVE_MAX_MS" "${PROVE_MAX_MS:-0}"
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

  # JSON entry assembly (R11-C). Uses printf %s for strings so missing
  # extractions surface as empty values (downstream consumers can spot
  # them); numeric fields default to 0 to keep JSON parseable.
  if [ -n "$JSON_OUT" ]; then
    JSON_ENTRIES+=("$(printf '{"model":"%s","status":"%s","constraints":%s,"compile":"%s","setup":"%s","prove_batches":%s,"prove_total_ms":%s,"prove_avg_ms":%s,"prove_min_ms":%s,"prove_max_ms":%s,"prove_first_ms":%s,"verify_first_ms":%s,"pk_bytes":%s,"vk_bytes":%s,"r1cs_bytes":%s,"userproof_rows":%s,"verifier_batch":"%s","verifier_user":"%s"}' \
        "$upper" "$STATUS" "${CONSTRAINTS:-0}" "${COMPILE_TIME:-}" "${SETUP_TIME:-}" \
        "${PROVE_BATCHES:-0}" "${PROVE_TOTAL_MS:-0}" "${PROVE_AVG_MS:-0}" "${PROVE_MIN_MS:-0}" "${PROVE_MAX_MS:-0}" \
        "${PROVE_MS:-0}" "${VERIFY_MS:-0}" \
        "${PK_BYTES:-0}" "${VK_BYTES:-0}" "${R1CS_BYTES:-0}" \
        "${USERPROOF_ROWS:-0}" "$BATCH_RESULT" "$USER_RESULT")")
  fi
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

# JSON output (R11-C optional, --json flag).
if [ -n "$JSON_OUT" ]; then
  mkdir -p "$(dirname "$JSON_OUT")"
  {
    printf '{\n'
    printf '  "run_id": "smoke_%s",\n' "$TS"
    printf '  "log_file": "%s",\n' "$LOG"
    printf '  "shape": "%s",\n' "${SHAPE:-unknown}"
    printf '  "capacity": "%s",\n' "${CAPACITY:-unknown}"
    printf '  "models": [\n    %s\n  ]\n' "$(IFS=','; echo "${JSON_ENTRIES[*]:-}")"
    printf '}\n'
  } > "$JSON_OUT"
  echo "wrote json: $JSON_OUT"
fi

# Strip unfilled placeholders by leaving them — operator can scan for
# `{{...}}` to see what wasn't auto-extracted.

echo "wrote: $OUT"
echo "unfilled placeholders remaining: $(grep -c '{{[A-Z_0-9]*}}' "$OUT" || true)"
