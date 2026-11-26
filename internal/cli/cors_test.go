package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// CORS() allows every origin. A generated project does not need CORS at all
// (the React app reaches the API through Vite's proxy, on the same origin),
// and on an internal network "*" lets any web page an employee opens read the
// API's answers. main.go shows how to allow a given origin instead.
func TestNewProjectsDoNotAllowEveryOrigin(t *testing.T) {
	for _, name := range []string{"api", "db", "fullstack"} {
		target := filepath.Join(t.TempDir(), "demo")
		if out, err := runNewForTest(t, target, name); err != nil {
			t.Fatalf("%s: %v\n%s", name, err, out)
		}
		main := read(t, target, "main.go")
		for _, line := range strings.Split(main, "\n") {
			code := strings.TrimSpace(line)
			if strings.Contains(code, "aemeath.CORS(") && !strings.HasPrefix(code, "//") {
				t.Errorf("%s: main.go enables CORS: %s", name, code)
			}
		}
		if !strings.Contains(main, `//	app.Use(aemeath.CORS(aemeath.CORSConfig{AllowOrigins: []string{"https://app.example.com"}}))`) {
			t.Errorf("%s: main.go does not show how to allow one origin", name)
		}
	}
}
