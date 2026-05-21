package spec

import "fmt"

// BatchShape defines the dimensions of a single circuit instance.
// Each shape maps 1:1 to a {pk, vk, r1cs} key triplet produced by keygen.
//
// One solvency model may support multiple shapes; one shape may be
// instantiated under multiple models. Key files identify both
// (see StandardKeyName).
type BatchShape struct {
	// AssetCountTier is the maximum number of (non-empty) asset entries
	// a user in this batch may hold. Maps to circuit param userAssetCounts.
	AssetCountTier int

	// UsersPerBatch is the number of user operations per circuit
	// instance. Maps to circuit param batchCounts.
	UsersPerBatch int
}

// NoExtensionID is the moduleID used when a circuit instance carries
// no ConstraintModule extension. Key names omit the module segment in
// this case.
const NoExtensionID = ""

// StandardKeyName returns the canonical filename stem for the key
// files associated with a (model, shape, optional module) triple.
//
//	zkpor.<model>.<assetTier>_<usersPerBatch>          (no module)
//	zkpor.<model>.<assetTier>_<usersPerBatch>.<module>
//
// Concrete files are "<stem>.pk", "<stem>.vk", "<stem>.r1cs".
func (s BatchShape) StandardKeyName(model SolvencyModelID, module string) string {
	if module == NoExtensionID {
		return fmt.Sprintf("zkpor.%s.%d_%d", model, s.AssetCountTier, s.UsersPerBatch)
	}
	return fmt.Sprintf("zkpor.%s.%d_%d.%s", model, s.AssetCountTier, s.UsersPerBatch, module)
}

// LegacyKeyName returns the pre-engine naming scheme — "zkpor50_700".
// Provided so existing tier_3bucket deployments can read their old
// keys without re-running keygen. New deployments SHOULD use
// StandardKeyName.
func (s BatchShape) LegacyKeyName() string {
	return fmt.Sprintf("zkpor%d_%d", s.AssetCountTier, s.UsersPerBatch)
}

// BatchShapeProvider supplies the set of batch shapes a customer
// deployment runs for a given solvency model.
//
//   - keygen iterates Shapes() to build every required (pk, vk, r1cs).
//   - witness routes each user into a batch via SelectFor(nonEmptyAssets).
//   - prover/verifier resolve the key file via KeyName(shape, module).
//
// Implementations MUST return shapes in ascending AssetCountTier order
// and MUST be deterministic. Each AssetCountTier value MUST be unique.
type BatchShapeProvider interface {
	// Model returns the solvency model this provider's shapes target.
	Model() SolvencyModelID

	// Shapes returns all shapes (ascending AssetCountTier).
	Shapes() []BatchShape

	// SelectFor returns the smallest shape whose AssetCountTier is at
	// least nonEmptyAssetCount. Returns a non-nil error if none fits.
	SelectFor(nonEmptyAssetCount int) (BatchShape, error)

	// KeyName returns the key-file stem for a (shape, module) pair.
	// Implementations SHOULD delegate to BatchShape.StandardKeyName.
	KeyName(s BatchShape, module string) string
}
