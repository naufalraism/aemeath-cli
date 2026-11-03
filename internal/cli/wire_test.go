package cli

import (
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const templateMain = `package main

import (
	"log"
	"os"

	"github.com/naufalraism/aemeath"
)

func main() {
	app := aemeath.New()
	app.Use(aemeath.Logger())

	// aemeath:plugins
	// aemeath:handlers

	app.EnableDocs(aemeath.Info{Title: "demo"})

	if err := app.Listen(":" + os.Getenv("PORT")); err != nil {
		log.Fatal(err)
	}
}
`

func writeMain(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readMain(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func mustParse(t *testing.T, src string) {
	t.Helper()
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", src, parser.ParseComments); err != nil {
		t.Fatalf("the result does not parse: %v\n%s", err, src)
	}
	if formatted, err := format.Source([]byte(src)); err != nil || string(formatted) != src {
		t.Fatalf("the result is not gofmt-clean (err %v):\n%s", err, src)
	}
}

func TestWireHandlerAfterTheMarkerAndAddsTheImport(t *testing.T) {
	dir := writeMain(t, templateMain)
	n, _ := parseName("Product")
	res, err := wireMain(dir, handlerWire("example.com/shop", n))
	if err != nil || res.Outcome != wired {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	got := readMain(t, dir)
	mustParse(t, got)
	if !strings.Contains(got, `"example.com/shop/handlers"`) {
		t.Errorf("import missing:\n%s", got)
	}
	marker := strings.Index(got, "// aemeath:handlers")
	call := strings.Index(got, "handlers.ProductHandler(app)")
	docs := strings.Index(got, "app.EnableDocs")
	if marker < 0 || call <= marker || call >= docs {
		t.Errorf("the call must sit between the marker and EnableDocs:\n%s", got)
	}
}

func TestWireIsIdempotentAndKeepsResourcesTogether(t *testing.T) {
	dir := writeMain(t, templateMain)
	for _, name := range []string{"Product", "Category", "Product"} {
		n, _ := parseName(name)
		if _, err := wireMain(dir, handlerWire("example.com/shop", n)); err != nil {
			t.Fatal(err)
		}
	}
	got := readMain(t, dir)
	mustParse(t, got)
	if strings.Count(got, "handlers.ProductHandler(app)") != 1 || strings.Count(got, "handlers.CategoryHandler(app)") != 1 {
		t.Errorf("each handler must be registered exactly once:\n%s", got)
	}
	if strings.Count(got, `"example.com/shop/handlers"`) != 1 {
		t.Errorf("the import must be added once:\n%s", got)
	}
	n, _ := parseName("Product")
	if res, _ := wireMain(dir, handlerWire("example.com/shop", n)); res.Outcome != alreadyWired {
		t.Errorf("a second run must report alreadyWired, got %+v", res)
	}
	if strings.Index(got, "ProductHandler") > strings.Index(got, "CategoryHandler") {
		t.Errorf("the second resource goes after the first:\n%s", got)
	}
}

func TestWireBeforeListenWhenThereAreNoMarkers(t *testing.T) {
	dir := writeMain(t, strings.ReplaceAll(strings.ReplaceAll(templateMain, "\t// aemeath:plugins\n", ""), "\t// aemeath:handlers\n", ""))
	n, _ := parseName("Task")
	if res, err := wireMain(dir, handlerWire("m", n)); err != nil || res.Outcome != wired {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	got := readMain(t, dir)
	mustParse(t, got)
	if strings.Index(got, "handlers.TaskHandler(app)") >= strings.Index(got, "app.Listen") {
		t.Errorf("without markers the call goes before app.Listen:\n%s", got)
	}
}

func TestWireDatabasePluginThenHandlersKeepsTheOrder(t *testing.T) {
	dir := writeMain(t, templateMain)
	n, _ := parseName("Task")
	for _, spec := range []wireSpec{dbPluginWire(), handlerWire("m", n), jobsRunnerWire()} {
		if res, err := wireMain(dir, spec); err != nil || res.Outcome != wired {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
	}
	got := readMain(t, dir)
	mustParse(t, got)
	for _, want := range []string{`db.Open("app.db")`, "aejobs.New(aejobs.WithWorkers(4))", `aejobs "github.com/naufalraism/aemeath/jobs"`, `"github.com/naufalraism/aemeath/db"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "db.Open") >= strings.Index(got, "handlers.TaskHandler") {
		t.Errorf("the database plugin must be registered before the handlers that use it:\n%s", got)
	}
	if strings.Count(got, `"log"`) != 1 {
		t.Errorf(`"log" is already imported and must not be added twice:`+"\n%s", got)
	}
}

func TestWireTurnsALoneImportIntoABlock(t *testing.T) {
	dir := writeMain(t, `package main

import "github.com/naufalraism/aemeath"

func main() {
	app := aemeath.New()
	_ = app.Listen(":3000")
}
`)
	n, _ := parseName("Task")
	if res, err := wireMain(dir, handlerWire("m", n)); err != nil || res.Outcome != wired {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	got := readMain(t, dir)
	mustParse(t, got)
	if !strings.Contains(got, "import (") || !strings.Contains(got, `"github.com/naufalraism/aemeath"`) || !strings.Contains(got, `"m/handlers"`) {
		t.Errorf("both imports must survive:\n%s", got)
	}
}

func TestWireWorksWhenThereIsNoImportAtAll(t *testing.T) {
	dir := writeMain(t, "package main\n\nfunc main() {\n\tapp := aemeath.New()\n\tapp.Listen(\":3000\")\n}\n")
	n, _ := parseName("Task")
	if res, err := wireMain(dir, handlerWire("m", n)); err != nil || res.Outcome != wired {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	mustParse(t, readMain(t, dir))
}

func TestWireFindsTheAppUnderAnyNameAndAlias(t *testing.T) {
	dir := writeMain(t, `package main

import (
	ae "github.com/naufalraism/aemeath"
)

func main() {
	server := ae.New()
	// aemeath:handlers
	_ = server.Listen(":3000")
}
`)
	n, _ := parseName("Task")
	if res, err := wireMain(dir, handlerWire("m", n)); err != nil || res.Outcome != wired {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	got := readMain(t, dir)
	mustParse(t, got)
	if !strings.Contains(got, "handlers.TaskHandler(server)") {
		t.Errorf("the call must use the variable that holds the app:\n%s", got)
	}
}

// When main.go cannot be edited safely it must stay byte for byte as it was.
func TestWireLeavesUnsuitableFilesUntouched(t *testing.T) {
	cases := map[string]string{
		"does not parse":     "package main\n\nfunc main() { app := aemeath.New(\n",
		"no main function":   "package main\n\nfunc other() {}\n",
		"no aemeath.New":     "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n",
		"no place to put it": "package main\n\nimport \"github.com/naufalraism/aemeath\"\n\nfunc main() {\n\tapp := aemeath.New()\n\t_ = app\n}\n",
		"marker in another function": `package main

import "github.com/naufalraism/aemeath"

func helper() {
	// aemeath:handlers
}

func main() {
	app := aemeath.New()
	_ = app
}
`,
	}
	n, _ := parseName("Task")
	for name, src := range cases {
		dir := writeMain(t, src)
		res, err := wireMain(dir, handlerWire("m", n))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if res.Outcome != notWired || res.Reason == "" {
			t.Errorf("%s: res = %+v, want notWired with a reason", name, res)
		}
		if readMain(t, dir) != src {
			t.Errorf("%s: main.go was modified although the wiring was refused", name)
		}
	}
	if res, err := wireMain(t.TempDir(), handlerWire("m", n)); err != nil || res.Outcome != notWired {
		t.Errorf("a missing main.go must be reported, not fail: %+v %v", res, err)
	}
}

func TestWireKeepsYourCodeAndCommentsExactly(t *testing.T) {
	src := `package main

import (
	"log"

	"github.com/naufalraism/aemeath"
)

// main starts the shop API.
func main() {
	app := aemeath.New() // the application

	/* a block comment
	   spanning lines */
	x := map[string]int{
		"a": 1,
	}
	_ = x

	// aemeath:handlers

	log.Fatal(app.Listen(":3000"))
}
`
	dir := writeMain(t, src)
	n, _ := parseName("Task")
	if _, err := wireMain(dir, handlerWire("m", n)); err != nil {
		t.Fatal(err)
	}
	got := readMain(t, dir)
	mustParse(t, got)
	// Remove what was added and the original must come back exactly.
	restored := strings.Replace(got, "\thandlers.TaskHandler(app)\n", "", 1)
	restored = strings.Replace(restored, "\t\"m/handlers\"\n", "", 1)
	if restored != src {
		t.Errorf("more than the registration changed.\n--- original\n%s\n--- after removing the additions\n%s", src, restored)
	}
}

func TestWirePreservesTheFileMode(t *testing.T) {
	dir := writeMain(t, templateMain)
	path := filepath.Join(dir, "main.go")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	n, _ := parseName("Task")
	_, _ = wireMain(dir, handlerWire("m", n))
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 && os.PathSeparator != '\\' {
		t.Errorf("mode = %o, want 600", info.Mode().Perm())
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestWireHandlesWindowsLineEndings(t *testing.T) {
	dir := writeMain(t, strings.ReplaceAll(templateMain, "\n", "\r\n"))
	n, _ := parseName("Task")
	res, err := wireMain(dir, handlerWire("m", n))
	if err != nil || res.Outcome != wired {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if got := readMain(t, dir); !strings.Contains(got, "handlers.TaskHandler(app)") {
		t.Errorf("the call was not added:\n%s", got)
	}
}
