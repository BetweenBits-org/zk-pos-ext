// Command verifier is the CLI shim around pkg/verifier. The engine
// logic lives in zkpor/pkg/verifier; this main only parses flags and
// dispatches into the chosen mode.
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
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/verifier"
)

func main() {
	userFlag := flag.Bool("user", false, "verify a single user's inclusion proof")
	hashFlag := flag.Bool("hash", false, "print Poseidon hash of two base64 arguments")
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required for batch + -user modes)")
	keysDir := flag.String("keys-dir", "", "directory containing the verifying-key .vk files (required for batch mode)")
	capacityOverride := flag.Int("asset-capacity", 0, "override profile.asset_capacity (smoke only; 0 = use toml value)")
	flag.Parse()

	opts := verifier.Options{
		ProfilePath:      *profilePath,
		KeysDir:          *keysDir,
		CapacityOverride: *capacityOverride,
		ConfigPath:       "config/config.json",
		UserConfigPath:   "config/user_config.json",
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch {
	case *hashFlag:
		args := flag.Args()
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "invalid hash command, it needs two arguments")
			os.Exit(2)
		}
		err = verifier.RunHash(args[0], args[1])
	case *userFlag:
		err = verifier.RunUser(ctx, opts)
	default:
		err = verifier.RunBatch(ctx, opts)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
