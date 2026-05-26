package spec

import corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"

// CreateUserOperation is the witness-side per-user batch entry for the
// t4_tiered_haircut_margin_3pool model. It carries the raw byte / uint64 values that the
// witness builder produces; SetBatchCreateUserCircuitWitness converts
// each operation into the gnark-Variable form consumed by the circuit.
//
// Mirrors utils.CreateUserOperation. AccountID is the raw account-id
// bytes; AccountIDHash is its 32-byte Poseidon hash. AccountProof has
// length corespec.AccountTreeDepth (sibling-per-level).
type CreateUserOperation struct {
	BeforeAccountTreeRoot []byte
	AfterAccountTreeRoot  []byte
	Assets                []AccountAsset
	AccountIndex          uint32
	AccountIDHash         []byte
	AccountProof          [corespec.AccountTreeDepth][]byte
}

// BatchCreateUserWitness is the snapshot-shaped witness for one batch
// proof under the t4_tiered_haircut_margin_3pool model. The prover service converts it
// into the in-circuit BatchCreateUserCircuit via
// SetBatchCreateUserCircuitWitness.
//
// Mirrors utils.BatchCreateUserWitness. All hash fields are 32-byte
// Poseidon outputs; BeforeCexAssets is indexed by AssetCatalog index.
type BatchCreateUserWitness struct {
	BatchCommitment           []byte
	BeforeAccountTreeRoot     []byte
	AfterAccountTreeRoot      []byte
	BeforeCEXAssetsCommitment []byte
	AfterCEXAssetsCommitment  []byte

	BeforeCexAssets []CexAssetInfo
	CreateUserOps   []CreateUserOperation
}

// IsAccountAssetEmpty reports whether the per-user, per-asset 5-tuple
// has no balances of any kind. Empty assets are skipped by the witness
// builder when sizing the in-circuit per-user asset slice.
func IsAccountAssetEmpty(a *AccountAsset) bool {
	return a.Debt == 0 && a.Equity == 0 && a.Margin == 0 && a.PortfolioMargin == 0 && a.Loan == 0
}

// CountNonEmptyAssets returns the number of non-empty AccountAssets in
// `assets`.
func CountNonEmptyAssets(assets []AccountAsset) int {
	n := 0
	for i := range assets {
		if !IsAccountAssetEmpty(&assets[i]) {
			n++
		}
	}
	return n
}

// PickAssetCountTier returns the smallest tier value in `tiers` that is
// >= count, or 0 if no tier in `tiers` accommodates the count. `tiers`
// MUST be sorted ascending. The result is the AssetCountTier dimension
// of the BatchShape the witness is being prepared for.
func PickAssetCountTier(count int, tiers []int) int {
	for _, v := range tiers {
		if count <= v {
			return v
		}
	}
	return 0
}
