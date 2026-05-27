package host

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	t3circuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t3_tiered_haircut_margin_1pool/circuit"
	t3spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t3_tiered_haircut_margin_1pool/spec"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
)

// BuildCexCommitments parses the model-typed CexAssetsInfo from raw JSON
// and computes the empty + final CEX commitments. T3 zeros the single
// Collateral aggregate (single pool) for the empty commitment; the tier
// ratios describe the asset's risk profile and remain.
func BuildCexCommitments(raw json.RawMessage, capacity int) (empty, final []byte, err error) {
	var entries []t3spec.CexAssetInfo
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, nil, fmt.Errorf("unmarshal CexAssetsInfo: %w", err)
	}

	cexAssetsInfo := make([]t3spec.CexAssetInfo, len(entries))
	for i := range entries {
		entry := entries[i]
		cexAssetsInfo[entry.Index] = entry
		if entry.TotalEquity < entry.TotalDebt {
			fmt.Printf("warning: %s asset equity %d less than debt %d (allowed by model; check distribution)\n",
				entry.Symbol, entry.TotalEquity, entry.TotalDebt)
		}
	}

	emptyCexAssetsInfo := make([]t3spec.CexAssetInfo, len(cexAssetsInfo))
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
// feeds to groth16.Verify against the T3 circuit's verifying key.
func NewVerifyPublicWitness(batchCommitment []byte) (witness.Witness, error) {
	verifyCircuit := t3circuit.NewVerifyBatchCreateUserCircuit(batchCommitment)
	w, err := frontend.NewWitness(verifyCircuit, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		return nil, fmt.Errorf("public witness: %w", err)
	}
	return w, nil
}

// VerifyUserInclusion reads a T3-typed UserConfig, recomputes the
// universal 5-input leaf hash, and checks Merkle inclusion against the
// embedded root.
func VerifyUserInclusion(plan *corehost.VerifierPlan, userConfigPath string) error {
	content, err := os.ReadFile(userConfigPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", userConfigPath, err)
	}
	userConfig := &UserConfig{}
	if err := json.Unmarshal(content, userConfig); err != nil {
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
