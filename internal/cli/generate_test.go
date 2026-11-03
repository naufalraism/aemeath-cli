package cli

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newProject makes a project directory with a go.mod and the main.go of a new project.
func newProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/shop\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	main, err := render("main.go.tmpl", projectData{Module: "example.com/shop", Title: "shop"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), main, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func assertParses(t *testing.T, dir string, files ...string) {
	t.Helper()
	for _, f := range files {
		if _, err := parser.ParseFile(token.NewFileSet(), f, read(t, dir, f), parser.AllErrors); err != nil {
			t.Errorf("%s does not parse: %v", f, err)
		}
	}
}

func TestGenerateResourceCreatesFilesAndRegistersThem(t *testing.T) {
	dir := newProject(t)
	var out bytes.Buffer
	if err := generateResource(&out, resourceOpts{Root: dir, Name: "product-item"}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"handlers/productitem.go", "handlers/productitem_test.go"} {
		if !fileExists(filepath.Join(dir, f)) {
			t.Errorf("%s was not created", f)
		}
	}
	main := read(t, dir, "main.go")
	if !strings.Contains(main, "handlers.ProductItemHandler(app)") || !strings.Contains(main, `"example.com/shop/handlers"`) {
		t.Errorf("main.go was not wired:\n%s", main)
	}
	if !strings.Contains(out.String(), "main.go: registered") {
		t.Errorf("the output should say what happened to main.go:\n%s", out.String())
	}
	assertParses(t, dir, "main.go", "handlers/productitem.go", "handlers/productitem_test.go")

	// A second resource is added next to the first, and running it again changes nothing.
	for i := 0; i < 2; i++ {
		if err := generateResource(&out, resourceOpts{Root: dir, Name: "Category", Force: i == 1}); err != nil {
			t.Fatal(err)
		}
	}
	main = read(t, dir, "main.go")
	if strings.Count(main, "handlers.CategoryHandler(app)") != 1 || strings.Count(main, "handlers.ProductItemHandler(app)") != 1 {
		t.Errorf("each resource must be registered exactly once:\n%s", main)
	}
}

func TestGenerateResourceWritesNothingWhenAnyFileExists(t *testing.T) {
	dir := newProject(t)
	if err := os.MkdirAll(filepath.Join(dir, "handlers"), 0o755); err != nil {
		t.Fatal(err)
	}
	mine := "package handlers\n\n// hand-written, do not lose me\n"
	if err := os.WriteFile(filepath.Join(dir, "handlers", "product_test.go"), []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	before := read(t, dir, "main.go")

	err := generateResource(&bytes.Buffer{}, resourceOpts{Root: dir, Name: "Product", DB: true})
	if err == nil || !strings.Contains(err.Error(), "product_test.go") || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("err = %v, want a message naming the existing file and --force", err)
	}
	if fileExists(filepath.Join(dir, "handlers", "product.go")) || fileExists(filepath.Join(dir, "models", "product.go")) {
		t.Error("files were written although one of them already existed (half a resource)")
	}
	if read(t, dir, "handlers/product_test.go") != mine {
		t.Error("the existing file was modified")
	}
	if read(t, dir, "main.go") != before {
		t.Error("main.go must not change when nothing was generated")
	}
}

func TestGenerateResourceWithADatabase(t *testing.T) {
	dir := newProject(t)
	// An existing model is used, not overwritten.
	if err := os.MkdirAll(filepath.Join(dir, "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := "package models\n\n// my own model\ntype Product struct{ ID uint; Name string; Price int }\n"
	if err := os.WriteFile(filepath.Join(dir, "models", "product.go"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := generateResource(&out, resourceOpts{Root: dir, Name: "Product", DB: true, Prefix: "api/"}); err != nil {
		t.Fatal(err)
	}
	if read(t, dir, "models/product.go") != custom {
		t.Error("the existing model was overwritten")
	}
	if !strings.Contains(out.String(), "already exists, so it is used as it is") {
		t.Errorf("the reused model should be mentioned:\n%s", out.String())
	}
	handler := read(t, dir, "handlers/product.go")
	for _, want := range []string{`app.Group("/api/products")`, "aemeath.Lookup[*gorm.DB](app)", "db.From(c)", "gorm.ErrRecordNotFound"} {
		if !strings.Contains(handler, want) {
			t.Errorf("handlers/product.go is missing %q", want)
		}
	}
	if !strings.Contains(read(t, dir, "handlers/product_test.go"), `"/api/products"`) {
		t.Error("the test must use the prefix")
	}
	main := read(t, dir, "main.go")
	if strings.Index(main, `db.Open("app.db")`) >= strings.Index(main, "handlers.ProductHandler(app)") {
		t.Errorf("the database plugin must be registered before the handlers:\n%s", main)
	}
	assertParses(t, dir, "main.go", "handlers/product.go", "handlers/product_test.go")

	// A second DB resource reuses the database plugin.
	if err := generateResource(&out, resourceOpts{Root: dir, Name: "Order", DB: true, Prefix: "/api"}); err != nil {
		t.Fatal(err)
	}
	if strings.Count(read(t, dir, "main.go"), `db.Open("app.db")`) != 1 {
		t.Error("the database plugin must be registered only once")
	}
}

func TestGenerateResourceLeavesAnUneditableMainAloneAndSaysWhatToAdd(t *testing.T) {
	dir := newProject(t)
	custom := "package main\n\nfunc main() {\n\tprintln(\"my own bootstrap\")\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := generateResource(&out, resourceOpts{Root: dir, Name: "Product"}); err != nil {
		t.Fatal(err)
	}
	if read(t, dir, "main.go") != custom {
		t.Error("main.go must be left exactly as it was")
	}
	if !strings.Contains(out.String(), "I left main.go alone") || !strings.Contains(out.String(), "handlers.ProductHandler(app)") {
		t.Errorf("the user must be told what to add:\n%s", out.String())
	}
	if !fileExists(filepath.Join(dir, "handlers", "product.go")) {
		t.Error("the handlers must still be generated")
	}
}

func TestGenerateResourceNoWireAndNoTest(t *testing.T) {
	dir := newProject(t)
	before := read(t, dir, "main.go")
	if err := generateResource(&bytes.Buffer{}, resourceOpts{Root: dir, Name: "Product", NoWire: true, NoTest: true}); err != nil {
		t.Fatal(err)
	}
	if read(t, dir, "main.go") != before {
		t.Error("--no-wire must not touch main.go")
	}
	if fileExists(filepath.Join(dir, "handlers", "product_test.go")) {
		t.Error("--no-test must not create the test file")
	}
}

func TestGenerateResourceNeedsAProjectAndASafeName(t *testing.T) {
	if err := generateResource(&bytes.Buffer{}, resourceOpts{Root: t.TempDir(), Name: "Product"}); err == nil || !strings.Contains(err.Error(), "go.mod") {
		t.Errorf("err = %v, want a go.mod message", err)
	}
	dir := newProject(t)
	for _, bad := range []string{"../../evil", "a/b", ""} {
		if err := generateResource(&bytes.Buffer{}, resourceOpts{Root: dir, Name: bad}); err == nil {
			t.Errorf("name %q must be rejected", bad)
		}
	}
	if err := generateResource(&bytes.Buffer{}, resourceOpts{Root: dir, Name: "Product", Prefix: "/a b"}); err == nil {
		t.Error("an invalid prefix must be rejected")
	}
}

func TestNormalizePrefix(t *testing.T) {
	for in, want := range map[string]string{"": "", "/": "", "api": "/api", "/api": "/api", "/api/": "/api", "api/v1": "/api/v1", " /api ": "/api"} {
		got, err := normalizePrefix(in)
		if err != nil || got != want {
			t.Errorf("normalizePrefix(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"/a b", "//", "/a//b", "/../x", "/a?b", "/{id}"} {
		if got, err := normalizePrefix(bad); err == nil {
			t.Errorf("normalizePrefix(%q) = %q, want an error", bad, got)
		}
	}
}

func TestGenerateJobCreatesTheJobAndRegistersTheRunnerOnce(t *testing.T) {
	dir := newProject(t)
	var out bytes.Buffer
	if err := generateJob(&out, dir, "send-email", false, false); err != nil {
		t.Fatal(err)
	}
	if err := generateJob(&out, dir, "ResizeImage", false, false); err != nil {
		t.Fatal(err)
	}
	job := read(t, dir, "jobs/sendemail.go")
	if !strings.Contains(job, "func (j *SendEmailJob) Run(ctx context.Context) error") {
		t.Errorf("the job must implement jobs.Job (Run(ctx)):\n%s", job)
	}
	main := read(t, dir, "main.go")
	if strings.Count(main, "aejobs.New(") != 1 || !strings.Contains(main, `aejobs "github.com/naufalraism/aemeath/jobs"`) {
		t.Errorf("the runner must be registered exactly once:\n%s", main)
	}
	assertParses(t, dir, "main.go", "jobs/sendemail.go", "jobs/resizeimage.go")
	if !strings.Contains(out.String(), "aejobs.From(c).Enqueue(&jobs.SendEmailJob{})") {
		t.Errorf("the output should show how to enqueue the job:\n%s", out.String())
	}
	// Regenerating without --force must refuse to overwrite the job.
	if err := generateJob(&out, dir, "send-email", false, false); err == nil {
		t.Error("an existing job must not be overwritten")
	}
}

func TestGenerateTestOnlyAndSimpleFiles(t *testing.T) {
	dir := newProject(t)
	if err := generateTestOnly(&bytes.Buffer{}, dir, "Task", "/api", true, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, dir, "handlers/task_test.go"), "db.Open(") {
		t.Error("--db must generate the database test")
	}
	if err := generateSimple(&bytes.Buffer{}, dir, simpleGen{tmpl: "middleware.go.tmpl", dir: "middleware", suffix: ".go"}, "auth", false); err != nil {
		t.Fatal(err)
	}
	assertParses(t, dir, "handlers/task_test.go", "middleware/auth.go")
}

// Every resource lives in the same package, so two of them must never declare
// the same top-level name. Parsing the files one by one cannot see that.
func TestGeneratedResourcesDoNotCollideInOnePackage(t *testing.T) {
	dir := newProject(t)
	for _, o := range []resourceOpts{
		{Name: "Task", DB: true, Prefix: "/api"},
		{Name: "Product", DB: true},
		{Name: "Order"},
		{Name: "order-line"},
		{Name: "Category", DB: true},
	} {
		o.Root = dir
		if err := generateResource(&bytes.Buffer{}, o); err != nil {
			t.Fatal(err)
		}
	}
	if err := generateTestOnly(&bytes.Buffer{}, dir, "Extra", "", false, false); err != nil {
		t.Fatal(err)
	}

	declared := map[string]string{} // "package.name" -> file
	files, _ := filepath.Glob(filepath.Join(dir, "handlers", "*.go"))
	if len(files) < 11 {
		t.Fatalf("only %d files were generated", len(files))
	}
	for _, f := range files {
		file, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			var names []string
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.Name != "init" {
					names = append(names, d.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch sp := spec.(type) {
					case *ast.TypeSpec:
						names = append(names, sp.Name.Name)
					case *ast.ValueSpec:
						for _, n := range sp.Names {
							names = append(names, n.Name)
						}
					}
				}
			}
			for _, n := range names {
				key := file.Name.Name + "." + n
				if other, dup := declared[key]; dup {
					t.Errorf("%s is declared in both %s and %s: two resources cannot live in one package", n, filepath.Base(other), filepath.Base(f))
				}
				declared[key] = f
			}
		}
	}
}
