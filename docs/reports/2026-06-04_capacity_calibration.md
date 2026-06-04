# Capacity-planner calibration — 2026-06-04

Calibration anchors for the OPS capacity planner (`cmd/plan`). The ENGINE
side (`pkg/estimate.Constraints`) computes the constraint count exactly and
needs no measurement; this report holds the **environment-specific resource
coefficients** that turn a constraint count into peak RAM / artifact size /
setup+prove time / instance recommendation.

**Machine**: AWS g6.4xlarge — NVIDIA L4 (24 GB), AMD EPYC 16 vCPU, 60 GB RAM,
CUDA 12.8. gnark = bnb-chain fork v0.10.1. GPU = Icicle v3.2.2.
**Workload**: `*_reference/testdata/happy` (SPARSE, density ≈ 0.0004) — see
the density note below for why that matters for ONE metric only.

## Measured sweep (8 cells, 4 models, 1.5M–6.6M constraints)

| model | constraints | Setup RAM (GB) | pk (MB) | Setup (s) | CPU prove (ms) | GPU prove (ms) |
|---|---:|---:|---:|---:|---:|---:|
| t1 | 2,241,759 | 4.4 | 431 | 93.8 | 6,879 | 2,745 |
| t1 | 6,619,676 | 11.4 | 1,145 | 253.5 | 16,105 | 6,374 |
| t2 | 1,498,620 | 2.6 | 281 | 64.4 | 3,996 | 1,784 |
| t2 | 4,343,494 | 8.9 | 880 | 199.5 | 12,282 | 4,792 |
| t3 | 1,784,868 | 2.9 | 340 | 75.4 | 3,476 | 1,759 |
| t3 | 3,495,252 | 6.1 | 655 | 145.3 | 6,707 | 3,291 |
| t4 | 3,295,729 | 5.7 | 651 | 145.2 | 5,521 | 1,910 |
| t4 | 4,543,952 | 9.1 | 969 | 213.2 | 9,756 | 4,508 |

Prior anchors folded in: t1 174k (CPU 660 / GPU 1,640 ms — GPU loses);
t1 4.44M (CPU 13,055 / GPU 4,918); t4 64M production (Setup RAM ~87 GB, pk
~24 GB, on a 128 GB CPU box). R11-D dense prove RSS ~120 GB at density 1.0
(`docs/BENCHMARK.md §1.3`).

## Derived coefficients

The per-constraint costs are **nearly model-independent** (Setup RAM 1.7–2.1
KB/c, pk 173–213 B/c across all 4 models), so the planner keys mostly off the
constraint count with a small fixed table — not a per-(model,shape) grid.

| quantity | coefficient | notes |
|---|---|---|
| Setup peak RAM | **1.8 GB / M-constraint** | density-FREE (circuit only) |
| pk size | **195 bytes / constraint** | density-free |
| r1cs size | **125 bytes / constraint** | density-free (from t1 4.44M anchor) |
| Setup time | **43 s / M-constraint** | g6.4xlarge 16 vCPU; ~n·log n, linear over 1.5–6.6M |
| CPU prove | **2,365 ms/M + 450 ms** | density-free (`per_batch_prove(d)≈prove(1.0)`) |
| GPU prove | **max(1,600, 896 ms/M + 440 ms)** | ~1.6 s device floor → GPU loses below ≈0.5M |
| GPU crossover | **≈ 0.5 M constraints** | below: CPU faster; above: GPU 2–2.6× |
| **prove peak RAM** | **(pk + r1cs) + 1.6 GB/M · density · workers** | density-DEPENDENT (§1.3) |

### Density note (per `docs/BENCHMARK.md §1.3/1.5`)

Density (how many assets each user actually holds) affects **only prove peak
RAM** — NOT constraints, NOT Setup RAM, NOT prove time, NOT artifact size:

- `per_batch_prove(density=d) ≈ per_batch_prove(1.0)` (≤10%) → prove TIME is
  density-free.
- `prove_peak_RSS ≈ pk + r1cs + k·constraints·density·workers` → prove RAM is
  LINEAR in density. Anchors: density 0.0004 → ~16 GB, 0.05 (exchange avg) →
  ~50 GB, 1.0 (worst) → ~120 GB.

The sweep above ran at SPARSE density, so its Setup-RAM / time / pk / prove-
time numbers are valid worst-case (density-free), but its *prove* RAM is the
LOW end. The planner therefore models prove RAM with the density term and
**defaults density = 1.0 (worst-case) for safe instance sizing**, overridable
by the snapshot's actual density.

## Provenance / refresh

Raw log: `.artifacts/reports/2026-06-04_calibration_sweep_raw.txt`. These
coefficients are tied to (machine, gnark version, CUDA). Re-measure with the
detached sweep when any of those change; the engine constraint estimate stays
exact regardless. T2/T3 have 2 anchors each (good for the linear terms);
add points if higher fidelity is needed.
