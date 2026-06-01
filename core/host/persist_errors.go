package host

import "errors"

// ErrNotFound is the persistence-port sentinel for "no row matched".
// It is the core/host-owned analogue of store.ErrNotFound: adapters
// translate their backing-store empty-result condition into this exact
// error so core callers can probe with IsNotFound without importing the
// store (or gorm) package. Returned by ClaimOldestByStatus on an empty
// queue, and by the Get* methods when the requested key is absent.
var ErrNotFound = errors.New("persist: no rows in result set")

// IsNotFound reports whether err is (or wraps) ErrNotFound. Callers use
// it to distinguish "absent" from a real backing-store failure — e.g.
// the prover's idempotent "have I already proved this batch?" probe.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// Witness-queue status values, pinned to the same numeric encoding as
// the legacy store.Status* constants so a partially-migrated cluster
// can read each other's rows. The prover state machine advances a batch
// Published -> Received -> Finished.
const (
	// StatusPublished marks a row written by the witness service and
	// awaiting prover pickup.
	StatusPublished int64 = 0
	// StatusReceived marks a row claimed by the prover with proving in
	// progress.
	StatusReceived int64 = 1
	// StatusFinished marks a row whose proof has been exported to the
	// proof table.
	StatusFinished int64 = 2
)
