package cli

import (
	"os"
	"path/filepath"
	"regexp"
)

// handlerWire registers handlers.<Name>Handler(app) in main().
func handlerWire(module string, n Names) wireSpec {
	return wireSpec{
		Imports: []importSpec{{Path: module + "/handlers"}},
		Stmt:    func(app string) string { return "handlers." + n.Pascal + "Handler(" + app + ")" },
		Already: regexp.MustCompile(`handlers\.` + n.Pascal + `Handler\(`),
		After:   regexp.MustCompile(`^handlers\.\w+Handler\(`),
		Marker:  markerHandlers,
	}
}

// Database kinds a project can use.
const (
	dbSQLite   = "sqlite"
	dbPostgres = "postgres"
	dbMySQL    = "mysql"
)

func validDatabase(k string) bool { return k == dbSQLite || k == dbPostgres || k == dbMySQL }

// databaseNames lists the choices for help and error messages.
const databaseNames = "sqlite, postgres, mysql"

var (
	postgresOpenRe = regexp.MustCompile(`\bpostgres\.Open\(`)
	mysqlOpenRe    = regexp.MustCompile(`\bmysql\.Open\(`)
)

// detectDatabase tells which database an existing project's main.go already
// registers, so "generate api --db" keeps using it. It defaults to SQLite.
func detectDatabase(root string) string {
	src, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		return dbSQLite
	}
	switch {
	case postgresOpenRe.Match(src):
		return dbPostgres
	case mysqlOpenRe.Match(src):
		return dbMySQL
	}
	return dbSQLite
}

// dbPluginWire registers the database plugin in main(). Each generated
// handler migrates its own model, so the plugin is opened without models.
// PostgreSQL and MySQL read their connection string from DATABASE_URL.
func dbPluginWire(kind string) wireSpec {
	open, imp := `db.Open("app.db")`, aemeathModule+"/db"
	switch kind {
	case dbPostgres:
		open, imp = `postgres.Open(os.Getenv("DATABASE_URL"))`, aemeathModule+"/db/postgres"
	case dbMySQL:
		open, imp = `mysql.Open(os.Getenv("DATABASE_URL"))`, aemeathModule+"/db/mysql"
	}
	imports := []importSpec{{Path: imp}, {Path: "log"}}
	if kind != dbSQLite {
		imports = append(imports, importSpec{Path: "os"})
	}
	return wireSpec{
		Imports: imports,
		Stmt: func(app string) string {
			return `if database, err := ` + open + `; err != nil {
	log.Fatal(err)
} else if err := ` + app + `.RegisterPlugin(database); err != nil {
	log.Fatal(err)
}`
		},
		// Any database plugin already registered counts: never add a second one.
		Already: regexp.MustCompile(`\bdb\.Open\(|\bpostgres\.Open\(|\bmysql\.Open\(|\bdb\.New\(`),
		After:   regexp.MustCompile(`RegisterPlugin\(`),
		Marker:  markerPlugins,
	}
}

// jobsRunnerWire registers the background job runner in main(). The framework's
// package is imported as aejobs because a project's own package is usually named jobs.
func jobsRunnerWire() wireSpec {
	return wireSpec{
		Imports: []importSpec{{Alias: "aejobs", Path: aemeathModule + "/jobs"}, {Path: "log"}},
		Stmt: func(app string) string {
			return `if err := ` + app + `.RegisterPlugin(aejobs.New(aejobs.WithWorkers(4))); err != nil {
	log.Fatal(err)
}`
		},
		Already: regexp.MustCompile(`\baejobs\.New\(|aemeath/jobs"`),
		After:   regexp.MustCompile(`RegisterPlugin\(`),
		Marker:  markerPlugins,
	}
}
