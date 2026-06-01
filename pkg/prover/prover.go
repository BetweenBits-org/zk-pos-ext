// Package prover is the zkpor-native Groth16 prover engine. It polls
// the witness table for Published batches, decodes each batch witness,
// runs groth16.Prove against the per-tier r1cs + proving key, verifies
// the proof locally, and writes the result to the proof table.
//
// Phase 3c (R10+1) swap: the model-typed decode + circuit-witness +
// Prove/Verify path is pulled into per-model DecodeAndProve runners
// at core/solvency/<model>/host/prover_runner.go. This package is the
// dispatch + persistence layer.
//
// R8-C/3 wiring foundation: AssetsCountTiers + ZkKeyName stems are
// derived from the declarative profile.toml; the stems are logical
// identifiers resolved against the injected vfs.KeyOpener (the cmd shim
// wraps the keys directory). config.json keeps DB DSN only.
//
// G1 carryover: solver.RegisterHint(corecircuit.IntegerDivision) is
// called inside Run at startup. Witness solving requires the prover's
// hint registration to match the .r1cs's reference.
//
// R12-B contract: Run returns error; in-process callers can drive the
// prover poll loop without recover() and propagate the error up. The
// cmd/prover shim is the only layer that converts errors into exit
// codes. ErrNotFound on the queue is a clean shutdown — Run returns
// nil. Transient claim errors continue to sleep+retry (no error escape,
// by design). A proveOne failure is fatal and propagates.
//
// R12-C contract: Run takes a context.Context for graceful shutdown.
// The prover is a long-running daemon, so cancellation is the normal
// way an operator stops it. Cancellation is batch-granular: the current
// proveOne (whose groth16.Prove cannot be interrupted) runs to
// completion, then the loop observes ctx before claiming the next batch
// and returns ctx.Err(). The cmd/prover shim wires SIGINT/SIGTERM into
// the context and treats context.Canceled as a clean shutdown (exit 0),
// distinct from a fatal proveOne error (exit 1).
//
// R12-EF contract: Run no longer reads files, parses the profile/config,
// or opens the store itself — those inputs arrive pre-built in Options
// (parsed *declarative.Profile, parsed *pconfig.Config, a vfs.KeyOpener
// for the .pk/.vk/.r1cs artifacts, and the witness-queue + proof-store
// ports). cmd/prover is the sole os/path + store wiring point.
package prover

import (
	"context"
	"errors"
	"fmt"
	"time"

	corecircuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/circuit"
	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/io/vfs"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	pconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/prover/config"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/constraint/solver"
)

// Options bundles the inputs Run needs to start a prover engine. The
// cmd/prover shim builds every injected value (parses the profile +
// config, wraps the keys directory in a vfs.KeyOpener, and wires the
// witness-queue + proof-store adapters); the engine never touches
// os/path or store itself.
type Options struct {
	// Profile is the parsed declarative profile.toml. Required.
	Profile *declarative.Profile

	// Keys opens the .pk / .vk / .r1cs artifacts written by keygen,
	// addressed by logical stem + extension. The cmd shim wraps the
	// keys directory into this opener. Required.
	Keys vfs.KeyOpener

	// Config is the parsed prover deployment config (DB DSN + DbSuffix).
	// Required.
	Config *pconfig.Config

	// WitnessQueue is the injected persistence port for the
	// witness↔prover artifact channel. Required.
	WitnessQueue corehost.WitnessQueue

	// Proofs is the injected persistence port for the prover→verifier
	// proof channel. Required; the cmd shim provides the MySQL adapter
	// and has already called EnsureSchema.
	Proofs corehost.ProofStore
}

// resolved holds the derived (tier, stem) plan the prover walks
// when loading snark params. Built once at startup from profile.toml.
// The stems are LOGICAL identifiers (provider.KeyName output only); the
// vfs.KeyOpener joins them against its directory at open time.
type resolved struct {
	model           corespec.SolvencyModelID
	assetCountTiers []int
	zkKeyStems      []string // same index as assetCountTiers
}

// snarkParams caches the lazy-loaded artifact triple for one
// AssetsCount tier. The prover keeps one set in memory at a time and
// reloads only when the next batch's tier differs from the cached
// tier — matches legacy LoadSnarkParamsOnce behaviour.
type snarkParams struct {
	tier         int
	r1cs         constraint.ConstraintSystem
	provingKey   groth16.ProvingKey
	verifyingKey groth16.VerifyingKey
}

// Run starts the prover poll loop. It blocks until the witness queue
// is empty (clean exit, returns nil), the context is cancelled (returns
// ctx.Err() after the in-flight batch finishes), or a fatal error
// escapes (returns that error).
//
// Run registers the IntegerDivision hint at startup. The registration
// is idempotent — repeated Run calls in the same process are safe.
func Run(ctx context.Context, opts Options) error {
	if opts.Profile == nil {
		return fmt.Errorf("prover: Profile is required")
	}
	if opts.Config == nil {
		return fmt.Errorf("prover: Config is required")
	}
	if opts.Keys == nil {
		return fmt.Errorf("prover: Keys is required")
	}
	if opts.WitnessQueue == nil {
		return fmt.Errorf("prover: WitnessQueue is required")
	}
	if opts.Proofs == nil {
		return fmt.Errorf("prover: Proofs is required")
	}

	plan, err := buildResolved(opts.Profile)
	if err != nil {
		return fmt.Errorf("prover: resolve snark params plan: %w", err)
	}

	// G1 carryover — the zkpor circuit's IntegerDivision hint must be
	// registered with the solver before groth16.Prove can solve the
	// witness, otherwise the solver can't resolve hint outputs.
	solver.RegisterHint(corecircuit.IntegerDivision)

	witnessStore := opts.WitnessQueue
	proofStore := opts.Proofs

	var params snarkParams
	for {
		// Batch-granular cancellation: observe ctx before claiming the
		// next batch so a SIGINT/SIGTERM received during the previous
		// proveOne exits cleanly once that batch's proof is persisted.
		if err := ctx.Err(); err != nil {
			fmt.Println("prover: context cancelled, shutting down")
			return err
		}
		row, err := witnessStore.ClaimOldestByStatus(corehost.StatusPublished, corehost.StatusReceived)
		if errors.Is(err, corehost.ErrNotFound) {
			fmt.Println("no published witness rows in queue, prover quitting")
			return nil
		}
		if err != nil {
			fmt.Println("claim witness failed:", err.Error())
			// Cancellable backoff: a cancel during the retry wait exits
			// immediately instead of blocking the full interval.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Second):
			}
			continue
		}
		if err := proveOne(ctx, row, &params, plan, opts.Keys, witnessStore, proofStore); err != nil {
			return fmt.Errorf("prover: prove batch %d: %w", row.Height, err)
		}
	}
}

// buildResolved derives the (tier, stem) plan from profile.toml. The
// stems are LOGICAL identifiers (provider.KeyName only); the directory
// join lives in the vfs.KeyOpener, so buildResolved no longer takes a
// keys directory.
func buildResolved(prof *declarative.Profile) (*resolved, error) {
	provider, err := declarative.BuildBatchShapeProvider(
		corespec.SolvencyModelID(prof.Profile.Model), prof.BatchShapes)
	if err != nil {
		return nil, err
	}
	shapes := provider.Shapes()
	out := &resolved{
		model:           corespec.SolvencyModelID(prof.Profile.Model),
		assetCountTiers: make([]int, len(shapes)),
		zkKeyStems:      make([]string, len(shapes)),
	}
	for i, s := range shapes {
		out.assetCountTiers[i] = s.AssetCountTier
		out.zkKeyStems[i] = provider.KeyName(s, prof.Constraint.Module)
	}
	return out, nil
}
