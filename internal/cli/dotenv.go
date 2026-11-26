package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// parseDotEnv reads the KEY=VALUE lines of a .env file. Blank lines and lines
// starting with # are skipped, "export " before a key is allowed, a value in
// single or double quotes is taken as it is, and in an unquoted value " #"
// starts a comment. Nothing is expanded. A line it cannot read is an error
// that names the line, so a typo never silently drops a setting.
func parseDotEnv(src string) ([]string, error) {
	var out []string
	for i, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line) // also drops the \r of CRLF files
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !envKeyRe.MatchString(key) {
			return nil, fmt.Errorf(".env line %d: expected KEY=value", i+1)
		}
		value = strings.TrimSpace(value)
		if q := value[:min(1, len(value))]; q == `"` || q == `'` {
			end := strings.LastIndex(value, q)
			if end == 0 {
				return nil, fmt.Errorf(".env line %d: the value of %s has no closing %s", i+1, key, q)
			}
			value = value[1:end]
		} else if c := strings.Index(value, " #"); c >= 0 {
			value = strings.TrimSpace(value[:c])
		}
		out = append(out, key+"="+value)
	}
	return out, nil
}

// serverEnv is the environment for the project's program: this process's
// environment plus the variables of dir/.env that it does not already set
// (the shell wins, so "PORT=4000 aemeath dev" still works). It also returns
// how many variables came from .env. A missing .env is not an error.
func serverEnv(dir string) ([]string, int, error) {
	env := os.Environ()
	b, err := os.ReadFile(filepath.Join(dir, ".env"))
	if errors.Is(err, os.ErrNotExist) {
		return env, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	vars, err := parseDotEnv(string(b))
	if err != nil {
		return nil, 0, err
	}
	n := 0
	for _, kv := range vars {
		key, _, _ := strings.Cut(kv, "=")
		if _, set := os.LookupEnv(key); !set {
			env = append(env, kv)
			n++
		}
	}
	return env, n, nil
}
