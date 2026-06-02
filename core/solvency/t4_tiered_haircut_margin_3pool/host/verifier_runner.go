package host

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
	t4circuit "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t4_tiered_haircut_margin_3pool/circuit"
	t4spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
)

// BuildCexCommitments parses the model-typed CexAssetsInfo from the raw
// JSON the verifier config carries, then computes the empty + final
// CEX commitments cmd/verifier needs for the batch chain check.
//
// Per-asset equity < debt is *allowed* in t4_tiered_haircut_margin_3pool
// (account-level invariants only), so violations surface as a warning
// instead of an error.
func BuildCexCommitments(raw json.RawMessage, capacity int) (empty, final []byte, err error) {
	var entries []t4spec.CexAssetInfo
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, nil, fmt.Errorf("unmarshal CexAssetsInfo: %w", err)
	}

	cexAssetsInfo := make([]t4spec.CexAssetInfo, len(entries))
	for i := range entries {
		entry := entries[i]
		cexAssetsInfo[entry.Index] = entry
		if entry.TotalEquity < entry.TotalDebt {
			fmt.Printf("warning: %s asset equity %d less than debt %d (allowed by model; check distribution)\n",
				entry.Symbol, entry.TotalEquity, entry.TotalDebt)
		}
	}

	emptyCexAssetsInfo := make([]t4spec.CexAssetInfo, len(cexAssetsInfo))
	copy(emptyCexAssetsInfo, cexAssetsInfo)
	for i := range emptyCexAssetsInfo {
		emptyCexAssetsInfo[i].TotalDebt = 0
		emptyCexAssetsInfo[i].TotalEquity = 0
		emptyCexAssetsInfo[i].LoanCollateral = 0
		emptyCexAssetsInfo[i].MarginCollateral = 0
		emptyCexAssetsInfo[i].PortfolioMarginCollateral = 0
	}
	empty = ComputeCexAssetsCommitment(emptyCexAssetsInfo, capacity)
	final = ComputeCexAssetsCommitment(cexAssetsInfo, capacity)
	return empty, final, nil
}

// NewVerifyPublicWitness builds the public-only witness cmd/verifier
// feeds to groth16.Verify. The witness shape is fixed by the model's
// circuit definition; only the BatchCommitment public input varies.
func NewVerifyPublicWitness(batchCommitment []byte) (witness.Witness, error) {
	verifyCircuit := t4circuit.NewVerifyBatchCreateUserCircuit(batchCommitment)
	w, err := frontend.NewWitness(verifyCircuit, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		return nil, fmt.Errorf("public witness: %w", err)
	}
	return w, nil
}

// VerifyUserInclusion decodes a model-typed UserConfig from the supplied
// bytes, recomputes the SMT leaf hash, and checks the Merkle proof
// against the root carried in the config. Prints the result. The caller
// (cmd/verifier) reads the bytes via a vfs.ByteSource; this runner stays
// IO-free.
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
