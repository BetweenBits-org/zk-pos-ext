package spec

// InvalidAccountAction is the disposition for an account that fails
// model-level validation (e.g. user solvency check in t4_tiered_haircut_margin_3pool).
type InvalidAccountAction int

const (
	// InvalidActionDrop excludes the account from the proof entirely.
	// Use when the account is in a known-bad state that the exchange
	// considers a normal pre-snapshot reality (e.g. liquidation pending).
	InvalidActionDrop InvalidAccountAction = iota

	// InvalidActionAbort halts proof generation. Use when discovering
	// invalid accounts indicates a snapshot or ETL bug that MUST be
	// resolved before publishing the proof.
	InvalidActionAbort

	// InvalidActionQuarantine sets the account aside for manual review
	// while continuing proof generation for the rest. Quarantined
	// accounts MUST be resolved before the next snapshot.
	InvalidActionQuarantine
)

// InvalidAccountPolicy decides what to do with accounts that fail
// validation. The set of validation rules is model-defined (e.g.
// t4_tiered_haircut_margin_3pool checks totalCollateral >= totalDebt); this interface
// only governs disposition, not detection.
//
// Model-independent — every solvency model has *some* notion of
// invalid input.
type InvalidAccountPolicy interface {
	// OnInsolventAccount is called for each account that fails the
	// model's solvency validation. The opaque `reason` string is
	// informational and may be logged. The returned action drives
	// downstream behaviour.
	OnInsolventAccount(internalUserID string, reason string) InvalidAccountAction
}
