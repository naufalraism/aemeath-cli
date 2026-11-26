package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The files from an earlier run are still there. A program that writes
// nothing this time (main returns before app.Listen, an old framework) must
// be reported, not taken as success because the files exist.
func TestTSCommandDoesNotTakeOldFilesForNewOnes(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go programs")
	}
	out := filepath.Join(t.TempDir(), "api")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range tsFiles {
		if err := os.WriteFile(filepath.Join(out, name), []byte("// from yesterday\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dir := writeProject(t, "package main\n\nfunc main() {}\n")
	err := buildAndGenerate(context.Background(), dir, ".", out, os.Stderr, os.Stderr)
	if err == nil || !strings.Contains(err.Error(), "did not write") {
		t.Fatalf("err = %v, want the 'did not write' error although old files exist", err)
	}
	if b, _ := os.ReadFile(filepath.Join(out, "client.ts")); string(b) != "// from yesterday\n" {
		t.Errorf("client.ts was changed to %q", b)
	}
}

// A file whose content did not change keeps its modification time, so a
// bundler watching the folder does not reload for nothing.
func TestTSCommandLeavesUnchangedFilesAlone(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go programs")
	}
	dir := writeProject(t, strings.Replace(tsProgram, "%s", "v1", 1))
	out := filepath.Join(t.TempDir(), "api")
	if err := buildAndGenerate(context.Background(), dir, ".", out, os.Stderr, os.Stderr); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	for _, name := range tsFiles {
		if err := os.Chtimes(filepath.Join(out, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := buildAndGenerate(context.Background(), dir, ".", out, os.Stderr, os.Stderr); err != nil {
		t.Fatal(err)
	}
	for _, name := range tsFiles {
		info, err := os.Stat(filepath.Join(out, name))
		if err != nil || !info.ModTime().Equal(old) {
			t.Errorf("%s was rewritten although its content is the same", name)
		}
	}
}
