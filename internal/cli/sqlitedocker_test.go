package cli

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// In Docker, a file written inside the container is lost when the container
// is recreated, which "docker compose up --build" does after every change.
// The SQLite file must live in a volume: compose names its path in
// DATABASE_PATH, mounts a volume on that folder, the image lets the app user
// write there, and main.go opens DATABASE_PATH (app.db when it is not set).
func TestSQLiteDataSurvivesARebuildInDocker(t *testing.T) {
	compose := render2(t, "", "docker-compose.yml.tmpl", dbSQLite)
	m := regexp.MustCompile(`(?m)^\s+DATABASE_PATH:\s*(\S+)\s*$`).FindStringSubmatch(compose)
	if m == nil {
		t.Fatalf("docker-compose.yml does not set DATABASE_PATH for the SQLite file:\n%s", compose)
	}
	dir := path.Dir(m[1])
	if !regexp.MustCompile(`(?m)^\s+- \w+:` + regexp.QuoteMeta(dir) + `\s*$`).MatchString(compose) {
		t.Errorf("no named volume is mounted on %s, where the SQLite file %s is:\n%s", dir, m[1], compose)
	}
	if !regexp.MustCompile(`(?m)^volumes:\s*$`).MatchString(compose) {
		t.Errorf("docker-compose.yml declares no volumes:\n%s", compose)
	}

	dockerfile, err := render("Dockerfile.tmpl", projectData{GoVersion: "1.27", Database: dbSQLite})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dockerfile), "mkdir -p "+dir) {
		t.Errorf("the Dockerfile does not create %s for the app user, so the volume would belong to root:\n%s", dir, dockerfile)
	}

	main := dbPluginWire(dbSQLite).Stmt("app")
	if !strings.Contains(main, `os.Getenv("DATABASE_PATH")`) || !strings.Contains(main, `"app.db"`) {
		t.Errorf("main.go does not open DATABASE_PATH (default app.db):\n%s", main)
	}
}

// Postgres and MySQL keep their data in the db service's volume; their
// compose file must not grow a SQLite variable.
func TestOnlySQLiteProjectsGetDatabasePath(t *testing.T) {
	for _, kind := range []string{dbPostgres, dbMySQL} {
		if c := render2(t, "", "docker-compose.yml.tmpl", kind); strings.Contains(c, "DATABASE_PATH") {
			t.Errorf("%s compose sets DATABASE_PATH:\n%s", kind, c)
		}
	}
}

// An API without a database gets neither the variable nor a volume.
func TestDockerForAProjectWithoutADatabase(t *testing.T) {
	c := render2(t, "", "docker-compose.yml.tmpl", "")
	if strings.Contains(c, "DATABASE_") || strings.Contains(c, "volumes:") {
		t.Errorf("compose for a project without a database:\n%s", c)
	}
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	if got := projectDatabase(dir); got != "" {
		t.Errorf("projectDatabase without a plugin = %q, want \"\"", got)
	}
	writeTestFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() { postgres.Open(os.Getenv(\"DATABASE_URL\")) }\n")
	if got := projectDatabase(dir); got != dbPostgres {
		t.Errorf("projectDatabase with postgres.Open = %q", got)
	}
}
