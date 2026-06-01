package host

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	t1circuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/circuit"
	t1spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/spec"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
)

// BuildCexCommitments parses the model-typed CexAssetsInfo from the raw
// JSON the verifier config carries, then computes the empty + final
// CEX commitments cmd/verifier needs for the batch chain check.
//
// T1 carries no collateral aggregates / tier ratios — only TotalEquity
// and TotalDebt are zeroed for the empty commitment. Per-asset equity <
// debt is allowed at the model level; surface as a warning only.
func BuildCexCommitments(raw json.RawMessage, capacity int) (empty, final []byte, err error) {
	var entries []t1spec.CexAssetInfo
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, nil, fmt.Errorf("unmarshal CexAssetsInfo: %w", err)
	}

	cexAssetsInfo := make([]t1spec.CexAssetInfo, len(entries))
	for i := range entries {
		entry := entries[i]
		cexAssetsInfo[entry.Index] = entry
		if entry.TotalEquity < entry.TotalDebt {
			fmt.Printf("warning: %s asset equity %d less than debt %d (allowed by model; check distribution)\n",
				entry.Symbol, entry.TotalEquity, entry.TotalDebt)
		}
	}

	emptyCexAssetsInfo := make([]t1spec.CexAssetInfo, len(cexAssetsInfo))
	copy(emptyCexAssetsInfo, cexAssetsInfo)
	for i := range emptyCexAssetsInfo {
		emptyCexAssetsInfo[i].TotalEquity = 0
		emptyCexAssetsInfo[i].TotalDebt = 0
	}
	empty = ComputeCexAssetsCommitment(emptyCexAssetsInfo, capacity)
	final = ComputeCexAssetsCommitment(cexAssetsInfo, capacity)
	return empty, final, nil
}

// NewVerifyPublicWitness builds the public-only witness cmd/verifier
// feeds to groth16.Verify against the T1 circuit's verifying key.
func NewVerifyPublicWitness(batchCommitment []byte) (witness.Witness, error) {
	verifyCircuit := t1circuit.NewVerifyBatchCreateUserCircuit(batchCommitment)
	w, err := frontend.NewWitness(verifyCircuit, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		return nil, fmt.Errorf("public witness: %w", err)
	}
	return w, nil
}

// VerifyUserInclusion decodes a T1-typed UserConfig (no TotalCollateral)
// from the supplied bytes, recomputes the universal 5-input leaf hash
// with slot-3 zero, and checks Merkle inclusion against the embedded
// root. The caller (cmd/verifier) reads the bytes via a vfs.ByteSource;
// this runner stays IO-free.
func VerifyUserInclusion(plan *corehost.VerifierPlan, userConfigBytes []byte) error {
	userConfig := &UserConfig{}
	if err := json.Unmarshal(userConfigBytes, userConfig); err != nil {
		return fmt.Errorf("unmarshal user config: %w", err)
	}

	root, err := hex.DecodeString(userConfig.Root)
	if err != nil || len(root) != 32 {
		return fmt.Errorf("invalid account tree root")
	}
	for i, p := range userConfig.Proof {
		if len(p) != 32 {
			return fmt.Errorf("invalid proof[%d] len=%d, want 32", i, len(p))
		}
	}

	assetCommitment := ComputeUserAssetsCommitment(userConfig.Assets, plan.AssetCountTiers)
	accountIDHash, err := hex.DecodeString(userConfig.AccountIdHash)
	if err != nil || len(accountIDHash) != 32 {
		return fmt.Errorf("invalid AccountIdHash")
	}
	var debtBytes []byte
	if userConfig.TotalDebt != nil {
		debtBytes = userConfig.TotalDebt.Bytes()
	}
	// T1 has no TotalCollateral — slot 3 is nil (PoseidonBytes maps it
	// to fr.Element{0,0,0,0}).
	accountHash := poseidon.PoseidonBytes(
		accountIDHash,
		userConfig.TotalEquity.Bytes(),
		debtBytes,
		nil,
		assetCommitment,
	)
	fmt.Println("user merkle leave hash base64 encode: ", base64.StdEncoding.EncodeToString(accountHash))
	fmt.Printf("user merkle leave hash hex encode: %x\n", accountHash)

	if corehost.VerifyMerkleProof(root, userConfig.AccountIndex, userConfig.Proof, accountHash) {
		fmt.Println("verify pass!!!")
	} else {
		fmt.Println("verify failed...")
	}
	return nil
}
