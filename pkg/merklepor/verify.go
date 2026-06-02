package merklepor

import (
	"context"
	"fmt"
)

// RunVerifyUser reads a single user's sum-inclusion artifact and checks it
// against the embedded published root (inclusion + sum-path) via
// corehost.VerifyMerkleSumProof. ctx is checked at entry; user-mode
// verification is a single fast recompute. Returns an error if the
// artifact cannot be read/decoded, or if the proof does not verify — so a
// failed check surfaces as a non-zero CLI exit, not a silent pass.
func RunVerifyUser(ctx context.Context, opts Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if opts.UserConfig == nil {
		return fmt.Errorf("merklepor: UserConfig is required")
	}
	raw, err := opts.UserConfig.ReadAll(ctx)
	if err != nil {
		return fmt.Errorf("merklepor: read user config: %w", err)
	}
	ok, err := verifyUserConfig(raw)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("merklepor verify: FAIL")
		return fmt.Errorf("merklepor: user inclusion proof did not verify")
	}
	fmt.Println("merklepor verify: pass")
	return nil
}
