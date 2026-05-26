// Command keygen is the zkpor-native trusted-setup generator. For each
// BatchShape the deployment's profile advertises, it compiles the
// t4_tiered_haircut_margin_3pool circuit at that shape's (userAssetCounts, assetCapacity,
// batchCounts) parameters, runs groth16.Setup, and writes the
// .pk / .vk / .r1cs artifact triplet to -out.
//
// File stems use BatchShape.StandardKeyName — e.g.
// "zkpor.t4_tiered_haircut_margin_3pool.5_10". The prover/verifier configs reference
// these stems verbatim in their ZkKeyName field. The asset capacity is
// NOT encoded in the stem; downstream services MUST configure the same
// AssetCapacity to land on a compatible witness shape.
//
// This is the engine-side trusted setup; in production each shape's
// triplet is the output of a real multi-party ceremony, not a
// single-process Setup call. For sample-data end-to-end smoke (R3
// step 4 exit criteria) a single-process Setup is sufficient.
//
// Run for production capacity + production shapes:
//
//	go run ./cmd/keygen -asset-capacity 500 -out .artifacts/
//
// Run for a tiny smoke shape + small capacity:
//
//	ZKPOR_BATCH_SHAPE_OVERRIDE=5_10 \
//	  go run ./cmd/keygen -asset-capacity 5 -out .artifacts/
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	t4circuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/circuit"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/binance"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

func main() {
	out := flag.String("out", ".", "output directory for .pk/.vk/.r1cs files")
	assetCapacity := flag.Int("asset-capacity", 0, "per-deployment asset slot count (must match witness/prover/verifier/userproof)")
	flag.Parse()

	if *assetCapacity <= 0 {
		panic("-asset-capacity must be > 0 (typical production value: 500)")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		panic(fmt.Sprintf("create output dir %q: %v", *out, err))
	}

	shapeProvider := binance.NewBatchShape()
	shapes := shapeProvider.Shapes()
	fmt.Printf("keygen for %d shape(s) at asset capacity %d:\n", len(shapes), *assetCapacity)
	for _, s := range shapes {
		fmt.Printf("  %d_%d (userAssetCounts=%d, allAssetCounts=%d, batchCounts=%d)\n",
			s.AssetCountTier, s.UsersPerBatch, s.AssetCountTier, *assetCapacity, s.UsersPerBatch)
	}

	for _, s := range shapes {
		stem := s.StandardKeyName(binance.SolvencyModel, corespec.NoExtensionID)
		if err := keygenShape(s, *assetCapacity, filepath.Join(*out, stem)); err != nil {
			panic(err.Error())
		}
	}
}

// keygenShape compiles the t4_tiered_haircut_margin_3pool circuit at the given shape and
// asset capacity, then writes .pk / .vk / .r1cs to "<stemPath>.<ext>".
func keygenShape(s corespec.BatchShape, assetCapacity int, stemPath string) error {
	circuit := t4circuit.NewBatchCreateUserCircuit(
		uint32(s.AssetCountTier),
		uint32(assetCapacity),
		uint32(s.UsersPerBatch),
	)

	compileStart := time.Now()
	cs, err := frontend.Compile(
		ecc.BN254.ScalarField(),
		r1cs.NewBuilder,
		circuit,
		frontend.IgnoreUnconstrainedInputs(),
	)
	if err != nil {
		return fmt.Errorf("compile %s: %w", stemPath, err)
	}
	fmt.Printf("%s: r1cs compiled in %s (%d constraints)\n",
		stemPath, time.Since(compileStart), cs.GetNbConstraints())

	setupStart := time.Now()
	pk, vk, err := groth16.Setup(cs)
	if err != nil {
		return fmt.Errorf("setup %s: %w", stemPath, err)
	}
	runtime.GC()
	fmt.Printf("%s: groth16.Setup done in %s\n", stemPath, time.Since(setupStart))

	// .pk uses WriteRawTo (uncompressed) so prover's UnsafeReadFrom can
	// load it; .vk uses WriteTo (compressed) so prover/verifier ReadFrom
	// can load it; .r1cs has only WriteTo / ReadFrom.
	if err := writeTo(stemPath+".pk", func(f *os.File) (int64, error) { return pk.WriteRawTo(f) }); err != nil {
		return err
	}
	if err := writeTo(stemPath+".vk", func(f *os.File) (int64, error) { return vk.WriteTo(f) }); err != nil {
		return err
	}
	if err := writeTo(stemPath+".r1cs", func(f *os.File) (int64, error) { return cs.WriteTo(f) }); err != nil {
		return err
	}
	return nil
}

// writeTo opens path for writing and invokes serialize to stream the
// artifact. Closes the file and reports bytes written on success.
func writeTo(path string, serialize func(*os.File) (int64, error)) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	n, err := serialize(f)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("%s: %d bytes\n", path, n)
	return nil
}
