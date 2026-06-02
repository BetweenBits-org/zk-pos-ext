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
//
// R12-C contract: Run takes a context.Context. groth16.Setup itself is
// not cancellable, so cancellation is shape-granular — Run observes ctx
// before compiling each shape and returns ctx.Err() between shapes. The
// in-flight Setup runs to completion. Keygen is a one-shot job, so
// cmd/keygen treats any error (including context.Canceled) as exit 1.
//
// R12-EF contract: Run no longer reads the profile or creates the output
// directory itself — the parsed *declarative.Profile and a vfs.KeySink
// writer arrive pre-built in Options. The key-writing path streams each
// artifact through opts.Keys.Create(stem, ext); cmd/keygen is the sole
// os/path wiring point (declarative.Load + osvfs.KeyDirSink).
package keygen

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/BetweenBits-org/zk-pos-ext/core/io/vfs"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/BetweenBits-org/zk-pos-ext/profile/declarative"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// Options bundles the inputs Run needs. The cmd/keygen shim builds every
// injected value (parses the profile, wraps the output directory in a
// vfs.KeySink); the engine never touches os/path itself.
type Options struct {
	// Profile is the parsed declarative profile.toml. Required.
	Profile *declarative.Profile

	// Keys creates the .pk / .vk / .r1cs artifacts, addressed by logical
	// stem + extension. The cmd shim wraps the output directory into this
	// sink. Required.
	Keys vfs.KeySink

	// CapacityOverride supersedes profile.asset_capacity when > 0.
	// Smoke-harness use only — production callers should leave at 0
	// so the profile.toml capacity is authoritative.
	CapacityOverride int
}

// Run walks every BatchShape declared by the profile and writes one
// .pk / .vk / .r1cs triple per shape. Returns an error describing the
// first compile/setup/write failure encountered; nil on success. A
// cancelled ctx stops the walk between shapes and returns ctx.Err().
func Run(ctx context.Context, opts Options) error {
	if opts.Profile == nil {
		return fmt.Errorf("keygen: Profile is required")
	}
	if opts.Keys == nil {
		return fmt.Errorf("keygen: Keys (vfs.KeySink) is required")
	}
	prof := opts.Profile

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
		// Shape-granular cancellation: a SIGINT received during the
		// previous (non-cancellable) Setup stops the walk here rather
		// than starting another multi-minute compile+setup.
		if err := ctx.Err(); err != nil {
			return err
		}
		// stem is the LOGICAL key identifier (StandardKeyName output
		// only); the vfs.KeySink joins it against its output directory at
		// create time, so the engine never assembles a path.
		stem := s.StandardKeyName(model, prof.Constraint.Module)
		if err := keygenShape(model, s, capacity, stem, opts.Keys); err != nil {
			return err
		}
	}
	return nil
}

// keygenShape compiles the model-appropriate circuit at the given
// shape and asset capacity, then writes .pk / .vk / .r1cs through the
// vfs.KeySink at logical stem "<stem>.<ext>".
func keygenShape(model corespec.SolvencyModelID, s corespec.BatchShape, assetCapacity int, stem string, keys vfs.KeySink) error {
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
		return fmt.Errorf("compile %s: %w", stem, err)
	}
	fmt.Printf("%s: r1cs compiled in %s (%d constraints)\n",
		stem, time.Since(compileStart), cs.GetNbConstraints())

	setupStart := time.Now()
	pk, vk, err := groth16.Setup(cs)
	if err != nil {
		return fmt.Errorf("setup %s: %w", stem, err)
	}
	runtime.GC()
	fmt.Printf("%s: groth16.Setup done in %s\n", stem, time.Since(setupStart))

	// .pk uses WriteTo (compressed G1/G2 points) — same encoding as the
	// legacy Binance keygen, halves on-disk size vs WriteRawTo (24GB →
	// 12GB for the t4_tiered_haircut_margin_3pool 50_700/500_92 shapes
	// at capacity=500). gnark's pk.UnsafeReadFrom auto-detects the
	// encoding, so prover read path is unchanged. .vk + .r1cs use
	// WriteTo for the same reason.
	if err := writeTo(keys, stem, ".pk", func(w io.Writer) (int64, error) { return pk.WriteTo(w) }); err != nil {
		return err
	}
	if err := writeTo(keys, stem, ".vk", func(w io.Writer) (int64, error) { return vk.WriteTo(w) }); err != nil {
		return err
	}
	if err := writeTo(keys, stem, ".r1cs", func(w io.Writer) (int64, error) { return cs.WriteTo(w) }); err != nil {
		return err
	}
	return nil
}

// writeTo creates the stem+ext key stream through the sink and invokes
// serialize to stream the artifact into it. Closes the stream and
// reports bytes written on success.
func writeTo(keys vfs.KeySink, stem, ext string, serialize func(io.Writer) (int64, error)) error {
	w, err := keys.Create(stem, ext)
	if err != nil {
		return fmt.Errorf("create %s%s: %w", stem, ext, err)
	}
	defer w.Close()

	n, err := serialize(w)
	if err != nil {
		return fmt.Errorf("write %s%s: %w", stem, ext, err)
	}
	fmt.Printf("%s%s: %d bytes\n", stem, ext, n)
	return nil
}
