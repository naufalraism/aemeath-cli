package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

type projectData struct {
	Module    string
	Title     string
	NPMName   string
	GoVersion string
	Replace   string
	Web       bool
	DB        bool
}

var modulePathRe = regexp.MustCompile(`^[A-Za-z0-9._~/-]+$`)

// projectTemplate is what "aemeath new --template" builds.
type projectTemplate struct {
	Desc     string
	DB       bool
	Web      bool
	Resource string // a CRUD resource created with "generate api", or ""
	Prefix   string // URL prefix of the API routes
}

var projectTemplates = map[string]projectTemplate{
	"api":       {Desc: "a minimal API (default)"},
	"db":        {Desc: "an API with SQLite and a Task CRUD resource", DB: true, Resource: "Task"},
	"fullstack": {Desc: "the db template plus a React + TypeScript app in web/, using the generated client, Zod schemas and hooks", DB: true, Web: true, Resource: "Task", Prefix: "/api"},
}

func templateNames() string {
	names := make([]string, 0, len(projectTemplates))
	for n := range projectTemplates {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// webFiles are copied verbatim (apart from the __TITLE__ and __NAME__ tokens)
// because JSX uses {{ }}, which would clash with Go templates.
var webFiles = []struct{ src, dest string }{
	{"web/package.json", "web/package.json"},
	{"web/tsconfig.json", "web/tsconfig.json"},
	{"web/vite.config.ts", "web/vite.config.ts"},
	{"web/index.html", "web/index.html"},
	{"web/gitignore.tmpl", "web/.gitignore"},
	{"web/src/main.tsx", "web/src/main.tsx"},
	{"web/src/App.tsx", "web/src/App.tsx"},
	{"web/src/index.css", "web/src/index.css"},
}

var npmInvalid = regexp.MustCompile(`[^a-z0-9-_.]+`)

// npmName makes a valid package name from a project name.
func npmName(title string) string {
	n := strings.Trim(npmInvalid.ReplaceAllString(strings.ToLower(title), "-"), "-._")
	if n == "" {
		n = "app"
	}
	return n + "-web"
}

func newNewCmd() *cobra.Command {
	var module, replace, template string
	var noTidy bool

	cmd := &cobra.Command{
		Use:   "new [project-name]",
		Short: "Create a new Aemeath project",
		Long: `Creates a project, registers its first resource and runs 'go mod tidy'.

Templates (--template):
  api        a minimal API (default)
  db         an API with SQLite and a Task CRUD resource
  fullstack  the db template plus a React + TypeScript app in web/ that uses the
             generated client, Zod schemas and React Query hooks`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return runNew(cmd, args[0], module, replace, template, noTidy)
		},
	}
	cmd.Flags().StringVar(&module, "module", "", "Go module path (default: the project name)")
	cmd.Flags().StringVar(&replace, "replace", "", "path of a local Aemeath checkout to use instead of the published module (for framework development)")
	cmd.Flags().StringVarP(&template, "template", "t", "api", "project template: "+templateNames())
	cmd.Flags().BoolVar(&noTidy, "no-tidy", false, "do not run 'go mod tidy' after creating the project")
	return cmd
}

func runNew(cmd *cobra.Command, target, module, replace, template string, noTidy bool) error {
	out := cmd.OutOrStdout()
	tpl, ok := projectTemplates[template]
	if !ok {
		return fmt.Errorf("unknown template %q: choose one of %s", template, templateNames())
	}
	base := filepath.Base(filepath.Clean(target))
	if !projectNameRe.MatchString(base) {
		return fmt.Errorf("invalid project name %q: use letters, digits, '.', '_' and '-', starting with a letter or digit", base)
	}
	if module == "" {
		module = base
	}
	if !modulePathRe.MatchString(module) {
		return fmt.Errorf("invalid module path %q", module)
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%s already exists", target)
	}

	data := projectData{Module: module, Title: base, NPMName: npmName(base), GoVersion: goVersion(), Web: tpl.Web, DB: tpl.DB}
	if replace != "" {
		abs, err := filepath.Abs(replace)
		if err != nil {
			return err
		}
		data.Replace = abs
	}

	files := []struct{ tmpl, dest string }{
		{"go.mod.tmpl", "go.mod"},
		{"main.go.tmpl", "main.go"},
		{"gitignore.tmpl", ".gitignore"},
		{"env.example.tmpl", ".env.example"},
		{"readme.md.tmpl", "README.md"},
	}
	for _, f := range files {
		if err := renderTo(f.tmpl, filepath.Join(target, f.dest), data, false); err != nil {
			return err
		}
	}
	if tpl.Web {
		for _, f := range webFiles {
			content, err := renderRaw(f.src, map[string]string{"__TITLE__": data.Title, "__NAME__": data.NPMName})
			if err != nil {
				return err
			}
			if err := writeFile(filepath.Join(target, f.dest), content, false); err != nil {
				return err
			}
		}
	}
	outf(out, "✨ Project %q created in %s (template: %s)\n", base, target, template)

	if tpl.Resource != "" {
		if err := generateResource(out, resourceOpts{Root: target, Name: tpl.Resource, DB: tpl.DB, Prefix: tpl.Prefix}); err != nil {
			return err
		}
	}

	tidied := false
	if !noTidy {
		outln(out, "📦 Running go mod tidy ...")
		tidy := exec.Command("go", "mod", "tidy")
		tidy.Dir = target
		tidy.Stdout = out
		tidy.Stderr = cmd.ErrOrStderr()
		if err := tidy.Run(); err != nil {
			return fmt.Errorf("the project was created, but 'go mod tidy' failed (%w); run it yourself inside %s", err, target)
		}
		tidied = true
	}

	if tpl.Web {
		apiDir := filepath.Join(target, "web", "src", "api")
		generated := false
		if tidied {
			outln(out, "📝 Generating the TypeScript client, Zod schemas and hooks ...")
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			err := buildAndGenerate(ctx, target, ".", apiDir, io.Discard, cmd.ErrOrStderr())
			stop()
			if err != nil {
				outf(cmd.ErrOrStderr(), "⚠️  Could not generate them now: %v\n", err)
			} else {
				generated = true
			}
		}
		if !generated {
			outf(out, "👉 Then generate them: cd %s && aemeath ts --out web/src/api\n", target)
		}
	}

	printNextSteps(out, target, tpl)
	return nil
}

func printNextSteps(out io.Writer, target string, tpl projectTemplate) {
	if tpl.Web {
		outf(out, "👉 Next, in two terminals:\n     cd %s && aemeath dev --ts-out web/src/api\n     cd %s/web && npm install && npm run dev      (then open http://localhost:5173)\n", target, target)
		return
	}
	outf(out, "👉 Next: cd %s && aemeath dev\n", target)
}
