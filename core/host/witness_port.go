package host

// BatchWitnessDTO is the gorm-free projection of a witness↔prover queue
// row. It mirrors the exported data fields of store.BatchWitness with
// gorm.Model dropped: Height is the batch number, WitnessData is the
// base64(s2(gob(BatchCreateUserWitness))) payload, and Status tracks the
// Published/Received/Finished state machine. WitnessData is carried
// verbatim end to end — adapters copy the string with no re-base64,
// trim, or re-marshal.
type BatchWitnessDTO struct {
	Height      int64
	WitnessData string
	Status      int64
}

// WitnessQueue is the injected persistence port for the witness↔prover
// artifact channel. The MySQL adapter (store.WitnessQueueAdapter) is the
// one shipped backing, but any queue honouring these invariants is
// substitutable.
type WitnessQueue interface {
	// EnsureSchema idempotently provisions the backing schema. Safe to
	// call on every service startup.
	EnsureSchema() error

	// Create inserts the given queue rows. The backing enforces a unique
	// index on Height: a duplicate Height is rejected (translated
	// backing error), keeping witness writes idempotent on crash
	// recovery. WitnessData is stored verbatim.
	Create(rows []BatchWitnessDTO) error

	// ClaimOldestByStatus atomically finds the oldest row (lowest
	// Height) at `from` status, flips it to `to`, and returns it — the
	// claim and the status flip happen in one transaction so concurrent
	// callers never claim the same row. Returns corehost.ErrNotFound
	// when no row matches (the prover reads this as "queue empty").
	ClaimOldestByStatus(from, to int64) (*BatchWitnessDTO, error)

	// MarkStatus sets the status of the single row at the given Height.
	// Returns corehost.ErrNotFound when no row matches.
	MarkStatus(height, status int64) error
}
