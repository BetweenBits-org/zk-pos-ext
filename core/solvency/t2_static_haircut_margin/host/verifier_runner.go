package host

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
	t2circuit "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t2_static_haircut_margin/circuit"
	t2spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t2_static_haircut_margin/spec"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
)

// BuildCexCommitments parses the model-typed CexAssetsInfo from raw JSON
// and computes the empty + final CEX commitments. T2 zeros the single
// Collateral aggregate for the empty commitment; Haircut basis points
// stay because they describe the asset's risk profile, not its state.
func BuildCexCommitments(raw json.RawMessage, capacity int) (empty, final []byte, err error) {
	var entries []t2spec.CexAssetInfo
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, nil, fmt.Errorf("unmarshal CexAssetsInfo: %w", err)
	}

	cexAssetsInfo := make([]t2spec.CexAssetInfo, len(entries))
	for i := range entries {
		entry := entries[i]
		cexAssetsInfo[entry.Index] = entry
		if entry.TotalEquity < entry.TotalDebt {
			fmt.Printf("warning: %s asset equity %d less than debt %d (allowed by model; check distribution)\n",
				entry.Symbol, entry.TotalEquity, entry.TotalDebt)
		}
	}

	emptyCexAssetsInfo := make([]t2spec.CexAssetInfo, len(cexAssetsInfo))
	copy(emptyCexAssetsInfo, cexAssetsInfo)
	for i := range emptyCexAssetsInfo {
		emptyCexAssetsInfo[i].TotalEquity = 0
		emptyCexAssetsInfo[i].TotalDebt = 0
		emptyCexAssetsInfo[i].Collateral = 0
	}
	empty = ComputeCexAssetsCommitment(emptyCexAssetsInfo, capacity)
	final = ComputeCexAssetsCommitment(cexAssetsInfo, capacity)
	return empty, final, nil
}

// NewVerifyPublicWitness builds the public-only witness cmd/verifier
// feeds to groth16.Verify against the T2 circuit's verifying key.
func NewVerifyPublicWitness(batchCommitment []byte) (witness.Witness, error) {
	verifyCircuit := t2circuit.NewVerifyBatchCreateUserCircuit(batchCommitment)
	w, err := frontend.NewWitness(verifyCircuit, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		return nil, fmt.Errorf("public witness: %w", err)
	}
	return w, nil
}

// VerifyUserInclusion decodes a T2-typed UserConfig from the supplied
// bytes, recomputes the universal 5-input leaf hash, and checks Merkle
// inclusion against the embedded root. The caller (cmd/verifier) reads
// the bytes via a vfs.ByteSource; this runner stays IO-free.
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
	accountHash := poseidon.PoseidonBytes(
		accountIDHash,
		userConfig.TotalEquity.Bytes(),
		userConfig.TotalDebt.Bytes(),
		userConfig.TotalCollateral.Bytes(),
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
