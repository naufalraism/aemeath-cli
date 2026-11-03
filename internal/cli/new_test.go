package cli

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func runNewForTest(t *testing.T, target, template string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err := runNew(cmd, target, "example.com/demo", "", template, true) // no network: skip go mod tidy
	return out.String() + errOut.String(), err
}

func TestNewTemplates(t *testing.T) {
	type want struct {
		files, absent []string
		mainHas       []string
		mainLacks     []string
	}
	cases := map[string]want{
		"api": {
			files:     []string{"go.mod", "main.go", ".gitignore", ".env.example", "README.md"},
			absent:    []string{"web", "handlers", "models"},
			mainHas:   []string{`"/"`, "HelloOut", "// aemeath:plugins", "// aemeath:handlers"},
			mainLacks: []string{"db.Open", "web/dist"},
		},
		"db": {
			files:     []string{"main.go", "handlers/task.go", "handlers/task_test.go", "models/task.go"},
			absent:    []string{"web"},
			mainHas:   []string{`db.Open("app.db")`, "handlers.TaskHandler(app)", "HelloOut"},
			mainLacks: []string{"web/dist"},
		},
		"fullstack": {
			files: []string{
				"main.go", "handlers/task.go", "models/task.go",
				"web/package.json", "web/tsconfig.json", "web/vite.config.ts", "web/index.html",
				"web/.gitignore", "web/src/main.tsx", "web/src/App.tsx", "web/src/index.css",
			},
			mainHas:   []string{`db.Open("app.db")`, "handlers.TaskHandler(app)", `app.Static("/", "web/dist")`},
			mainLacks: []string{"HelloOut"},
		},
	}
	for name, w := range cases {
		t.Run(name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "Demo App-1")
			if _, err := runNewForTest(t, target, name); err == nil {
				t.Fatal("a project name with a space must be rejected")
			}
			target = filepath.Join(t.TempDir(), "demo")
			out, err := runNewForTest(t, target, name)
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			for _, f := range w.files {
				if !fileExists(filepath.Join(target, f)) {
					t.Errorf("%s is missing", f)
				}
			}
			for _, f := range w.absent {
				if fileExists(filepath.Join(target, f)) {
					t.Errorf("%s must not exist in the %s template", f, name)
				}
			}
			main := read(t, target, "main.go")
			for _, s := range w.mainHas {
				if !strings.Contains(main, s) {
					t.Errorf("main.go lacks %q:\n%s", s, main)
				}
			}
			for _, s := range w.mainLacks {
				if strings.Contains(main, s) {
					t.Errorf("main.go must not contain %q:\n%s", s, main)
				}
			}
			assertParses(t, target, "main.go")
			if name != "api" {
				assertParses(t, target, "handlers/task.go", "handlers/task_test.go", "models/task.go")
			}
		})
	}
}

func TestFullstackTemplateUsesTheAPIPrefixEverywhere(t *testing.T) {
	target := filepath.Join(t.TempDir(), "shop")
	if out, err := runNewForTest(t, target, "fullstack"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(read(t, target, "handlers/task.go"), `app.Group("/api/tasks")`) {
		t.Error("the handlers must live under /api")
	}
	if !strings.Contains(read(t, target, "web/vite.config.ts"), `"/api": api`) {
		t.Error("the dev server must proxy /api")
	}
	pkg := read(t, target, "web/package.json")
	if !strings.Contains(pkg, `"name": "shop-web"`) || strings.Contains(pkg, "__") {
		t.Errorf("the tokens in package.json were not replaced:\n%s", pkg)
	}
	if html := read(t, target, "web/index.html"); !strings.Contains(html, "<title>shop</title>") {
		t.Errorf("index.html title:\n%s", html)
	}
	for _, f := range []string{"web/src/App.tsx", "web/src/main.tsx", "web/vite.config.ts"} {
		if strings.Contains(read(t, target, f), "__TITLE__") {
			t.Errorf("%s still has an unreplaced token", f)
		}
	}
}

func TestNewRejectsUnknownTemplatesExistingFoldersAndBadModules(t *testing.T) {
	if _, err := runNewForTest(t, filepath.Join(t.TempDir(), "x"), "nope"); err == nil || !strings.Contains(err.Error(), "api, db, fullstack") {
		t.Errorf("err = %v, want the list of templates", err)
	}
	existing := t.TempDir()
	if _, err := runNewForTest(t, existing, "api"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("err = %v", err)
	}
	cmd := &cobra.Command{}
	if err := runNew(cmd, filepath.Join(t.TempDir(), "ok"), "bad module!", "", "api", true); err == nil {
		t.Error("an invalid module path must be rejected")
	}
}

func TestEveryWebFileIsEmbedded(t *testing.T) {
	for _, f := range webFiles {
		if _, err := fs.Stat(templateFS, "templates/"+f.src); err != nil {
			t.Errorf("%s: %v", f.src, err)
		}
	}
	// The reverse: nothing in templates/web is forgotten in the list.
	listed := map[string]bool{}
	for _, f := range webFiles {
		listed["templates/"+f.src] = true
	}
	_ = fs.WalkDir(templateFS, "templates/web", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !listed[p] {
			t.Errorf("%s is embedded but never copied into a new project", p)
		}
		return nil
	})
}

func TestNPMName(t *testing.T) {
	for in, want := range map[string]string{"shop": "shop-web", "My_App.v2": "my_app.v2-web", "UPPER": "upper-web", "a b": "a-b-web"} {
		if got := npmName(in); got != want {
			t.Errorf("npmName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVersionFallsBackToDev(t *testing.T) {
	if got := resolveVersion(); got == "" {
		t.Fatal("the version must never be empty")
	}
}
