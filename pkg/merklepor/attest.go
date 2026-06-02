package merklepor

import (
	"context"
	"fmt"
)

// RunAttest streams the snapshot, runs the auditor reconcile checks, builds
// the dense Merkle sum tree, and persists the attested root plus one
// sum-inclusion proof row per account. Returns an error on the first
// failure (reconcile, build, or persistence); nil on success. The attested
// (root, total) is printed for the operator to publish.
func RunAttest(ctx context.Context, opts Options) error {
	if opts.Snapshot == nil {
		return fmt.Errorf("merklepor: Snapshot is required")
	}
	if opts.Attest == nil {
		return fmt.Errorf("merklepor: Attest store is required")
	}
	r, err := resolve(opts)
	if err != nil {
		return err
	}
	leaves, err := dispatchCollectSumLeaves(r.model, collectDeps{
		ctx: ctx, sourceType: r.sourceType, snapshot: opts.Snapshot,
		snapID: r.snapID, capacity: r.capacity, pricing: r.pricing, tiers: r.tiers,
	})
	if err != nil {
		return fmt.Errorf("merklepor: collect leaves: %w", err)
	}
	if len(leaves) == 0 {
		return fmt.Errorf("merklepor: snapshot yielded no accounts")
	}
	root, written, err := buildAndPersist(leaves, opts.Attest, opts.PublishedTotal, opts.MaxBalance)
	if err != nil {
		return err
	}
	fmt.Printf("merklepor attest: root=%x total=%s leaves=%d rows=%d\n", root.Hash, root.Sum, len(leaves), written)
	return nil
}
