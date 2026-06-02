// Command attest is the CLI shim around pkg/merklepor.RunAttest — the
// non-zk Merkle-sum proof-of-liabilities attest service (PRODUCTION_ROADMAP
// Stage MS). It parses flags, builds the inputs the engine needs (profile,
// config, snapshot opener, attest store port), and dispatches into
// RunAttest. The engine logic lives in pkg/merklepor; this shim is the only
// layer that touches os/path/store and converts errors into exit codes.
//
// Usage:
//
//	attest -profile path/to/profile.toml \
//	    [-config config/config.json] [-user-data-dir DIR] [-snapshot-id ID] \
//	    [-asset-capacity N] [-published-total D] [-max-balance D]
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
	mconfig "github.com/BetweenBits-org/zk-pos-ext/pkg/merklepor/config"
	"github.com/BetweenBits-org/zk-pos-ext/profile/declarative"
	"github.com/BetweenBits-org/zk-pos-ext/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	configPath := flag.String("config", "config/config.json", "path to the merklepor deployment config JSON")
	userDataDir := flag.String("user-data-dir", "", "override profile.snapshot.user_data_dir")
	snapshotID := flag.String("snapshot-id", "", "override profile.snapshot.snapshot_id")
	capacityOverride := flag.Int("asset-capacity", 0, "override profile.asset_capacity (smoke only; 0 = toml value)")
	publishedTotal := flag.String("published-total", "", "optional claimed total liabilities to reconcile against (decimal)")
	maxBalance := flag.String("max-balance", "", "optional per-account balance ceiling (decimal)")
	dumpUserIndex := flag.Int("dump-user-index", 0, "positional index dumped when -dump-user-path is set")
	dumpUserPath := flag.String("dump-user-path", "", "if set, write that account's SumUserConfig JSON here after attesting")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *profilePath == "" {
		return fmt.Errorf("attest: -profile is required (path to profile.toml)")
	}
	prof, err := declarative.Load(*profilePath)
	if err != nil {
		return fmt.Errorf("attest: load profile %q: %w", *profilePath, err)
	}
	raw, err := os.ReadFile(*configPath)
	if err != nil {
		return fmt.Errorf("attest: read config %q: %w", *configPath, err)
	}
	cfg, err := mconfig.Parse(raw)
	if err != nil {
		return fmt.Errorf("attest: parse config %q: %w", *configPath, err)
	}

	dataDir := prof.Snapshot.UserDataDir
	if *userDataDir != "" {
		dataDir = *userDataDir
	}

	db, err := store.Open(cfg.MysqlDataSource)
	if err != nil {
		return fmt.Errorf("attest: open mysql: %w", err)
	}
	att := store.NewAttestStoreAdapter(store.NewAttestationStore(db, cfg.DbSuffix))
	if err := att.EnsureSchema(); err != nil {
		return fmt.Errorf("attest: ensure schema: %w", err)
	}

	publishedTotalBig, err := parseBigOpt(*publishedTotal)
	if err != nil {
		return fmt.Errorf("attest: -published-total: %w", err)
	}
	maxBalanceBig, err := parseBigOpt(*maxBalance)
	if err != nil {
		return fmt.Errorf("attest: -max-balance: %w", err)
	}

	return merklepor.RunAttest(ctx, merklepor.Options{
		Profile:          prof,
		Snapshot:         osvfs.Dir(dataDir),
		Attest:           att,
		PublishedTotal:   publishedTotalBig,
		MaxBalance:       maxBalanceBig,
		CapacityOverride: *capacityOverride,
		SnapshotID:       *snapshotID,
		DumpUserIndex:    *dumpUserIndex,
		DumpUserPath:     *dumpUserPath,
	})
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
