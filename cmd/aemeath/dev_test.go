package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// lockedBuffer is a bytes.Buffer that is safe to share with child processes.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

const devServerSource = `package main

import (
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("%s")) })
	_ = http.ListenAndServe(":"+os.Getenv("PORT"), nil)
}
`

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return port
}

func fetch(port string) string {
	client := http.Client{Timeout: time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/")
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func portFree(port string) bool {
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 200*time.Millisecond)
	if err != nil {
		return true
	}
	_ = c.Close()
	return false
}

// The reload must really stop the old server (the port is reused by the new
// one), a failed build must keep the old version alive, and stopping the dev
// server must free the port.
func TestDevReloadsAndFreesThePort(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go programs")
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module devtest\n\ngo 1.22\n"), 0o644)
	mainGo := filepath.Join(dir, "main.go")
	_ = os.WriteFile(mainGo, fmt.Appendf(nil, devServerSource, "v1"), 0644)

	port := freePort(t)
	t.Setenv("PORT", port)

	ctx, cancel := context.WithCancel(context.Background())
	var out lockedBuffer
	finished := make(chan error, 1)
	go func() {
		finished <- runDev(ctx, devOptions{dir: dir, pkg: ".", debounce: 100 * time.Millisecond, stdout: &out, stderr: &out})
	}()

	waitFor(t, "v1 to be served", func() bool { return fetch(port) == "v1" })

	// 1. A source change restarts the server on the SAME port.
	_ = os.WriteFile(mainGo, fmt.Appendf(nil, devServerSource, "v2"), 0644)
	waitFor(t, "v2 to be served (the old server must have released the port)", func() bool { return fetch(port) == "v2" })

	// 2. A broken build keeps v2 running.
	_ = os.WriteFile(mainGo, []byte("package main\n\nfunc main() { this is not go }\n"), 0o644)
	waitFor(t, "the build failure to be reported", func() bool { return strings.Contains(out.String(), "Build failed") })
	if got := fetch(port); got != "v2" {
		t.Fatalf("after a failed build the server returned %q, want v2", got)
	}

	// 3. Fixing the build gets the new version running again.
	_ = os.WriteFile(mainGo, fmt.Appendf(nil, devServerSource, "v3"), 0644)
	waitFor(t, "v3 to be served after fixing the build", func() bool { return fetch(port) == "v3" })

	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("runDev: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("runDev did not return after cancel")
	}
	if !portFree(port) {
		t.Fatal("the server is still listening after the dev server stopped (orphaned process)")
	}
}

func TestRelevantAndIgnored(t *testing.T) {
	if !ignoredDir(".git") || !ignoredDir("node_modules") || !ignoredDir("vendor") || ignoredDir("handlers") || ignoredDir(".") {
		t.Fatal("ignoredDir is wrong")
	}
}
