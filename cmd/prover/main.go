// Command prover is the CLI shim around pkg/prover. The engine logic
// lives in zkpor/pkg/prover; this main parses flags, builds every input
// the engine needs (profile, config, keys opener, witness-queue +
// proof-store ports), and dispatches into Run.
//
// Usage:
//
//	prover -profile path/to/profile.toml -keys-dir .artifacts/<profile>
//
// R12-B/3: pkg/prover returns errors. This shim is the only layer that
// converts them into exit codes — stderr + os.Exit(1) on failure.
//
// R12-C: SIGINT/SIGTERM are wired into the prover's context via
// signal.NotifyContext, so an operator can stop the daemon gracefully.
// A context.Canceled return is a clean shutdown (exit 0); any other
// error is fatal (stderr + exit 1).
//
// R12-EF: input construction (profile/config parse, keys-directory
// wrapping into a vfs.KeyOpener, witness-queue + proof-store adapter
// wiring) moved out of the engine and into this shim. The engine
// receives injected values and never touches os/path or store itself.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/io/vfs/osvfs"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/prover"
	pconfig "github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/prover/config"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/profile/declarative"
	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"
)

func main() {
	if err := run(); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	keysDir := flag.String("keys-dir", "", "directory containing .pk/.vk/.r1cs artifacts (required)")
	configPath := flag.String("config", "config/config.json", "path to the prover deployment config JSON")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *profilePath == "" {
		return fmt.Errorf("prover: -profile is required (path to profile.toml)")
	}
	if *keysDir == "" {
		return fmt.Errorf("prover: -keys-dir is required (path to keygen .artifacts/)")
	}

	prof, err := declarative.Load(*profilePath)
	if err != nil {
		return fmt.Errorf("prover: load profile %q: %w", *profilePath, err)
	}

	raw, err := os.ReadFile(*configPath)
	if err != nil {
		return fmt.Errorf("prover: read config %q: %w", *configPath, err)
	}
	cfg, err := pconfig.Parse(raw)
	if err != nil {
		return fmt.Errorf("prover: parse config %q: %w", *configPath, err)
	}

	keys := osvfs.KeyDir(*keysDir)

	db, err := store.Open(cfg.MysqlDataSource)
	if err != nil {
		return fmt.Errorf("prover: open mysql: %w", err)
	}
	wq := store.NewWitnessQueueAdapter(store.NewWitnessStore(db, cfg.DbSuffix))
	ps := store.NewProofStoreAdapter(store.NewProofStore(db, cfg.DbSuffix))
	if err := ps.EnsureSchema(); err != nil {
		return fmt.Errorf("prover: create proof table: %w", err)
	}

	return prover.Run(ctx, prover.Options{
		Profile:      prof,
		Keys:         keys,
		Config:       cfg,
		WitnessQueue: wq,
		Proofs:       ps,
	})
}
