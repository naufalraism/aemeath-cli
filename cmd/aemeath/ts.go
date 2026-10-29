package main

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
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), envTypeScriptOut+"="+abs)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	runErr := cmd.Run()
	if runErr != nil && ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("the program did not exit within 60 seconds; does main do work before app.Listen that blocks?\n%s", tail(out.String()))
	}

	var missing []string
	for _, name := range tsFiles {
		if _, err := os.Stat(filepath.Join(abs, name)); err != nil {
			missing = append(missing, name)
		}
	}
	if runErr != nil {
		// Projects written as log.Fatal(app.Listen(...)) exit with status 1 even
		// though Listen returned nil. If the program says it wrote the files and
		// they are there, the generation worked: warn instead of failing.
		if len(missing) == 0 && strings.Contains(out.String(), envTypeScriptOut) {
			return "main exits with status 1 after app.Listen returns nil (log.Fatal(nil) prints <nil>). Use `if err := app.Listen(addr); err != nil { log.Fatal(err) }` so a clean shutdown exits with status 0.", nil
		}
		return "", fmt.Errorf("running the program to generate TypeScript failed: %w\n%s", runErr, tail(out.String()))
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("the program did not write %s to %s. Does main call app.Listen (or app.Serve), and is the Aemeath framework recent enough to support it?\n%s",
			strings.Join(missing, ", "), abs, tail(out.String()))
	}
	return "", nil
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
		fmt.Fprintln(stderr, "⚠️  "+note)
	}
	return err
}
