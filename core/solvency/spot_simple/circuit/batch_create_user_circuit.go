package circuit

import (
	corecircuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/circuit"
	spotspec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/spot_simple/spec"
	"github.com/consensys/gnark/std/hash/poseidon"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"github.com/consensys/gnark/std/rangecheck"
)

// BatchCreateUserCircuit is the gnark Circuit type for the spot_simple
// model. Substantially simpler than tier_3bucket — no haircut, no
// 3-bucket collateral, no tier table — but follows the same
// architectural pattern (sum equality enforced via After=Before+Δ on
// the CEX side and per-user Merkle tree updates).
//
// module is the unexported alpha-layer ConstraintModule hook. gnark's
// frontend reflects only on exported, Variable-bearing fields, so this
// field is invisible to Compile and adds no in-circuit cost when left
// nil or set to a noop module. Wire customer/regulator-specific
// constraints via SetConstraintModule before Compile.
type BatchCreateUserCircuit struct {
	BatchCommitment           Variable `gnark:",public"`
	BeforeAccountTreeRoot     Variable
	AfterAccountTreeRoot      Variable
	BeforeCEXAssetsCommitment Variable
	AfterCEXAssetsCommitment  Variable
	BeforeCexAssets           []CexAssetInfo
	CreateUserOps             []CreateUserOperation

	module spotspec.ConstraintModule
}

// SetConstraintModule wires the alpha-layer ConstraintModule hook
// invoked at the end of Define after every base constraint has been
// emitted. Setting nil (the default) reverts to the no-hook shape.
//
// Composing a non-nil module forks the trusted setup: the resulting
// .pk/.vk pair is unique to the (spot_simple, module) pair.
func (b *BatchCreateUserCircuit) SetConstraintModule(m spotspec.ConstraintModule) {
	b.module = m
}

// NewVerifyBatchCreateUserCircuit returns a circuit instance with only
// the public BatchCommitment populated — used on the verifier side to
// check a serialized proof.
func NewVerifyBatchCreateUserCircuit(commitment []byte) *BatchCreateUserCircuit {
	return &BatchCreateUserCircuit{BatchCommitment: commitment}
}

// NewBatchCreateUserCircuit allocates a fully zero-valued circuit
// instance sized for the given shape. Used during trusted setup and as
// the witness template at proving time.
//
// userAssetCounts is the per-user asset-tier (the smallest tier
// accommodating any user in this batch). allAssetCounts is the
// per-deployment CEX asset capacity. batchCounts is the number of
// users per batch.
func NewBatchCreateUserCircuit(userAssetCounts, allAssetCounts, batchCounts uint32) *BatchCreateUserCircuit {
	c := &BatchCreateUserCircuit{
		BeforeCexAssets: make([]CexAssetInfo, allAssetCounts),
		CreateUserOps:   make([]CreateUserOperation, batchCounts),
	}
	for i := range c.CreateUserOps {
		c.CreateUserOps[i] = CreateUserOperation{
			Assets:             make([]UserAssetInfo, userAssetCounts),
			AssetsForUpdateCex: make([]UserAssetMeta, allAssetCounts),
		}
		for j := uint32(0); j < userAssetCounts; j++ {
			c.CreateUserOps[i].Assets[j].AssetIndex = j
		}
	}
	return c
}

// Define emits the spot_simple constraint set:
//
//  1. BatchCommitment == Poseidon(BeforeRoot, AfterRoot, BeforeCEX, AfterCEX)
//  2. BeforeCEXAssetsCommitment is the Poseidon of the packed BeforeCexAssets
//     (TotalEquity ∥ BasePrice per asset, one field element per asset).
//  3. Each user's account proof verifies (before) and updates (after).
//  4. Asset indexes in the user's Assets slice are strictly increasing
//     (uniqueness across the user's assets).
//  5. Linear-combination check: the user's Assets slice covers every
//     non-zero AssetsForUpdateCex entry. Challenge = Poseidon of the
//     per-user asset-id hashes ++ batch commitment.
//  6. AfterCEXAssetsCommitment is the Poseidon of the packed AfterCexAssets
//     (accumulated from BeforeCexAssets + per-user Equity deltas).
//  7. CreateUserOps roots chain (op[i].After == op[i+1].Before).
//  8. ConstraintModule hook (if non-nil) fires last with the same
//     before/after CEX view + per-user totals the base circuit produced.
func (b BatchCreateUserCircuit) Define(api API) error {
	actualBatchCommitment := corecircuit.BatchCommitment(
		api, b.BeforeAccountTreeRoot, b.AfterAccountTreeRoot,
		b.BeforeCEXAssetsCommitment, b.AfterCEXAssetsCommitment,
	)
	api.AssertIsEqual(b.BatchCommitment, actualBatchCommitment)

	const countOfCexAsset = 1 // {TotalEquity, BasePrice} packed in one field element
	cexAssets := make([]Variable, len(b.BeforeCexAssets)*countOfCexAsset)
	afterCexAssets := make([]CexAssetInfo, len(b.BeforeCexAssets))

	r := rangecheck.New(api)
	for i := range b.BeforeCexAssets {
		r.Check(b.BeforeCexAssets[i].TotalEquity, 64)
		r.Check(b.BeforeCexAssets[i].BasePrice, 64)

		fillCexAssetCommitment(api, b.BeforeCexAssets[i], i, cexAssets)
		afterCexAssets[i] = b.BeforeCexAssets[i]
	}
	actualCexAssetsCommitment := poseidon.Poseidon(api, cexAssets...)
	api.AssertIsEqual(b.BeforeCEXAssetsCommitment, actualCexAssetsCommitment)
	api.AssertIsEqual(b.BeforeAccountTreeRoot, b.CreateUserOps[0].BeforeAccountTreeRoot)
	api.AssertIsEqual(b.AfterAccountTreeRoot, b.CreateUserOps[len(b.CreateUserOps)-1].AfterAccountTreeRoot)

	userAssetIdHashes := make([]Variable, len(b.CreateUserOps)+1)
	userAssetsResults := make([][]Variable, len(b.CreateUserOps))
	userAssetsQueries := make([][]Variable, len(b.CreateUserOps))
	moduleUserOps := make([]spotspec.CircuitUserOp, len(b.CreateUserOps))

	for i := range b.CreateUserOps {
		accountIndexHelper := corecircuit.AccountIndexToMerkleHelper(api, b.CreateUserOps[i].AccountIndex)
		corecircuit.VerifyMerkleProof(
			api, b.CreateUserOps[i].BeforeAccountTreeRoot,
			EmptyAccountLeafNodeHash, b.CreateUserOps[i].AccountProof[:], accountIndexHelper,
		)

		userAssets := b.CreateUserOps[i].Assets

		// Per-slot Equity lookup table for the linear-combination check.
		userAssetsLookupTable := logderivlookup.New(api)
		for j := range b.CreateUserOps[i].AssetsForUpdateCex {
			userAssetsLookupTable.Insert(b.CreateUserOps[i].AssetsForUpdateCex[j].Equity)
		}

		// Strictly-increasing AssetIndex enforces uniqueness across the user's assets.
		for j := 0; j < len(userAssets)-1; j++ {
			r.Check(userAssets[j].AssetIndex, 16)
			cr := api.CmpNOp(userAssets[j+1].AssetIndex, userAssets[j].AssetIndex, 16, true)
			api.AssertIsEqual(cr, 1)
		}

		// Pack 15 asset indexes (each <2^16) per field element, then hash —
		// matches tier_3bucket's identical step so the challenge derivation
		// stays universal in shape.
		assetIdsToVariables := make([]Variable, (len(userAssets)+14)/15)
		for j := range assetIdsToVariables {
			var v Variable = 0
			for p := j * 15; p < (j+1)*15 && p < len(userAssets); p++ {
				v = api.Add(v, api.Mul(userAssets[p].AssetIndex, PowersOfSixteenBits[p%15]))
			}
			assetIdsToVariables[j] = v
		}
		userAssetIdHashes[i] = poseidon.Poseidon(api, assetIdsToVariables...)

		// One lookup query per asset for the linear-combination cross-check.
		userAssetsQueries[i] = make([]Variable, len(userAssets))
		flattenAssetFieldsForHash := make([]Variable, len(userAssets)*2)
		for j := range userAssets {
			userAssetsQueries[i][j] = userAssets[j].AssetIndex

			r.Check(userAssets[j].Equity, 64)
			flattenAssetFieldsForHash[j*2] = userAssets[j].AssetIndex
			flattenAssetFieldsForHash[j*2+1] = userAssets[j].Equity
		}
		userAssetsResults[i] = userAssetsLookupTable.Lookup(userAssetsQueries[i]...)

		// Cross-check: each lookup result must equal the user's claimed Equity.
		// This binds the user's Assets slice values to the AssetsForUpdateCex
		// accumulation vector at the same indexes.
		var totalUserEquity Variable = 0
		for j := range userAssets {
			api.AssertIsEqual(userAssetsResults[i][j], userAssets[j].Equity)
			totalUserEquity = api.Add(totalUserEquity, userAssets[j].Equity)
		}
		r.Check(totalUserEquity, 128)

		// Accumulate per-slot equity into the running AfterCex view.
		for j := range b.CreateUserOps[i].AssetsForUpdateCex {
			afterCexAssets[j].TotalEquity = api.Add(
				afterCexAssets[j].TotalEquity,
				b.CreateUserOps[i].AssetsForUpdateCex[j].Equity,
			)
		}

		// Account leaf is 5-input Poseidon with debt and collateral positions
		// pinned to zero — keeps the universal core/tree empty-leaf hash
		// (Poseidon(0,0,0,0,0)) valid for spot.
		userAssetsCommitment := corecircuit.ComputeFlatUint64Commitment(api, flattenAssetFieldsForHash)
		accountHash := poseidon.Poseidon(
			api, b.CreateUserOps[i].AccountIdHash, totalUserEquity, 0, 0, userAssetsCommitment,
		)
		actualAccountTreeRoot := corecircuit.UpdateMerkleProof(
			api, accountHash, b.CreateUserOps[i].AccountProof[:], accountIndexHelper,
		)
		api.AssertIsEqual(actualAccountTreeRoot, b.CreateUserOps[i].AfterAccountTreeRoot)

		moduleUserOps[i] = spotspec.CircuitUserOp{
			AccountIndex:    b.CreateUserOps[i].AccountIndex,
			AccountIDHash:   b.CreateUserOps[i].AccountIdHash,
			TotalUserEquity: totalUserEquity,
		}
	}

	// Random-linear-combination cross-check across the whole batch: the
	// user.Assets sequence must cover every non-zero AssetsForUpdateCex
	// entry. Challenge = Poseidon(userAssetIdHashes... ∥ batchCommitment).
	// Only one challenge power per CEX slot (vs five for tier_3bucket)
	// since spot users carry only Equity.
	userAssetIdHashes[len(b.CreateUserOps)] = b.BatchCommitment
	randomChallenge := poseidon.Poseidon(api, userAssetIdHashes...)
	powersOfRandomChallenge := make([]Variable, len(b.BeforeCexAssets))
	powersOfRandomChallenge[0] = randomChallenge
	powersOfRandomChallengeLookupTable := logderivlookup.New(api)
	powersOfRandomChallengeLookupTable.Insert(randomChallenge)
	for i := 1; i < len(powersOfRandomChallenge); i++ {
		powersOfRandomChallenge[i] = api.Mul(powersOfRandomChallenge[i-1], randomChallenge)
		powersOfRandomChallengeLookupTable.Insert(powersOfRandomChallenge[i])
	}

	for i := range b.CreateUserOps {
		powersOfRCResults := powersOfRandomChallengeLookupTable.Lookup(userAssetsQueries[i]...)
		var sumA Variable = 0
		for j := range powersOfRCResults {
			sumA = api.Add(sumA, api.Mul(powersOfRCResults[j], userAssetsResults[i][j]))
		}
		var sumB Variable = 0
		for j := range b.CreateUserOps[i].AssetsForUpdateCex {
			sumB = api.Add(sumB, api.Mul(b.CreateUserOps[i].AssetsForUpdateCex[j].Equity, powersOfRandomChallenge[j]))
		}
		api.AssertIsEqual(sumA, sumB)
	}

	// After-CEX commitment over the accumulated state.
	tempAfterCexAssets := make([]Variable, len(b.BeforeCexAssets)*countOfCexAsset)
	for j := range b.BeforeCexAssets {
		r.Check(afterCexAssets[j].TotalEquity, 64)
		fillCexAssetCommitment(api, afterCexAssets[j], j, tempAfterCexAssets)
	}
	actualAfterCEXAssetsCommitment := poseidon.Poseidon(api, tempAfterCexAssets...)
	api.AssertIsEqual(actualAfterCEXAssetsCommitment, b.AfterCEXAssetsCommitment)

	for i := 0; i < len(b.CreateUserOps)-1; i++ {
		api.AssertIsEqual(b.CreateUserOps[i].AfterAccountTreeRoot, b.CreateUserOps[i+1].BeforeAccountTreeRoot)
	}

	if b.module != nil {
		ctx := spotspec.ConstraintContext{
			BeforeCexAssets: toCircuitCexAssetView(b.BeforeCexAssets),
			AfterCexAssets:  toCircuitCexAssetView(afterCexAssets),
			UserOps:         moduleUserOps,
			R:               r,
		}
		if err := b.module.Define(api, ctx); err != nil {
			return err
		}
	}
	return nil
}

// fillCexAssetCommitment writes the field-element representation of one
// CexAssetInfo into commitments[currentIndex]. spot_simple packs
// TotalEquity ∥ BasePrice in a single field element (each 64 bits, so
// the 128-bit packing fits well under the field modulus).
func fillCexAssetCommitment(api API, asset CexAssetInfo, currentIndex int, commitments []Variable) {
	commitments[currentIndex] = api.Add(
		api.Mul(asset.TotalEquity, corecircuit.TwoToTheSixtyFour),
		asset.BasePrice,
	)
}

// toCircuitCexAssetView translates the in-circuit CexAssetInfo slice
// into the spotspec.CircuitCexAsset view shape exposed to
// ConstraintModule hooks. Field types match underneath, so this is a
// flat copy — no in-circuit constraints emitted.
func toCircuitCexAssetView(src []CexAssetInfo) []spotspec.CircuitCexAsset {
	out := make([]spotspec.CircuitCexAsset, len(src))
	for i := range src {
		out[i] = spotspec.CircuitCexAsset{
			TotalEquity: src[i].TotalEquity,
			BasePrice:   src[i].BasePrice,
		}
	}
	return out
}
