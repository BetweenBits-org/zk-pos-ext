// Lazy snark-params loader. The prover keeps one set of (r1cs, pk, vk)
// in memory at a time and only reloads when the requested tier differs
// from the cached one — matching legacy LoadSnarkParamsOnce behaviour.
// This pays off because the witness builder emits batches in
// ascending tier order, so most consecutive batches hit the cache.

package prover

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
)

// loadSnarkParams is the lazy-load cache: reload r1cs/pk/vk only when
// the requested tier differs from the cached one.
func loadSnarkParams(params *snarkParams, plan *resolved, targetTier int) error {
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
	r1csBytes, err := os.ReadFile(stem + ".r1cs")
	if err != nil {
		return fmt.Errorf("read r1cs: %w", err)
	}
	r1cs := groth16.NewCS(ecc.BN254)
	if _, err := r1cs.ReadFrom(bytes.NewBuffer(r1csBytes)); err != nil {
		return fmt.Errorf("parse r1cs: %w", err)
	}
	runtime.GC()
	fmt.Println("r1cs loaded in", time.Since(loadStart))

	loadStart = time.Now()
	fmt.Println("loading proving key of", targetTier, "assets")
	pkBytes, err := os.ReadFile(stem + ".pk")
	if err != nil {
		return fmt.Errorf("read pk: %w", err)
	}
	pk := groth16.NewProvingKey(ecc.BN254)
	if _, err := pk.UnsafeReadFrom(bytes.NewBuffer(pkBytes)); err != nil {
		return fmt.Errorf("parse pk: %w", err)
	}
	runtime.GC()
	fmt.Println("proving key loaded in", time.Since(loadStart))

	loadStart = time.Now()
	fmt.Println("loading verifying key of", targetTier, "assets")
	vkBytes, err := os.ReadFile(stem + ".vk")
	if err != nil {
		return fmt.Errorf("read vk: %w", err)
	}
	vk := groth16.NewVerifyingKey(ecc.BN254)
	if _, err := vk.ReadFrom(bytes.NewBuffer(vkBytes)); err != nil {
		return fmt.Errorf("parse vk: %w", err)
	}
	fmt.Println("verifying key loaded in", time.Since(loadStart))

	*params = snarkParams{tier: targetTier, r1cs: r1cs, provingKey: pk, verifyingKey: vk}
	return nil
}
