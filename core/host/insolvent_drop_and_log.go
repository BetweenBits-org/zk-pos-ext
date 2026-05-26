package host

import (
	"fmt"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
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
// This is the default disposition for both profile/binance
// (t4_tiered_haircut_margin_3pool, where it mostly fires on totalCollateral <
// totalDebt) and profile/sea_reference (t1_simple_margin, where it
// mostly fires on data-quality failures like hex-decode and uint64
// overflow). Promoted from per-profile copies in R8-A.
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
