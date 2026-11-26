package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// frameworkVersion is the framework release this CLI's templates are tested
// with. "aemeath new" writes it into go.mod, and "aemeath generate" refuses a
// project on an older release, whose API the generated code may not match.
// Raise it with every CLI release that needs a newer framework.
const frameworkVersion = "v0.17.0"

var (
	requireAemeathRe = regexp.MustCompile(`(?m)^\s*(?:require\s+)?github\.com/naufalraism/aemeath\s+(v\S+)`)
	replaceAemeathRe = regexp.MustCompile(`(?m)^\s*(?:replace\s+)?github\.com/naufalraism/aemeath(?:\s+v\S+)?\s*=>`)
)

// checkFramework returns an error when root/go.mod requires a framework
// release older than frameworkVersion. A project that replaces the framework
// with a local checkout, or does not require it yet, is not checked.
func checkFramework(root string) error {
	src, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || replaceAemeathRe.Match(src) {
		return nil
	}
	m := requireAemeathRe.FindSubmatch(src)
	if m == nil {
		return nil
	}
	if v := string(m[1]); versionLess(v, frameworkVersion) {
		return fmt.Errorf("this project uses Aemeath %s, and the code this CLI generates needs %s or later; run: go get github.com/naufalraism/aemeath@%s (then go mod tidy), or use an older aemeath CLI", v, frameworkVersion, frameworkVersion)
	}
	return nil
}

// versionLess reports whether semantic version a is older than b. A version
// with a pre-release suffix (v1.2.0-rc.1, or a pseudo-version) is older than
// the same version without one.
func versionLess(a, b string) bool {
	pa, ra := splitVersion(a)
	pb, rb := splitVersion(b)
	for i := range pa {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return ra && !rb
}

func splitVersion(v string) ([3]int, bool) {
	var n [3]int
	core, _, pre := strings.Cut(strings.TrimPrefix(v, "v"), "-") // pre: there is a pre-release part
	core, _, _ = strings.Cut(core, "+")
	for i, part := range strings.SplitN(core, ".", 3) {
		n[i], _ = strconv.Atoi(part)
	}
	return n, pre
}
