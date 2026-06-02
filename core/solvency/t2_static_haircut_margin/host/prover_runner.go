package host

import (
	"bytes"
	"fmt"
	"time"

	t2circuit "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t2_static_haircut_margin/circuit"
	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
)

// DecodeAndProve — T2 variant. See t4 host docstring for full notes.
func DecodeAndProve(
	encodedWitness []byte,
	r1cs constraint.ConstraintSystem,
	pk groth16.ProvingKey,
	vk groth16.VerifyingKey,
	assetCountTiers []int,
	batchNumber int64,
) (*corehost.BatchProofResult, error) {
	witnessForCircuit, err := DecodeBatchWitness(encodedWitness)
	if err != nil {
		return nil, fmt.Errorf("decode witness: %w", err)
	}

	startTime := time.Now().UnixMilli()
	fmt.Println("begin to generate proof for batch:", batchNumber)

	circuitWitness, err := t2circuit.SetBatchCreateUserCircuitWitness(witnessForCircuit, assetCountTiers)
	if err != nil {
		return nil, fmt.Errorf("build circuit witness: %w", err)
	}
	targetAssetsCount := len(circuitWitness.CreateUserOps[0].Assets)

	witness, err := frontend.NewWitness(circuitWitness, ecc.BN254.ScalarField())
	if err != nil {
		return nil, fmt.Errorf("witness: %w", err)
	}
	verifyWitness := t2circuit.NewVerifyBatchCreateUserCircuit(witnessForCircuit.BatchCommitment)
	vWitness, err := frontend.NewWitness(verifyWitness, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		return nil, fmt.Errorf("public witness: %w", err)
	}

	proof, err := groth16.Prove(r1cs, pk, witness, corehost.ProverOptions()...)
	if err != nil {
		return nil, fmt.Errorf("groth16.Prove: %w", err)
	}
	fmt.Println("proof generation cost", time.Now().UnixMilli()-startTime, "ms")

	verifyStart := time.Now().UnixMilli()
	if err := groth16.Verify(proof, vk, vWitness); err != nil {
		return nil, fmt.Errorf("groth16.Verify: %w", err)
	}
	fmt.Println("proof verification cost", time.Now().UnixMilli()-verifyStart, "ms")

	var proofBuf bytes.Buffer
	if _, err := proof.WriteRawTo(&proofBuf); err != nil {
		return nil, fmt.Errorf("serialise proof: %w", err)
	}

	return &corehost.BatchProofResult{
		AssetsCount:               targetAssetsCount,
		ProofRaw:                  proofBuf.Bytes(),
		BatchCommitment:           witnessForCircuit.BatchCommitment,
		BeforeAccountTreeRoot:     witnessForCircuit.BeforeAccountTreeRoot,
		AfterAccountTreeRoot:      witnessForCircuit.AfterAccountTreeRoot,
		BeforeCEXAssetsCommitment: witnessForCircuit.BeforeCEXAssetsCommitment,
		AfterCEXAssetsCommitment:  witnessForCircuit.AfterCEXAssetsCommitment,
	}, nil
}
