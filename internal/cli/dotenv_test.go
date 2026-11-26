package cli

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseDotEnv(t *testing.T) {
	src := "# a comment\r\n" +
		"\r\n" + // a blank line of a Windows file
		"\t\n" +
		"PORT=4567\r\n" +
		"export DATABASE_URL=postgres://app:app@localhost:5432/app?sslmode=disable\n" +
		"  SPACED = value with spaces  \n" +
		"QUOTED=\"a # not a comment\"\n" +
		"SINGLE='$HOME stays'\n" +
		"TRAILING=plain # a comment\n" +
		"EMPTY=\n"
	got, err := parseDotEnv(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"PORT=4567",
		"DATABASE_URL=postgres://app:app@localhost:5432/app?sslmode=disable",
		"SPACED=value with spaces",
		"QUOTED=a # not a comment",
		"SINGLE=$HOME stays",
		"TRAILING=plain",
		"EMPTY=",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("parseDotEnv =\n%q\nwant\n%q", got, want)
	}
}

func TestParseDotEnvNamesTheBadLine(t *testing.T) {
	for _, src := range []string{"PORT=1\nnot a variable\n", "PORT=1\n1BAD=x\n", "PORT=1\nOPEN=\"no end\n"} {
		_, err := parseDotEnv(src)
		if err == nil || !strings.Contains(err.Error(), "line 2") {
			t.Errorf("parseDotEnv(%q) = %v, want an error naming line 2", src, err)
		}
	}
}

// A variable set in the shell wins over .env, as with other tools that read
// .env, so a one-off PORT=4000 aemeath dev still works.
func TestServerEnvKeepsTheShellFirst(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("FROM_FILE=file\nBOTH=file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BOTH", "shell")
	env, n, err := serverEnv(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || !slices.Contains(env, "FROM_FILE=file") || slices.Contains(env, "BOTH=file") || !slices.Contains(env, "BOTH=shell") {
		t.Fatalf("serverEnv: %d from .env, BOTH/FROM_FILE entries: %q", n, filter(env, "BOTH=", "FROM_FILE="))
	}

	if _, n, err := serverEnv(t.TempDir()); err != nil || n != 0 {
		t.Fatalf("without .env: n=%d err=%v, want 0 and nil", n, err)
	}
}

func filter(env []string, prefixes ...string) []string {
	var out []string
	for _, kv := range env {
		for _, p := range prefixes {
			if strings.HasPrefix(kv, p) {
				out = append(out, kv)
			}
		}
	}
	return out
}

// .env.example tells the user to copy it to .env; the server must then see it.
func TestDevPassesDotEnvToTheServer(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go programs")
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module devtest\n\ngo 1.22\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import (
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(os.Getenv("GREETING"))) })
	_ = http.ListenAndServe(":"+os.Getenv("PORT"), nil)
}
`), 0o644)
	port := freePort(t)
	dotenv := filepath.Join(dir, ".env")
	_ = os.WriteFile(dotenv, []byte("PORT="+port+"\nGREETING=from-dotenv\n"), 0o600)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out lockedBuffer
	finished := make(chan error, 1)
	go func() {
		finished <- runDev(ctx, devOptions{dir: dir, pkg: ".", debounce: 100 * time.Millisecond, stdout: &out, stderr: &out})
	}()
	waitFor(t, "the server to answer with GREETING from .env on PORT from .env", func() bool { return fetch(port) == "from-dotenv" })

	// Editing .env restarts the server with the new values.
	_ = os.WriteFile(dotenv, []byte("PORT="+port+"\nGREETING=changed\n"), 0o600)
	waitFor(t, "the edited .env to reach the server", func() bool { return fetch(port) == "changed" })

	cancel()
	select {
	case <-finished:
	case <-time.After(15 * time.Second):
		t.Fatal("runDev did not return after cancel")
	}
}
