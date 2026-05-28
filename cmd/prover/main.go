// Command prover is the CLI shim around pkg/prover. The engine logic
// lives in zkpor/pkg/prover; this main only parses flags and dispatches
// into Run.
//
// Usage:
//
//	prover -profile path/to/profile.toml -keys-dir .artifacts/<profile>
//
// R12-A library extraction: the previous 375-line main.go body moved
// to zkpor/pkg/prover. config/ holds only the runtime JSON file now;
// the Go schema moved alongside the library.
package main

import (
	"flag"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/pkg/prover"
)

func main() {
	profilePath := flag.String("profile", "", "path to the declarative profile.toml (required)")
	keysDir := flag.String("keys-dir", "", "directory containing .pk/.vk/.r1cs artifacts (required)")
	flag.Parse()

	prover.Run(prover.Options{
		ProfilePath: *profilePath,
		KeysDir:     *keysDir,
		ConfigPath:  "config/config.json",
	})
}
