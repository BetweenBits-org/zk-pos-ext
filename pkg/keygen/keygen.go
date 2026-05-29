// Package keygen is the zkpor-native trusted-setup generator engine.
// For each BatchShape advertised by the customer's declarative
// profile, it compiles the solvency-model circuit at that shape's
// (userAssetCounts, assetCapacity, batchCounts) parameters, runs
// groth16.Setup, and writes the .pk / .vk / .r1cs artifact triplet
// to the output directory.
//
// File stems use BatchShape.StandardKeyName, with the SolvencyModelID
// drawn from the profile — e.g. "zkpor.t4_tiered_haircut_margin_3pool.5_10"
// for the t4_reference profile, "zkpor.t1_simple_margin.50_1000" for
// t1_reference. Asset capacity is NOT encoded in the stem; downstream
// services MUST configure the same AssetCapacity to land on a
// compatible witness shape.
//
// This is the engine-side trusted setup; in production each shape's
// triplet is the output of a real multi-party ceremony, not a
// single-process Setup call. For sample-data end-to-end smoke a
// single-process Setup is sufficient.
//
// R8-C/1 wiring: profile.toml is the source-of-truth (no hard-coded
// per-profile constructors). Keygen does not consume snapshot
// connectors; those are registered only by services that read
// canonical snapshot data.
//
// R12-B contract: Run returns error; in-process callers can drive
// keygen without recover() and propagate the error up. The cmd/keygen
// shim is the only layer that converts errors into exit codes.
package keygen

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// Options bundles the inputs Run needs.
type Options struct {
	// ProfilePath points at the declarative profile.toml. Required.
	ProfilePath string

	// OutDir is the destination directory for the .pk / .vk / .r1cs
	// triples. Defaults to "." when empty. Created if missing.
	OutDir string

	// CapacityOverride supersedes profile.asset_capacity when > 0.
	// Smoke-harness use only — production callers should leave at 0
	// so the profile.toml capacity is authoritative.
	CapacityOverride int
}

// Run walks every BatchShape declared by the profile and writes one
// .pk / .vk / .r1cs triple per shape. Returns an error describing the
// first compile/setup/write failure encountered; nil on success.
func Run(opts Options) error {
	if opts.ProfilePath == "" {
		return fmt.Errorf("keygen: ProfilePath is required (path to profile.toml)")
	}
	outDir := opts.OutDir
	if outDir == "" {
		outDir = "."
	}

	prof, err := declarative.Load(opts.ProfilePath)
	if err != nil {
		return fmt.Errorf("keygen: load profile %q: %w", opts.ProfilePath, err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("keygen: create output dir %q: %w", outDir, err)
	}

	shapes, err := declarative.BuildBatchShape(prof.BatchShapes)
	if err != nil {
		return fmt.Errorf("keygen: BuildBatchShape: %w", err)
	}
	model := corespec.SolvencyModelID(prof.Profile.Model)
	capacity := prof.Profile.AssetCapacity
	if opts.CapacityOverride > 0 {
		capacity = opts.CapacityOverride
		fmt.Printf("keygen: asset-capacity override %d (profile.toml = %d)\n",
			capacity, prof.Profile.AssetCapacity)
	}
	fmt.Printf("keygen for profile %q (model %s, capacity %d): %d shape(s)\n",
		prof.Profile.Name, model, capacity, len(shapes))
	for _, s := range shapes {
		fmt.Printf("  %d_%d (userAssetCounts=%d, allAssetCounts=%d, batchCounts=%d)\n",
			s.AssetCountTier, s.UsersPerBatch, s.AssetCountTier, capacity, s.UsersPerBatch)
	}

	for _, s := range shapes {
		stem := s.StandardKeyName(model, prof.Constraint.Module)
		if err := keygenShape(model, s, capacity, filepath.Join(outDir, stem)); err != nil {
			return err
		}
	}
	return nil
}

// keygenShape compiles the model-appropriate circuit at the given
// shape and asset capacity, then writes .pk / .vk / .r1cs to
// "<stemPath>.<ext>".
func keygenShape(model corespec.SolvencyModelID, s corespec.BatchShape, assetCapacity int, stemPath string) error {
	circuit, err := newCircuit(model, s, assetCapacity)
	if err != nil {
		return err
	}

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

	// .pk uses WriteTo (compressed G1/G2 points) — same encoding as the
	// legacy Binance keygen, halves on-disk size vs WriteRawTo (24GB →
	// 12GB for the t4_tiered_haircut_margin_3pool 50_700/500_92 shapes
	// at capacity=500). gnark's pk.UnsafeReadFrom auto-detects the
	// encoding, so prover read path is unchanged. .vk + .r1cs use
	// WriteTo for the same reason.
	if err := writeTo(stemPath+".pk", func(f *os.File) (int64, error) { return pk.WriteTo(f) }); err != nil {
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
