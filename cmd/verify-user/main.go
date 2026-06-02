// Command verify-user is the CLI shim around pkg/merklepor.RunVerifyUser —
// single-account sum-inclusion verification for the non-zk Merkle-sum
// proof-of-liabilities side line (PRODUCTION_ROADMAP Stage MS). It reads a
// SumUserConfig artifact (as emitted by `attest -dump-user-path`) and
// checks the user's leaf flows into the published root (inclusion +
// sum-path). Exits non-zero if the proof does not verify.
//
// Usage:
//
//	verify-user [-user-config user_config.json]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/BetweenBits-org/zk-pos-ext/core/io/vfs/osvfs"
	"github.com/BetweenBits-org/zk-pos-ext/pkg/merklepor"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	userConfigPath := flag.String("user-config", "user_config.json", "path to the sum-inclusion artifact to verify")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return merklepor.RunVerifyUser(ctx, merklepor.Options{
		UserConfig: osvfs.File(*userConfigPath),
	})
}
