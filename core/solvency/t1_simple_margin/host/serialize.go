package host

import (
	"bytes"
	"encoding/gob"
	"fmt"

	t1spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/spec"
	"github.com/klauspost/compress/s2"
)

// EncodeBatchWitness serialises a BatchCreateUserWitness to the
// witness service's on-wire form: gob-encoded then s2-compressed.
// Storage callers typically base64 the result for DB string columns.
// Same wire shape as t4_tiered_haircut_margin_3pool/host.EncodeBatchWitness — gob + s2.
func EncodeBatchWitness(w *t1spec.BatchCreateUserWitness) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(w); err != nil {
		return nil, fmt.Errorf("encode batch witness: %w", err)
	}
	return s2.Encode(nil, buf.Bytes()), nil
}

// DecodeBatchWitness reverses EncodeBatchWitness. CreateUserOps[*].Assets
// in the encoded form only carries non-empty user assets (sparse); this
// helper expands it back to a dense capacity-length slice so the prover
// can feed it straight into SetBatchCreateUserCircuitWitness.
//
// Capacity is self-described by the encoded witness — len(w.BeforeCexAssets)
// is the per-deployment asset capacity baked into the trusted setup at
// keygen time. Mirrors t4_tiered_haircut_margin_3pool/host.DecodeBatchWitness; the
// per-asset 1-tuple shape is the model-specific difference.
func DecodeBatchWitness(data []byte) (*t1spec.BatchCreateUserWitness, error) {
	uncompressed, err := s2.Decode(nil, data)
	if err != nil {
		return nil, fmt.Errorf("decompress batch witness: %w", err)
	}
	var w t1spec.BatchCreateUserWitness
	if err := gob.NewDecoder(bytes.NewReader(uncompressed)).Decode(&w); err != nil {
		return nil, fmt.Errorf("decode batch witness: %w", err)
	}
	capacity := len(w.BeforeCexAssets)
	if capacity == 0 {
		return nil, fmt.Errorf("decode batch witness: BeforeCexAssets is empty (capacity unknown)")
	}
	for i := range w.CreateUserOps {
		dense := make([]t1spec.AccountAsset, capacity)
		for p := range capacity {
			dense[p] = t1spec.AccountAsset{Index: uint16(p)}
		}
		for _, a := range w.CreateUserOps[i].Assets {
			dense[a.Index] = a
		}
		w.CreateUserOps[i].Assets = dense
	}
	return &w, nil
}
