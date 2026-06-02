package spec

import corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"

// CreateUserOperation is the witness-side per-user batch entry for
// the t3 model.
type CreateUserOperation struct {
	BeforeAccountTreeRoot []byte
	AfterAccountTreeRoot  []byte
	Assets                []AccountAsset
	AccountIndex          uint32
	AccountIDHash         []byte
	AccountProof          [corespec.AccountTreeDepth][]byte
}

// BatchCreateUserWitness is the snapshot-shaped witness for one batch
// proof under the t3 model.
type BatchCreateUserWitness struct {
	BatchCommitment           []byte
	BeforeAccountTreeRoot     []byte
	AfterAccountTreeRoot      []byte
	BeforeCEXAssetsCommitment []byte
	AfterCEXAssetsCommitment  []byte

	BeforeCexAssets []CexAssetInfo
	CreateUserOps   []CreateUserOperation
}

// IsAccountAssetEmpty reports whether the per-user, per-asset 4-tuple
// has no balances of any kind.
func IsAccountAssetEmpty(a *AccountAsset) bool {
	return a.Equity == 0 && a.Debt == 0 && a.Collateral == 0
}

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
// be sorted ascending.
func PickAssetCountTier(count int, tiers []int) int {
	for _, v := range tiers {
		if count <= v {
			return v
		}
	}
	return 0
}
