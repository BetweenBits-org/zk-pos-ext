package main

// Calibration coefficients — ENVIRONMENT-SPECIFIC, dated, refreshable. This
// is the OPS half of capacity planning: these numbers are NOT engine
// contract, they depend on the machine / gnark version / CUDA. Keeping them
// in their own file (and out of pkg/) makes the Scope Boundary explicit and
// re-calibration a single-file edit. The engine constraint estimate
// (pkg/estimate) is exact regardless of anything here.
//
// Measured on g6.4xlarge (L4, 16 vCPU), gnark bnb v0.10.1, CUDA 12.8, Icicle
// v3.2.2, 2026-06-04. Derivation + raw sweep:
// docs/reports/2026-06-04_capacity_calibration.md. Re-measure with
// tools/sweep when the machine / gnark / CUDA change.
const (
	calibDate    = "2026-06-04"
	calibMachine = "g6.4xlarge (L4, 16 vCPU)"

	setupRAMGBPerM = 1.8   // Setup peak RAM per M-constraint (density-free)
	pkBytesPerC    = 195.0 // .pk bytes per constraint
	r1csBytesPerC  = 125.0 // .r1cs bytes per constraint
	setupSecPerM   = 43.0  // Setup time per M-constraint (this machine, 16 vCPU)
	cpuProveMsPerM = 2365.0
	cpuProveMsBase = 450.0
	gpuProveMsPerM = 896.0
	gpuProveMsBase = 440.0
	gpuFloorMs     = 1600.0   // GPU device-setup floor
	gpuCrossoverC  = 500000.0 // below this, CPU prove is faster

	// prove RAM density term (docs/BENCHMARK.md §1.3): GB per M-constraint
	// per unit density per worker. Only prove RAM is density-dependent.
	proveRAMGBPerMxDxW = 1.6
)

type instance struct {
	name     string
	vcpu     int
	ramGB    int
	gpu      string
	usdPerHr float64
}

// Cheapest/smallest-first so pick returns the smallest viable box. r-family
// (1:8 vCPU:RAM) is preferred — c-family OOMs on these RAM-bound jobs and
// Graviton lacks gnark's amd64 fast-path (README §EC2).
var cpuBoxes = []instance{
	{"m7a.2xlarge", 8, 32, "", 0.46},
	{"r7a.2xlarge", 8, 64, "", 0.53},
	{"r7a.4xlarge", 16, 128, "", 1.06},
	{"r7a.8xlarge", 32, 256, "", 2.11},
	{"r7a.16xlarge", 64, 512, "", 4.22},
}
var gpuBoxes = []instance{
	{"g6.4xlarge", 16, 60, "L4 24GB", 1.30},
	{"g6e.4xlarge", 16, 128, "L40S 48GB", 4.19},
}
