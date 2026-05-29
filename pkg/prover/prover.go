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
// derived from the declarative profile.toml + the KeysDir option.
// config.json keeps DB DSN only.
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
package prover

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	corecircuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/circuit"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	pconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/prover/config"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/constraint/solver"
)

// Options bundles the inputs Run needs to start a prover engine.
type Options struct {
	// ProfilePath points at the declarative profile.toml. Required.
	ProfilePath string

	// KeysDir holds the .pk / .vk / .r1cs artifacts written by keygen.
	// Required.
	KeysDir string

	// ConfigPath points at the prover's deployment config JSON (DB DSN
	// + DbSuffix). Defaults to "config/config.json" when empty.
	ConfigPath string
}

// resolved holds the derived (tier, stem) plan the prover walks
// when loading snark params. Built once at startup from profile.toml.
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
// is empty (clean exit, returns nil) or a fatal error escapes (returns
// an error). Library callers wishing to stop a long-running engine
// should kill the process; cancellation via context is a future-slice
// change.
//
// Run registers the IntegerDivision hint at startup. The registration
// is idempotent — repeated Run calls in the same process are safe.
func Run(opts Options) error {
	if opts.ProfilePath == "" {
		return fmt.Errorf("prover: ProfilePath is required (path to profile.toml)")
	}
	if opts.KeysDir == "" {
		return fmt.Errorf("prover: KeysDir is required (path to keygen .artifacts/)")
	}
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = "config/config.json"
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	prof, err := declarative.Load(opts.ProfilePath)
	if err != nil {
		return fmt.Errorf("prover: load profile %q: %w", opts.ProfilePath, err)
	}
	plan, err := buildResolved(prof, opts.KeysDir)
	if err != nil {
		return fmt.Errorf("prover: resolve snark params plan: %w", err)
	}

	// G1 carryover — the zkpor circuit's IntegerDivision hint must be
	// registered with the solver before groth16.Prove can solve the
	// witness, otherwise the solver can't resolve hint outputs.
	solver.RegisterHint(corecircuit.IntegerDivision)

	db, err := store.Open(cfg.MysqlDataSource)
	if err != nil {
		return fmt.Errorf("prover: open mysql: %w", err)
	}
	witnessStore := store.NewWitnessStore(db, cfg.DbSuffix)
	proofStore := store.NewProofStore(db, cfg.DbSuffix)
	if err := proofStore.CreateTable(); err != nil {
		return fmt.Errorf("prover: create proof table: %w", err)
	}

	var params snarkParams
	for {
		row, err := witnessStore.ClaimOldestByStatus(store.StatusPublished, store.StatusReceived)
		if errors.Is(err, store.ErrNotFound) {
			fmt.Println("no published witness rows in queue, prover quitting")
			return nil
		}
		if err != nil {
			fmt.Println("claim witness failed:", err.Error())
			time.Sleep(10 * time.Second)
			continue
		}
		if err := proveOne(row, &params, plan, witnessStore, proofStore); err != nil {
			return fmt.Errorf("prover: prove batch %d: %w", row.Height, err)
		}
	}
}

// buildResolved derives the (tier, stem) plan from profile.toml.
func buildResolved(prof *declarative.Profile, keysDir string) (*resolved, error) {
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
		out.zkKeyStems[i] = filepath.Join(keysDir, provider.KeyName(s, prof.Constraint.Module))
	}
	return out, nil
}

// loadConfig reads and parses the on-disk JSON config.
func loadConfig(path string) (*pconfig.Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("prover: read config %q: %w", path, err)
	}
	cfg := &pconfig.Config{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("prover: parse config %q: %w", path, err)
	}
	return cfg, nil
}
