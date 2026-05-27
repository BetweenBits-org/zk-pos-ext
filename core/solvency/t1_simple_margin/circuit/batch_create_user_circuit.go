package circuit

import (
	corecircuit "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/circuit"
	t1spec "github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/solvency/t1_simple_margin/spec"
	"github.com/consensys/gnark/std/hash/poseidon"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"github.com/consensys/gnark/std/rangecheck"
)

// BatchCreateUserCircuit is the gnark Circuit type for the
// t1_simple_margin model. Substantially simpler than
// t4_tiered_haircut_margin_3pool — no haircut, no 3-bucket collateral,
// no tier table — but enforces:
//
//   - per-asset sum equality (After = Before + Δ on the CEX side, for
//     both Equity and Debt)
//   - per-user account-level TotalEquity ≥ TotalDebt
//   - Merkle tree before/after consistency per user op.
//
// Spot customers use this circuit with Debt = 0 throughout (sum
// equality on debt becomes 0 = 0, account-level constraint trivially
// satisfied). See docs/04-solvency-models.md §4 for the absorption
// trail.
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

	module t1spec.ConstraintModule
}

// SetConstraintModule wires the alpha-layer ConstraintModule hook
// invoked at the end of Define after every base constraint has been
// emitted. Setting nil (the default) reverts to the no-hook shape.
//
// Composing a non-nil module forks the trusted setup: the resulting
// .pk/.vk pair is unique to the (t1_simple_margin, module) pair.
func (b *BatchCreateUserCircuit) SetConstraintModule(m t1spec.ConstraintModule) {
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

// Define emits the t1_simple_margin constraint set:
//
//  1. BatchCommitment == Poseidon(BeforeRoot, AfterRoot, BeforeCEX, AfterCEX)
//  2. BeforeCEXAssetsCommitment is the Poseidon of the packed BeforeCexAssets
//     (TotalEquity ∥ TotalDebt ∥ BasePrice per asset, one field element per asset).
//  3. Each user's account proof verifies (before) and updates (after).
//  4. Asset indexes in the user's Assets slice are strictly increasing
//     (uniqueness across the user's assets).
//  5. Linear-combination check (twice — Equity side AND Debt side): the
//     user's Assets slice covers every non-zero AssetsForUpdateCex entry.
//     Challenge = Poseidon of the per-user asset-id hashes ++ batch commitment.
//  6. Per-user TotalEquity ≥ TotalDebt (account-level solvency).
//  7. AfterCEXAssetsCommitment is the Poseidon of the packed AfterCexAssets
//     (accumulated from BeforeCexAssets + per-user Equity / Debt deltas).
//  8. CreateUserOps roots chain (op[i].After == op[i+1].Before).
//  9. ConstraintModule hook (if non-nil) fires last with the same
//     before/after CEX view + per-user totals the base circuit produced.
func (b BatchCreateUserCircuit) Define(api API) error {
	actualBatchCommitment := corecircuit.BatchCommitment(
		api, b.BeforeAccountTreeRoot, b.AfterAccountTreeRoot,
		b.BeforeCEXAssetsCommitment, b.AfterCEXAssetsCommitment,
	)
	api.AssertIsEqual(b.BatchCommitment, actualBatchCommitment)

	const countOfCexAsset = 1 // {TotalEquity, TotalDebt, BasePrice} packed in one field element (192 bits < bn254 modulus)
	cexAssets := make([]Variable, len(b.BeforeCexAssets)*countOfCexAsset)
	afterCexAssets := make([]CexAssetInfo, len(b.BeforeCexAssets))

	// Per-asset BasePrice lookup table. Mirrors the T4 pattern: the
	// per-user TotalEquity / TotalDebt totals folded into the account
	// leaf hash are *USD-scaled* (Σ equity × basePrice), not raw asset
	// quantities. Raw cross-asset sums are unit-meaningless and would
	// make AssertIsLessOrEqual(debt, equity) a contentless check.
	// Identical to T4's assetPriceTable construction.
	assetPriceTable := logderivlookup.New(api)

	r := rangecheck.New(api)
	for i := range b.BeforeCexAssets {
		r.Check(b.BeforeCexAssets[i].TotalEquity, 64)
		r.Check(b.BeforeCexAssets[i].TotalDebt, 64)
		r.Check(b.BeforeCexAssets[i].BasePrice, 64)

		fillCexAssetCommitment(api, b.BeforeCexAssets[i], i, cexAssets)
		afterCexAssets[i] = b.BeforeCexAssets[i]
		assetPriceTable.Insert(b.BeforeCexAssets[i].BasePrice)
	}
	actualCexAssetsCommitment := poseidon.Poseidon(api, cexAssets...)
	api.AssertIsEqual(b.BeforeCEXAssetsCommitment, actualCexAssetsCommitment)
	api.AssertIsEqual(b.BeforeAccountTreeRoot, b.CreateUserOps[0].BeforeAccountTreeRoot)
	api.AssertIsEqual(b.AfterAccountTreeRoot, b.CreateUserOps[len(b.CreateUserOps)-1].AfterAccountTreeRoot)

	userAssetIdHashes := make([]Variable, len(b.CreateUserOps)+1)
	userAssetsEquityResults := make([][]Variable, len(b.CreateUserOps))
	userAssetsDebtResults := make([][]Variable, len(b.CreateUserOps))
	userAssetsQueries := make([][]Variable, len(b.CreateUserOps))
	moduleUserOps := make([]t1spec.CircuitUserOp, len(b.CreateUserOps))

	for i := range b.CreateUserOps {
		accountIndexHelper := corecircuit.AccountIndexToMerkleHelper(api, b.CreateUserOps[i].AccountIndex)
		corecircuit.VerifyMerkleProof(
			api, b.CreateUserOps[i].BeforeAccountTreeRoot,
			EmptyAccountLeafNodeHash, b.CreateUserOps[i].AccountProof[:], accountIndexHelper,
		)

		userAssets := b.CreateUserOps[i].Assets

		// Per-slot Equity AND Debt lookup tables for the linear-combination check.
		userEquityLookup := logderivlookup.New(api)
		userDebtLookup := logderivlookup.New(api)
		for j := range b.CreateUserOps[i].AssetsForUpdateCex {
			userEquityLookup.Insert(b.CreateUserOps[i].AssetsForUpdateCex[j].Equity)
			userDebtLookup.Insert(b.CreateUserOps[i].AssetsForUpdateCex[j].Debt)
		}

		// Strictly-increasing AssetIndex enforces uniqueness across the user's assets.
		for j := 0; j < len(userAssets)-1; j++ {
			r.Check(userAssets[j].AssetIndex, 16)
			cr := api.CmpNOp(userAssets[j+1].AssetIndex, userAssets[j].AssetIndex, 16, true)
			api.AssertIsEqual(cr, 1)
		}

		// Pack 15 asset indexes (each <2^16) per field element, then hash —
		// matches t4_tiered_haircut_margin_3pool's identical step so the
		// challenge derivation stays universal in shape.
		assetIdsToVariables := make([]Variable, (len(userAssets)+14)/15)
		for j := range assetIdsToVariables {
			var v Variable = 0
			for p := j * 15; p < (j+1)*15 && p < len(userAssets); p++ {
				v = api.Add(v, api.Mul(userAssets[p].AssetIndex, PowersOfSixteenBits[p%15]))
			}
			assetIdsToVariables[j] = v
		}
		userAssetIdHashes[i] = poseidon.Poseidon(api, assetIdsToVariables...)

		// One lookup query per asset for the linear-combination cross-check
		// (same queries serve both Equity and Debt lookups).
		userAssetsQueries[i] = make([]Variable, len(userAssets))
		flattenAssetFieldsForHash := make([]Variable, len(userAssets)*3)
		for j := range userAssets {
			userAssetsQueries[i][j] = userAssets[j].AssetIndex

			r.Check(userAssets[j].Equity, 64)
			r.Check(userAssets[j].Debt, 64)
			flattenAssetFieldsForHash[j*3] = userAssets[j].AssetIndex
			flattenAssetFieldsForHash[j*3+1] = userAssets[j].Equity
			flattenAssetFieldsForHash[j*3+2] = userAssets[j].Debt
		}
		userAssetsEquityResults[i] = userEquityLookup.Lookup(userAssetsQueries[i]...)
		userAssetsDebtResults[i] = userDebtLookup.Lookup(userAssetsQueries[i]...)

		// Per-user assetPriceResponses[j] = BasePrice[userAssets[j].Index].
		// Used to compute USD-scaled totals (totalUserEquity /
		// totalUserDebt) the account leaf hash binds. Same T4 pattern.
		assetPriceResponses := assetPriceTable.Lookup(userAssetsQueries[i]...)

		// Cross-check: each lookup result must equal the user's claimed
		// Equity / Debt at the same asset index. This binds the user's
		// Assets slice values to the AssetsForUpdateCex accumulation
		// vector at the same indexes.
		var totalUserEquity Variable = 0
		var totalUserDebt Variable = 0
		for j := range userAssets {
			api.AssertIsEqual(userAssetsEquityResults[i][j], userAssets[j].Equity)
			api.AssertIsEqual(userAssetsDebtResults[i][j], userAssets[j].Debt)
			// USD-scaled accumulation: equity × basePrice. Matches the
			// host-side AccountLeafHash input (account.TotalEquity is
			// the parser's Σ equity×basePrice).
			totalUserEquity = api.Add(totalUserEquity, api.Mul(userAssets[j].Equity, assetPriceResponses[j]))
			totalUserDebt = api.Add(totalUserDebt, api.Mul(userAssets[j].Debt, assetPriceResponses[j]))
		}
		r.Check(totalUserEquity, 128)
		r.Check(totalUserDebt, 128)

		// Account-level solvency: TotalEquity ≥ TotalDebt in USD value.
		// Trivially satisfied for spot users (debt=0). The defining T1
		// constraint. NOp form (T4-style) for 128-bit scaled values.
		api.AssertIsLessOrEqualNOp(totalUserDebt, totalUserEquity, 128, true)

		// Accumulate per-slot equity AND debt into the running AfterCex view.
		for j := range b.CreateUserOps[i].AssetsForUpdateCex {
			afterCexAssets[j].TotalEquity = api.Add(
				afterCexAssets[j].TotalEquity,
				b.CreateUserOps[i].AssetsForUpdateCex[j].Equity,
			)
			afterCexAssets[j].TotalDebt = api.Add(
				afterCexAssets[j].TotalDebt,
				b.CreateUserOps[i].AssetsForUpdateCex[j].Debt,
			)
		}

		// Account leaf: universal 5-input Poseidon
		// (AccountID, TotalEquity, TotalDebt, 0, AssetsCommitment).
		// Slot 4 (TotalCollateral) is pinned to zero — T1 has no
		// risk-weighted collateral. Empty slots remain at the universal
		// core/tree empty-leaf hash (Poseidon(0,0,0,0,0)).
		userAssetsCommitment := corecircuit.ComputeFlatUint64Commitment(api, flattenAssetFieldsForHash)
		accountHash := poseidon.Poseidon(
			api, b.CreateUserOps[i].AccountIdHash, totalUserEquity, totalUserDebt, 0, userAssetsCommitment,
		)
		actualAccountTreeRoot := corecircuit.UpdateMerkleProof(
			api, accountHash, b.CreateUserOps[i].AccountProof[:], accountIndexHelper,
		)
		api.AssertIsEqual(actualAccountTreeRoot, b.CreateUserOps[i].AfterAccountTreeRoot)

		moduleUserOps[i] = t1spec.CircuitUserOp{
			AccountIndex:    b.CreateUserOps[i].AccountIndex,
			AccountIDHash:   b.CreateUserOps[i].AccountIdHash,
			TotalUserEquity: totalUserEquity,
			TotalUserDebt:   totalUserDebt,
		}
	}

	// Random-linear-combination cross-check across the whole batch: the
	// user.Assets sequence must cover every non-zero AssetsForUpdateCex
	// entry, for BOTH Equity and Debt. Challenge = Poseidon(userAssetIdHashes
	// ∥ batchCommitment); one challenge power per CEX slot, applied twice.
	userAssetIdHashes[len(b.CreateUserOps)] = b.BatchCommitment
	randomChallenge := poseidon.Poseidon(api, userAssetIdHashes...)
	powersOfRandomChallenge := make([]Variable, len(b.BeforeCexAssets))
	powersOfRandomChallenge[0] = randomChallenge
	powersOfRCLookup := logderivlookup.New(api)
	powersOfRCLookup.Insert(randomChallenge)
	for i := 1; i < len(powersOfRandomChallenge); i++ {
		powersOfRandomChallenge[i] = api.Mul(powersOfRandomChallenge[i-1], randomChallenge)
		powersOfRCLookup.Insert(powersOfRandomChallenge[i])
	}

	for i := range b.CreateUserOps {
		powersOfRCResults := powersOfRCLookup.Lookup(userAssetsQueries[i]...)

		// Equity side.
		var sumAEquity Variable = 0
		for j := range powersOfRCResults {
			sumAEquity = api.Add(sumAEquity, api.Mul(powersOfRCResults[j], userAssetsEquityResults[i][j]))
		}
		var sumBEquity Variable = 0
		for j := range b.CreateUserOps[i].AssetsForUpdateCex {
			sumBEquity = api.Add(sumBEquity, api.Mul(b.CreateUserOps[i].AssetsForUpdateCex[j].Equity, powersOfRandomChallenge[j]))
		}
		api.AssertIsEqual(sumAEquity, sumBEquity)

		// Debt side.
		var sumADebt Variable = 0
		for j := range powersOfRCResults {
			sumADebt = api.Add(sumADebt, api.Mul(powersOfRCResults[j], userAssetsDebtResults[i][j]))
		}
		var sumBDebt Variable = 0
		for j := range b.CreateUserOps[i].AssetsForUpdateCex {
			sumBDebt = api.Add(sumBDebt, api.Mul(b.CreateUserOps[i].AssetsForUpdateCex[j].Debt, powersOfRandomChallenge[j]))
		}
		api.AssertIsEqual(sumADebt, sumBDebt)
	}

	// After-CEX commitment over the accumulated state.
	tempAfterCexAssets := make([]Variable, len(b.BeforeCexAssets)*countOfCexAsset)
	for j := range b.BeforeCexAssets {
		r.Check(afterCexAssets[j].TotalEquity, 64)
		r.Check(afterCexAssets[j].TotalDebt, 64)
		fillCexAssetCommitment(api, afterCexAssets[j], j, tempAfterCexAssets)
	}
	actualAfterCEXAssetsCommitment := poseidon.Poseidon(api, tempAfterCexAssets...)
	api.AssertIsEqual(actualAfterCEXAssetsCommitment, b.AfterCEXAssetsCommitment)

	for i := 0; i < len(b.CreateUserOps)-1; i++ {
		api.AssertIsEqual(b.CreateUserOps[i].AfterAccountTreeRoot, b.CreateUserOps[i+1].BeforeAccountTreeRoot)
	}

	if b.module != nil {
		ctx := t1spec.ConstraintContext{
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
// CexAssetInfo into commitments[currentIndex]. t1_simple_margin packs
// TotalEquity ∥ TotalDebt ∥ BasePrice in a single field element
// (3*64 = 192 bits, well under the bn254 modulus ~254 bits) — matches
// t4_tiered_haircut_margin_3pool's 3-field-per-asset packing layout
// (without the Loan/Margin/PM extension).
func fillCexAssetCommitment(api API, asset CexAssetInfo, currentIndex int, commitments []Variable) {
	commitments[currentIndex] = api.Add(
		api.Add(
			api.Mul(asset.TotalEquity, corecircuit.TwoToTheOneTwentyEight),
			api.Mul(asset.TotalDebt, corecircuit.TwoToTheSixtyFour),
		),
		asset.BasePrice,
	)
}

// toCircuitCexAssetView translates the in-circuit CexAssetInfo slice
// into the t1spec.CircuitCexAsset view shape exposed to
// ConstraintModule hooks. Field types match underneath, so this is a
// flat copy — no in-circuit constraints emitted.
func toCircuitCexAssetView(src []CexAssetInfo) []t1spec.CircuitCexAsset {
	out := make([]t1spec.CircuitCexAsset, len(src))
	for i := range src {
		out[i] = t1spec.CircuitCexAsset{
			TotalEquity: src[i].TotalEquity,
			TotalDebt:   src[i].TotalDebt,
			BasePrice:   src[i].BasePrice,
		}
	}
	return out
}
