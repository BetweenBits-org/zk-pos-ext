// Package merklepor is the zkpor non-zk Merkle-sum proof-of-liabilities
// engine — the side line to the main zk PoS product (PRODUCTION_ROADMAP
// Stage MS, gate G19). It is exposed as a Go library with the same
// Options/dispatch/ctx/error discipline as the zk service trio.
//
// Three modes:
//
//	RunAttest      build the dense Merkle sum tree from the snapshot, run
//	               the auditor reconcile checks, and persist the attested
//	               root + one per-user sum-inclusion proof row per account.
//	RunVerifyUser  verify a single user's sum-inclusion artifact against
//	               the published root (inclusion + sum-path).
//	RunAudit       recompute the reconcile report over the full leaf set
//	               and, given an audited reserves total, assert
//	               Reserves >= Liabilities.
//
// The engine is dispatched on profile.Model but T1-only by product scope
// (gate G19, D3): the net liability a sum tree commits to is well defined
// only for t1_simple_margin. It holds zero dependency on core/circuit,
// gnark, keygen, or prover — the zk PoS line is unaffected.
package merklepor

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
	"github.com/BetweenBits-org/zk-pos-ext/core/io/vfs"
	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/BetweenBits-org/zk-pos-ext/core/sumtree"
	"github.com/BetweenBits-org/zk-pos-ext/profile/declarative"
)

// attestBatchSize is the per-Create chunk size for attest rows, matching
// the userproof service's round-trip amortisation.
const attestBatchSize = 100

// Options bundles the inputs the Run* entry points need; cmd/attest and
// cmd/audit build every injected value (profile parse, snapshot opener,
// store adapter) so the engine never touches os/path or store directly.
// Fields irrelevant to a mode are ignored.
type Options struct {
	// Profile is the parsed declarative profile.toml. Required for all modes
	// except RunVerifyUser (which reads only the user-config artifact).
	Profile *declarative.Profile

	// Snapshot opens the customer snapshot inputs. Required for RunAttest /
	// RunAudit.
	Snapshot vfs.Opener

	// Attest is the injected persistence port. Required for RunAttest.
	Attest corehost.AttestStore

	// UserConfig reads the per-user sum-inclusion artifact. Required for
	// RunVerifyUser.
	UserConfig vfs.ByteSource

	// PublishedTotal, when non-nil, is the exchange's claimed total
	// liabilities; RunAttest/RunAudit reconcile the recomputed sum against
	// it. Nil skips the sum-equality check.
	PublishedTotal *big.Int

	// MaxBalance, when non-nil, bounds each per-account balance (e.g. the
	// uint64 ceiling). Nil disables the range check.
	MaxBalance *big.Int

	// Reserves is the audited on-chain reserves total fed to RunAudit for
	// the Reserves >= Liabilities assertion (gate G19, D4 — the engine does
	// not query chains). Nil skips the solvency comparison.
	Reserves *big.Int

	// CapacityOverride supersedes profile.asset_capacity when > 0 (smoke
	// harness only).
	CapacityOverride int

	// SnapshotID overrides profile.snapshot.snapshot_id when non-empty.
	SnapshotID string
}

// sumNodeJSON is the wire form of a sum-tree node in the published
// artifact: hex hash + decimal sum.
type sumNodeJSON struct {
	Hash string `json:"hash"`
	Sum  string `json:"sum"`
}

// SumUserConfig is the per-account published sum-inclusion artifact. It is
// embedded as JSON in each attest row's Config column and is exactly what
// RunVerifyUser reads back to recompute the proof. Self-describing: it
// carries the leaf, the sibling chain, and the published root so a user
// can verify offline against the widely-published (root, total).
type SumUserConfig struct {
	Index     int           `json:"index"`
	AccountId string        `json:"account_id"`
	LeafHash  string        `json:"leaf_hash"`
	Balance   string        `json:"balance"`
	Siblings  []sumNodeJSON `json:"siblings"`
	Root      string        `json:"root"`
	RootSum   string        `json:"root_sum"`
}

// resolved bundles the profile-derived plan shared by RunAttest / RunAudit.
type resolved struct {
	model      corespec.SolvencyModelID
	sourceType string
	snapID     string
	capacity   int
	pricing    corespec.PriceScaleProvider
	tiers      []int
}

// resolve derives the (model, source, snapshot id, capacity, pricing,
// tiers) plan from the injected profile. Mirrors userproof.Run.
func resolve(opts Options) (*resolved, error) {
	if opts.Profile == nil {
		return nil, fmt.Errorf("merklepor: Profile is required")
	}
	prof := opts.Profile
	pricing, err := declarative.BuildPricing(prof.Pricing)
	if err != nil {
		return nil, fmt.Errorf("merklepor: BuildPricing: %w", err)
	}
	model := corespec.SolvencyModelID(prof.Profile.Model)
	shapeProvider, err := declarative.BuildBatchShapeProvider(model, prof.BatchShapes)
	if err != nil {
		return nil, fmt.Errorf("merklepor: BuildBatchShapeProvider: %w", err)
	}
	capacity := prof.Profile.AssetCapacity
	if opts.CapacityOverride > 0 {
		capacity = opts.CapacityOverride
	}
	snapID := prof.Snapshot.SnapshotID
	if opts.SnapshotID != "" {
		snapID = opts.SnapshotID
	}
	return &resolved{
		model:      model,
		sourceType: prof.Snapshot.SourceType,
		snapID:     snapID,
		capacity:   capacity,
		pricing:    pricing,
		tiers:      tiersFromShapes(shapeProvider.Shapes()),
	}, nil
}

func tiersFromShapes(shapes []corespec.BatchShape) []int {
	out := make([]int, len(shapes))
	for i, s := range shapes {
		out[i] = s.AssetCountTier
	}
	return out
}

func siblingsToJSON(sibs []sumtree.Node) []sumNodeJSON {
	out := make([]sumNodeJSON, len(sibs))
	for i, s := range sibs {
		out[i] = sumNodeJSON{Hash: hex.EncodeToString(s.Hash), Sum: s.Sum.String()}
	}
	return out
}

// buildAndPersist runs the reconcile checks, builds the dense Merkle sum
// tree, and writes one attest row per leaf. It is model-blind: the leaves
// arrive as corehost.SumLeafRecord. Returns the published root and the
// number of rows written. Reconcile failures (negative / duplicate / range
// / sum-mismatch) abort before any tree is built — an exchange cannot
// attest a forged dataset.
func buildAndPersist(leaves []corehost.SumLeafRecord, store corehost.AttestStore, publishedTotal, maxBalance *big.Int) (sumtree.Node, int, error) {
	liab := make([]corehost.LiabilityLeaf, len(leaves))
	stLeaves := make([]sumtree.Leaf, len(leaves))
	for i, lf := range leaves {
		liab[i] = corehost.LiabilityLeaf{Index: uint32(lf.Position), Id: hex.EncodeToString(lf.AccountID), Balance: lf.Balance}
		stLeaves[i] = sumtree.Leaf{Hash: lf.LeafHash, Sum: lf.Balance}
	}
	if rep := corehost.Reconcile(liab, maxBalance, publishedTotal); !rep.OK() {
		v := rep.Violations[0]
		return sumtree.Node{}, 0, fmt.Errorf("merklepor: reconcile failed (%d violations); first: %s @%d %s", len(rep.Violations), v.Kind, v.Index, v.Detail)
	}

	tree, err := sumtree.Build(stLeaves)
	if err != nil {
		return sumtree.Node{}, 0, fmt.Errorf("merklepor: build sum tree: %w", err)
	}
	root := tree.Root()
	rootHex := hex.EncodeToString(root.Hash)
	rootSum := root.Sum.String()

	batch := make([]corehost.AttestProofDTO, 0, attestBatchSize)
	written := 0
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := store.Create(batch); err != nil {
			return err
		}
		written += len(batch)
		batch = batch[:0]
		return nil
	}

	for _, lf := range leaves {
		sibs, err := tree.Proof(lf.Position)
		if err != nil {
			return sumtree.Node{}, written, fmt.Errorf("merklepor: proof for position %d: %w", lf.Position, err)
		}
		sibJSON := siblingsToJSON(sibs)
		proofJSON, err := json.Marshal(sibJSON)
		if err != nil {
			return sumtree.Node{}, written, fmt.Errorf("merklepor: marshal proof: %w", err)
		}
		idHex := hex.EncodeToString(lf.AccountID)
		leafHex := hex.EncodeToString(lf.LeafHash)
		cfgJSON, err := json.Marshal(SumUserConfig{
			Index: lf.Position, AccountId: idHex, LeafHash: leafHex, Balance: lf.Balance.String(),
			Siblings: sibJSON, Root: rootHex, RootSum: rootSum,
		})
		if err != nil {
			return sumtree.Node{}, written, fmt.Errorf("merklepor: marshal config: %w", err)
		}
		batch = append(batch, corehost.AttestProofDTO{
			Index: uint32(lf.Position), AccountId: idHex, LeafHash: leafHex, Balance: lf.Balance.String(),
			Proof: string(proofJSON), Root: rootHex, RootSum: rootSum, Config: string(cfgJSON),
		})
		if len(batch) >= attestBatchSize {
			if err := flush(); err != nil {
				return sumtree.Node{}, written, err
			}
		}
	}
	if err := flush(); err != nil {
		return sumtree.Node{}, written, err
	}
	return root, written, nil
}

// verifyUserConfig decodes a SumUserConfig and checks its sum-inclusion
// proof against the embedded published root via
// corehost.VerifyMerkleSumProof. Returns (verified, error); a decode
// failure is an error, a clean "did not verify" is (false, nil).
func verifyUserConfig(raw []byte) (bool, error) {
	var cfg SumUserConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return false, fmt.Errorf("merklepor: unmarshal user config: %w", err)
	}
	leafHash, err := hex.DecodeString(cfg.LeafHash)
	if err != nil {
		return false, fmt.Errorf("merklepor: decode leaf hash: %w", err)
	}
	rootHash, err := hex.DecodeString(cfg.Root)
	if err != nil {
		return false, fmt.Errorf("merklepor: decode root: %w", err)
	}
	balance, ok := new(big.Int).SetString(cfg.Balance, 10)
	if !ok {
		return false, fmt.Errorf("merklepor: invalid balance %q", cfg.Balance)
	}
	rootSum, ok := new(big.Int).SetString(cfg.RootSum, 10)
	if !ok {
		return false, fmt.Errorf("merklepor: invalid root sum %q", cfg.RootSum)
	}
	sibs := make([]sumtree.Node, len(cfg.Siblings))
	for i, s := range cfg.Siblings {
		h, err := hex.DecodeString(s.Hash)
		if err != nil {
			return false, fmt.Errorf("merklepor: decode sibling %d hash: %w", i, err)
		}
		sum, ok := new(big.Int).SetString(s.Sum, 10)
		if !ok {
			return false, fmt.Errorf("merklepor: invalid sibling %d sum %q", i, s.Sum)
		}
		sibs[i] = sumtree.Node{Hash: h, Sum: sum}
	}
	leaf := sumtree.Node{Hash: leafHash, Sum: balance}
	return corehost.VerifyMerkleSumProof(rootHash, rootSum, cfg.Index, leaf, sibs), nil
}
