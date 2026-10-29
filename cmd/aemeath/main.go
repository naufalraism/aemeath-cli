// Command aemeath is the command line interface of the Aemeath web framework.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// version is set at build time: go build -ldflags "-X main.version=v0.2.0".
var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		// cobra has already printed the error.
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "aemeath",
		Short:         "The Aemeath framework CLI",
		Version:       version,
		SilenceErrors: false,
	}
	root.AddCommand(newNewCmd(), newDevCmd(), newTSCmd(), newGenerateCmd())
	return root
}

// ---------------------------------------------------------------- new

type projectData struct {
	Module    string
	Title     string
	GoVersion string
	Replace   string
}

var modulePathRe = regexp.MustCompile(`^[A-Za-z0-9._~/-]+$`)

func newNewCmd() *cobra.Command {
	var module, replace string
	var noTidy bool

	cmd := &cobra.Command{
		Use:   "new [project-name]",
		Short: "Create a new Aemeath project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return runNew(cmd, args[0], module, replace, noTidy)
		},
	}
	cmd.Flags().StringVar(&module, "module", "", "Go module path (default: the project name)")
	cmd.Flags().StringVar(&replace, "replace", "", "path of a local Aemeath checkout to use instead of the published module (for framework development)")
	cmd.Flags().BoolVar(&noTidy, "no-tidy", false, "do not run 'go mod tidy' after creating the project")
	return cmd
}

func runNew(cmd *cobra.Command, target, module, replace string, noTidy bool) error {
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

	data := projectData{Module: module, Title: base, GoVersion: goVersion()}
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
	fmt.Fprintf(cmd.OutOrStdout(), "✨ Project %q created in %s\n", base, target)

	if !noTidy {
		fmt.Fprintln(cmd.OutOrStdout(), "📦 Running go mod tidy ...")
		tidy := exec.Command("go", "mod", "tidy")
		tidy.Dir = target
		tidy.Stdout = cmd.OutOrStdout()
		tidy.Stderr = cmd.ErrOrStderr()
		if err := tidy.Run(); err != nil {
			return fmt.Errorf("the project was created, but 'go mod tidy' failed (%w); run it yourself inside %s", err, target)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "👉 Next: cd %s && aemeath dev\n", target)
	return nil
}

// ---------------------------------------------------------------- dev

func newTSCmd() *cobra.Command {
	var pkg, out string

	cmd := &cobra.Command{
		Use:   "ts",
		Short: "Generate the TypeScript client, Zod schemas and React Query hooks",
		Long: `Builds your project and asks it to write client.ts, schemas.ts and hooks.ts
(generated from your Go routes) into the output directory.

The files are only rewritten when their content changed, so a bundler that
watches the directory does not reload for nothing.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := buildAndGenerate(ctx, ".", pkg, out, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "📝 Wrote %s to %s\n", strings.Join(tsFiles, ", "), out)
			fmt.Fprintln(cmd.OutOrStdout(), "👉 schemas.ts needs the 'zod' package and hooks.ts needs '@tanstack/react-query' (v5).")
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "out", "o", "api-client", "directory to write the TypeScript files to")
	cmd.Flags().StringVar(&pkg, "pkg", ".", "package to build and run")
	return cmd
}

func newDevCmd() *cobra.Command {
	var pkg, tsOut string
	var delay time.Duration

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run the server and restart it when Go files change",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runDev(ctx, devOptions{
				dir:      ".",
				pkg:      pkg,
				tsOut:    tsOut,
				debounce: delay,
				stdout:   cmd.OutOrStdout(),
				stderr:   cmd.ErrOrStderr(),
			})
		},
	}
	cmd.Flags().StringVar(&pkg, "pkg", ".", "package to build and run")
	cmd.Flags().StringVar(&tsOut, "ts-out", "", "also regenerate the TypeScript client, schemas and hooks in this directory after every successful build")
	cmd.Flags().DurationVar(&delay, "delay", 300*time.Millisecond, "wait this long after the last change before rebuilding")
	return cmd
}

// ---------------------------------------------------------------- generate

type genData struct {
	Names
	Module string
}

type genSpec struct {
	use, short  string
	tmpl        string
	dir         string
	fileSuffix  string
	needsModule bool
	hints       func(n Names, module string) []string
}

func newGenerateCmd() *cobra.Command {
	var force bool

	gen := &cobra.Command{
		Use:   "generate",
		Short: "Generate code (api, model, middleware, test, job, docker)",
	}
	gen.PersistentFlags().BoolVarP(&force, "force", "f", false, "overwrite files that already exist")

	specs := []genSpec{
		{
			use: "api [Name]", short: "Generate CRUD handlers for a resource",
			tmpl: "handler.go.tmpl", dir: "handlers", fileSuffix: ".go",
			hints: func(n Names, module string) []string {
				return []string{
					"👉 Register it in main.go:",
					fmt.Sprintf("     import %q", moduleOr(module)+"/handlers"),
					fmt.Sprintf("     handlers.%sHandler(app)", n.Pascal),
				}
			},
		},
		{
			use: "model [Name]", short: "Generate a GORM model",
			tmpl: "model.go.tmpl", dir: "models", fileSuffix: ".go",
			hints: func(n Names, module string) []string {
				return []string{
					"👉 Run 'go mod tidy' to add gorm.io/gorm, then register the database plugin in main.go:",
					"     import \"github.com/naufalraism/aemeath/db\"",
					"     plugin, err := db.Open(\"app.db\", &models." + n.Pascal + "{})",
					"     app.RegisterPlugin(plugin)   // handlers then use db.From(c)",
				}
			},
		},
		{
			use: "middleware [Name]", short: "Generate a custom middleware",
			tmpl: "middleware.go.tmpl", dir: "middleware", fileSuffix: ".go",
			hints: func(n Names, module string) []string {
				return []string{
					"👉 Use it in main.go:",
					fmt.Sprintf("     import %q", moduleOr(module)+"/middleware"),
					fmt.Sprintf("     app.Use(middleware.%sMiddleware())", n.Pascal),
				}
			},
		},
		{
			use: "test [Name]", short: "Generate tests for the handlers of a resource",
			tmpl: "test.go.tmpl", dir: "handlers", fileSuffix: "_test.go", needsModule: true,
			hints: func(n Names, module string) []string {
				return []string{"👉 The tests need handlers/" + n.File + ".go (aemeath generate api " + n.Pascal + "). Run: go test ./..."}
			},
		},
		{
			use: "job [Name]", short: "Generate a background job",
			tmpl: "job.go.tmpl", dir: "jobs", fileSuffix: ".go",
		},
	}
	for _, s := range specs {
		s := s
		gen.AddCommand(&cobra.Command{
			Use:   s.use,
			Short: s.short,
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.SilenceUsage = true
				return runGenerate(cmd, s, args[0], force)
			},
		})
	}

	gen.AddCommand(&cobra.Command{
		Use:   "docker",
		Short: "Generate Dockerfile, docker-compose.yml and .dockerignore",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			data := projectData{GoVersion: goVersion()}
			for _, f := range []struct{ tmpl, dest string }{
				{"Dockerfile.tmpl", "Dockerfile"},
				{"docker-compose.yml.tmpl", "docker-compose.yml"},
				{"dockerignore.tmpl", ".dockerignore"},
			} {
				if err := renderTo(f.tmpl, f.dest, data, force); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "✨ %s created\n", f.dest)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "👉 Run: docker compose up --build")
			return nil
		},
	})
	return gen
}

func moduleOr(module string) string {
	if module == "" {
		return "<your-module>"
	}
	return module
}

func runGenerate(cmd *cobra.Command, s genSpec, rawName string, force bool) error {
	names, err := parseName(rawName)
	if err != nil {
		return err
	}
	module := modulePath(".")
	if s.needsModule && module == "" {
		return fmt.Errorf("go.mod not found: run this command from the root of your project")
	}

	dest := filepath.Join(s.dir, names.File+s.fileSuffix)
	if err := renderTo(s.tmpl, dest, genData{Names: names, Module: module}, force); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✨ %s created\n", dest)
	if s.hints != nil {
		for _, line := range s.hints(names, module) {
			fmt.Fprintln(cmd.OutOrStdout(), line)
		}
	}
	return nil
}
