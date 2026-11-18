package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

// The statements the README and docs/cli make about the commands, checked
// against the commands themselves. Each subtest's name is the claim.

func rootForTest(t *testing.T) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "aemeath", Version: "test"}
	root.AddCommand(newNewCmd(), newGenerateCmd(), newTSCmd(), newDevCmd())
	return root
}

func find(t *testing.T, root *cobra.Command, path string) *cobra.Command {
	t.Helper()
	cmd, _, err := root.Find(strings.Fields(path))
	if err != nil || cmd == root {
		t.Fatalf("no command %q: %v", path, err)
	}
	return cmd
}

func TestDocumentedFlagsExist(t *testing.T) {
	root := rootForTest(t)
	for path, flags := range map[string][]string{
		"new":                 {"module", "replace", "template", "database", "no-tidy"},
		"generate api":        {"db", "database", "prefix", "no-wire", "no-test", "force"},
		"generate test":       {"db", "prefix", "force"},
		"generate job":        {"no-wire", "force"},
		"generate model":      {"force"},
		"generate middleware": {"force"},
		"generate docker":     {"force"},
		"ts":                  {"out", "pkg"},
		"dev":                 {"pkg", "delay", "ts-out"},
	} {
		t.Run("documented flags of 'aemeath "+path+"'", func(t *testing.T) {
			cmd := find(t, root, path)
			for _, name := range flags {
				if cmd.Flags().Lookup(name) == nil && cmd.InheritedFlags().Lookup(name) == nil {
					t.Errorf("--%s is documented but does not exist", name)
				}
			}
		})
	}
	t.Run("-t is short for --template and -o for --out, -f for --force", func(t *testing.T) {
		if find(t, root, "new").Flags().ShorthandLookup("t") == nil {
			t.Error("new has no -t")
		}
		if find(t, root, "ts").Flags().ShorthandLookup("o") == nil {
			t.Error("ts has no -o")
		}
		if find(t, root, "generate api").InheritedFlags().ShorthandLookup("f") == nil {
			t.Error("generate has no -f")
		}
	})
	t.Run("aemeath --version reports a version", func(t *testing.T) {
		if resolveVersion() == "" {
			t.Error("resolveVersion() is empty")
		}
	})
	t.Run("--delay waits 300ms by default", func(t *testing.T) {
		if got := find(t, root, "dev").Flags().Lookup("delay").DefValue; got != "300ms" {
			t.Errorf("default %q", got)
		}
	})
}

func TestNameFormsAreEquivalent(t *testing.T) {
	// "<Name> may be written as Product, product-item, product_item or ProductItem."
	for _, raw := range []string{"product-item", "product_item", "ProductItem", "productItem"} {
		t.Run(raw+" names the same resource as ProductItem", func(t *testing.T) {
			a, err := parseName(raw)
			if err != nil {
				t.Fatal(err)
			}
			b, err := parseName("ProductItem")
			if err != nil {
				t.Fatal(err)
			}
			if a != b {
				t.Errorf("%+v != %+v", a, b)
			}
		})
	}
}

func TestGenerateClaims(t *testing.T) {
	newProject := func(t *testing.T) string {
		t.Helper()
		root := filepath.Join(t.TempDir(), "demo")
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		if err := runNew(cmd, root, "example.com/demo", "", "api", "", true); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
		return root
	}
	t.Run("nothing is written if any of the files already exists", func(t *testing.T) {
		root := newProject(t)
		if err := os.MkdirAll(filepath.Join(root, "handlers"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "handlers", "product_test.go"), []byte("// mine\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(filepath.Join(root, "main.go"))
		if err != nil {
			t.Fatal(err)
		}
		if err := generateResource(io.Discard, resourceOpts{Root: root, Name: "Product"}); err == nil {
			t.Fatal("no error although a destination existed")
		}
		if _, err := os.Stat(filepath.Join(root, "handlers", "product.go")); !os.IsNotExist(err) {
			t.Error("handlers/product.go was written anyway")
		}
		if got, _ := os.ReadFile(filepath.Join(root, "handlers", "product_test.go")); string(got) != "// mine\n" {
			t.Error("the existing file was overwritten")
		}
		if after, _ := os.ReadFile(filepath.Join(root, "main.go")); !bytes.Equal(before, after) {
			t.Error("main.go was edited although nothing was generated")
		}
	})
	t.Run("--force overwrites existing files", func(t *testing.T) {
		root := newProject(t)
		if err := generateResource(io.Discard, resourceOpts{Root: root, Name: "Product"}); err != nil {
			t.Fatal(err)
		}
		if err := generateResource(io.Discard, resourceOpts{Root: root, Name: "Product", Force: true}); err != nil {
			t.Fatalf("--force: %v", err)
		}
	})
	t.Run("running a command twice never registers anything twice", func(t *testing.T) {
		root := newProject(t)
		for i := 0; i < 2; i++ {
			if err := generateResource(io.Discard, resourceOpts{Root: root, Name: "Product", Force: true}); err != nil {
				t.Fatal(err)
			}
		}
		main, err := os.ReadFile(filepath.Join(root, "main.go"))
		if err != nil {
			t.Fatal(err)
		}
		if n := bytes.Count(main, []byte("handlers.ProductHandler(app)")); n != 1 {
			t.Errorf("registered %d times:\n%s", n, main)
		}
	})
	t.Run("a main.go it cannot edit safely is left exactly as it was, and the lines are printed", func(t *testing.T) {
		root := newProject(t)
		broken := "package main\n\nfunc notMain() {\n}\n"
		if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(broken), 0o600); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := generateResource(&out, resourceOpts{Root: root, Name: "Product"}); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(filepath.Join(root, "main.go")); string(got) != broken {
			t.Errorf("main.go was changed:\n%s", got)
		}
		if !strings.Contains(out.String(), "handlers.ProductHandler(app)") {
			t.Errorf("the lines to add were not printed:\n%s", out.String())
		}
	})
	t.Run("generate registers at the // aemeath:plugins and // aemeath:handlers comments", func(t *testing.T) {
		root := newProject(t)
		if err := generateResource(io.Discard, resourceOpts{Root: root, Name: "Product", DB: true}); err != nil {
			t.Fatal(err)
		}
		main, err := os.ReadFile(filepath.Join(root, "main.go"))
		if err != nil {
			t.Fatal(err)
		}
		plugins := bytes.Index(main, []byte("db.Open("))
		handlers := bytes.Index(main, []byte("handlers.ProductHandler(app)"))
		if plugins < 0 || handlers < 0 || plugins > handlers {
			t.Errorf("plugin at %d, handler at %d:\n%s", plugins, handlers, main)
		}
	})
	t.Run("generate docker builds web/ too when it exists", func(t *testing.T) {
		root := newProject(t)
		data := projectData{GoVersion: goVersion(), Web: fileExists(filepath.Join(root, "web", "package.json"))}
		var withWeb, withoutWeb strings.Builder
		if err := renderTo("Dockerfile.tmpl", filepath.Join(root, "Dockerfile"), data, false); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
		if err != nil {
			t.Fatal(err)
		}
		withoutWeb.Write(b)
		data.Web = true
		if err := renderTo("Dockerfile.tmpl", filepath.Join(root, "Dockerfile"), data, true); err != nil {
			t.Fatal(err)
		}
		b, err = os.ReadFile(filepath.Join(root, "Dockerfile"))
		if err != nil {
			t.Fatal(err)
		}
		withWeb.Write(b)
		if strings.Contains(withoutWeb.String(), "npm run build") {
			t.Error("a project without web/ gets a node stage")
		}
		if !strings.Contains(withWeb.String(), "npm run build") {
			t.Error("a project with web/ gets no node stage")
		}
		for _, s := range []string{"USER app", "FROM golang:"} { // "multi-stage, non-root"
			if !strings.Contains(withWeb.String(), s) {
				t.Errorf("the Dockerfile has no %q", s)
			}
		}
	})
}

func TestDevIgnoresAndStopClaims(t *testing.T) {
	t.Run("hidden folders, vendor, node_modules, tmp and uploads are not watched", func(t *testing.T) {
		for _, d := range []string{".git", "vendor", "node_modules", "tmp", "uploads"} {
			if !ignoredDir(d) {
				t.Errorf("%s is watched", d)
			}
		}
		for _, d := range []string{"handlers", "web", "."} {
			if ignoredDir(d) {
				t.Errorf("%s is ignored", d)
			}
		}
	})
	t.Run("_test.go files do not trigger a rebuild, .go, go.mod, go.sum and .env do", func(t *testing.T) {
		for _, name := range []string{"handlers/user.go", "go.mod", "go.sum", ".env"} {
			if !relevant(fsnotify.Event{Name: name, Op: fsnotify.Write}) {
				t.Errorf("%s does not trigger a rebuild", name)
			}
		}
		for _, name := range []string{"handlers/user_test.go", "web/src/App.tsx", "README.md"} {
			if relevant(fsnotify.Event{Name: name, Op: fsnotify.Write}) {
				t.Errorf("%s triggers a rebuild", name)
			}
		}
	})
	t.Run("the README describes how dev stops the server on this platform", func(t *testing.T) {
		readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
		if err != nil {
			t.Fatal(err)
		}
		// stop() signals SIGTERM on Unix, but Windows has no SIGTERM for
		// processes, so there it kills the process at once.
		if runtime.GOOS == "windows" && !bytes.Contains(readme, []byte("Windows")) {
			t.Error("the README promises SIGTERM, which dev cannot send on Windows, and does not mention Windows")
		}
	})
}
