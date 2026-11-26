package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A project created without a version gets whatever framework is newest when
// "go mod tidy" runs, so a later breaking framework release could break
// projects this CLI creates. The CLI writes the release its templates are
// tested with.
func TestNewPinsTheTestedFramework(t *testing.T) {
	target := filepath.Join(t.TempDir(), "demo")
	if out, err := runNewForTest(t, target, "api"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if want := "require github.com/naufalraism/aemeath " + frameworkVersion; !strings.Contains(read(t, target, "go.mod"), want) {
		t.Fatalf("go.mod lacks %q:\n%s", want, read(t, target, "go.mod"))
	}
}

func projectWith(t *testing.T, gomod string) string {
	t.Helper()
	dir := newProject(t)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// The generated code uses the framework's current API (sqlite.Open since
// v0.12.0, for example), so generating into a project on an older release
// gives code that does not compile. generate says what to run instead, and
// writes nothing.
func TestGenerateRefusesAnOlderFramework(t *testing.T) {
	// "go mod tidy" writes the requirement inside a require ( ... ) block.
	inBlock := projectWith(t, "module example.com/shop\n\ngo 1.27\n\nrequire (\n\tgithub.com/naufalraism/aemeath v0.11.0\n\tgorm.io/gorm v1.31.2\n)\n")
	if err := generateResource(&bytes.Buffer{}, resourceOpts{Root: inBlock, Name: "Product"}); err == nil || !strings.Contains(err.Error(), "v0.11.0") {
		t.Errorf("require block: err = %v, want the version error", err)
	}

	dir := projectWith(t, "module example.com/shop\n\ngo 1.27\n\nrequire github.com/naufalraism/aemeath v0.11.0\n")
	var out bytes.Buffer
	err := generateResource(&out, resourceOpts{Root: dir, Name: "Product", DB: true})
	if err == nil || !strings.Contains(err.Error(), "v0.11.0") || !strings.Contains(err.Error(), "go get github.com/naufalraism/aemeath@"+frameworkVersion) {
		t.Fatalf("err = %v, want one naming v0.11.0 and the go get command", err)
	}
	if fileExists(filepath.Join(dir, "handlers", "product.go")) {
		t.Error("a handler was written anyway")
	}
	for name, run := range map[string]func() error{
		"job": func() error { return generateJob(&out, dir, "Send", false, true) },
		"middleware": func() error {
			return generateSimple(&out, dir, simpleGen{tmpl: "middleware.go.tmpl", dir: "middleware", suffix: ".go"}, "Audit", false)
		},
		"test": func() error { return generateTestOnly(&out, dir, "Product", "", false, false) },
	} {
		if err := run(); err == nil || !strings.Contains(err.Error(), "v0.11.0") {
			t.Errorf("generate %s: err = %v, want the version error", name, err)
		}
	}
}

func TestGenerateAcceptsTheTestedFrameworkAndNewer(t *testing.T) {
	for _, req := range []string{
		"require github.com/naufalraism/aemeath " + frameworkVersion + "\n",
		"require github.com/naufalraism/aemeath v9.0.0\n",
		"require (\n\tgithub.com/naufalraism/aemeath v99.1.0 // indirect\n)\n",
		"require github.com/naufalraism/aemeath v0.11.0\n\nreplace github.com/naufalraism/aemeath => ../aemeath\n", // a local checkout: its own version
		"", // not required yet ("go mod tidy" not run)
	} {
		dir := projectWith(t, "module example.com/shop\n\ngo 1.27\n\n"+req)
		if err := generateResource(&bytes.Buffer{}, resourceOpts{Root: dir, Name: "Product"}); err != nil {
			t.Errorf("go.mod with %q: %v", req, err)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		less bool
	}{
		{"v0.11.0", "v0.17.0", true},
		{"v0.9.9", "v0.17.0", true},
		{"v0.17.0", "v0.17.0", false},
		{"v0.17.1", "v0.17.0", false},
		{"v0.18.0-0.20261127000000-abcdef123456", "v0.17.0", false},
		{"v0.17.0-rc.1", "v0.17.0", true},
		{"v1.0.0", "v0.17.0", false},
	} {
		if got := versionLess(tc.a, tc.b); got != tc.less {
			t.Errorf("versionLess(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.less)
		}
	}
}
