// Command prover is the CLI shim around pkg/prover. The engine logic
// lives in zkpor/pkg/prover; this main only parses flags and dispatches
// into Run.
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
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/prover"
)

func main() {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	keysDir := flag.String("keys-dir", "", "directory containing .pk/.vk/.r1cs artifacts (required)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := prover.Run(ctx, prover.Options{
		ProfilePath: *profilePath,
		KeysDir:     *keysDir,
		ConfigPath:  "config/config.json",
	}); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
