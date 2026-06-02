package host

import (
	"math/big"

	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
	t2spec "github.com/BetweenBits-org/zk-pos-ext/core/solvency/t2_static_haircut_margin/spec"
)

// UserConfig is the per-account inclusion-proof artifact. The
// userproof service writes one per user (embedded as JSON in the
// userproof DB row's Config column); the verifier's -user mode reads
// it from disk and recomputes the leaf to check inclusion against
// Root.
//
// On-wire format is JSON. The Go field types are chosen to round-trip
// naturally:
//
//   - *big.Int values marshal as their decimal string via TextMarshaler
//     (matching legacy userproof);
//   - [][]byte values marshal as a JSON array of base64 strings
//     (Go's default), so the verifier reads them back as raw bytes
//     without an extra base64 decode step;
//   - Root and AccountIdHash are hex-encoded 32-byte values.
//
// Mirrors legacy src/userproof/model.UserConfig. Shared between
// userproof (writer) and verifier (reader) so format drift between
// the two is impossible.
type UserConfig struct {
	AccountIndex    uint32
	AccountIdHash   string
	TotalEquity     *big.Int
	TotalDebt       *big.Int
	TotalCollateral *big.Int
	Assets          []t2spec.AccountAsset
	Root            string
	Proof           [][]byte
}

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
func AccountLeafHash(account *t2spec.AccountInfo, assetCountTiers []int) []byte {
	assetsCommitment := ComputeUserAssetsCommitment(account.Assets, assetCountTiers)
	return corehost.AccountLeafHash(
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
	accounts []t2spec.AccountInfo,
	assetKey int,
	paddingStartIndex int,
	usersPerBatch int,
) (int, []t2spec.AccountInfo) {
	batchCounts := (len(accounts) + usersPerBatch - 1) / usersPerBatch
	paddingAccountCounts := batchCounts*usersPerBatch - len(accounts)
	for range paddingAccountCounts {
		assets := make([]t2spec.AccountAsset, assetKey)
		for j := range assetKey {
			assets[j] = t2spec.AccountAsset{Index: uint16(j)}
		}
		accounts = append(accounts, t2spec.AccountInfo{
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
