package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

type devOptions struct {
	dir      string
	pkg      string
	tsOut    string // if set, the TypeScript files are regenerated here after every successful build
	debounce time.Duration
	stdout   io.Writer
	stderr   io.Writer
}

// runner builds the project into a temporary binary and runs that binary
// directly. Running the binary itself (instead of "go run", which starts the
// server as a child process) means stopping it really stops the server, so
// the port is free again for the next start.
type runner struct {
	dir, pkg, bin string
	stdout        io.Writer
	stderr        io.Writer
	cmd           *exec.Cmd
	done          chan struct{}
}

func (r *runner) build(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "go", "build", "-o", r.bin, r.pkg)
	cmd.Dir = r.dir
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	return cmd.Run()
}

func (r *runner) start() error {
	cmd := exec.Command(r.bin)
	cmd.Dir = r.dir
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	r.cmd, r.done = cmd, done
	return nil
}

// stop asks the server to shut down gracefully and kills it if it does not
// exit within the grace period. It always waits for the process to be gone.
func (r *runner) stop(grace time.Duration) {
	if r.cmd == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = r.cmd.Process.Kill() // Windows has no SIGTERM for processes
	} else {
		_ = r.cmd.Process.Signal(syscall.SIGTERM)
	}
	select {
	case <-r.done:
	case <-time.After(grace):
		_ = r.cmd.Process.Kill()
		<-r.done
	}
	r.cmd, r.done = nil, nil
}

// binaryName adds the platform's executable suffix.
func binaryName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// ignoredDir reports whether a directory should not be watched.
func ignoredDir(name string) bool {
	if name == "." || name == ".." {
		return false
	}
	switch name {
	case "node_modules", "vendor", "tmp", "uploads":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// relevant reports whether a file event should trigger a rebuild.
func relevant(ev fsnotify.Event) bool {
	if !ev.Has(fsnotify.Write) && !ev.Has(fsnotify.Create) && !ev.Has(fsnotify.Rename) && !ev.Has(fsnotify.Remove) {
		return false
	}
	base := filepath.Base(ev.Name)
	switch base {
	case "go.mod", "go.sum", ".env":
		return true
	}
	return strings.HasSuffix(base, ".go") && !strings.HasSuffix(base, "_test.go")
}

// addTree watches root and every directory below it that is not ignored.
func addTree(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped, never fatal
		}
		if d.IsDir() {
			if path != root && ignoredDir(d.Name()) {
				return filepath.SkipDir
			}
			return w.Add(path)
		}
		return nil
	})
}

// runDev builds and runs the project, and rebuilds when relevant files change.
// A failed build keeps the previous version running. It returns when ctx is done.
func runDev(ctx context.Context, opts devOptions) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("the go command was not found in PATH")
	}
	if opts.debounce <= 0 {
		opts.debounce = 300 * time.Millisecond
	}

	tmp, err := os.MkdirTemp("", "aemeath-dev-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	bin := filepath.Join(tmp, binaryName("app"))

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating the file watcher: %w", err)
	}
	defer func() { _ = watcher.Close() }()
	if err := addTree(watcher, opts.dir); err != nil {
		return fmt.Errorf("watching %s: %w", opts.dir, err)
	}

	r := &runner{dir: opts.dir, pkg: opts.pkg, bin: bin, stdout: opts.stdout, stderr: opts.stderr}
	defer r.stop(5 * time.Second)

	warnedLogFatal := false // say it once, not after every change
	say := func(format string, a ...any) { fmt.Fprintf(opts.stdout, format+"\n", a...) }

	rebuild := func() {
		say("🔨 Building ...")
		if err := r.build(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			if r.cmd != nil {
				say("❌ Build failed. The previous version keeps running. Fix the error and save again.")
			} else {
				say("❌ Build failed. Fix the error and save again.")
			}
			return
		}
		if opts.tsOut != "" {
			note, err := generateTypeScript(ctx, r.bin, r.dir, opts.tsOut)
			switch {
			case err != nil && ctx.Err() != nil:
				return
			case err != nil:
				say("⚠️  Could not update the TypeScript files (the server still restarts): %v", err)
			default:
				say("📝 TypeScript files up to date in %s", opts.tsOut)
				if note != "" && !warnedLogFatal {
					say("⚠️  %s", note)
					warnedLogFatal = true
				}
			}
		}
		r.stop(5 * time.Second)
		if err := r.start(); err != nil {
			say("❌ Could not start the server: %v", err)
			return
		}
		say("✅ Server running. Watching for changes (Ctrl+C to stop).")
	}

	say("⚡ Aemeath dev server (hot reload)")
	rebuild()

	trigger := make(chan struct{}, 1)
	var timer *time.Timer
	schedule := func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(opts.debounce, func() {
			select {
			case trigger <- struct{}{}:
			default:
			}
		})
	}
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			say("\n👋 Stopping the dev server ...")
			return nil

		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// Directories created later (for example by "aemeath generate") must be watched too.
			if ev.Has(fsnotify.Create) {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() && !ignoredDir(info.Name()) {
					_ = addTree(watcher, ev.Name)
				}
			}
			if relevant(ev) {
				say("🔄 Change detected: %s", ev.Name)
				schedule()
			}

		case <-trigger:
			rebuild()

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			say("⚠️  Watcher error: %v", err)
		}
	}
}
