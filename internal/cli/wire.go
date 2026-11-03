package cli

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// aemeathModule is the import path of the framework.
const aemeathModule = "github.com/naufalraism/aemeath"

// Marker comments that main.go carries (see templates/main.go.tmpl). Registrations
// added by "aemeath generate" go right after them, so you control where they land.
const (
	markerPlugins  = "aemeath:plugins"
	markerHandlers = "aemeath:handlers"
)

type importSpec struct {
	Alias string // empty for the default name
	Path  string
}

func (i importSpec) text() string {
	if i.Alias != "" {
		return i.Alias + " " + `"` + i.Path + `"`
	}
	return `"` + i.Path + `"`
}

// wireSpec describes one registration to add to func main() of main.go.
type wireSpec struct {
	Imports []importSpec
	// Stmt renders the statement for the variable that holds the aemeath app (usually "app").
	Stmt func(app string) string
	// Already matches source that already has this registration: nothing is changed.
	Already *regexp.Regexp
	// After, if it matches a top-level statement of main, places the new statement
	// after the last such statement, which keeps related registrations together.
	After *regexp.Regexp
	// Marker is a comment that says where registrations of this kind go.
	Marker string
}

type wireOutcome int

const (
	wired wireOutcome = iota
	alreadyWired
	notWired // main.go was left untouched; see the reason
)

type wireResult struct {
	Outcome wireOutcome
	Reason  string
}

// wireMain adds a registration to func main() of <dir>/main.go. It is
// deliberately conservative: it only edits a file that parses, only inserts
// text (nothing of yours is rewritten), refuses to write a result that does not
// parse, and otherwise leaves main.go alone and reports why so the caller can
// print instructions for doing it by hand.
func wireMain(dir string, spec wireSpec) (wireResult, error) {
	path := filepath.Join(dir, "main.go")
	src, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return wireResult{notWired, "there is no main.go here"}, nil
		}
		return wireResult{}, err
	}
	if spec.Already != nil && spec.Already.Match(src) {
		return wireResult{Outcome: alreadyWired}, nil
	}

	out, reason := insertRegistration(src, spec)
	if out == nil {
		return wireResult{notWired, reason}, nil
	}
	if err := atomicWrite(path, out); err != nil {
		return wireResult{}, err
	}
	return wireResult{Outcome: wired}, nil
}

// edit replaces src[start:end] with text. All edits are computed against the
// original source and applied from the end of the file to the start, so no
// edit shifts the offsets of another.
type edit struct {
	start, end int
	text       string
}

func applyEdits(src []byte, edits []edit) []byte {
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := append([]byte(nil), src...)
	for _, e := range edits {
		out = append(out[:e.start], append([]byte(e.text), out[e.end:]...)...)
	}
	return out
}

// insertRegistration returns the new source, or nil and the reason it could not do it.
func insertRegistration(src []byte, spec wireSpec) ([]byte, string) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", src, parser.ParseComments)
	if err != nil {
		return nil, "main.go does not parse yet"
	}
	mainFn := findMain(file)
	if mainFn == nil || mainFn.Body == nil {
		return nil, "there is no func main() in main.go"
	}
	app := findAppVar(file, mainFn)
	if app == "" {
		return nil, "I could not find where main.go creates the app (aemeath.New())"
	}

	off := func(p token.Pos) int { return fset.Position(p).Offset }
	lineStart := func(o int) int { return bytes.LastIndexByte(src[:o], '\n') + 1 }
	lineEnd := func(o int) int { // offset just after the newline that ends the line containing o
		if i := bytes.IndexByte(src[o:], '\n'); i >= 0 {
			return o + i + 1
		}
		return len(src)
	}

	// Where the statement goes: after the last related statement, else after the
	// marker comment, else just before the statement that starts the server.
	insertAt := -1
	if spec.After != nil {
		for _, st := range mainFn.Body.List {
			if spec.After.Match(src[off(st.Pos()):off(st.End())]) {
				insertAt = lineEnd(off(st.End()))
			}
		}
	}
	if insertAt < 0 && spec.Marker != "" {
		for _, cg := range file.Comments {
			for _, c := range cg.List {
				if strings.Contains(c.Text, spec.Marker) && c.Pos() > mainFn.Body.Lbrace && c.End() < mainFn.Body.Rbrace {
					insertAt = lineEnd(off(c.End()))
				}
			}
		}
	}
	if insertAt < 0 {
		listen := regexp.MustCompile(`\.(Listen|ListenTLS|Serve)\(`)
		for _, st := range mainFn.Body.List {
			if listen.Match(src[off(st.Pos()):off(st.End())]) {
				insertAt = lineStart(off(st.Pos()))
				break
			}
		}
	}
	if insertAt < 0 {
		return nil, "I could not find a place in main() (no `// " + spec.Marker + "` comment and no app.Listen call)"
	}
	edits := []edit{{insertAt, insertAt, "\t" + strings.ReplaceAll(spec.Stmt(app), "\n", "\n\t") + "\n"}}

	// Imports that are missing.
	have := map[string]bool{}
	for _, im := range file.Imports {
		have[strings.Trim(im.Path.Value, `"`)] = true
	}
	var missing []string
	for _, im := range spec.Imports {
		if !have[im.Path] {
			missing = append(missing, im.text())
		}
	}
	if len(missing) > 0 {
		edits = append(edits, importEdit(src, fset, file, missing))
	}

	formatted, err := format.Source(applyEdits(src, edits))
	if err != nil {
		return nil, "the edited main.go would not parse, so I did not touch it"
	}
	return formatted, ""
}

func findMain(file *ast.File) *ast.FuncDecl {
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "main" {
			return fn
		}
	}
	return nil
}

// findAppVar returns the name of the variable assigned from aemeath.New().
func findAppVar(file *ast.File, mainFn *ast.FuncDecl) string {
	pkg := "aemeath"
	for _, im := range file.Imports {
		if strings.Trim(im.Path.Value, `"`) == aemeathModule && im.Name != nil {
			pkg = im.Name.Name
		}
	}
	isNew := func(e ast.Expr) bool {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "New" {
			return false
		}
		id, ok := sel.X.(*ast.Ident)
		return ok && id.Name == pkg
	}
	name := ""
	ast.Inspect(mainFn.Body, func(n ast.Node) bool {
		if name != "" {
			return false
		}
		switch v := n.(type) {
		case *ast.AssignStmt:
			if len(v.Rhs) == 1 && len(v.Lhs) == 1 && isNew(v.Rhs[0]) {
				if id, ok := v.Lhs[0].(*ast.Ident); ok {
					name = id.Name
				}
			}
		case *ast.ValueSpec:
			if len(v.Values) == 1 && len(v.Names) == 1 && isNew(v.Values[0]) {
				name = v.Names[0].Name
			}
		}
		return true
	})
	return name
}

// importEdit adds import specs. A block `import ( ... )` gets them before its
// closing parenthesis, a lone `import "x"` is rewritten as a block, and a file
// without imports gets a block after the package clause.
func importEdit(src []byte, fset *token.FileSet, file *ast.File, specs []string) edit {
	off := func(p token.Pos) int { return fset.Position(p).Offset }
	var decl *ast.GenDecl
	for _, d := range file.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			decl = g
			break
		}
	}
	add := ""
	for _, s := range specs {
		add += "\t" + s + "\n"
	}
	switch {
	case decl == nil:
		at := off(file.Name.End())
		at += bytes.IndexByte(src[at:], '\n') + 1
		return edit{at, at, "\nimport (\n" + add + ")\n"}
	case decl.Lparen.IsValid():
		at := off(decl.Rparen)
		if at > 0 && src[at-1] != '\n' {
			return edit{at, at, "\n" + add}
		}
		return edit{at, at, add}
	default: // import "x"
		start, end := off(decl.Pos()), off(decl.End())
		old := strings.TrimSpace(strings.TrimPrefix(string(src[start:end]), "import"))
		return edit{start, end, "import (\n\t" + old + "\n" + add + ")"}
	}
}

func atomicWrite(path string, content []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".aemeath-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	return nil
}
