//go:build !icicle

package host

import "github.com/consensys/gnark/backend"

// ProverOptions returns the gnark prover options the model runners pass to
// groth16.Prove. The default build (no `icicle` tag) returns none — a pure
// CPU prover with no CGO/CUDA dependency.
//
// GPU acceleration is a BUILD-TIME concern, not a runtime engine flag
// (PRODUCTION_ROADMAP R13-B, core philosophy): building with `-tags icicle`
// swaps in prove_icicle.go, which returns backend.WithIcicleAcceleration().
// The engine's prove call site is byte-identical in both builds — only this
// one-file seam differs, so the CPU binary never imports the icicle backend.
func ProverOptions() []backend.ProverOption {
	return nil
}
