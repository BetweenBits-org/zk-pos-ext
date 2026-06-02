// Command verifier is the CLI shim around pkg/verifier. The engine
// logic lives in zkpor/pkg/verifier; this main parses flags, builds the
// inputs the chosen mode needs (profile, config, keys opener, proof
// store port, user-config source), and dispatches into that mode.
//
// Modes:
//
//	verifier                 batch mode — verify every proof in the
//	                         proof table chains correctly and the final
//	                         CEX commitment matches the published totals
//	verifier -user           single-user inclusion verification against
//	                         config/user_config.json
//	verifier -hash A B       print Poseidon(A, B) for two base64 inputs
//
// R12-B/1: pkg/verifier returns errors. This shim is the only layer
// that converts them into exit codes — stderr + os.Exit(1) on failure.
//
// R12-C: SIGINT/SIGTERM are wired into a context via
// signal.NotifyContext and passed to RunBatch/RunUser so a long batch
// verification can be aborted. RunHash is instant and ctx-free.
// Verification is a one-shot job — any error (including
// context.Canceled) exits 1.
//
// R12-EF: input construction (profile/config parse, keys-directory
// wrapping into a vfs.KeyOpener, proof-store adapter wiring,
// user-config wrapping into a vfs.ByteSource) moved out of the engine
// and into this shim. Construction is LAZY per mode: -hash reads only
// flag.Args() and needs no profile/config/keys at all, so a bare
// `verifier -hash A B` with an empty -profile still works; -user builds
// Profile + Keys + UserConfig; default(batch) builds Profile + Keys +
// Config + Proofs.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/BetweenBits-org/zk-pos-ext/core/io/vfs/osvfs"
	"github.com/BetweenBits-org/zk-pos-ext/pkg/verifier"
	vconfig "github.com/BetweenBits-org/zk-pos-ext/pkg/verifier/config"
	"github.com/BetweenBits-org/zk-pos-ext/profile/declarative"
	"github.com/BetweenBits-org/zk-pos-ext/store"
)

func main() {
	userFlag := flag.Bool("user", false, "verify a single user's inclusion proof")
	hashFlag := flag.Bool("hash", false, "print Poseidon hash of two base64 arguments")
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required for batch + -user modes)")
	keysDir := flag.String("keys-dir", "", "directory containing the verifying-key .vk files (required for batch + -user modes)")
	configPath := flag.String("config", "config/config.json", "path to the verifier deployment config JSON (batch mode)")
	userConfigPath := flag.String("user-config", "config/user_config.json", "path to the per-user inclusion-proof artifact (-user mode)")
	capacityOverride := flag.Int("asset-capacity", 0, "override profile.asset_capacity (smoke only; 0 = use toml value)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *hashFlag, *userFlag, *profilePath, *keysDir, *configPath, *userConfigPath, *capacityOverride); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, hashMode, userMode bool, profilePath, keysDir, configPath, userConfigPath string, capacityOverride int) error {
	switch {
	case hashMode:
		// -hash reads ONLY flag.Args(); it needs no profile/config/keys.
		// A bare `verifier -hash A B` with an empty -profile must work.
		args := flag.Args()
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "invalid hash command, it needs two arguments")
			os.Exit(2)
		}
		return verifier.RunHash(args[0], args[1])

	case userMode:
		prof, err := loadProfile(profilePath)
		if err != nil {
			return err
		}
		return verifier.RunUser(ctx, verifier.Options{
			Profile:          prof,
			Keys:             osvfs.KeyDir(keysDir),
			CapacityOverride: capacityOverride,
			UserConfig:       osvfs.File(userConfigPath),
		})

	default:
		prof, err := loadProfile(profilePath)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("verifier: read config %q: %w", configPath, err)
		}
		cfg, err := vconfig.Parse(raw)
		if err != nil {
			return fmt.Errorf("verifier: parse config %q: %w", configPath, err)
		}

		opts := verifier.Options{
			Profile:          prof,
			Keys:             osvfs.KeyDir(keysDir),
			CapacityOverride: capacityOverride,
			Config:           cfg,
		}
		// Wire exactly one proof source: the MySQL store port when a DSN is
		// configured, otherwise the legacy CSV wrapped as a vfs.ByteSource so
		// the engine reads no path itself.
		if cfg.MysqlDataSource != "" {
			db, err := store.Open(cfg.MysqlDataSource)
			if err != nil {
				return fmt.Errorf("verifier: open mysql: %w", err)
			}
			opts.Proofs = store.NewProofStoreAdapter(store.NewProofStore(db, cfg.DbSuffix))
		} else {
			opts.ProofCSV = osvfs.File(cfg.ProofTable)
		}
		return verifier.RunBatch(ctx, opts)
	}
}

// loadProfile reads + parses the declarative profile.toml the batch and
// -user modes need. -hash never reaches here.
func loadProfile(profilePath string) (*declarative.Profile, error) {
	if profilePath == "" {
		return nil, fmt.Errorf("verifier: -profile is required (path to profile.toml)")
	}
	prof, err := declarative.Load(profilePath)
	if err != nil {
		return nil, fmt.Errorf("verifier: load profile %q: %w", profilePath, err)
	}
	return prof, nil
}
