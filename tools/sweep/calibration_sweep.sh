#!/bin/bash
# Calibration sweep — runs ON a prepared GPU box to collect the resource
# anchors the capacity planner (tools/plan) is calibrated from: per (model,
# shape) it records constraints, Setup peak RAM (GNU time), Setup time, .pk
# size, and CPU + GPU prove times, then deletes the .pk to bound disk.
#
# This is OPS tooling (env-coupled), not engine. See tools/sweep/README.md
# for box prerequisites (docs/R13_GPU_RUNBOOK.md), how to launch detached,
# and how to parse the output into tools/plan/calibration.go coefficients.
#
# Edit the SHAPES list below for the grid you want. Keep sizes within the
# box's RAM/disk: density-free metrics (Setup RAM/time, pk, prove time) are
# what we calibrate here; worst-case prove RAM is modelled separately from
# the dense anchor in docs/BENCHMARK.md §1.3.

set -uo pipefail

RDIR="${ZKPOR_REMOTE_DIR:-$HOME/zkmerkle-proof-of-solvency}"
CUDA="${CUDA_DIR:-/usr/local/cuda-12.8}"
export PATH=$PATH:/usr/local/go/bin:$CUDA/bin
export CUDA_HOME=$CUDA
export ICICLE_BACKEND_INSTALL_DIR=/usr/local/lib/backend
export LD_LIBRARY_PATH=/usr/local/lib:$CUDA/lib64
MYSQL="docker exec zkpor-smoke-mysql mysql -uzkpor -pzkpor@123 zkpor"
DSN='zkpor:zkpor@123@tcp(127.0.0.1:3306)/zkpor?parseTime=true'
OUT="${OUT:-$HOME/sweep_results.txt}"
PROVER_CPU="${PROVER_CPU:-/tmp/prover-cpu}"
PROVER_GPU="${PROVER_GPU:-/tmp/prover-gpu}"

# "model profile_dir assetCountTier usersPerBatch capacity"
SHAPES=(
  "t1_simple_margin                t1_reference 50 100 200"
  "t1_simple_margin                t1_reference 50 300 200"
  "t2_static_haircut_margin        t2_reference 50  50 200"
  "t2_static_haircut_margin        t2_reference 50 150 200"
  "t3_tiered_haircut_margin_1pool  t3_reference 50  20 200"
  "t3_tiered_haircut_margin_1pool  t3_reference 50  60 200"
  "t4_tiered_haircut_margin_3pool  t4_reference 50   8 200"
  "t4_tiered_haircut_margin_3pool  t4_reference 50  24 200"
)

cd "$RDIR/zkpor" || { echo "missing $RDIR/zkpor (sync the repo first)" >&2; exit 1; }
: > "$OUT"
for svc in witness prover keygen; do
  mkdir -p cmd/$svc/config
  printf '{ "MysqlDataSource": "%s", "DbSuffix": "", "TreeDB": { "Driver": "memory", "Option": { "Addr": "" } } }\n' "$DSN" > cmd/$svc/config/config.json
done

measure() {
  local model=$1 pdir=$2 tier=$3 users=$4 cap=$5
  local shape="${tier}_${users}"
  local prof="$RDIR/zkpor/profile/$pdir/$pdir.toml"
  local data="$RDIR/zkpor/profile/$pdir/testdata/happy"
  echo "=== SHAPE model=$model cap=$cap shape=$shape ($(date -u +%H:%M:%S)) ===" >> "$OUT"
  $MYSQL -e "DELETE FROM proof; DELETE FROM witness;" 2>/dev/null
  timeout 1500 /usr/bin/time -v env ZKPOR_BATCH_SHAPE_OVERRIDE=$shape go run ./cmd/keygen -profile "$prof" -asset-capacity $cap -out .artifacts/ >/tmp/kg.log 2>/tmp/kg.err
  grep -E "compiled in|Setup done|\.pk:" /tmp/kg.log | sed 's/^/KEYGEN /' >> "$OUT"
  grep -E "Maximum resident" /tmp/kg.err | sed 's/^/RAM /' >> "$OUT"
  local stem
  stem=$(ls .artifacts/ 2>/dev/null | grep -E "${model}\.${shape}\.pk$" | sed 's/\.pk$//' | head -1)
  if [ -z "$stem" ]; then echo "RESULT $model $shape KEYGEN_FAILED" >> "$OUT"; tail -2 /tmp/kg.err >> "$OUT"; return; fi
  echo "PKBYTES $(stat -c%s .artifacts/$stem.pk)" >> "$OUT"
  timeout 600 bash -c "cd cmd/witness && ZKPOR_BATCH_SHAPE_OVERRIDE=$shape go run . -profile '$prof' -user-data-dir '$data' -asset-capacity $cap" >/tmp/wit.log 2>&1
  echo "WROWS $($MYSQL -N -e 'SELECT COUNT(*) FROM witness' 2>/dev/null)" >> "$OUT"
  $MYSQL -e "DELETE FROM proof; UPDATE witness SET status=0;" 2>/dev/null
  timeout 300 bash -c "cd cmd/prover && ZKPOR_BATCH_SHAPE_OVERRIDE=$shape '$PROVER_CPU' -profile '$prof' -keys-dir '$RDIR/zkpor/.artifacts'" 2>&1 | grep -E "acceleration=|proof generation cost" | sed 's/^/CPU /' >> "$OUT"
  $MYSQL -e "DELETE FROM proof; UPDATE witness SET status=0;" 2>/dev/null
  timeout 300 bash -c "cd cmd/prover && ZKPOR_BATCH_SHAPE_OVERRIDE=$shape '$PROVER_GPU' -profile '$prof' -keys-dir '$RDIR/zkpor/.artifacts'" 2>&1 | grep -E "acceleration=|proof generation cost" | sed 's/^/GPU /' >> "$OUT"
  rm -f .artifacts/$stem.pk .artifacts/$stem.r1cs
  echo "RESULT $model $shape OK" >> "$OUT"
}

for s in "${SHAPES[@]}"; do
  # shellcheck disable=SC2086
  measure $s
done
echo "=== SWEEP COMPLETE ($(date -u +%H:%M:%S)) ===" >> "$OUT"
