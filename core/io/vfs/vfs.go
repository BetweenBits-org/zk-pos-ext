// Package vfs declares the engine-owned, source-agnostic IO ports for
// the large inputs that the solvency engine consumes — the snapshot
// (accounts, batches, prices) and the proving/verifying keys.
//
// These inputs are too large to inject by value, so the engine takes
// READERS instead: the caller supplies whatever opens the bytes, and
// the core stays agnostic about where they live. A local directory, an
// S3 bucket, and a database BLOB column all satisfy the same interface;
// each backing provides its own implementation (the shipped local one
// lives in core/io/vfs/osvfs). This package depends on context and io
// only — no os, no filepath, no gorm — so the core build graph never
// pins a concrete source.
//
// Opener and KeyOpener carry a context.Context for cancellation and
// deadlines on remote opens; KeySink is a write-side counterpart used
// when the engine emits freshly generated keys.
package vfs

import (
	"context"
	"io"
)

// Opener opens a named input stream — the snapshot side of the engine.
// name is a backing-relative identifier (a filename for the local
// implementation, an object key for S3, a row key for a DB BLOB). The
// returned stream is the caller's to Close.
type Opener interface {
	Open(ctx context.Context, name string) (io.ReadCloser, error)
}

// KeyOpener opens a proving/verifying key stream identified by a stem
// and an extension (e.g. stem "zkpor" + ext ".vk.save"). Splitting stem
// from ext lets backings join them however they store keys without the
// core assembling a path. The returned stream is the caller's to Close.
type KeyOpener interface {
	Open(ctx context.Context, stem, ext string) (io.ReadCloser, error)
}

// KeySink creates a writable key stream identified by stem and ext, the
// write-side counterpart of KeyOpener used when the engine emits a
// freshly generated key. It takes no context: creation is a local,
// synchronous setup step for the shipped backing. The returned stream
// is the caller's to Close.
type KeySink interface {
	Create(stem, ext string) (io.WriteCloser, error)
}

// ByteSource reads an entire small input into memory in one call — used
// for inputs (such as a config or profile blob) that are bounded enough
// to slurp whole rather than stream.
type ByteSource interface {
	ReadAll(ctx context.Context) ([]byte, error)
}
