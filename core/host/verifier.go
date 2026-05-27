package host

// VerifierPlan is the universal (model-blind) plan the verifier walks
// for both batch mode (verifying-key per tier) and -user mode (asset
// count tiers for user-leaf padding). cmd/verifier derives this from
// profile.toml + the -keys-dir flag, then hands it off to the per-model
// runner that knows how to use it.
type VerifierPlan struct {
	AssetCapacity   int
	AssetCountTiers []int
	// ZkKeyStems[i] is the (path-prefixed) artifact stem for tier
	// AssetCountTiers[i]; append .vk/.r1cs/.pk for the artifact file.
	ZkKeyStems []string
}

// ProofRow is the model-blind row shape cmd/verifier loads from the
// prover's proof table (DB or CSV). Mirrors what the prover writes:
// fields are wire-shaped (base64 strings for proof bytes / commitments).
type ProofRow struct {
	BatchNumber        int64    `csv:"batch_number"`
	ZkProof            string   `csv:"proof_info"`
	CexAssetCommitment []string `csv:"cex_asset_list_commitments"`
	AccountTreeRoots   []string `csv:"account_tree_roots"`
	BatchCommitment    string   `csv:"batch_commitment"`
	AssetsCount        int      `csv:"assets_count"`
}
