package store

import (
	"errors"
	"testing"

	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
)

// TestBatchWitnessDTORoundTrip locks the on-wire invariant that the
// adapter's DTO->row->DTO mapping copies WitnessData (and the other
// fields) VERBATIM — no re-base64, trim, or re-marshal. The payload is a
// realistic base64(s2(gob(...))) shape including '+', '/', '=' and a
// trailing newline so any accidental normalisation would be caught.
func TestBatchWitnessDTORoundTrip(t *testing.T) {
	const payload = "AbC+/9=\nzZ9876543210\t==" // deliberately gnarly
	in := corehost.BatchWitnessDTO{
		Height:      42,
		WitnessData: payload,
		Status:      StatusPublished,
	}

	row := batchWitnessFromDTO(in)
	if row.WitnessData != payload {
		t.Fatalf("DTO->row mutated WitnessData: got %q want %q", row.WitnessData, payload)
	}

	out := batchWitnessToDTO(&row)
	if out != in {
		t.Fatalf("round-trip changed DTO: got %+v want %+v", out, in)
	}
	if out.WitnessData != payload {
		t.Fatalf("round-trip changed WitnessData bytes: got %q want %q", out.WitnessData, payload)
	}
	if len(out.WitnessData) != len(payload) {
		t.Fatalf("round-trip changed WitnessData length: got %d want %d", len(out.WitnessData), len(payload))
	}
}

// TestProofDTORoundTrip locks the verbatim field copy for the proof
// adapter, including the JSON-array string payloads.
func TestProofDTORoundTrip(t *testing.T) {
	in := corehost.ProofDTO{
		ProofInfo:               "cHJvb2Y+/w==",
		CexAssetListCommitments: `["YQ==","Yg=="]`,
		AccountTreeRoots:        `["cm9vdDA=","cm9vdDE="]`,
		BatchCommitment:         "Y29tbWl0",
		AssetsCount:             7,
		BatchNumber:             99,
	}
	row := proofFromDTO(&in)
	out := proofToDTO(&row)
	if out != in {
		t.Fatalf("proof round-trip changed DTO: got %+v want %+v", out, in)
	}
}

// TestUserProofDTORoundTrip locks the verbatim field copy for the
// user-proof adapter across all string payloads.
func TestUserProofDTORoundTrip(t *testing.T) {
	in := corehost.UserProofDTO{
		AccountIndex:    17,
		AccountId:       "0xdeadbeef",
		AccountLeafHash: "bGVhZitoYXNo",
		TotalEquity:     "123456789012345678901234567890",
		TotalDebt:       "0",
		TotalCollateral: "987654321",
		Assets:          `[{"Index":1,"Equity":"10"}]`,
		Proof:           "cHJvb2Y/+w==",
		Config:          `{"foo":"bar"}`,
	}
	row := userProofFromDTO(in)
	out := userProofToDTO(&row)
	if out != in {
		t.Fatalf("userproof round-trip changed DTO: got %+v want %+v", out, in)
	}
}

// TestNotFoundRemap confirms the adapter sentinel translation: the inner
// store.ErrNotFound becomes corehost.ErrNotFound, while other errors pass
// through unchanged and nil stays nil.
func TestNotFoundRemap(t *testing.T) {
	if got := notFound(ErrNotFound); !errors.Is(got, corehost.ErrNotFound) {
		t.Fatalf("notFound(store.ErrNotFound) = %v, want corehost.ErrNotFound", got)
	}
	other := errors.New("backing down")
	if got := notFound(other); got != other {
		t.Fatalf("notFound(other) = %v, want pass-through", got)
	}
	if got := notFound(nil); got != nil {
		t.Fatalf("notFound(nil) = %v, want nil", got)
	}
}
