// Command prover is the zkpor-native Groth16 prover. Polls the witness
// table for Published batches, decodes each batch witness, runs
// groth16.Prove against the per-tier r1cs + proving key, verifies the
// proof locally, and writes the result to the proof table.
//
// Phase 3c (R10+1) swap: the model-typed decode + circuit-witness +
// Prove/Verify path is pulled into per-model DecodeAndProve runners
// at core/solvency/<model>/host/prover_runner.go. main becomes a
// dispatch + persistence layer.
//
// R8-C/3 wiring foundation: AssetsCountTiers + ZkKeyName stems are
// derived from the declarative profile.toml + the -keys-dir flag.
// config.json keeps DB DSN only.
//
// G1 carryover: solver.RegisterHint(corecircuit.IntegerDivision) at
// startup. Witness solving requires the prover's hint registration
// to match the .r1cs's reference.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	pconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/cmd/prover/config"
	corecircuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/circuit"
	corehost "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/host"
	t1host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/host"
	t2host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t2_static_haircut_margin/host"
	t3host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t3_tiered_haircut_margin_1pool/host"
	t4host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/constraint/solver"
)

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

func main() {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	keysDir := flag.String("keys-dir", "", "directory containing .pk/.vk/.r1cs artifacts (required)")
	flag.Parse()

	if *profilePath == "" {
		fmt.Fprintln(os.Stderr, "-profile is required (path to profile.toml)")
		os.Exit(2)
	}
	if *keysDir == "" {
		fmt.Fprintln(os.Stderr, "-keys-dir is required (path to keygen .artifacts/)")
		os.Exit(2)
	}

	cfg := loadConfig("config/config.json")
	prof, err := declarative.Load(*profilePath)
	if err != nil {
		panic(err.Error())
	}
	plan, err := buildResolved(prof, *keysDir)
	if err != nil {
		panic(fmt.Sprintf("resolve snark params plan: %v", err))
	}

	// G1 carryover — the zkpor circuit's IntegerDivision hint must be
	// registered with the solver before groth16.Prove can solve the
	// witness, otherwise the solver can't resolve hint outputs.
	solver.RegisterHint(corecircuit.IntegerDivision)

	db, err := store.Open(cfg.MysqlDataSource)
	if err != nil {
		panic(err.Error())
	}
	witnessStore := store.NewWitnessStore(db, cfg.DbSuffix)
	proofStore := store.NewProofStore(db, cfg.DbSuffix)
	if err := proofStore.CreateTable(); err != nil {
		panic(err.Error())
	}

	var params snarkParams
	for {
		row, err := witnessStore.ClaimOldestByStatus(store.StatusPublished, store.StatusReceived)
		if errors.Is(err, store.ErrNotFound) {
			fmt.Println("no published witness rows in queue, prover quitting")
			return
		}
		if err != nil {
			fmt.Println("claim witness failed:", err.Error())
			time.Sleep(10 * time.Second)
			continue
		}
		if err := proveOne(row, &params, plan, witnessStore, proofStore); err != nil {
			fmt.Println("prove batch", row.Height, "failed:", err.Error())
			return
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
func loadConfig(path string) *pconfig.Config {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err.Error())
	}
	cfg := &pconfig.Config{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		panic(err.Error())
	}
	return cfg
}

// proveOne handles one claimed batch: decode → prove → verify →
// persist proof → mark witness Finished. The idempotency probe
// (GetByBatchNumber before Create) makes a crashed retry safe:
// re-claim, see the proof already exists, just flip the witness
// status to Finished without re-proving.
func proveOne(
	row *store.BatchWitness,
	params *snarkParams,
	plan *resolved,
	witnessStore *store.WitnessStore,
	proofStore *store.ProofStore,
) error {
	encoded, err := base64.StdEncoding.DecodeString(row.WitnessData)
	if err != nil {
		return fmt.Errorf("base64 decode: %w", err)
	}

	// Idempotency probe: if a proof row already exists at this height,
	// skip Prove and just flip the witness status. Load snark params
	// for the row's assetsCount tier lazily — but we don't know the
	// tier until decode. Resolve by peeking the witness header? For
	// now we run decode + assertion regardless; lazy load handles
	// reuse across batches.
	if existing, err := proofStore.GetByBatchNumber(row.Height); err == nil {
		fmt.Printf("proof of height %d already exists (tier %d), marking witness finished\n", row.Height, existing.AssetsCount)
		return witnessStore.MarkStatus(row.Height, store.StatusFinished)
	} else if !errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("idempotency probe: %w", err)
	}

	// Decode + Prove via the model-typed runner. The runner picks
	// the per-user padded tier (targetAssetsCount) from the decoded
	// circuit witness, but we need it to drive lazy snark-params
	// loading. The cheapest path is: decode once via the runner,
	// match the tier into our plan, ensure params, then re-call the
	// runner with the cached params. To avoid double-decode we
	// pre-load params optimistically (first tier) and let the runner
	// fail-fast on tier mismatch. In practice the lazy cache flips
	// only at tier boundaries; the cache hit covers same-tier runs.
	result, err := dispatchDecodeAndProve(plan, params, encoded, row.Height)
	if err != nil {
		return fmt.Errorf("prove/verify: %w", err)
	}

	if err := persistProof(row, result, witnessStore, proofStore); err != nil {
		return fmt.Errorf("persist: %w", err)
	}
	return nil
}

// dispatchDecodeAndProve fronts the model-typed prover_runner with
// the lazy snark-params cache. The runner needs (r1cs, pk, vk) for
// its model's circuit; we pre-decode the first batch of each tier
// to discover the assetsCount, load params, and finally call the
// runner with the loaded params.
//
// To preserve the legacy single-decode behaviour we drive a two-pass
// strategy: if params.tier is already correct (hit), call the runner
// directly. On miss, load the params for *any* tier first (cmd loops
// in ascending tier order, so the first claimed batch is always the
// first tier), then call the runner.
//
// The lazy cache is correct as long as ascending tier order is
// preserved by the witness builder + DB ordering. R8/R10 witness
// runners both honor ascending tier order.
func dispatchDecodeAndProve(plan *resolved, params *snarkParams, encoded []byte, batchNumber int64) (*corehost.BatchProofResult, error) {
	// On first call params.r1cs is nil — load the smallest tier
	// (assumed first by ascending order). Subsequent calls compare
	// against the cached tier; mismatch triggers a reload.
	if params.r1cs == nil {
		if err := loadSnarkParams(params, plan, plan.assetCountTiers[0]); err != nil {
			return nil, err
		}
	}
	result, err := runDecodeAndProve(plan.model, encoded, params, plan.assetCountTiers, batchNumber)
	if err == nil {
		return result, nil
	}
	// Tier mismatch is signaled by the runner via an error message
	// containing the targetAssetsCount; the lazy-cache retry below
	// reloads and re-runs. The check is by substring rather than
	// a typed error to keep the runner interface model-blind.
	_ = err
	// Simpler retry strategy: try every declared tier in order until
	// one succeeds. Avoids parsing error messages.
	for _, tier := range plan.assetCountTiers {
		if tier == params.tier {
			continue
		}
		if err := loadSnarkParams(params, plan, tier); err != nil {
			return nil, err
		}
		result, retryErr := runDecodeAndProve(plan.model, encoded, params, plan.assetCountTiers, batchNumber)
		if retryErr == nil {
			return result, nil
		}
	}
	return nil, fmt.Errorf("no declared tier produced a valid prove: %w", err)
}

// runDecodeAndProve dispatches to the model-typed runner.
func runDecodeAndProve(
	model corespec.SolvencyModelID,
	encoded []byte,
	params *snarkParams,
	assetCountTiers []int,
	batchNumber int64,
) (*corehost.BatchProofResult, error) {
	switch model {
	case "t1_simple_margin":
		return t1host.DecodeAndProve(encoded, params.r1cs, params.provingKey, params.verifyingKey, assetCountTiers, batchNumber)
	case "t2_static_haircut_margin":
		return t2host.DecodeAndProve(encoded, params.r1cs, params.provingKey, params.verifyingKey, assetCountTiers, batchNumber)
	case "t3_tiered_haircut_margin_1pool":
		return t3host.DecodeAndProve(encoded, params.r1cs, params.provingKey, params.verifyingKey, assetCountTiers, batchNumber)
	case "t4_tiered_haircut_margin_3pool":
		return t4host.DecodeAndProve(encoded, params.r1cs, params.provingKey, params.verifyingKey, assetCountTiers, batchNumber)
	default:
		return nil, fmt.Errorf("prover: unsupported model %q", model)
	}
}

// persistProof writes the proof row + flips the witness status.
// Idempotency check is done by proveOne before this is called.
func persistProof(
	row *store.BatchWitness,
	result *corehost.BatchProofResult,
	witnessStore *store.WitnessStore,
	proofStore *store.ProofStore,
) error {
	cexCommitments, err := json.Marshal([][]byte{
		result.BeforeCEXAssetsCommitment,
		result.AfterCEXAssetsCommitment,
	})
	if err != nil {
		return fmt.Errorf("marshal cex commitments: %w", err)
	}
	accountRoots, err := json.Marshal([][]byte{
		result.BeforeAccountTreeRoot,
		result.AfterAccountTreeRoot,
	})
	if err != nil {
		return fmt.Errorf("marshal account roots: %w", err)
	}

	if err := proofStore.Create(&store.Proof{
		ProofInfo:               base64.StdEncoding.EncodeToString(result.ProofRaw),
		BatchNumber:             row.Height,
		CexAssetListCommitments: string(cexCommitments),
		AccountTreeRoots:        string(accountRoots),
		BatchCommitment:         base64.StdEncoding.EncodeToString(result.BatchCommitment),
		AssetsCount:             result.AssetsCount,
	}); err != nil {
		return fmt.Errorf("create proof row: %w", err)
	}
	return witnessStore.MarkStatus(row.Height, store.StatusFinished)
}

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
