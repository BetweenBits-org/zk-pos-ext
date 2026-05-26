package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	pconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/cmd/prover/config"
)

// TestLoadConfig_RoundTrip confirms the prover's config loader
// reconstructs the deployment tier/key arrays in order. Drift in
// AssetsCountTiers vs ZkKeyName indices would silently load the
// wrong .vk for a batch — the main() length check catches mismatched
// arrays but not reordering.
func TestLoadConfig_RoundTrip(t *testing.T) {
	src := pconfig.Config{
		MysqlDataSource:  "user:pass@tcp(host)/db",
		DbSuffix:         "_test",
		ZkKeyName:        []string{"keys/zkpor.tier_3bucket.50_700", "keys/zkpor.tier_3bucket.500_92"},
		AssetsCountTiers: []int{50, 500},
	}
	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := loadConfig(path)
	if got.MysqlDataSource != src.MysqlDataSource || got.DbSuffix != src.DbSuffix {
		t.Fatalf("scalar mismatch: got=%+v want=%+v", got, src)
	}
	if len(got.AssetsCountTiers) != 2 || got.AssetsCountTiers[0] != 50 || got.AssetsCountTiers[1] != 500 {
		t.Fatalf("AssetsCountTiers = %v, want [50 500]", got.AssetsCountTiers)
	}
	if len(got.ZkKeyName) != 2 || got.ZkKeyName[0] != src.ZkKeyName[0] || got.ZkKeyName[1] != src.ZkKeyName[1] {
		t.Fatalf("ZkKeyName mismatch: got=%v want=%v", got.ZkKeyName, src.ZkKeyName)
	}
}

// TestProofMetadataJSONShape locks the [][]byte → JSON → []string
// pipe the prover-to-verifier contract rides. The DB column is the
// JSON string of [][]byte (two entries: before, after); the verifier
// reads back into []string (each entry base64-decoded into the
// commitment bytes). A drift here breaks every proof in the table.
func TestProofMetadataJSONShape(t *testing.T) {
	before := []byte{0xde, 0xad, 0xbe, 0xef}
	after := []byte{0x01, 0x23, 0x45, 0x67}

	raw, err := json.Marshal([][]byte{before, after})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Round-trip as [][]byte: prover writes, dbtool reads.
	var asBytes [][]byte
	if err := json.Unmarshal(raw, &asBytes); err != nil {
		t.Fatalf("unmarshal [][]byte: %v", err)
	}
	if !bytes.Equal(asBytes[0], before) || !bytes.Equal(asBytes[1], after) {
		t.Fatalf("[][]byte round-trip mismatch")
	}

	// Verifier's view: []string of base64 entries.
	var asStrings []string
	if err := json.Unmarshal(raw, &asStrings); err != nil {
		t.Fatalf("unmarshal []string: %v", err)
	}
	if asStrings[0] != base64.StdEncoding.EncodeToString(before) {
		t.Fatalf("[]string[0] = %q, want %q", asStrings[0], base64.StdEncoding.EncodeToString(before))
	}
	if asStrings[1] != base64.StdEncoding.EncodeToString(after) {
		t.Fatalf("[]string[1] = %q, want %q", asStrings[1], base64.StdEncoding.EncodeToString(after))
	}
}
