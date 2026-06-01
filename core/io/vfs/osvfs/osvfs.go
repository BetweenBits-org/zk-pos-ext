// Package osvfs is the shipped local-filesystem backing for the
// engine-owned vfs ports. It is the only os/path point on the input
// side: every other layer talks to the vfs interfaces, and this package
// turns those calls into os.Open / os.Create / os.ReadFile against a
// directory or file on the local disk.
//
// Each constructor returns a small unexported struct that holds only the
// configured root; the structs carry no open handles and no other
// mutable state. Every Open/Create/ReadAll opens a fresh handle, so a
// single value is safe to share across goroutines and may be re-opened
// any number of times — the returned stream is the caller's to Close.
//
// The context passed to Open/ReadAll is ignored: a local open is a
// synchronous syscall with no remote round-trip to cancel. The ports
// still take a context so that remote backings (S3, a DB BLOB) can honor
// cancellation and deadlines behind the same interface.
package osvfs

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/io/vfs"
)

// Compile-time assertions that the unexported backings satisfy the vfs
// ports they are constructed to return.
var (
	_ vfs.Opener     = dirOpener{}
	_ vfs.KeyOpener  = keyDirOpener{}
	_ vfs.KeySink    = keyDirSink{}
	_ vfs.ByteSource = fileSource{}
)

// dirOpener opens snapshot inputs by name relative to a base directory.
type dirOpener struct {
	dir string
}

// Dir returns a vfs.Opener that opens named inputs as files under dir.
// Open(ctx, name) resolves to os.Open(filepath.Join(dir, name)); ctx is
// ignored because a local open has no remote round-trip to cancel.
func Dir(dir string) vfs.Opener {
	return dirOpener{dir: dir}
}

// Open opens the file named name under the configured directory. ctx is
// ignored for the local backing. The returned stream is the caller's to
// Close.
func (d dirOpener) Open(_ context.Context, name string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(d.dir, name))
}

// keyDirOpener opens proving/verifying keys by stem+ext under a base
// directory.
type keyDirOpener struct {
	dir string
}

// KeyDir returns a vfs.KeyOpener that opens keys as files under dir.
// Open(ctx, stem, ext) resolves to os.Open(filepath.Join(dir, stem+ext));
// ctx is ignored for the local backing.
func KeyDir(dir string) vfs.KeyOpener {
	return keyDirOpener{dir: dir}
}

// Open opens the key file stem+ext under the configured directory. ctx is
// ignored for the local backing. The returned stream is the caller's to
// Close.
func (k keyDirOpener) Open(_ context.Context, stem, ext string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(k.dir, stem+ext))
}

// keyDirSink creates proving/verifying keys by stem+ext under a base
// directory.
type keyDirSink struct {
	out string
}

// KeyDirSink returns a vfs.KeySink that creates keys as files under out.
// Create(stem, ext) resolves to os.Create(filepath.Join(out, stem+ext)).
func KeyDirSink(out string) vfs.KeySink {
	return keyDirSink{out: out}
}

// Create creates (truncating any existing file) the key file stem+ext
// under the configured output directory. The returned stream is the
// caller's to Close.
func (s keyDirSink) Create(stem, ext string) (io.WriteCloser, error) {
	return os.Create(filepath.Join(s.out, stem+ext))
}

// fileSource reads a single fixed file whole.
type fileSource struct {
	path string
}

// File returns a vfs.ByteSource that slurps the whole file at path.
// ReadAll(ctx) resolves to os.ReadFile(path); ctx is ignored because a
// local read has no remote round-trip to cancel.
func File(path string) vfs.ByteSource {
	return fileSource{path: path}
}

// ReadAll reads the entire configured file into memory. ctx is ignored
// for the local backing.
func (f fileSource) ReadAll(_ context.Context) ([]byte, error) {
	return os.ReadFile(f.path)
}
