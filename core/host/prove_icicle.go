//go:build icicle

package host

import "github.com/consensys/gnark/backend"

// ProverOptions returns backend.WithIcicleAcceleration() so groth16.Prove
// runs the MSM / NTT on GPU via the gnark fork's ICICLE backend. Selected
// only in the `-tags icicle` build, which links the CGO native CUDA/ICICLE
// libraries (see scripts/ec2/bootstrap-gpu.sh and PRODUCTION_ROADMAP
// R13-A/B). gnark's groth16.Prove still gates on icicle_bn254.HasIcicle, so
// the option is a no-op if the backend is not linked — but this file is
// only compiled when the tag (and thus the backend) is present.
func ProverOptions() []backend.ProverOption {
	return []backend.ProverOption{backend.WithIcicleAcceleration()}
}
