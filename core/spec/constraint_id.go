package spec

// ConstraintModuleID identifies a model-extension constraint module
// in key file names. The string becomes a segment of the .pk/.vk
// filename, so it MUST be filesystem-safe (lowercase letters, digits,
// dots, underscores; no slashes, no spaces).
//
// The ConstraintModule *interface* itself is model-specific (the
// Define() signature varies by model context type) and is therefore
// defined under each core/solvency/<model>/spec/ package. This file
// declares only the universal ID type and naming conventions.
//
// Convention:
//
//	<exchange>.<rule>_v<version>
//	e.g. "upbit.kor_regulator_v1", "binance.concentration_v1"
//
// Modules MUST publish source so verifiers can audit what additional
// constraints a given .vk encodes.
type ConstraintModuleID string

// Noop is the conventional ID for "no additional constraints".
// Use ConstraintModuleID(NoExtensionID) (empty string) to omit the
// module segment entirely from key names.
const Noop ConstraintModuleID = "noop"
