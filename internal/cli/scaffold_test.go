package cli

import (
	"bytes"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileNeverOverwritesWithoutForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "handlers", "product.go")
	if err := writeFile(path, []byte("v1"), false); err != nil {
		t.Fatal(err)
	}
	err := writeFile(path, []byte("v2"), false)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("second write without force: err = %v, want an 'already exists' error", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "v1" {
		t.Fatalf("existing file was modified: %q", got)
	}
	if err := writeFile(path, []byte("v3"), true); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "v3" {
		t.Fatalf("force did not overwrite: %q", got)
	}
}

func TestModulePath(t *testing.T) {
	for content, want := range map[string]string{
		"module github.com/me/app\n\ngo 1.27\n":  "github.com/me/app",
		"module \"my app\"\n":                    "my app",
		"// comment\nmodule   example.com/x  \n": "example.com/x",
		"go 1.27\n":                              "",
	} {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644)
		if got := modulePath(dir); got != want {
			t.Errorf("modulePath(%q) = %q, want %q", content, got, want)
		}
	}
	if got := modulePath(t.TempDir()); got != "" {
		t.Errorf("modulePath without go.mod = %q, want empty", got)
	}
}

// Every Go template must produce code that parses, for awkward names too.
func TestGoTemplatesProduceValidCode(t *testing.T) {
	templates := []string{"handler.go.tmpl", "handler_db.go.tmpl", "test.go.tmpl", "test_db.go.tmpl", "model.go.tmpl", "middleware.go.tmpl", "job.go.tmpl", "main.go.tmpl"}
	for _, raw := range []string{"Product", "product-item", "APIKey", "category", "type", "map"} {
		names, err := parseName(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, prefix := range []string{"", "/api/v1"} {
			for _, web := range []bool{false, true} {
				data := struct {
					Names
					Module, Title, Prefix string
					Web, DB               bool
				}{names, "example.com/app", "app", prefix, web, true}
				for _, tmpl := range templates {
					out, err := render(tmpl, data)
					if err != nil {
						t.Errorf("%s with name %q: %v", tmpl, raw, err)
						continue
					}
					if _, err := parser.ParseFile(token.NewFileSet(), tmpl, out, parser.AllErrors); err != nil {
						t.Errorf("%s with name %q does not parse: %v\n%s", tmpl, raw, err, out)
					}
					if bytes.Contains(out, []byte("\r")) {
						t.Errorf("%s output contains CR characters", tmpl)
					}
				}
			}
		}
	}
}

func TestTemplatesAreInEnglish(t *testing.T) {
	checked := 0
	err := fs.WalkDir(templateFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, _ := templateFS.ReadFile(path)
		checked++
		for _, w := range []string{"Halo", "Jangan", "Menjalankan", "Tambahkan", "adalah", "berjalan", "untuk "} {
			if strings.Contains(string(b), w) {
				t.Errorf("%s still contains the Indonesian word %q", path, w)
			}
		}
		return nil
	})
	if err != nil || checked < 20 {
		t.Fatalf("checked %d files, err = %v", checked, err)
	}
}

func TestGoModTemplate(t *testing.T) {
	out, err := render("go.mod.tmpl", projectData{Module: "example.com/app", GoVersion: "1.27"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "module example.com/app") || !strings.Contains(s, "go 1.27") {
		t.Fatalf("unexpected go.mod:\n%s", s)
	}
	if strings.Contains(s, "replace") {
		t.Fatalf("go.mod must not contain a replace directive unless --replace is used:\n%s", s)
	}
	out, _ = render("go.mod.tmpl", projectData{Module: "m", GoVersion: "1.27", Replace: "/path with space/aemeath"})
	if !strings.Contains(string(out), `replace github.com/naufalraism/aemeath => "/path with space/aemeath"`) {
		t.Fatalf("replace directive missing or unquoted:\n%s", out)
	}
}

// log.Fatal(app.Listen(...)) prints "<nil>" and exits with status 1 after a clean shutdown.
func TestMainTemplateChecksTheListenError(t *testing.T) {
	out, err := render("main.go.tmpl", projectData{Module: "m", Title: "app", GoVersion: "1.27"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "log.Fatal(app.Listen") || !strings.Contains(string(out), "err := app.Listen(") {
		t.Fatalf("main.go must check the error returned by Listen:\n%s", out)
	}
}

func TestDockerfileForAPIAndFullstack(t *testing.T) {
	plain, err := render("Dockerfile.tmpl", projectData{GoVersion: "1.27"})
	if err != nil {
		t.Fatal(err)
	}
	full, err := render("Dockerfile.tmpl", projectData{GoVersion: "1.27", Web: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"FROM golang:1.27-alpine AS builder", "USER app", "chown app:app /app", "ENTRYPOINT"} {
		if !strings.Contains(string(plain), want) || !strings.Contains(string(full), want) {
			t.Errorf("both Dockerfiles must contain %q", want)
		}
	}
	if strings.Contains(string(plain), "node:") || strings.Contains(string(plain), "web/dist") {
		t.Errorf("an API-only project must not build a frontend:\n%s", plain)
	}
	if !strings.Contains(string(full), "FROM node:22-alpine AS web") || !strings.Contains(string(full), "COPY --from=web /web/dist ./web/dist") {
		t.Errorf("a fullstack project must build the frontend and copy it:\n%s", full)
	}
	// The stage numbers in the comments must follow the number of stages.
	if !strings.Contains(string(plain), "# Stage 2: run") || !strings.Contains(string(full), "# Stage 3: run") {
		t.Errorf("stage comments are off:\n%s\n%s", plain, full)
	}
	if n := strings.Count(string(full), "\nFROM "); n != 3 {
		t.Errorf("a fullstack Dockerfile has %d stages, want 3", n)
	}
}
