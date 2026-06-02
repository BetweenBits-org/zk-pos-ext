// Lazy snark-params loader. The prover keeps one set of (r1cs, pk, vk)
// in memory at a time and only reloads when the requested tier differs
// from the cached one — matching legacy LoadSnarkParamsOnce behaviour.
// This pays off because the witness builder emits batches in
// ascending tier order, so most consecutive batches hit the cache.
//
// R12-EF: keys are streamed through an injected vfs.KeyOpener instead of
// os.ReadFile(stem+ext). The opener joins the logical stem against its
// directory and returns a stream; gnark reads straight from the stream
// (UnsafeReadFrom is kept for the proving key, with runtime.GC after the
// large allocations, exactly as before).

package prover

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/BetweenBits-org/zk-pos-ext/core/io/vfs"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
)

// loadSnarkParams is the lazy-load cache: reload r1cs/pk/vk only when
// the requested tier differs from the cached one. Each artifact is
// streamed through keys.Open(ctx, stem, ext); the stream is closed
// before the next is opened.
func loadSnarkParams(ctx context.Context, params *snarkParams, plan *resolved, keys vfs.KeyOpener, targetTier int) error {
	if params.tier == targetTier && params.r1cs != nil {
		return nil
	}

	index := -1
	for i, v := range plan.assetCountTiers {
		if v == targetTier {
			index = i
			break
		}
	}
	if index == -1 {
		return fmt.Errorf("assets count tier %d not present in profile (resolved=%v)", targetTier, plan.assetCountTiers)
	}
	stem := plan.zkKeyStems[index]

	loadStart := time.Now()
	fmt.Println("loading r1cs of", targetTier, "assets")
	r1cs := groth16.NewCS(ecc.BN254)
	if err := func() error {
		r, err := keys.Open(ctx, stem, ".r1cs")
		if err != nil {
			return fmt.Errorf("read r1cs: %w", err)
		}
		defer r.Close()
		if _, err := r1cs.ReadFrom(r); err != nil {
			return fmt.Errorf("parse r1cs: %w", err)
		}
		return nil
	}(); err != nil {
		return err
	}
	runtime.GC()
	fmt.Println("r1cs loaded in", time.Since(loadStart))

	loadStart = time.Now()
	fmt.Println("loading proving key of", targetTier, "assets")
	pk := groth16.NewProvingKey(ecc.BN254)
	if err := func() error {
		r, err := keys.Open(ctx, stem, ".pk")
		if err != nil {
			return fmt.Errorf("read pk: %w", err)
		}
		defer r.Close()
		if _, err := pk.UnsafeReadFrom(r); err != nil {
			return fmt.Errorf("parse pk: %w", err)
		}
		return nil
	}(); err != nil {
		return err
	}
	runtime.GC()
	fmt.Println("proving key loaded in", time.Since(loadStart))

	loadStart = time.Now()
	fmt.Println("loading verifying key of", targetTier, "assets")
	vk := groth16.NewVerifyingKey(ecc.BN254)
	if err := func() error {
		r, err := keys.Open(ctx, stem, ".vk")
		if err != nil {
			return fmt.Errorf("read vk: %w", err)
		}
		defer r.Close()
		if _, err := vk.ReadFrom(r); err != nil {
			return fmt.Errorf("parse vk: %w", err)
		}
		return nil
	}(); err != nil {
		return err
	}
	fmt.Println("verifying key loaded in", time.Since(loadStart))

	*params = snarkParams{tier: targetTier, r1cs: r1cs, provingKey: pk, verifyingKey: vk}
	return nil
}
