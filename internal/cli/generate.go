package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// genData is what the generator templates receive.
type genData struct {
	Names
	Module string
	Prefix string
	DB     bool
}

var prefixRe = regexp.MustCompile(`^(/[A-Za-z0-9._~-]+)+$`)

// normalizePrefix turns "api", "/api" or "/api/" into "/api"; "" stays empty.
func normalizePrefix(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" || p == "/" {
		return "", nil
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = strings.TrimRight(p, "/")
	if !prefixRe.MatchString(p) {
		return "", fmt.Errorf("invalid --prefix %q: use path segments such as /api or /api/v1", p)
	}
	for _, seg := range strings.Split(p[1:], "/") {
		if seg == "." || seg == ".." {
			return "", fmt.Errorf("invalid --prefix %q: '.' and '..' are not allowed", p)
		}
	}
	return p, nil
}

type resourceOpts struct {
	Root   string // the project directory
	Name   string
	Prefix string
	DB     bool // handlers backed by GORM instead of an in-memory store
	Force  bool
	NoWire bool // do not edit main.go
	NoTest bool
}

type plannedFile struct {
	tmpl         string
	dest         string
	content      []byte
	skipIfExists bool // an existing file is used as it is (the shared model)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// generateResource creates the handlers (and the model and tests) of a CRUD
// resource and registers them in main.go. Nothing is written unless every file
// can be: a name collision never leaves half of a resource behind.
func generateResource(out io.Writer, o resourceOpts) error {
	names, err := parseName(o.Name)
	if err != nil {
		return err
	}
	module := modulePath(o.Root)
	if module == "" {
		return fmt.Errorf("go.mod not found: run this command from the root of your project")
	}
	prefix, err := normalizePrefix(o.Prefix)
	if err != nil {
		return err
	}
	data := genData{Names: names, Module: module, Prefix: prefix, DB: o.DB}

	handlerTmpl, testTmpl := "handler.go.tmpl", "test.go.tmpl"
	if o.DB {
		handlerTmpl, testTmpl = "handler_db.go.tmpl", "test_db.go.tmpl"
	}
	plan := []plannedFile{{tmpl: handlerTmpl, dest: filepath.Join("handlers", names.File+".go")}}
	if !o.NoTest {
		plan = append(plan, plannedFile{tmpl: testTmpl, dest: filepath.Join("handlers", names.File+"_test.go")})
	}
	if o.DB {
		plan = append(plan, plannedFile{tmpl: "model.go.tmpl", dest: filepath.Join("models", names.File+".go"), skipIfExists: true})
	}

	// Render everything and check for collisions before writing anything.
	var taken []string
	for i := range plan {
		content, err := render(plan[i].tmpl, data)
		if err != nil {
			return err
		}
		plan[i].content = content
		if !o.Force && !plan[i].skipIfExists && fileExists(filepath.Join(o.Root, plan[i].dest)) {
			taken = append(taken, plan[i].dest)
		}
	}
	if len(taken) > 0 {
		return fmt.Errorf("nothing was written because %s already exist(s) (use --force to overwrite)", strings.Join(taken, ", "))
	}

	for _, f := range plan {
		path := filepath.Join(o.Root, f.dest)
		if f.skipIfExists && fileExists(path) {
			outf(out, "ℹ️  %s already exists, so it is used as it is (the handlers expect the fields ID and Name)\n", f.dest)
			continue
		}
		if err := writeFile(path, f.content, o.Force); err != nil {
			return err
		}
		outf(out, "✨ %s created\n", f.dest)
	}

	if o.NoWire {
		printManualHandlerWiring(out, module, names, o.DB)
		return nil
	}
	if o.DB {
		wireAndReport(out, o.Root, dbPluginWire(), "the database plugin", []string{
			`if database, err := db.Open("app.db"); err != nil { log.Fatal(err) } else if err := app.RegisterPlugin(database); err != nil { log.Fatal(err) }`,
		}, "import \"github.com/naufalraism/aemeath/db\"")
	}
	wireAndReport(out, o.Root, handlerWire(module, names), "handlers."+names.Pascal+"Handler(app)", []string{
		"handlers." + names.Pascal + "Handler(app)",
	}, fmt.Sprintf("import %q", module+"/handlers"))
	return nil
}

func printManualHandlerWiring(out io.Writer, module string, n Names, db bool) {
	outln(out, "👉 Register it in main.go:")
	outf(out, "     import %q\n", module+"/handlers")
	if db {
		outln(out, "     // and the database plugin, before the handlers:")
		outln(out, `     //   database, err := db.Open("app.db") ... app.RegisterPlugin(database)`)
	}
	outf(out, "     handlers.%sHandler(app)\n", n.Pascal)
}

// wireAndReport edits main.go and tells the user what happened, or what to do by hand.
func wireAndReport(out io.Writer, root string, spec wireSpec, what string, manual []string, importLine string) {
	res, err := wireMain(root, spec)
	switch {
	case err != nil:
		outf(out, "⚠️  Could not edit main.go (%v). Add this yourself:\n", err)
	case res.Outcome == wired:
		outf(out, "✨ main.go: registered %s\n", what)
		return
	case res.Outcome == alreadyWired:
		outf(out, "ℹ️  main.go already registers %s\n", what)
		return
	default:
		outf(out, "⚠️  I left main.go alone (%s). Add this yourself:\n", res.Reason)
	}
	outf(out, "     %s\n", importLine)
	for _, line := range manual {
		outf(out, "     %s\n", line)
	}
}

type simpleGen struct {
	use, short string
	tmpl       string
	dir        string
	suffix     string
}

// generateJob creates a background job and registers the runner in main.go.
func generateJob(out io.Writer, root, rawName string, force, noWire bool) error {
	names, err := parseName(rawName)
	if err != nil {
		return err
	}
	module := modulePath(root)
	dest := filepath.Join("jobs", names.File+".go")
	if err := renderTo("job.go.tmpl", filepath.Join(root, dest), genData{Names: names, Module: module}, force); err != nil {
		return err
	}
	outf(out, "✨ %s created\n", dest)

	manual := []string{`if err := app.RegisterPlugin(aejobs.New(aejobs.WithWorkers(4))); err != nil { log.Fatal(err) }`}
	imp := `import aejobs "github.com/naufalraism/aemeath/jobs"`
	if noWire {
		outf(out, "👉 Register the job runner in main.go:\n     %s\n     %s\n", imp, manual[0])
	} else {
		wireAndReport(out, root, jobsRunnerWire(), "the job runner (aejobs.New)", manual, imp)
	}
	outf(out, "👉 Enqueue it from a handler: aejobs.From(c).Enqueue(&jobs.%sJob{})\n", names.Pascal)
	return nil
}

func newGenerateCmd() *cobra.Command {
	var force bool

	gen := &cobra.Command{
		Use:   "generate",
		Short: "Generate code (api, job, model, middleware, test, docker)",
	}
	gen.PersistentFlags().BoolVarP(&force, "force", "f", false, "overwrite files that already exist")

	var apiOpts resourceOpts
	api := &cobra.Command{
		Use:   "api [Name]",
		Short: "Generate CRUD handlers (and tests) for a resource and register them in main.go",
		Long: `Generates handlers/<name>.go with list, create, get, update and delete routes, a test
file for them, and registers the handlers in main.go.

By default the handlers keep their data in memory, so they work immediately. With
--db they use GORM and SQLite instead: a model is created in models/, the database
plugin is registered in main.go, and the table is created when the server starts.

main.go is only edited at the "// aemeath:plugins" and "// aemeath:handlers" comments,
or before app.Listen, and only when it parses. If it cannot be edited safely it is
left alone and the lines to add are printed. Use --no-wire to skip the edit.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			apiOpts.Root, apiOpts.Name, apiOpts.Force = ".", args[0], force
			return generateResource(cmd.OutOrStdout(), apiOpts)
		},
	}
	api.Flags().BoolVar(&apiOpts.DB, "db", false, "back the handlers with GORM and SQLite instead of an in-memory store")
	api.Flags().StringVar(&apiOpts.Prefix, "prefix", "", "URL prefix of the routes, for example /api")
	api.Flags().BoolVar(&apiOpts.NoWire, "no-wire", false, "do not edit main.go")
	api.Flags().BoolVar(&apiOpts.NoTest, "no-test", false, "do not generate the test file")
	gen.AddCommand(api)

	var testDB bool
	var testPrefix string
	testCmd := &cobra.Command{
		Use:   "test [Name]",
		Short: "Generate tests for the handlers of a resource (use the same --db and --prefix as for api)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return generateTestOnly(cmd.OutOrStdout(), ".", args[0], testPrefix, testDB, force)
		},
	}
	testCmd.Flags().BoolVar(&testDB, "db", false, "the handlers use the database")
	testCmd.Flags().StringVar(&testPrefix, "prefix", "", "URL prefix of the routes")
	gen.AddCommand(testCmd)

	var noWireJob bool
	job := &cobra.Command{
		Use:   "job [Name]",
		Short: "Generate a background job and register the job runner in main.go",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return generateJob(cmd.OutOrStdout(), ".", args[0], force, noWireJob)
		},
	}
	job.Flags().BoolVar(&noWireJob, "no-wire", false, "do not edit main.go")
	gen.AddCommand(job)

	for _, s := range []simpleGen{
		{use: "model [Name]", short: "Generate a GORM model", tmpl: "model.go.tmpl", dir: "models", suffix: ".go"},
		{use: "middleware [Name]", short: "Generate a custom middleware", tmpl: "middleware.go.tmpl", dir: "middleware", suffix: ".go"},
	} {
		s := s
		gen.AddCommand(&cobra.Command{
			Use:   s.use,
			Short: s.short,
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.SilenceUsage = true
				return generateSimple(cmd.OutOrStdout(), ".", s, args[0], force)
			},
		})
	}

	gen.AddCommand(&cobra.Command{
		Use:   "docker",
		Short: "Generate Dockerfile, docker-compose.yml and .dockerignore",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			data := projectData{GoVersion: goVersion(), Web: fileExists(filepath.Join("web", "package.json"))}
			for _, f := range []struct{ tmpl, dest string }{
				{"Dockerfile.tmpl", "Dockerfile"},
				{"docker-compose.yml.tmpl", "docker-compose.yml"},
				{"dockerignore.tmpl", ".dockerignore"},
			} {
				if err := renderTo(f.tmpl, f.dest, data, force); err != nil {
					return err
				}
				outf(cmd.OutOrStdout(), "✨ %s created\n", f.dest)
			}
			outln(cmd.OutOrStdout(), "👉 Run: docker compose up --build")
			return nil
		},
	})
	return gen
}

func generateTestOnly(out io.Writer, root, rawName, rawPrefix string, db, force bool) error {
	names, err := parseName(rawName)
	if err != nil {
		return err
	}
	module := modulePath(root)
	if module == "" {
		return fmt.Errorf("go.mod not found: run this command from the root of your project")
	}
	prefix, err := normalizePrefix(rawPrefix)
	if err != nil {
		return err
	}
	tmpl := "test.go.tmpl"
	if db {
		tmpl = "test_db.go.tmpl"
	}
	dest := filepath.Join("handlers", names.File+"_test.go")
	if err := renderTo(tmpl, filepath.Join(root, dest), genData{Names: names, Module: module, Prefix: prefix, DB: db}, force); err != nil {
		return err
	}
	outf(out, "✨ %s created\n", dest)
	outf(out, "👉 It needs handlers/%s.go (aemeath generate api %s). Run: go test ./...\n", names.File, names.Pascal)
	return nil
}

func generateSimple(out io.Writer, root string, s simpleGen, rawName string, force bool) error {
	names, err := parseName(rawName)
	if err != nil {
		return err
	}
	module := modulePath(root)
	dest := filepath.Join(s.dir, names.File+s.suffix)
	if err := renderTo(s.tmpl, filepath.Join(root, dest), genData{Names: names, Module: module}, force); err != nil {
		return err
	}
	outf(out, "✨ %s created\n", dest)
	switch s.dir {
	case "models":
		outln(out, "👉 Run 'go mod tidy' to add gorm.io/gorm. aemeath generate api "+names.Pascal+" --db uses this model.")
	case "middleware":
		outln(out, "👉 Use it in main.go (the order of middlewares matters, so this is not done for you):")
		outf(out, "     import %q\n     app.Use(middleware.%sMiddleware())\n", moduleOr(module)+"/middleware", names.Pascal)
	}
	return nil
}

func moduleOr(module string) string {
	if module == "" {
		return "<your-module>"
	}
	return module
}
