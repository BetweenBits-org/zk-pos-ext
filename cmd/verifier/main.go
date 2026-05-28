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
// R12-A library extraction: the previous 530-line main.go body moved
// to zkpor/pkg/verifier. config/ holds only the runtime JSON files
// now; the Go schema moved alongside the library.
package main

import (
	"flag"
	"fmt"
	"os"

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

	switch {
	case *hashFlag:
		args := flag.Args()
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "invalid hash command, it needs two arguments")
			os.Exit(2)
		}
		verifier.RunHash(args[0], args[1])
	case *userFlag:
		verifier.RunUser(opts)
	default:
		verifier.RunBatch(opts)
	}
}
