package spec

import corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"

// CreateUserOperation is the witness-side per-user batch entry for the
// t1_simple_margin model. AccountID is the raw account-id bytes;
// AccountIDHash is its bn254-fr-reduced 32-byte form (matching
// AccountIDProvider.Scheme()). AccountProof has length
// corespec.AccountTreeDepth (sibling-per-level).
type CreateUserOperation struct {
	BeforeAccountTreeRoot []byte
	AfterAccountTreeRoot  []byte
	Assets                []AccountAsset
	AccountIndex          uint32
	AccountIDHash         []byte
	AccountProof          [corespec.AccountTreeDepth][]byte
}

// BatchCreateUserWitness is the snapshot-shaped witness for one batch
// proof under the t1_simple_margin model. The prover service converts it
// into the in-circuit BatchCreateUserCircuit via
// SetBatchCreateUserCircuitWitness (defined in the circuit package).
//
// All hash fields are 32-byte Poseidon outputs. BeforeCexAssets has
// length == deployment capacity (snapshot pads with reserved entries).
type BatchCreateUserWitness struct {
	BatchCommitment           []byte
	BeforeAccountTreeRoot     []byte
	AfterAccountTreeRoot      []byte
	BeforeCEXAssetsCommitment []byte
	AfterCEXAssetsCommitment  []byte

	BeforeCexAssets []CexAssetInfo
	CreateUserOps   []CreateUserOperation
}

// IsAccountAssetEmpty reports whether the per-user, per-asset record
// has zero equity. Empty assets are skipped by the witness builder
// when sizing the in-circuit per-user asset slice.
func IsAccountAssetEmpty(a *AccountAsset) bool { return a.Equity == 0 }

// CountNonEmptyAssets returns the number of non-empty AccountAssets.
func CountNonEmptyAssets(assets []AccountAsset) int {
	n := 0
	for i := range assets {
		if !IsAccountAssetEmpty(&assets[i]) {
			n++
		}
	}
	return n
}

// PickAssetCountTier returns the smallest tier value in `tiers` that
// is >= count, or 0 if no tier accommodates the count. `tiers` MUST
// be sorted ascending. Matches the t4_tiered_haircut_margin_3pool helper of the same
// name — the algorithm is universal across solvency models.
func PickAssetCountTier(count int, tiers []int) int {
	for _, v := range tiers {
		if count <= v {
			return v
		}
	}
	return 0
}
