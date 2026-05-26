package host

import (
	"math/big"

	spotspec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/spot_simple/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// UserConfig is the per-account inclusion-proof artifact for the
// spot_simple model. The userproof service writes one per user
// (embedded as JSON in the userproof DB row's Config column); the
// verifier's -user mode reads it from disk and recomputes the leaf
// to check inclusion against Root.
//
// Compared to tier_3bucket/host.UserConfig: drops TotalDebt +
// TotalCollateral (spot users have no liabilities). Assets is the
// spot 1-tuple. Root + AccountIdHash stay hex-encoded 32-byte values.
type UserConfig struct {
	AccountIndex  uint32
	AccountIdHash string
	TotalEquity   *big.Int
	Assets        []spotspec.AccountAsset
	Root          string
	Proof         [][]byte
}

// AccountLeafHash returns the SMT leaf value for one user account in
// the spot_simple model:
//
//	Poseidon(AccountID, TotalEquity, 0, 0, AssetsCommitment)
//
// AssetsCommitment is produced by ComputeUserAssetsCommitment with
// the same assetCountTiers — the padded tier the user's asset slice
// was sized to. Positions 3 and 4 are fixed zeros so the universal
// core/tree empty-leaf hash (Poseidon(0,0,0,0,0)) applies unchanged
// for untouched account slots — matches the in-circuit
// account-leaf shape in spot_simple/circuit. PoseidonBytes converts
// nil byte slices to fr.Element{0,0,0,0}, so passing nil for the
// fixed-zero positions is correct and allocation-free.
func AccountLeafHash(account *spotspec.AccountInfo, assetCountTiers []int) []byte {
	assetsCommitment := ComputeUserAssetsCommitment(account.Assets, assetCountTiers)
	return poseidon.PoseidonBytes(
		account.AccountID,
		account.TotalEquity.Bytes(),
		nil,
		nil,
		assetsCommitment,
	)
}

// PaddingAccounts pads a per-tier account slice up to a whole number
// of batches (size usersPerBatch). Padding entries carry zero equity
// and `assetKey` zero AccountAssets at indices [0..assetKey); their
// AccountIndex is assigned sequentially starting from paddingStartIndex.
// Returns the new paddingStartIndex (advanced by the number of padding
// rows appended) and the extended account slice.
//
// Mirrors tier_3bucket/host.PaddingAccounts in shape; spot-typed
// AccountInfo/AccountAsset and drops the debt/collateral big.Int
// initialisation.
func PaddingAccounts(
	accounts []spotspec.AccountInfo,
	assetKey int,
	paddingStartIndex int,
	usersPerBatch int,
) (int, []spotspec.AccountInfo) {
	batchCounts := (len(accounts) + usersPerBatch - 1) / usersPerBatch
	paddingAccountCounts := batchCounts*usersPerBatch - len(accounts)
	for range paddingAccountCounts {
		assets := make([]spotspec.AccountAsset, assetKey)
		for j := range assetKey {
			assets[j] = spotspec.AccountAsset{Index: uint16(j)}
		}
		accounts = append(accounts, spotspec.AccountInfo{
			AccountIndex: uint32(paddingStartIndex),
			TotalEquity:  new(big.Int).SetInt64(0),
			Assets:       assets,
		})
		paddingStartIndex++
	}
	return paddingStartIndex, accounts
}
