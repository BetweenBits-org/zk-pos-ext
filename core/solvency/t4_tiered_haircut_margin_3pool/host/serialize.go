package host

import (
	"bytes"
	"encoding/gob"
	"fmt"

	t4spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	"github.com/klauspost/compress/s2"
)

// EncodeBatchWitness serialises a BatchCreateUserWitness to the
// witness service's on-wire form: gob-encoded then s2-compressed.
// Storage callers typically base64 the result for DB string columns.
// Mirrors the legacy witness service's inline encode (gob → s2);
// pairs with DecodeBatchWitness for the prover's read path.
func EncodeBatchWitness(w *t4spec.BatchCreateUserWitness) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(w); err != nil {
		return nil, fmt.Errorf("encode batch witness: %w", err)
	}
	return s2.Encode(nil, buf.Bytes()), nil
}

// DecodeBatchWitness reverses EncodeBatchWitness: s2-decompresses then
// gob-decodes the input. CreateUserOps[*].Assets in the encoded form
// only carries non-empty user assets (sparse); this helper expands it
// back to a dense capacity-length slice so the prover can feed it
// straight into SetBatchCreateUserCircuitWitness.
//
// Capacity is self-described by the encoded witness — len(w.BeforeCexAssets)
// is the per-deployment asset capacity baked into the trusted setup
// at keygen time. Mirrors legacy src/utils.DecodeBatchWitness, but no
// longer reads a process-global constant.
func DecodeBatchWitness(data []byte) (*t4spec.BatchCreateUserWitness, error) {
	uncompressed, err := s2.Decode(nil, data)
	if err != nil {
		return nil, fmt.Errorf("decompress batch witness: %w", err)
	}
	var w t4spec.BatchCreateUserWitness
	if err := gob.NewDecoder(bytes.NewReader(uncompressed)).Decode(&w); err != nil {
		return nil, fmt.Errorf("decode batch witness: %w", err)
	}
	capacity := len(w.BeforeCexAssets)
	if capacity == 0 {
		return nil, fmt.Errorf("decode batch witness: BeforeCexAssets is empty (capacity unknown)")
	}
	for i := range w.CreateUserOps {
		dense := make([]t4spec.AccountAsset, capacity)
		for p := range capacity {
			dense[p] = t4spec.AccountAsset{Index: uint16(p)}
		}
		for _, a := range w.CreateUserOps[i].Assets {
			dense[a.Index] = a
		}
		w.CreateUserOps[i].Assets = dense
	}
	return &w, nil
}
