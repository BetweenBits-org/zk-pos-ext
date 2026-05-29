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
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/prover"
)

func main() {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	keysDir := flag.String("keys-dir", "", "directory containing .pk/.vk/.r1cs artifacts (required)")
	flag.Parse()

	if err := prover.Run(prover.Options{
		ProfilePath: *profilePath,
		KeysDir:     *keysDir,
		ConfigPath:  "config/config.json",
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
