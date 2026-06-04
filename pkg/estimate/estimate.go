// Package estimate predicts a batch's R1CS constraint count WITHOUT
// running the expensive trusted setup, by compiling the model circuit at a
// couple of small users-per-batch probes and extrapolating.
//
// The circuit is per-user loop-driven, so nbConstraints is exactly affine
// in usersPerBatch for a fixed (assetCountTier, capacity):
//
//	nbConstraints(users) = base + perUser * users
//
// Two compiles pin that line, and any target usersPerBatch is then exact —
// there are no fitted/magic coefficients, and no drift when the audited
// circuit changes, because it recompiles the real circuit (the same one
// keygen would). The probes use small users, so they are cheap even when
// the target shape (e.g. 92 users × 500 assets) would take minutes and
// gigabytes to compile in full.
//
// This is the ENGINE half of capacity planning (PRODUCTION_ROADMAP R13
// follow-up): the constraint count is intrinsic to the frozen circuit
// contract. The OPS half — translating constraints into peak RAM / .pk
// size / prove time / EC2 instance type — lives OUTSIDE the engine
// (cmd/plan), because those coefficients are environment-specific
// deployment concerns (Scope Boundary).
package estimate

import (
	"fmt"

	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/BetweenBits-org/zk-pos-ext/pkg/keygen"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// probeLo and probeHi are the small users-per-batch values Constraints
// compiles to pin the per-user line. A wider spread reduces the relative
// error of any minor non-linearity (e.g. the per-batch random-challenge
// Poseidon over the user hashes).
const (
	probeLo = 2
	probeHi = 8
)

// compileAt compiles the model circuit at usersPerBatch=n (holding tier +
// capacity) and returns the exact constraint count. This is exactly what
// keygen does before Setup, minus the Setup.
func compileAt(model corespec.SolvencyModelID, tier, capacity, n int) (int, error) {
	circuit, err := keygen.NewCircuit(model, corespec.BatchShape{AssetCountTier: tier, UsersPerBatch: n}, capacity)
	if err != nil {
		return 0, err
	}
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit, frontend.IgnoreUnconstrainedInputs())
	if err != nil {
		return 0, fmt.Errorf("estimate: compile %s tier=%d cap=%d users=%d: %w", model, tier, capacity, n, err)
	}
	return cs.GetNbConstraints(), nil
}

// Constraints predicts the R1CS constraint count for the model at the given
// shape and asset capacity. For usersPerBatch <= probeHi it compiles the
// shape directly (exact); otherwise it compiles the two small probes and
// extrapolates along the per-user line. Returns an error if the model is
// unsupported or any probe compile fails.
func Constraints(model corespec.SolvencyModelID, shape corespec.BatchShape, capacity int) (int, error) {
	tier := shape.AssetCountTier
	users := shape.UsersPerBatch
	if users <= 0 {
		return 0, fmt.Errorf("estimate: usersPerBatch must be > 0, got %d", users)
	}
	if tier <= 0 {
		return 0, fmt.Errorf("estimate: assetCountTier must be > 0, got %d", tier)
	}
	if users <= probeHi {
		return compileAt(model, tier, capacity, users)
	}
	nLo, err := compileAt(model, tier, capacity, probeLo)
	if err != nil {
		return 0, err
	}
	nHi, err := compileAt(model, tier, capacity, probeHi)
	if err != nil {
		return 0, err
	}
	perUser := float64(nHi-nLo) / float64(probeHi-probeLo)
	base := float64(nLo) - perUser*float64(probeLo)
	return int(base + perUser*float64(users) + 0.5), nil
}

// CompileConstraints returns the EXACT constraint count by compiling the
// target shape directly. Slow + memory-heavy for large shapes — that is the
// cost Constraints avoids — but it is the ground truth Constraints is
// validated against, and is the right call when the shape is small or when
// exactness matters more than speed.
func CompileConstraints(model corespec.SolvencyModelID, shape corespec.BatchShape, capacity int) (int, error) {
	if shape.UsersPerBatch <= 0 || shape.AssetCountTier <= 0 {
		return 0, fmt.Errorf("estimate: shape must have positive assetCountTier and usersPerBatch")
	}
	return compileAt(model, shape.AssetCountTier, capacity, shape.UsersPerBatch)
}
