package host

import (
	"math/big"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	t1spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/spec"
)

// UserConfig is the per-account inclusion-proof artifact for the
// t1_simple_margin model. The userproof service writes one per user
// (embedded as JSON in the userproof DB row's Config column); the
// verifier's -user mode reads it from disk and recomputes the leaf
// to check inclusion against Root.
//
// Compared to t4_tiered_haircut_margin_3pool/host.UserConfig: drops
// TotalCollateral (T1 has no risk-weighted collateral). TotalDebt is
// present so spot-vs-margin customers share one config schema —
// spot writers supply TotalDebt = 0. Root + AccountIdHash stay hex-
// encoded 32-byte values.
type UserConfig struct {
	AccountIndex  uint32
	AccountIdHash string
	TotalEquity   *big.Int
	TotalDebt     *big.Int
	Assets        []t1spec.AccountAsset
	Root          string
	Proof         [][]byte
}

// AccountLeafHash returns the SMT leaf value for one user account in
// the t1_simple_margin model:
//
//	Poseidon(AccountID, TotalEquity, TotalDebt, 0, AssetsCommitment)
//
// AssetsCommitment is produced by ComputeUserAssetsCommitment with
// the same assetCountTiers — the padded tier the user's asset slice
// was sized to. Position 3 (TotalCollateral) is fixed zero so the
// universal core/tree empty-leaf hash (Poseidon(0,0,0,0,0)) applies
// unchanged for untouched account slots — matches the in-circuit
// account-leaf shape in t1_simple_margin/circuit. PoseidonBytes
// converts nil byte slices to fr.Element{0,0,0,0}, so passing nil for
// position 3 is correct and allocation-free.
//
// Spot customers always have TotalDebt = 0, so slot 2 of the leaf
// hash is effectively unused for them — same byte output as the
// historical "spot-only" leaf without Debt.
func AccountLeafHash(account *t1spec.AccountInfo, assetCountTiers []int) []byte {
	assetsCommitment := ComputeUserAssetsCommitment(account.Assets, assetCountTiers)
	var debtBytes []byte
	if account.TotalDebt != nil {
		debtBytes = account.TotalDebt.Bytes()
	}
	return corehost.AccountLeafHash(
		account.AccountID,
		account.TotalEquity.Bytes(),
		debtBytes,
		nil, // T1 has no risk-weighted collateral
		assetsCommitment,
	)
}

// PaddingAccounts pads a per-tier account slice up to a whole number
// of batches (size usersPerBatch). Padding entries carry zero
// equity / zero debt and `assetKey` zero AccountAssets at indices
// [0..assetKey); their AccountIndex is assigned sequentially starting
// from paddingStartIndex. Returns the new paddingStartIndex (advanced
// by the number of padding rows appended) and the extended account
// slice.
//
// Mirrors t4_tiered_haircut_margin_3pool/host.PaddingAccounts in shape;
// T1 drops TotalCollateral (no risk-weighted collateral). TotalDebt
// is explicitly zeroed via big.Int{0} so AccountLeafHash doesn't
// dereference a nil pointer.
func PaddingAccounts(
	accounts []t1spec.AccountInfo,
	assetKey int,
	paddingStartIndex int,
	usersPerBatch int,
) (int, []t1spec.AccountInfo) {
	batchCounts := (len(accounts) + usersPerBatch - 1) / usersPerBatch
	paddingAccountCounts := batchCounts*usersPerBatch - len(accounts)
	for range paddingAccountCounts {
		assets := make([]t1spec.AccountAsset, assetKey)
		for j := range assetKey {
			assets[j] = t1spec.AccountAsset{Index: uint16(j)}
		}
		accounts = append(accounts, t1spec.AccountInfo{
			AccountIndex: uint32(paddingStartIndex),
			TotalEquity:  new(big.Int).SetInt64(0),
			TotalDebt:    new(big.Int).SetInt64(0),
			Assets:       assets,
		})
		paddingStartIndex++
	}
	return paddingStartIndex, accounts
}
