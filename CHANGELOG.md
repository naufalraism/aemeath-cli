# Changelog

## [0.2.0] - 2026-11-03

### Added (templates, wiring, jobs, releases)

- **`aemeath new --template api|db|fullstack`.** `db` adds SQLite and a `Task` CRUD resource with tests that run against a real database; `fullstack` adds a Vite + React + TypeScript app in `web/` that uses the generated client, Zod schemas and hooks (generated for you), with a dev proxy and Go serving `web/dist` in production.
- **`generate` now registers what it creates in `main.go`**, at the `// aemeath:plugins` and `// aemeath:handlers` comments of a new project (or before `app.Listen`). It only inserts lines, only into a `main.go` that parses, and leaves the file untouched and prints what to add when it cannot do it safely. `--no-wire` skips it.
- `generate api` also writes the test file (`--no-test` skips it) and accepts `--db` (GORM + SQLite, a model, the database plugin) and `--prefix`.
- **`generate job`** now works: jobs implement `Run(ctx)` and the command registers the framework's retrying job runner (`aemeath/jobs`).
- `generate` checks every destination first and writes nothing if one already exists, instead of stopping half way.
- GoReleaser configuration and a release workflow that publish `aemeath` binaries for Windows, macOS and Linux (amd64 and arm64) when a tag is pushed.
- `aemeath --version` reports the module version when installed with `go install ...@v0.1.0`.
- The Dockerfile builds `web/` for fullstack projects and lets the non-root user write the SQLite file.

### Changed

- **The command moved to `cmd/aemeath`**, so `go install github.com/naufalraism/aemeath-cli/cmd/aemeath@latest` installs a command named `aemeath` (it used to be `aemeath-cli`). The code lives in `internal/cli`; the old `go install github.com/naufalraism/aemeath-cli@latest` no longer works.
- Generated tests and handlers use a route prefix when asked, and a database test uses a temporary SQLite file that it closes (Windows cannot delete an open database).

### Fixed


- **`aemeath dev` left the server running** after a reload: it killed the `go run` process but not the server it had started, so the next restart failed with `address already in use` (the very problem the README promised to solve). It now builds a binary and runs it directly, and stops it with `SIGTERM`.
- `aemeath dev` could crash with a nil pointer when a directory was unreadable, missed files saved by editors that write through a rename, restarted several times for one save, never noticed directories created later (for example `handlers/` after `generate api`), and always ran only `main.go` (`go run main.go`), which breaks as soon as the package has a second file.
- **Generated projects did not build for anyone but the author:** `go.mod` contained `replace ... => "../../Aemeath Go"`. It is gone; `go mod tidy` picks the real version (use `--replace` for a local checkout).
- **`generate` silently overwrote existing files**, including hand-written handlers. It now refuses unless `--force` is passed.
- **Generated tests failed out of the box** (they requested `/product` while the handler used `/products`, and never registered the handler). They now exercise the generated CRUD handlers and pass.
- `generate api` produced only two routes although the README promised CRUD; it now generates list, create, get, update and delete.
- Names such as `product-item` produced invalid Go code and names such as `../../x` wrote files outside the project. Names are now normalized (`ProductItem`) and anything unsafe is rejected.
- Every error exited with status 0, so scripts and CI could not detect failures. Errors now exit with status 1.
- A template rendering error left a truncated file behind. Output is rendered and gofmt-ed in memory before anything is written.
- The Dockerfile ran as root, used `alpine:latest` and a Go version unrelated to your toolchain; the compose file required a `.env` that did not exist.

### Added (TypeScript)

- **`aemeath ts [--out dir]`** writes the typed client, Zod schemas and React Query hooks, and **`aemeath dev --ts-out dir`** rewrites them after every successful build (a failed build keeps the previous files). It works by running your program with `AEMEATH_TS_OUT`, which Aemeath's `app.Listen` honors, so `main.go` needs no change.
- A project whose `main` is `log.Fatal(app.Listen(...))` exits with status 1 even when it wrote the files; the CLI accepts that and warns once.
- The generated `main.go` checks the error returned by `app.Listen`, so a clean shutdown exits with status 0.

### Added

- `aemeath new --module`, `--replace`, `--no-tidy`; `.gitignore`, `.env.example` and `README.md` in new projects; `PORT` support.
- `aemeath dev --pkg`, `--delay`.
- `generate --force`, `aemeath --version`, and a `.dockerignore`.
- Tests (names, templates, safe writing, hot reload) and CI on Linux, macOS and Windows.

### Changed

- Everything user-facing (messages, templates, README) is now in English.
- Build failures during `aemeath dev` keep the previous version running.
