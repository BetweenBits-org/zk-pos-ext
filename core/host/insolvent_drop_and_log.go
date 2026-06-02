package host

import (
	"fmt"

	"github.com/BetweenBits-org/zk-pos-ext/core/spec"
)

// InsolventActionDropAndLogV0 is the v1 frozen action ID for the
// canonical engine drop-and-log policy (G7 default).
//
// Behaviour: every account that fails the solvency model's
// validation is dropped from the proof — its row is excluded from
// the witness — and a single log line is emitted to stderr with the
// internal user ID and the validator's reason string. The action
// returned is spec.InvalidActionDrop.
//
// This is the default disposition for every v1 catalog model — fires
// on totalCollateral < totalDebt for the margin variants
// (T2/T3/T4) and on data-quality failures (hex-decode, uint64 overflow)
// for T1. Promoted from per-profile copies in R8-A.
//
// Customers needing abort-on-invalid or quarantine semantics MUST
// register a separate action ID under this registry rather than
// re-using "drop_and_log".
const InsolventActionDropAndLogV0 = "drop_and_log.v0"

func init() {
	RegisterInsolventPolicy(InsolventActionDropAndLogV0, newDropAndLogV0)
}

type dropAndLogV0 struct{}

var dropAndLogV0Instance = dropAndLogV0{}

func newDropAndLogV0() spec.InvalidAccountPolicy { return dropAndLogV0Instance }

func (dropAndLogV0) OnInsolventAccount(internalUserID string, reason string) spec.InvalidAccountAction {
	fmt.Println("invalid account dropped:", internalUserID, "reason:", reason)
	return spec.InvalidActionDrop
}
