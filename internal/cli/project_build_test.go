package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// TestGeneratedProjectsBuild creates a project from every template and
// database, runs the generators in it, and compiles, vets and tests the
// result against a real checkout of the framework. The other tests only look
// at the text of the generated files; this one is the only proof that the
// text is a program.
//
// It needs the framework's source, so it only runs when AEMEATH_FRAMEWORK_DIR
// is the full path of a checkout:
//
//	AEMEATH_FRAMEWORK_DIR=/full/path/to/aemeath go test ./internal/cli -run TestGeneratedProjectsBuild -v
//
// AEMEATH_TEST_GOMOD may name a file whose lines are appended to each
// project's go.mod (replace directives for a machine without network access;
// 'go mod tidy' is then allowed to leave gaps, which the build still reports).
func TestGeneratedProjectsBuild(t *testing.T) {
	framework := os.Getenv("AEMEATH_FRAMEWORK_DIR")
	if framework == "" {
		t.Skip("set AEMEATH_FRAMEWORK_DIR to a checkout of github.com/naufalraism/aemeath to run this test")
	}
	framework, err := filepath.Abs(framework)
	if err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(framework, "go.mod")) {
		t.Fatalf("AEMEATH_FRAMEWORK_DIR=%s has no go.mod", framework)
	}
	var extra []byte
	if p := os.Getenv("AEMEATH_TEST_GOMOD"); p != "" {
		if extra, err = os.ReadFile(p); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name, template, database string
		addDB                    string // database of an extra "generate api --db" resource
	}{
		{name: "api", template: "api"},
		{name: "api + generate --db sqlite", template: "api", addDB: dbSQLite},
		{name: "api + generate --db postgres", template: "api", addDB: "postgres"},
		{name: "api + generate --db mysql", template: "api", addDB: "mysql"},
		{name: "db sqlite", template: "db", database: dbSQLite},
		{name: "db postgres", template: "db", database: "postgres"},
		{name: "db mysql", template: "db", database: "mysql"},
		{name: "fullstack sqlite", template: "fullstack", database: dbSQLite},
		{name: "fullstack postgres", template: "fullstack", database: "postgres"},
		{name: "fullstack mysql", template: "fullstack", database: "mysql"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), "demo")
			var out bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			if err := runNew(cmd, root, "example.com/demo", framework, tc.template, tc.database, true); err != nil {
				t.Fatalf("new: %v\n%s", err, out.String())
			}
			if len(extra) > 0 {
				f, err := os.OpenFile(filepath.Join(root, "go.mod"), os.O_APPEND|os.O_WRONLY, 0o600)
				if err != nil {
					t.Fatal(err)
				}
				_, err = f.Write(append([]byte("\n"), extra...))
				if cerr := f.Close(); err != nil || cerr != nil {
					t.Fatal(err, cerr)
				}
			}
			check := func(stage string) {
				t.Helper()
				tidy := []string{"mod", "tidy"}
				if len(extra) > 0 {
					tidy = append(tidy, "-e")
				}
				for _, args := range [][]string{tidy, {"build", "./..."}, {"vet", "./..."}, {"test", "./..."}} {
					c := exec.Command("go", args...)
					c.Dir = root
					if b, err := c.CombinedOutput(); err != nil {
						t.Fatalf("%s: go %v: %v\n%s", stage, args, err, b)
					}
				}
			}
			check("new project")

			withDB := projectTemplates[tc.template].DB
			gens := []struct {
				what string
				run  func() error
			}{
				{"generate api Product --prefix /api", func() error {
					return generateResource(io.Discard, resourceOpts{Root: root, Name: "Product", Prefix: "/api"})
				}},
				{"generate api OrderItem --db", func() error {
					if !withDB && tc.addDB == "" {
						return nil
					}
					return generateResource(io.Discard, resourceOpts{Root: root, Name: "OrderItem", DB: true, Database: tc.addDB})
				}},
				{"generate job SendReport", func() error { return generateJob(io.Discard, root, "SendReport", false, false) }},
				{"generate middleware Audit", func() error {
					return generateSimple(io.Discard, root, simpleGen{tmpl: "middleware.go.tmpl", dir: "middleware", suffix: ".go"}, "Audit", false)
				}},
				{"generate model Invoice", func() error {
					return generateSimple(io.Discard, root, simpleGen{tmpl: "model.go.tmpl", dir: "models", suffix: ".go"}, "Invoice", false)
				}},
			}
			for _, g := range gens {
				if err := g.run(); err != nil {
					t.Fatalf("%s: %v", g.what, err)
				}
			}
			// The time limit of a generated job only works if its method matches the
			// framework's interface. Older frameworks have no such interface; there the method is unused.
			if fileExists(filepath.Join(framework, "jobs", "jobs.go")) {
				src, err := os.ReadFile(filepath.Join(framework, "jobs", "jobs.go"))
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Contains(src, []byte("type Timeouter interface")) {
					assert := "package jobs\n\nimport aejobs \"github.com/naufalraism/aemeath/jobs\"\n\nvar _ aejobs.Timeouter = (*SendReportJob)(nil)\n"
					if err := os.WriteFile(filepath.Join(root, "jobs", "timeouter_assert.go"), []byte(assert), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			check("after the generators")

			// What 'aemeath ts' and 'aemeath dev --ts-out' do: build the app and let it write its client.
			apiDir := filepath.Join(root, "web", "src", "api")
			var errOut bytes.Buffer
			if err := buildAndGenerate(context.Background(), root, ".", apiDir, io.Discard, &errOut); err != nil {
				t.Fatalf("aemeath ts: %v\n%s", err, errOut.String())
			}
			for _, name := range []string{"client.ts", "schemas.ts"} {
				b, err := os.ReadFile(filepath.Join(apiDir, name))
				if err != nil {
					t.Fatalf("aemeath ts: %v\n%s", err, errOut.String())
				}
				if !bytes.Contains(b, []byte("Product")) {
					t.Errorf("%s does not mention the generated Product resource", name)
				}
			}
		})
	}
}
