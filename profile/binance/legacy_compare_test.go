package binance

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	modelspec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/tier_3bucket/spec"
	legacy "github.com/binance/zkmerkle-proof-of-solvency/src/utils"
)

// legacySamplePath is the path from this test file (inside
// zkpor/profile/binance/) to the legacy `src/sampledata` directory.
// The test skips when that directory is missing so a future
// stand-alone zkpor extraction does not fail this corpus check —
// only the in-tree run sees it.
const legacySamplePath = "../../../src/sampledata"

// TestLegacyCompare_SampleDataAccountIDs is the G1 byte-equivalence
// regression gate at the snapshot layer. The test feeds the legacy
// `src/sampledata/sample_users0.csv` corpus (100 rows) through both
// pipelines:
//
//   - legacy: utils.ParseAssetIndexFromUserFile + ParseCexAssetInfoFromFile
//     + ReadUserDataFromCsvFile, with the resulting tier-bucketed
//     map flattened back into source row order via AccountIndex.
//   - zkpor: profile/binance.csvSnapshot.AccountStream, draining the
//     yielded channel in order.
//
// AccountIDs of corresponding rows MUST be byte-equal. Both pipelines
// apply the same bn254 fr.Element SetBytes→Marshal round-trip (legacy
// src/utils/utils.go:553, zkpor parseAccountRow); this test confirms
// that the round-trip lands at the same bytes across a real-data
// corpus rather than only on the synthetic above-modulus fixture used
// by TestParseAccountRow_NormalizesAccountID.
//
// The corpus is small (100 rows) but the IDs are 32-byte hex of
// incrementing small integers — they all sit below the modulus, so
// the round-trip is a no-op on every row. The synthetic unit test
// covers the actual reduction path; this corpus test guards against
// any silent divergence in *how* the two pipelines apply the formula.
//
// Skipped under -short to keep the inner-loop fast.
func TestLegacyCompare_SampleDataAccountIDs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping legacy-corpus compare; rerun without -short")
	}
	cexSrc := filepath.Join(legacySamplePath, "cex_assets_info.csv")
	userSrc := filepath.Join(legacySamplePath, "sample_users0.csv")
	if _, err := os.Stat(cexSrc); err != nil {
		t.Skipf("legacy sampledata missing at %q: %v", legacySamplePath, err)
	}

	dir := t.TempDir()
	cexDst := filepath.Join(dir, "cex_assets_info.csv")
	userDst := filepath.Join(dir, "sample_users0.csv")
	copyFile(t, cexSrc, cexDst)
	copyFile(t, userSrc, userDst)

	legacyAccounts := readLegacyAccounts(t, cexDst, userDst)
	zkporAccounts := readZkporAccounts(t, dir)

	if len(legacyAccounts) != len(zkporAccounts) {
		t.Fatalf("account count diverged: legacy=%d zkpor=%d", len(legacyAccounts), len(zkporAccounts))
	}
	if len(legacyAccounts) == 0 {
		t.Fatalf("test premise broken: sample corpus yielded zero valid accounts")
	}
	for i := range legacyAccounts {
		if legacyAccounts[i].AccountIndex != zkporAccounts[i].AccountIndex {
			t.Fatalf("row %d AccountIndex diverged: legacy=%d zkpor=%d",
				i, legacyAccounts[i].AccountIndex, zkporAccounts[i].AccountIndex)
		}
		if !bytes.Equal(legacyAccounts[i].AccountId, zkporAccounts[i].AccountID) {
			t.Fatalf("row %d (AccountIndex %d) AccountID diverged:\n  legacy=%x\n  zkpor =%x",
				i, legacyAccounts[i].AccountIndex,
				legacyAccounts[i].AccountId, zkporAccounts[i].AccountID)
		}
	}
	t.Logf("compared %d accounts across legacy and zkpor — all AccountID bytes equal", len(legacyAccounts))
}

// readLegacyAccounts drives the three legacy ETL entry points against
// the supplied cex / user file pair and returns the accounts flattened
// into source row order. Legacy's map keying by AssetCountsTiers
// scrambles cross-tier order; sorting by AccountIndex restores the
// monotonic, dense numbering legacy assigns only to valid rows.
func readLegacyAccounts(t *testing.T, cexPath, userPath string) []legacy.AccountInfo {
	t.Helper()
	assetOrder, err := legacy.ParseAssetIndexFromUserFile(userPath)
	if err != nil {
		t.Fatalf("legacy ParseAssetIndexFromUserFile: %v", err)
	}
	cexInfo, err := legacy.ParseCexAssetInfoFromFile(cexPath, assetOrder)
	if err != nil {
		t.Fatalf("legacy ParseCexAssetInfoFromFile: %v", err)
	}
	bucketed, _, err := legacy.ReadUserDataFromCsvFile(userPath, cexInfo)
	if err != nil {
		t.Fatalf("legacy ReadUserDataFromCsvFile: %v", err)
	}
	var flat []legacy.AccountInfo
	for _, accs := range bucketed {
		flat = append(flat, accs...)
	}
	sort.Slice(flat, func(i, j int) bool {
		return flat[i].AccountIndex < flat[j].AccountIndex
	})
	return flat
}

// readZkporAccounts drains the zkpor csvSnapshot.AccountStream on the
// supplied user-data dir, returning accounts in emission order (which
// is the dense-by-validIndex order matching legacy's AccountIndex).
func readZkporAccounts(t *testing.T, dir string) []modelspec.AccountInfo {
	t.Helper()
	src := NewSnapshotCSV(SnapshotConfig{UserDataDir: dir, SnapshotID: "test"})
	ch, err := src.AccountStream(context.Background())
	if err != nil {
		t.Fatalf("zkpor AccountStream: %v", err)
	}
	var out []modelspec.AccountInfo
	for a := range ch {
		out = append(out, a)
	}
	if c := src.InvalidCount(); c != 0 {
		t.Logf("zkpor InvalidCount = %d (legacy parity check tolerates skipped rows on both sides equally)", c)
	}
	return out
}

// copyFile copies src to dst, failing the test on any IO error. Used
// to materialise a clean temp-dir fixture from the legacy sampledata
// without modifying the source tree.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatalf("copy %s → %s: %v", src, dst, err)
	}
}
