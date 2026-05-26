package sea_reference

import (
	"fmt"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// insolventPolicy: drop + log. Same default as profile/binance — invalid
// accounts (malformed row, balance overflow, etc.) are excluded from
// the snapshot, with a log line for audit.
//
// The "insolvent" naming is held over from t4_tiered_haircut_margin_3pool's vocabulary
// (where it primarily denoted accounts with TotalCollateral < TotalDebt).
// For t1_simple_margin there's no per-account solvency invariant — the
// policy here mostly fires on data-quality failures (hex decode, uint64
// overflow). The name stays for interface compatibility.
type insolventPolicy struct{}

// NewInsolventPolicy returns sea_reference's InvalidAccountPolicy.
func NewInsolventPolicy() spec.InvalidAccountPolicy { return insolventPolicy{} }

func (insolventPolicy) OnInsolventAccount(internalUserID string, reason string) spec.InvalidAccountAction {
	fmt.Println("invalid account dropped:", internalUserID, "reason:", reason)
	return spec.InvalidActionDrop
}
