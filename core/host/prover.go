package host

// BatchProofResult is the model-blind output of one prove cycle that
// cmd/prover persists into the proof table. Each model's
// DecodeAndProve runner produces this shape after running
// groth16.Prove + groth16.Verify against its model-typed circuit.
//
// All byte fields are 32-byte Poseidon outputs. Proof carries the
// raw uncompressed serialization (proof.WriteRawTo) — cmd/prover
// base64-encodes it for the DB row.
type BatchProofResult struct {
	// AssetsCount is the in-circuit padded tier the prover used. The
	// verifier reads this from the proof row to pick the matching
	// verifying key.
	AssetsCount int

	// ProofRaw is the uncompressed groth16 proof bytes.
	ProofRaw []byte

	// BatchCommitment is the Poseidon-of-state-pairs commitment the
	// circuit's public input asserts equals.
	BatchCommitment []byte

	// Before/After roots + cex commitments describe the state
	// transition the proof attests.
	BeforeAccountTreeRoot     []byte
	AfterAccountTreeRoot      []byte
	BeforeCEXAssetsCommitment []byte
	AfterCEXAssetsCommitment  []byte
}
