// Command plan is the OPS-side capacity planner: given a (model, shape,
// capacity), it predicts Setup/prove peak RAM, artifact sizes, Setup/prove
// time, the GPU benefit, and recommends EC2 instance types — to drive batch
// optimization and instance-bootstrap automation.
//
// Split (PRODUCTION_ROADMAP R13 follow-up): the constraint count is computed
// EXACTLY by the engine (pkg/estimate, density-free, intrinsic to the frozen
// circuit). This tool layers the ENVIRONMENT-SPECIFIC resource coefficients
// on top — they live here, not in the engine, because they depend on the
// machine / gnark version / CUDA (Scope Boundary). Coefficients are dated +
// sourced in docs/reports/2026-06-04_capacity_calibration.md.
//
// Density only affects prove peak RAM (docs/BENCHMARK.md §1.3): constraints,
// Setup RAM, prove time, and artifact sizes are density-free. -density
// defaults to 1.0 (worst-case) for safe sizing.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"strings"

	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/BetweenBits-org/zk-pos-ext/pkg/estimate"
	"github.com/consensys/gnark/logger"
	"github.com/rs/zerolog"
)

// Calibration coefficients — g6.4xlarge (L4, 16 vCPU), gnark bnb v0.10.1,
// CUDA 12.8, Icicle v3.2.2, 2026-06-04. See the calibration report for the
// underlying sweep. Re-measure when machine / gnark / CUDA change.
const (
	calibDate    = "2026-06-04"
	calibMachine = "g6.4xlarge (L4, 16 vCPU)"

	setupRAMGBPerM = 1.8   // Setup peak RAM per M-constraint (density-free)
	pkBytesPerC    = 195.0 // .pk bytes per constraint
	r1csBytesPerC  = 125.0 // .r1cs bytes per constraint
	setupSecPerM   = 43.0  // Setup time per M-constraint (this machine)
	cpuProveMsPerM = 2365.0
	cpuProveMsBase = 450.0
	gpuProveMsPerM = 896.0
	gpuProveMsBase = 440.0
	gpuFloorMs     = 1600.0  // GPU device-setup floor
	gpuCrossoverC  = 500000.0 // below this, CPU prove is faster
	// prove RAM density term (docs/BENCHMARK.md §1.3): GB per M-constraint
	// per unit density per worker.
	proveRAMGBPerMxDxW = 1.6
)

type instance struct {
	name     string
	vcpu     int
	ramGB    int
	gpu      string
	usdPerHr float64
}

// A small catalog (README §EC2 + this session). Ordered cheapest-first per
// class so the first match is the smallest viable box.
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

// pick returns the smallest box whose RAM clears the need with 20% headroom.
// Disk is reported separately — it is EBS, sized independently of the box.
func pick(boxes []instance, ramNeedGB float64) (instance, bool) {
	for _, b := range boxes {
		if float64(b.ramGB) >= ramNeedGB*1.2 {
			return b, true
		}
	}
	return instance{}, false
}

func main() {
	model := flag.String("model", "", "solvency model id (e.g. t1_simple_margin)")
	tier := flag.Int("tier", 50, "assetCountTier")
	users := flag.Int("users", 0, "usersPerBatch (required)")
	capacity := flag.Int("capacity", 0, "asset capacity (required)")
	density := flag.Float64("density", 1.0, "user-data density 0..1 (1.0 = worst-case, default)")
	workers := flag.Int("workers", 1, "concurrent prove workers (prove RAM scales with this)")
	flag.Parse()

	logger.Set(zerolog.Nop()) // silence gnark's compile logs; only the plan goes to stdout

	if *model == "" || *users <= 0 || *capacity <= 0 {
		fmt.Fprintln(os.Stderr, "usage: plan -model <id> -users N -capacity N [-tier 50] [-density 1.0] [-workers 1]")
		os.Exit(2)
	}

	c, err := estimate.Constraints(
		corespec.SolvencyModelID(*model),
		corespec.BatchShape{AssetCountTier: *tier, UsersPerBatch: *users},
		*capacity,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "plan:", err)
		os.Exit(1)
	}
	cM := float64(c) / 1e6

	setupRAM := setupRAMGBPerM * cM
	pkGB := pkBytesPerC * float64(c) / 1e9
	r1csGB := r1csBytesPerC * float64(c) / 1e9
	setupS := setupSecPerM * cM
	cpuMs := cpuProveMsPerM*cM + cpuProveMsBase
	gpuMs := math.Max(gpuFloorMs, gpuProveMsPerM*cM+gpuProveMsBase)
	proveRAM := (pkGB + r1csGB) + proveRAMGBPerMxDxW*cM*(*density)*float64(*workers)
	diskGB := (pkGB + r1csGB) * 1.5
	gpuWins := float64(c) > gpuCrossoverC
	peakRAM := math.Max(setupRAM, proveRAM)

	keygenBox, kok := pick(cpuBoxes, setupRAM)
	var proveBox instance
	var pok bool
	if gpuWins {
		proveBox, pok = pick(gpuBoxes, proveRAM)
	} else {
		proveBox, pok = pick(cpuBoxes, proveRAM)
	}

	fmt.Printf("capacity plan — model=%s tier=%d users=%d capacity=%d\n", *model, *tier, *users, *capacity)
	fmt.Printf("  (calibration %s on %s; density=%.4f workers=%d)\n\n", calibDate, calibMachine, *density, *workers)
	fmt.Printf("  constraints     %s  (exact, engine compile-extrapolate)\n", commaInt(c))
	fmt.Printf("  artifacts       pk≈%.2f GB  r1cs≈%.2f GB  → disk≈%.1f GB\n", pkGB, r1csGB, diskGB)
	fmt.Printf("  Setup           RAM≈%.1f GB  time≈%s   (density-free)\n", setupRAM, dur(setupS))
	fmt.Printf("  prove RAM       ≈%.1f GB     (density-scaled; ×workers)\n", proveRAM)
	fmt.Printf("  prove time      CPU≈%s   GPU≈%s   GPU %s\n", durMs(cpuMs), durMs(gpuMs), gpuVerdict(gpuWins, cpuMs, gpuMs))
	fmt.Printf("  peak RAM (max)  ≈%.1f GB\n\n", peakRAM)

	fmt.Printf("  recommend keygen: %s\n", boxLine(keygenBox, kok, setupRAM))
	fmt.Printf("  recommend prove : %s\n", boxLine(proveBox, pok, proveRAM))

	fmt.Printf("\n  assumptions: prove RAM uses density=%.4f (override -density with the\n", *density)
	fmt.Printf("  snapshot's real density); coefficients are %s/%s — re-measure if the\n", calibDate, calibMachine)
	fmt.Printf("  machine/gnark/CUDA changes. Constraints are exact; resource numbers ±~20%%.\n")
}

func boxLine(b instance, ok bool, ramNeed float64) string {
	if !ok {
		return fmt.Sprintf("NONE in catalog fits %.0f GB RAM — extend the catalog or split the batch", ramNeed*1.2)
	}
	gpu := ""
	if b.gpu != "" {
		gpu = " " + b.gpu
	}
	return fmt.Sprintf("%s (%d vCPU, %d GB%s, ~$%.2f/hr)", b.name, b.vcpu, b.ramGB, gpu, b.usdPerHr)
}

func gpuVerdict(wins bool, cpuMs, gpuMs float64) string {
	if wins {
		return fmt.Sprintf("WINS (%.1fx)", cpuMs/gpuMs)
	}
	return "loses (below ~0.5M crossover — use CPU)"
}

func dur(s float64) string {
	if s < 90 {
		return fmt.Sprintf("%.0fs", s)
	}
	return fmt.Sprintf("%.1fmin", s/60)
}
func durMs(ms float64) string { return dur(ms / 1000) }

func commaInt(n int) string {
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	return b.String()
}
