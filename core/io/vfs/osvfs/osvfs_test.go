package osvfs

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestKeyDirOpenJoinsStemAndExt is the load-bearing case: it pins the
// filepath.Join(dir, stem+ext) contract that KeyDir relies on. Using the
// briefing's key name "zkpor.t4.50_700"+".vk", it writes that exact file
// into a temp dir and asserts KeyDir(dir).Open opens it and returns the
// bytes back. This carries the join coverage that the prover/verifier
// tests will drop once they route keys through this port.
func TestKeyDirOpenJoinsStemAndExt(t *testing.T) {
	dir := t.TempDir()
	const (
		stem = "zkpor.t4.50_700"
		ext  = ".vk"
	)
	want := []byte("verifying-key-bytes")

	// The file the join must resolve to: <dir>/zkpor.t4.50_700.vk.
	path := filepath.Join(dir, stem+ext)
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatalf("seed key file: %v", err)
	}

	rc, err := KeyDir(dir).Open(context.Background(), stem, ext)
	if err != nil {
		t.Fatalf("KeyDir.Open(%q, %q): %v", stem, ext, err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read key stream: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("key bytes = %q, want %q", got, want)
	}
}

// TestDirOpenJoinsName checks the Opener side resolves
// filepath.Join(dir, name).
func TestDirOpenJoinsName(t *testing.T) {
	dir := t.TempDir()
	const name = "snapshot.json"
	want := []byte("{}")

	if err := os.WriteFile(filepath.Join(dir, name), want, 0o600); err != nil {
		t.Fatalf("seed input file: %v", err)
	}

	rc, err := Dir(dir).Open(context.Background(), name)
	if err != nil {
		t.Fatalf("Dir.Open(%q): %v", name, err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read input stream: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("input bytes = %q, want %q", got, want)
	}
}

// TestKeyDirSinkCreateRoundTrip checks the write side joins stem+ext and
// that a KeyDir opener reads back exactly what the sink wrote.
func TestKeyDirSinkCreateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	const (
		stem = "zkpor.t4.50_700"
		ext  = ".vk"
	)
	want := []byte("freshly-generated-key")

	wc, err := KeyDirSink(dir).Create(stem, ext)
	if err != nil {
		t.Fatalf("KeyDirSink.Create(%q, %q): %v", stem, ext, err)
	}
	if _, err := wc.Write(want); err != nil {
		t.Fatalf("write key stream: %v", err)
	}
	if err := wc.Close(); err != nil {
		t.Fatalf("close key sink: %v", err)
	}

	rc, err := KeyDir(dir).Open(context.Background(), stem, ext)
	if err != nil {
		t.Fatalf("reopen written key: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read written key: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("written key bytes = %q, want %q", got, want)
	}
}

// TestFileReadAll checks ByteSource slurps the whole file at the fixed
// path.
func TestFileReadAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.toml")
	want := []byte("[profile]\n")

	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	got, err := File(path).ReadAll(context.Background())
	if err != nil {
		t.Fatalf("File.ReadAll(%q): %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("file bytes = %q, want %q", got, want)
	}
}
