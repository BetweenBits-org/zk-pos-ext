package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	pconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/cmd/prover/config"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
)

// TestLoadConfig_RoundTrip confirms the slim R8-C/3 config carries
// the DB-connection fields cleanly. AssetsCountTiers + ZkKeyName are
// no longer config concerns — they're derived from profile.toml in
// buildResolved.
func TestLoadConfig_RoundTrip(t *testing.T) {
	src := pconfig.Config{
		MysqlDataSource: "user:pass@tcp(host)/db",
		DbSuffix:        "_test",
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
}

// TestBuildResolved_DerivesTiersAndStems locks the toml → (tier, stem)
// derivation the prover relies on for snark-param lookup. A mis-ordered
// stem slice would silently load the wrong .vk; this test pins the
// ascending-tier order and the StandardKeyName template.
func TestBuildResolved_DerivesTiersAndStems(t *testing.T) {
	t.Setenv("ZKPOR_BATCH_SHAPE_OVERRIDE", "") // never inherit; we want toml shapes
	os.Unsetenv("ZKPOR_BATCH_SHAPE_OVERRIDE")
	prof := &declarative.Profile{
		Profile: declarative.ProfileMeta{
			Name:          "test",
			Model:         "t4_tiered_haircut_margin_3pool",
			AssetCapacity: 500,
		},
		Identity:  declarative.Identity{Scheme: "passthrough_hex_bn254_reduced.v0"},
		Insolvent: declarative.Insolvent{Action: "drop_and_log.v0"},
		Snapshot:  declarative.Snapshot{SourceType: "binance_csv.v1"},
		BatchShapes: []declarative.BatchShape{
			{AssetCountTier: 500, UsersPerBatch: 92},
			{AssetCountTier: 50, UsersPerBatch: 700},
		},
		Pricing: declarative.Pricing{DefaultPriceScale: 1e8, DefaultBalanceScale: 1e8},
	}
	plan, err := buildResolved(prof, "/keys")
	if err != nil {
		t.Fatalf("buildResolved: %v", err)
	}
	wantTiers := []int{50, 500} // ascending
	wantStems := []string{
		"/keys/zkpor.t4_tiered_haircut_margin_3pool.50_700",
		"/keys/zkpor.t4_tiered_haircut_margin_3pool.500_92",
	}
	for i := range wantTiers {
		if plan.assetCountTiers[i] != wantTiers[i] {
			t.Errorf("tier[%d] = %d, want %d", i, plan.assetCountTiers[i], wantTiers[i])
		}
		if plan.zkKeyStems[i] != wantStems[i] {
			t.Errorf("stem[%d] = %q, want %q", i, plan.zkKeyStems[i], wantStems[i])
		}
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
