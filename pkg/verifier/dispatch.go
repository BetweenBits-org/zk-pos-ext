// Model dispatch helpers. The verifier is model-blind in its outer
// loop; per-model logic (CEX commitment build, public-witness
// construction, user-leaf reconstruction) is reached through these
// three switch funnels. Adding a new solvency model means adding a
// case here plus the matching <model>/host runner — nothing else in
// this package changes.

package verifier

import (
	"context"
	"encoding/json"
	"fmt"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/io/vfs"
	t1host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/host"
	t2host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t2_static_haircut_margin/host"
	t3host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t3_tiered_haircut_margin_1pool/host"
	t4host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/backend/witness"
)

// dispatchBuildCexCommitments routes raw CexAssetsInfo through the
// model's typed unmarshal + commitment computation.
func dispatchBuildCexCommitments(model corespec.SolvencyModelID, raw json.RawMessage, capacity int) (empty, final []byte, err error) {
	switch model {
	case "t1_simple_margin":
		return t1host.BuildCexCommitments(raw, capacity)
	case "t2_static_haircut_margin":
		return t2host.BuildCexCommitments(raw, capacity)
	case "t3_tiered_haircut_margin_1pool":
		return t3host.BuildCexCommitments(raw, capacity)
	case "t4_tiered_haircut_margin_3pool":
		return t4host.BuildCexCommitments(raw, capacity)
	default:
		return nil, nil, fmt.Errorf("verifier: unsupported model %q", model)
	}
}

// dispatchNewVerifyPublicWitness builds the model-specific public
// witness for groth16.Verify.
func dispatchNewVerifyPublicWitness(model corespec.SolvencyModelID, batchCommitment []byte) (witness.Witness, error) {
	switch model {
	case "t1_simple_margin":
		return t1host.NewVerifyPublicWitness(batchCommitment)
	case "t2_static_haircut_margin":
		return t2host.NewVerifyPublicWitness(batchCommitment)
	case "t3_tiered_haircut_margin_1pool":
		return t3host.NewVerifyPublicWitness(batchCommitment)
	case "t4_tiered_haircut_margin_3pool":
		return t4host.NewVerifyPublicWitness(batchCommitment)
	default:
		return nil, fmt.Errorf("verifier: unsupported model %q", model)
	}
}

// dispatchVerifyUserInclusion runs the model-typed user-leaf
// reconstruction + Merkle inclusion check. The user-config bytes are
// read by the caller (cmd/verifier) via a vfs.ByteSource and threaded
// in, so the runners stay IO-free.
func dispatchVerifyUserInclusion(model corespec.SolvencyModelID, plan *corehost.VerifierPlan, userConfigBytes []byte) error {
	switch model {
	case "t1_simple_margin":
		return t1host.VerifyUserInclusion(plan, userConfigBytes)
	case "t2_static_haircut_margin":
		return t2host.VerifyUserInclusion(plan, userConfigBytes)
	case "t3_tiered_haircut_margin_1pool":
		return t3host.VerifyUserInclusion(plan, userConfigBytes)
	case "t4_tiered_haircut_margin_3pool":
		return t4host.VerifyUserInclusion(plan, userConfigBytes)
	default:
		return fmt.Errorf("verifier: unsupported model %q", model)
	}
}

// loadVerifyingKey streams a groth16 BN254 verifying key from the
// injected vfs.KeyOpener. The opener joins the logical stem against its
// directory and the ".vk" extension; gnark reads straight from the
// stream. osvfs is stateless/goroutine-safe, so concurrent loads across
// the verify worker pool are fine.
func loadVerifyingKey(ctx context.Context, keys vfs.KeyOpener, stem string) (groth16.VerifyingKey, error) {
	r, err := keys.Open(ctx, stem, ".vk")
	if err != nil {
		return nil, err
	}
	defer r.Close()
	vk := groth16.NewVerifyingKey(ecc.BN254)
	if _, err := vk.ReadFrom(r); err != nil {
		return nil, err
	}
	return vk, nil
}
