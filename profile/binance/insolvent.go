package binance

import (
	"fmt"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

// insolventPolicy matches the legacy utils.ReadUserDataFromCsvFile
// behaviour: accounts whose total collateral < total debt are dropped
// and the occurrence is logged.
type insolventPolicy struct{}

// NewInsolventPolicy returns Binance's InvalidAccountPolicy (drop + log).
func NewInsolventPolicy() spec.InvalidAccountPolicy { return insolventPolicy{} }

func (insolventPolicy) OnInsolventAccount(internalUserID string, reason string) spec.InvalidAccountAction {
	fmt.Println("invalid account dropped:", internalUserID, "reason:", reason)
	return spec.InvalidActionDrop
}
