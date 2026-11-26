package cli

import (
	"path"
	"strings"
	"testing"
)

// ignoredByDocker reports whether one of the .dockerignore patterns excludes
// file: a pattern matches the path or one of its parent directories, and a
// leading "**/" matches at any depth (the rules Docker documents).
func ignoredByDocker(patterns []string, file string) bool {
	parts := strings.Split(file, "/")
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}
		anyDepth := strings.HasPrefix(p, "**/")
		p = strings.TrimPrefix(p, "**/")
		for start := 0; start < len(parts); start++ {
			if start > 0 && !anyDepth {
				break
			}
			for end := start + 1; end <= len(parts); end++ {
				if ok, _ := path.Match(p, strings.Join(parts[start:end], "/")); ok {
					return true
				}
			}
		}
	}
	return false
}

// The Dockerfile runs npm install in Linux, then COPY web/ ./ copies the
// project's web/ over it. Without these lines, the node_modules installed on
// the developer's machine (Windows binaries of esbuild and rollup, from
// Windows) replace the Linux ones and the build in Docker fails; web/dist
// from the machine would be copied in for nothing.
func TestDockerignoreKeepsLocalNodeModulesOutOfTheBuild(t *testing.T) {
	b, err := render("dockerignore.tmpl", projectData{GoVersion: "1.27", Web: true})
	if err != nil {
		t.Fatal(err)
	}
	patterns := strings.Split(string(b), "\n")
	for _, f := range []string{"web/node_modules/@rollup/rollup-win32-x64-msvc/rollup.node", "web/dist/index.html", ".env", "app.db"} {
		if !ignoredByDocker(patterns, f) {
			t.Errorf(".dockerignore lets %s into the build context:\n%s", f, b)
		}
	}
	for _, f := range []string{"web/src/main.tsx", "web/package.json", "main.go", "go.sum"} {
		if ignoredByDocker(patterns, f) {
			t.Errorf(".dockerignore leaves %s out of the build, but the Dockerfile needs it", f)
		}
	}
}
