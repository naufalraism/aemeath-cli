package main

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

// render executes an embedded template. Go output is run through gofmt, which
// also catches templates that would generate uncompilable code.
func render(tmplName string, data any) ([]byte, error) {
	content, err := templateFS.ReadFile("templates/" + tmplName)
	if err != nil {
		return nil, fmt.Errorf("reading template %s: %w", tmplName, err)
	}
	tmpl, err := template.New(tmplName).Option("missingkey=error").Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parsing template %s: %w", tmplName, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing template %s: %w", tmplName, err)
	}
	if strings.HasSuffix(tmplName, ".go.tmpl") {
		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return nil, fmt.Errorf("template %s generated invalid Go code: %w", tmplName, err)
		}
		return formatted, nil
	}
	return buf.Bytes(), nil
}

// writeFile writes content to path, creating parent directories. It refuses
// to overwrite an existing file unless force is set, so re-running a
// generator can never destroy hand-written code.
func writeFile(path string, content []byte, force bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}
		return err
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// renderTo renders a template and writes it to dest.
func renderTo(tmplName, dest string, data any, force bool) error {
	content, err := render(tmplName, data)
	if err != nil {
		return err
	}
	return writeFile(dest, content, force)
}

var moduleLine = regexp.MustCompile(`(?m)^\s*module\s+("[^"]+"|\S+)`)

// modulePath returns the module path declared in dir/go.mod, or "" if there is none.
func modulePath(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	m := moduleLine.FindSubmatch(data)
	if m == nil {
		return ""
	}
	return strings.Trim(string(m[1]), `"`)
}

// goVersion returns the installed toolchain as "1.27" (major.minor), falling
// back to the version the CLI itself was built with.
func goVersion() string {
	out, err := exec.Command("go", "env", "GOVERSION").Output()
	if err == nil {
		if v := parseGoVersion(strings.TrimSpace(string(out))); v != "" {
			return v
		}
	}
	return "1.27"
}

var goVersionRe = regexp.MustCompile(`^go(\d+\.\d+)`)

func parseGoVersion(s string) string {
	if m := goVersionRe.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

var projectNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
