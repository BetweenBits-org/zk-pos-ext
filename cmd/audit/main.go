// Command audit is the CLI shim around pkg/merklepor.RunAudit — the
// auditor reconciliation pass for the non-zk Merkle-sum proof-of-
// liabilities side line (PRODUCTION_ROADMAP Stage MS). It recomputes the
// reconcile report (non-negative / duplicate / range / sum-equality) over
// the full leaf set and, given an audited reserves total, asserts
// Reserves >= Liabilities. Reserves are supplied as a flag, not queried
// on-chain (gate G19, D4). No DB is needed — audit only reads the snapshot.
//
// Exit non-zero on any reconcile violation or on insolvency.
//
// Usage:
//
//	audit -profile path/to/profile.toml \
//	    [-user-data-dir DIR] [-snapshot-id ID] [-asset-capacity N] \
//	    [-published-total D] [-max-balance D] [-reserves D]
package main

import (
	"context"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"syscall"

	"github.com/BetweenBits-org/zk-pos-ext/core/io/vfs/osvfs"
	"github.com/BetweenBits-org/zk-pos-ext/pkg/merklepor"
	"github.com/BetweenBits-org/zk-pos-ext/profile/declarative"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	userDataDir := flag.String("user-data-dir", "", "override profile.snapshot.user_data_dir")
	snapshotID := flag.String("snapshot-id", "", "override profile.snapshot.snapshot_id")
	capacityOverride := flag.Int("asset-capacity", 0, "override profile.asset_capacity (smoke only; 0 = toml value)")
	publishedTotal := flag.String("published-total", "", "optional claimed total liabilities to reconcile against (decimal)")
	maxBalance := flag.String("max-balance", "", "optional per-account balance ceiling (decimal)")
	reserves := flag.String("reserves", "", "audited on-chain reserves total for the Reserves>=Liabilities check (decimal)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *profilePath == "" {
		return fmt.Errorf("audit: -profile is required (path to profile.toml)")
	}
	prof, err := declarative.Load(*profilePath)
	if err != nil {
		return fmt.Errorf("audit: load profile %q: %w", *profilePath, err)
	}

	dataDir := prof.Snapshot.UserDataDir
	if *userDataDir != "" {
		dataDir = *userDataDir
	}

	publishedTotalBig, err := parseBigOpt(*publishedTotal)
	if err != nil {
		return fmt.Errorf("audit: -published-total: %w", err)
	}
	maxBalanceBig, err := parseBigOpt(*maxBalance)
	if err != nil {
		return fmt.Errorf("audit: -max-balance: %w", err)
	}
	reservesBig, err := parseBigOpt(*reserves)
	if err != nil {
		return fmt.Errorf("audit: -reserves: %w", err)
	}

	report, err := merklepor.RunAudit(ctx, merklepor.Options{
		Profile:          prof,
		Snapshot:         osvfs.Dir(dataDir),
		PublishedTotal:   publishedTotalBig,
		MaxBalance:       maxBalanceBig,
		Reserves:         reservesBig,
		CapacityOverride: *capacityOverride,
		SnapshotID:       *snapshotID,
	})
	if err != nil {
		return err
	}
	if !report.OK() {
		return fmt.Errorf("audit: %d reconcile violation(s) — dataset not attestable", len(report.Violations))
	}
	if report.ReservesChecked && !report.Solvent {
		return fmt.Errorf("audit: INSOLVENT — reserves %s < liabilities %s", report.Reserves, report.Total)
	}
	return nil
}

// parseBigOpt parses an optional decimal big.Int flag; "" yields nil.
func parseBigOpt(s string) (*big.Int, error) {
	if s == "" {
		return nil, nil
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("invalid decimal %q", s)
	}
	return v, nil
}
