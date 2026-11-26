package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// envTypeScriptOut is the variable that makes an Aemeath application write its
// TypeScript files and exit instead of serving (App.Listen checks it).
const envTypeScriptOut = "AEMEATH_TS_OUT"

// tsFiles are the files an application writes.
var tsFiles = []string{"client.ts", "schemas.ts", "hooks.ts"}

// generateTypeScript runs an already built application binary in "write the
// TypeScript and exit" mode. The CLI cannot read your Go types itself: only
// the program knows its routes, so it is asked to write the files.
//
// The returned note is a warning for the user (empty when there is nothing to warn about).
func generateTypeScript(ctx context.Context, bin, dir, outDir string) (note string, err error) {
	abs, err := filepath.Abs(outDir)
	if err != nil {
		return "", err
	}
	// The program writes into an empty folder of its own: files it did not
	// write are then missing, even when an earlier run left them in outDir.
	stage, err := os.MkdirTemp("", "aemeath-ts-out-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(stage) }()

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	env, _, err := serverEnv(dir)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, bin)
	cmd.Dir = dir
	cmd.Env = append(env, envTypeScriptOut+"="+stage)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	runErr := cmd.Run()
	if runErr != nil && ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("the program did not exit within 60 seconds; does main do work before app.Listen that blocks?\n%s", tail(out.String()))
	}

	var missing []string
	for _, name := range tsFiles {
		if _, err := os.Stat(filepath.Join(stage, name)); err != nil {
			missing = append(missing, name)
		}
	}
	if runErr != nil && (len(missing) > 0 || !strings.Contains(out.String(), envTypeScriptOut)) {
		return "", fmt.Errorf("running the program to generate TypeScript failed: %w\n%s", runErr, tail(out.String()))
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("the program did not write %s. Does main call app.Listen (or app.Serve), and is the Aemeath framework recent enough to support it?\n%s",
			strings.Join(missing, ", "), tail(out.String()))
	}
	if err := copyChanged(stage, abs, tsFiles); err != nil {
		return "", err
	}
	if runErr != nil {
		// Projects written as log.Fatal(app.Listen(...)) exit with status 1 even
		// though Listen returned nil. The program said it wrote the files and
		// they are there, so the generation worked: warn instead of failing.
		return "main exits with status 1 after app.Listen returns nil (log.Fatal(nil) prints <nil>). Use `if err := app.Listen(addr); err != nil { log.Fatal(err) }` so a clean shutdown exits with status 0.", nil
	}
	return "", nil
}

// copyChanged copies the named files from src to dst, creating dst, and
// leaves a file alone when its content is already the same, so a bundler
// watching dst does not reload for nothing.
func copyChanged(src, dst string, names []string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, name := range names {
		b, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			return err
		}
		if old, err := os.ReadFile(filepath.Join(dst, name)); err == nil && bytes.Equal(old, b) {
			continue
		}
		if err := os.WriteFile(filepath.Join(dst, name), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// tail keeps the last lines of program output for an error message.
func tail(s string) string {
	s = strings.TrimSpace(s)
	lines := strings.Split(s, "\n")
	if len(lines) > 15 {
		lines = append([]string{"..."}, lines[len(lines)-15:]...)
	}
	return strings.Join(lines, "\n")
}

// buildAndGenerate builds the project once into a temporary binary and asks it for its TypeScript.
func buildAndGenerate(ctx context.Context, dir, pkg, outDir string, stdout, stderr io.Writer) error {
	tmp, err := os.MkdirTemp("", "aemeath-ts-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	r := &runner{dir: dir, pkg: pkg, bin: filepath.Join(tmp, binaryName("app")), stdout: stdout, stderr: stderr}
	if err := r.build(ctx); err != nil {
		return fmt.Errorf("build failed (see the errors above): %w", err)
	}
	note, err := generateTypeScript(ctx, r.bin, dir, outDir)
	if note != "" {
		outln(stderr, "⚠️  "+note)
	}
	return err
}
