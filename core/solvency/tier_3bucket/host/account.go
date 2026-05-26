package host

import (
	"math/big"

	tier3spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// AccountLeafHash returns the SMT leaf value for one user account:
// Poseidon(AccountID, TotalEquity, TotalDebt, TotalCollateral,
// AssetsCommitment). AssetsCommitment is produced by
// ComputeUserAssetsCommitment with the same assetCountTiers — the
// padded tier the user's asset slice was sized to. Byte-equivalent to
// legacy src/utils.AccountInfoToHash.
//
// The witness service stores this hash at account.AccountIndex in the
// SMT; the userproof service recomputes it for inclusion proofs; the
// verifier's -user mode recomputes it independently from the user's
// own config to check inclusion locally.
func AccountLeafHash(account *tier3spec.AccountInfo, assetCountTiers []int) []byte {
	assetsCommitment := ComputeUserAssetsCommitment(account.Assets, assetCountTiers)
	return poseidon.PoseidonBytes(
		account.AccountID,
		account.TotalEquity.Bytes(),
		account.TotalDebt.Bytes(),
		account.TotalCollateral.Bytes(),
		assetsCommitment,
	)
}

// PaddingAccounts pads a per-tier account slice up to a whole number
// of batches (size usersPerBatch). Padding entries carry zero balances
// and `assetKey` zero AccountAssets at indices [0..assetKey); their
// AccountIndex is assigned sequentially starting from paddingStartIndex.
// Returns the new paddingStartIndex (advanced by the number of padding
// rows appended) and the extended account slice.
//
// Byte-equivalent to legacy src/utils.PaddingAccounts. The usersPerBatch
// parameter is the value the legacy looked up in the module-global
// BatchCreateUserOpsCountsTiers[assetKey]; in zkpor it is supplied by
// the caller from the binance profile's BatchShapeProvider, removing
// the global dependency.
func PaddingAccounts(
	accounts []tier3spec.AccountInfo,
	assetKey int,
	paddingStartIndex int,
	usersPerBatch int,
) (int, []tier3spec.AccountInfo) {
	batchCounts := (len(accounts) + usersPerBatch - 1) / usersPerBatch
	paddingAccountCounts := batchCounts*usersPerBatch - len(accounts)
	for range paddingAccountCounts {
		assets := make([]tier3spec.AccountAsset, assetKey)
		for j := range assetKey {
			assets[j] = tier3spec.AccountAsset{Index: uint16(j)}
		}
		accounts = append(accounts, tier3spec.AccountInfo{
			AccountIndex:    uint32(paddingStartIndex),
			TotalEquity:     new(big.Int).SetInt64(0),
			TotalDebt:       new(big.Int).SetInt64(0),
			TotalCollateral: new(big.Int).SetInt64(0),
			Assets:          assets,
		})
		paddingStartIndex++
	}
	return paddingStartIndex, accounts
}
