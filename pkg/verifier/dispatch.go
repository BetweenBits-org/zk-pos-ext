// Model dispatch helpers. The verifier is model-blind in its outer
// loop; per-model logic (CEX commitment build, public-witness
// construction, user-leaf reconstruction) is reached through these
// three switch funnels. Adding a new solvency model means adding a
// case here plus the matching <model>/host runner — nothing else in
// this package changes.

package verifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
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
// reconstruction + Merkle inclusion check.
func dispatchVerifyUserInclusion(model corespec.SolvencyModelID, plan *corehost.VerifierPlan, userConfigPath string) error {
	switch model {
	case "t1_simple_margin":
		return t1host.VerifyUserInclusion(plan, userConfigPath)
	case "t2_static_haircut_margin":
		return t2host.VerifyUserInclusion(plan, userConfigPath)
	case "t3_tiered_haircut_margin_1pool":
		return t3host.VerifyUserInclusion(plan, userConfigPath)
	case "t4_tiered_haircut_margin_3pool":
		return t4host.VerifyUserInclusion(plan, userConfigPath)
	default:
		return fmt.Errorf("verifier: unsupported model %q", model)
	}
}

// loadVerifyingKey reads a groth16 BN254 verifying key from disk.
func loadVerifyingKey(path string) (groth16.VerifyingKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	vk := groth16.NewVerifyingKey(ecc.BN254)
	if _, err := vk.ReadFrom(bytes.NewBuffer(raw)); err != nil {
		return nil, err
	}
	return vk, nil
}
