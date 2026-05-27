// Command prover is the zkpor-native Groth16 prover. Polls the witness
// table for Published batches, decodes each batch witness, runs
// groth16.Prove against the per-tier r1cs + proving key, verifies the
// proof locally, and writes the result to the proof table.
//
// R8-C/3 swap: AssetsCountTiers + ZkKeyName stems are derived from
// the declarative profile.toml (model + batch_shapes + constraint
// module) + the -keys-dir flag. config.json keeps DB DSN only.
//
// This is the R3 step 4 core-path service: single-instance, DB-poll
// task pump (no Redis), no rerun mode. Multi-worker scaling and
// offline rerun are tracked as follow-up slices.
//
// G1 carryover: solver.RegisterHint(corecircuit.IntegerDivision) at
// startup resolves the hint-identifier divergence the byte-equivalence
// proof intentionally excluded. Witness solving requires the prover's
// hint registration to match the .r1cs's reference; registering
// zkpor's IntegerDivision (not legacy's) is what closes the loop.
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
	t4circuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/circuit"
	t4host "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/host"
	t4spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t4_tiered_haircut_margin_3pool/spec"
	corespec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
)

// expectedModel is the solvency model this prover binary supports.
// V1-PROD scope is T4 only — the SetBatchCreateUserCircuitWitness
// call below is model-typed.
const expectedModel corespec.SolvencyModelID = "t4_tiered_haircut_margin_3pool"

// resolved holds the derived (tier, stem) plan the prover walks
// when loading snark params. Built once at startup from profile.toml.
type resolved struct {
	assetCountTiers []int
	zkKeyStems      []string // same index as assetCountTiers; absolute or keys-dir-relative
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
	if model := corespec.SolvencyModelID(prof.Profile.Model); model != expectedModel {
		panic(fmt.Sprintf("prover binary supports %q only; profile.toml model = %q", expectedModel, model))
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

// buildResolved derives the (tier, stem) plan from profile.toml. The
// tiers come from BuildBatchShape (declarative builder honours
// ZKPOR_BATCH_SHAPE_OVERRIDE for smoke); stems come from
// BatchShape.StandardKeyName(model, constraint module) joined with
// keysDir.
func buildResolved(prof *declarative.Profile, keysDir string) (*resolved, error) {
	provider, err := declarative.BuildBatchShapeProvider(
		corespec.SolvencyModelID(prof.Profile.Model), prof.BatchShapes)
	if err != nil {
		return nil, err
	}
	shapes := provider.Shapes()
	out := &resolved{
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
	witnessForCircuit, err := t4host.DecodeBatchWitness(encoded)
	if err != nil {
		return fmt.Errorf("decode witness: %w", err)
	}

	proof, assetsCount, err := generateAndVerify(witnessForCircuit, params, plan, row.Height)
	if err != nil {
		return fmt.Errorf("prove/verify: %w", err)
	}

	if err := persistProof(row, witnessForCircuit, proof, assetsCount, witnessStore, proofStore); err != nil {
		return fmt.Errorf("persist: %w", err)
	}
	return nil
}

// generateAndVerify wires witness → r1cs → groth16.Prove → groth16.Verify.
// Returns the proof + the per-user asset count (the in-circuit
// padded tier) so the persisted row can carry it for the verifier's
// .vk selection.
func generateAndVerify(
	witnessForCircuit *t4spec.BatchCreateUserWitness,
	params *snarkParams,
	plan *resolved,
	batchNumber int64,
) (groth16.Proof, int, error) {
	startTime := time.Now().UnixMilli()
	fmt.Println("begin to generate proof for batch:", batchNumber)

	circuitWitness, err := t4circuit.SetBatchCreateUserCircuitWitness(witnessForCircuit, plan.assetCountTiers)
	if err != nil {
		return nil, 0, fmt.Errorf("build circuit witness: %w", err)
	}
	targetAssetsCount := len(circuitWitness.CreateUserOps[0].Assets)
	if err := loadSnarkParams(params, plan, targetAssetsCount); err != nil {
		return nil, 0, fmt.Errorf("load snark params: %w", err)
	}

	witness, err := frontend.NewWitness(circuitWitness, ecc.BN254.ScalarField())
	if err != nil {
		return nil, 0, fmt.Errorf("witness: %w", err)
	}
	verifyWitness := t4circuit.NewVerifyBatchCreateUserCircuit(witnessForCircuit.BatchCommitment)
	vWitness, err := frontend.NewWitness(verifyWitness, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		return nil, 0, fmt.Errorf("public witness: %w", err)
	}

	proof, err := groth16.Prove(params.r1cs, params.provingKey, witness)
	if err != nil {
		return nil, 0, fmt.Errorf("groth16.Prove: %w", err)
	}
	fmt.Println("proof generation cost", time.Now().UnixMilli()-startTime, "ms")

	verifyStart := time.Now().UnixMilli()
	if err := groth16.Verify(proof, params.verifyingKey, vWitness); err != nil {
		return nil, 0, fmt.Errorf("groth16.Verify: %w", err)
	}
	fmt.Println("proof verification cost", time.Now().UnixMilli()-verifyStart, "ms")

	return proof, targetAssetsCount, nil
}

// persistProof writes the proof row (idempotently — skip if a row
// already exists at this height) and flips the witness row to
// Finished. The idempotency check makes a crash between Prove and
// Create safe: a re-claim sees the proof already present and just
// updates the witness status.
func persistProof(
	row *store.BatchWitness,
	witnessForCircuit *t4spec.BatchCreateUserWitness,
	proof groth16.Proof,
	assetsCount int,
	witnessStore *store.WitnessStore,
	proofStore *store.ProofStore,
) error {
	var proofBuf bytes.Buffer
	if _, err := proof.WriteRawTo(&proofBuf); err != nil {
		return fmt.Errorf("serialise proof: %w", err)
	}

	if _, err := proofStore.GetByBatchNumber(row.Height); err == nil {
		fmt.Printf("proof of height %d already exists, marking witness finished\n", row.Height)
		return witnessStore.MarkStatus(row.Height, store.StatusFinished)
	} else if !errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("idempotency probe: %w", err)
	}

	cexCommitments, err := json.Marshal([][]byte{
		witnessForCircuit.BeforeCEXAssetsCommitment,
		witnessForCircuit.AfterCEXAssetsCommitment,
	})
	if err != nil {
		return fmt.Errorf("marshal cex commitments: %w", err)
	}
	accountRoots, err := json.Marshal([][]byte{
		witnessForCircuit.BeforeAccountTreeRoot,
		witnessForCircuit.AfterAccountTreeRoot,
	})
	if err != nil {
		return fmt.Errorf("marshal account roots: %w", err)
	}

	if err := proofStore.Create(&store.Proof{
		ProofInfo:               base64.StdEncoding.EncodeToString(proofBuf.Bytes()),
		BatchNumber:             row.Height,
		CexAssetListCommitments: string(cexCommitments),
		AccountTreeRoots:        string(accountRoots),
		BatchCommitment:         base64.StdEncoding.EncodeToString(witnessForCircuit.BatchCommitment),
		AssetsCount:             assetsCount,
	}); err != nil {
		return fmt.Errorf("create proof row: %w", err)
	}
	return witnessStore.MarkStatus(row.Height, store.StatusFinished)
}

// loadSnarkParams is the lazy-load cache: reload r1cs/pk/vk only when
// the requested tier differs from the cached one. The legacy ran a
// background goroutine triggering runtime.GC every 10s during load;
// we collapse it to a single GC at end-of-load — same intent (keep
// peak RSS bounded under several-GB proving keys), simpler code.
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
