# tools/sweep — capacity-planner calibration sweep

Collects the resource anchors `tools/plan` is calibrated from. OPS tooling
(env-coupled), not engine. The engine constraint estimate (`pkg/estimate`) is
exact and needs no measurement; this sweep only measures the
environment-specific costs (Setup RAM/time, `.pk` size, CPU/GPU prove time).

## Box prerequisites

Run on a GPU box prepared per `docs/R13_GPU_RUNBOOK.md`:
Go, the vendored gnark fork (`./gnark-fork` + Icicle v3), the native Icicle
CUDA libs, `/tmp/prover-cpu` + `/tmp/prover-gpu` built (`-tags icicle`), the
`zkpor-smoke-mysql` container, and GNU `time` (`sudo dnf install -y time`).

## Run (detached, survives ssh drops)

```bash
# from your laptop, with scripts/ec2/.env pointing at the (running) box:
scripts/ec2/sync.sh                                   # push the repo
scp -i "$KEY" tools/sweep/calibration_sweep.sh "$HOST:~/sweep.sh"
ssh -i "$KEY" "$HOST" 'nohup bash ~/sweep.sh > ~/sweep.log 2>&1 & echo started'
# poll ~/sweep_results.txt; when "=== SWEEP COMPLETE ===":
scp -i "$KEY" "$HOST:~/sweep_results.txt" .
# then STOP the box (cost): aws ec2 stop-instances --instance-ids <id>
```

Edit the `SHAPES` array in the script for the grid. Keep sizes within the
box's RAM/disk; the sweep deletes each `.pk` after measuring.

## Parse → coefficients

```bash
awk '
/^=== SHAPE/{for(i=1;i<=NF;i++){if($i~/model=/){m=$i;sub("model=","",m)} if($i~/shape=/){s=$i;sub("shape=","",s)}}}
/r1cs compiled in/{for(i=1;i<=NF;i++) if($i~/constraints\)/){c=$(i-1);sub(/\(/,"",c)}}
/Setup done in/{st=$NF} /Maximum resident/{ram=$NF} /^PKBYTES/{pk=$2}
/^CPU proof generation cost/{cpu=$5} /^GPU proof generation cost/{gpu=$5}
/^RESULT/{printf "%-32s %-7s c=%-9s ram=%.1fGB pk=%dMB setup=%s cpu=%sms gpu=%sms\n",m,s,c,ram/1048576,pk/1048576,st,cpu,gpu; ram=pk=cpu=gpu=st=""}
' sweep_results.txt
```

Fit the per-constraint slopes (they are nearly model-independent — see the
report) and update **`tools/plan/calibration.go`** + a dated
`docs/reports/<date>_capacity_calibration.md`. Worst-case prove RAM is NOT
swept here (sparse testdata); it is modelled from the dense anchor in
`docs/BENCHMARK.md §1.3`.
