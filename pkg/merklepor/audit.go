package merklepor

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
)

// AuditReport is the auditor's reconciliation result: the engine half of
// the Stage MS trust deliverable. It embeds the core reconcile Report
// (recomputed total + violations) and, when a reserves figure was
// supplied, the Reserves >= Liabilities solvency verdict.
type AuditReport struct {
	corehost.Report
	// ReservesChecked is true when a reserves total was supplied.
	ReservesChecked bool
	// Reserves is the audited on-chain reserves total that was compared
	// (nil when ReservesChecked is false).
	Reserves *big.Int
	// Solvent reports Reserves >= recomputed liabilities; meaningful only
	// when ReservesChecked is true.
	Solvent bool
}

// RunAudit recomputes the reconcile report (non-negative / duplicate /
// range / sum-equality) over the full leaf set and, given an audited
// reserves total (Options.Reserves), asserts Reserves >= Liabilities.
// Reserves are an input, not queried on-chain (gate G19, D4). Returns the
// report; a non-OK report is not itself an error (the caller decides how
// to act on violations), but an unreadable snapshot is.
func RunAudit(ctx context.Context, opts Options) (*AuditReport, error) {
	if opts.Snapshot == nil {
		return nil, fmt.Errorf("merklepor: Snapshot is required")
	}
	r, err := resolve(opts)
	if err != nil {
		return nil, err
	}
	leaves, err := dispatchCollectSumLeaves(r.model, collectDeps{
		ctx: ctx, sourceType: r.sourceType, snapshot: opts.Snapshot,
		snapID: r.snapID, capacity: r.capacity, pricing: r.pricing, tiers: r.tiers,
	})
	if err != nil {
		return nil, fmt.Errorf("merklepor: collect leaves: %w", err)
	}

	liab := make([]corehost.LiabilityLeaf, len(leaves))
	for i, lf := range leaves {
		liab[i] = corehost.LiabilityLeaf{Index: uint32(lf.Position), Id: hex.EncodeToString(lf.AccountID), Balance: lf.Balance}
	}
	ar := &AuditReport{Report: corehost.Reconcile(liab, opts.MaxBalance, opts.PublishedTotal)}
	if opts.Reserves != nil {
		ar.ReservesChecked = true
		ar.Reserves = opts.Reserves
		ar.Solvent = opts.Reserves.Cmp(ar.Total) >= 0
	}

	fmt.Printf("merklepor audit: leaves=%d total=%s violations=%d ok=%v\n", len(leaves), ar.Total, len(ar.Violations), ar.OK())
	if ar.ReservesChecked {
		fmt.Printf("merklepor audit: reserves=%s liabilities=%s solvent=%v\n", ar.Reserves, ar.Total, ar.Solvent)
	}
	return ar, nil
}
