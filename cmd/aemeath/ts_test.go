package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A program that honors the same contract as Aemeath's App.Listen: when
// AEMEATH_TS_OUT is set it writes the three files and exits; otherwise it serves.
const tsProgram = `package main

import (
	"net/http"
	"os"
	"path/filepath"
)

const version = "%s"

func main() {
	if dir := os.Getenv("AEMEATH_TS_OUT"); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		for _, f := range []string{"client.ts", "schemas.ts", "hooks.ts"} {
			_ = os.WriteFile(filepath.Join(dir, f), []byte("// "+f+" "+version+"\n"), 0o644)
		}
		return
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(version)) })
	_ = http.ListenAndServe(":"+os.Getenv("PORT"), nil)
}
`

func writeProject(t *testing.T, mainGo string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module tstest\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestTSCommandBuildsRunsAndWritesTheFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go programs")
	}
	dir := writeProject(t, fmt.Sprintf(tsProgram, "v1"))
	out := filepath.Join(t.TempDir(), "web", "src", "api") // does not exist yet
	var log lockedBuffer
	if err := buildAndGenerate(context.Background(), dir, ".", out, &log, &log); err != nil {
		t.Fatalf("buildAndGenerate: %v\n%s", err, log.String())
	}
	for _, name := range tsFiles {
		b, err := os.ReadFile(filepath.Join(out, name))
		if err != nil || !strings.Contains(string(b), name+" v1") {
			t.Errorf("%s: %q, %v", name, b, err)
		}
	}
}

func TestTSCommandReportsAProgramThatWritesNothing(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go programs")
	}
	// An application that ignores the variable (for example an old framework version).
	dir := writeProject(t, "package main\n\nfunc main() {}\n")
	err := buildAndGenerate(context.Background(), dir, ".", filepath.Join(t.TempDir(), "o"), os.Stderr, os.Stderr)
	if err == nil || !strings.Contains(err.Error(), "did not write") || !strings.Contains(err.Error(), "app.Listen") {
		t.Fatalf("err = %v, want a clear 'did not write ... app.Listen' message", err)
	}
}

func TestTSCommandShowsWhyTheProgramFailed(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go programs")
	}
	dir := writeProject(t, "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tfmt.Fprintln(os.Stderr, \"cannot open the database\")\n\tos.Exit(3)\n}\n")
	err := buildAndGenerate(context.Background(), dir, ".", filepath.Join(t.TempDir(), "o"), os.Stderr, os.Stderr)
	if err == nil || !strings.Contains(err.Error(), "cannot open the database") {
		t.Fatalf("err = %v, want the program's own message", err)
	}
}

func TestTSCommandReportsBuildErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go programs")
	}
	dir := writeProject(t, "package main\n\nfunc main() { this is not go }\n")
	var log lockedBuffer
	err := buildAndGenerate(context.Background(), dir, ".", filepath.Join(t.TempDir(), "o"), &log, &log)
	if err == nil || !strings.Contains(err.Error(), "build failed") {
		t.Fatalf("err = %v", err)
	}
}

// With --ts-out, every successful build regenerates the files; a failed build
// keeps the previous TypeScript and the previous server.
func TestDevRegeneratesTypeScriptAfterEachBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go programs")
	}
	dir := writeProject(t, fmt.Sprintf(tsProgram, "v1"))
	mainGo := filepath.Join(dir, "main.go")
	out := filepath.Join(t.TempDir(), "api")
	port := freePort(t)
	t.Setenv("PORT", port)

	ctx, cancel := context.WithCancel(context.Background())
	var log lockedBuffer
	finished := make(chan error, 1)
	go func() {
		finished <- runDev(ctx, devOptions{dir: dir, pkg: ".", tsOut: out, debounce: 100 * time.Millisecond, stdout: &log, stderr: &log})
	}()

	read := func() string { b, _ := os.ReadFile(filepath.Join(out, "client.ts")); return string(b) }
	waitFor(t, "the first TypeScript", func() bool { return strings.Contains(read(), "client.ts v1") })
	waitFor(t, "the first server", func() bool { return fetch(port) == "v1" })

	_ = os.WriteFile(mainGo, fmt.Appendf(nil, tsProgram, "v2"), 0644)
	waitFor(t, "the regenerated TypeScript", func() bool { return strings.Contains(read(), "client.ts v2") })
	waitFor(t, "the new server", func() bool { return fetch(port) == "v2" })

	_ = os.WriteFile(mainGo, []byte("package main\n\nfunc main() { this is not go }\n"), 0o644)
	waitFor(t, "the build failure", func() bool { return strings.Contains(log.String(), "Build failed") })
	if got := read(); !strings.Contains(got, "client.ts v2") {
		t.Fatalf("a failed build must keep the previous TypeScript, got %q", got)
	}
	if got := fetch(port); got != "v2" {
		t.Fatalf("a failed build must keep the previous server, got %q", got)
	}

	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("runDev did not return")
	}
	if !portFree(port) {
		t.Fatal("the server is still listening after the dev server stopped")
	}
}

func TestTailKeepsTheLastLines(t *testing.T) {
	var lines []string
	for i := 1; i <= 40; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	got := tail(strings.Join(lines, "\n"))
	if !strings.HasPrefix(got, "...\n") || !strings.HasSuffix(got, "line 40") || strings.Contains(got, "line 5\n") {
		t.Fatalf("tail = %q", got)
	}
	if short := tail("  one line \n"); short != "one line" {
		t.Fatalf("tail of a short text = %q", short)
	}
}

// Projects written as log.Fatal(app.Listen(...)) exit with status 1 after writing the files.
func TestTSCommandToleratesLogFatalNilButWarns(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go programs")
	}
	dir := writeProject(t, `package main

import (
	"log"
	"os"
	"path/filepath"
)

func main() {
	if dir := os.Getenv("AEMEATH_TS_OUT"); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		for _, f := range []string{"client.ts", "schemas.ts", "hooks.ts"} {
			_ = os.WriteFile(filepath.Join(dir, f), []byte("// "+f+"\n"), 0o644)
		}
		log.Println("aemeath: AEMEATH_TS_OUT is set: wrote the TypeScript files")
		log.Fatal(error(nil)) // what log.Fatal(app.Listen(...)) does when Listen returns nil
	}
}
`)
	var log lockedBuffer
	out := filepath.Join(t.TempDir(), "o")
	if err := buildAndGenerate(context.Background(), dir, ".", out, &log, &log); err != nil {
		t.Fatalf("the files were written, so this must not fail: %v", err)
	}
	if !strings.Contains(log.String(), "log.Fatal") {
		t.Fatalf("a warning about log.Fatal(nil) is expected, got: %s", log.String())
	}
	if _, err := os.Stat(filepath.Join(out, "hooks.ts")); err != nil {
		t.Fatal(err)
	}
}
