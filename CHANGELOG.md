# Changelog

## Unreleased

- **The `RealIP` example in the generated `main.go` calls `aemeath.PrivateNetworks()`**, a function since framework v0.16.0; uncommenting the old `PrivateNetworks...` gave code that did not compile. The project build test now compiles every `app.*` example that `main.go` shows in a comment.

- **`aemeath dev` and `aemeath ts` read `.env`.** A generated project ships `.env.example` saying "copy to .env", and `aemeath dev` restarted on every change to `.env`, but nothing read the file: the server never saw `PORT` or `DATABASE_URL` from it. They are now passed to the server; a variable already set in the shell wins, and a line that is not `KEY=value` stops the restart with its line number. The generated README no longer tells PostgreSQL and MySQL users to run `DATABASE_URL=... aemeath dev`, which does not work in PowerShell, and shows the PowerShell way to set `TEST_DATABASE_URL`.

- **`.dockerignore` leaves out `node_modules` and `web/dist`.** The Dockerfile of a fullstack project runs `npm install` in Linux and then copies `web/` over the result, so the `node_modules` installed on your machine (with Windows or macOS builds of esbuild and rollup) replaced the Linux ones and `npm run build` inside Docker could fail. Run `aemeath generate docker --force` in an existing project to get the new file (it also rewrites `Dockerfile` and `docker-compose.yml`).

- **SQLite data survives `docker compose up --build`.** The SQLite file was written inside the container, which compose replaces on every rebuild, so the data was lost. The generated `main.go` now opens `sqlite.Open(cmp.Or(os.Getenv("DATABASE_PATH"), "app.db"))`, and `aemeath generate docker` sets `DATABASE_PATH=/app/data/app.db` with a volume on `/app/data`, which the image lets the app user write. Outside Docker nothing changes: without `DATABASE_PATH` the file is still `app.db`. A project without a database gets no volume. In an existing SQLite project, change the `sqlite.Open("app.db")` line the same way, then run `aemeath generate docker --force`.

- **New projects no longer call `aemeath.CORS()`**, which allows every origin. No generated project needs CORS (the React app reaches the API through Vite's proxy, on the same origin), and on an internal network `*` lets any web page an employee opens read the API's answers. `main.go` shows, in a comment, how to allow one origin: `aemeath.CORS(aemeath.CORSConfig{AllowOrigins: []string{"https://app.example.com"}})`; the project build test compiles that line. Existing projects are not changed.

- **`aemeath new` writes the framework version into `go.mod`** (`require github.com/naufalraism/aemeath v0.17.0` for this release), the one the templates are tested with. Before, `go mod tidy` took whatever framework was newest, so a later breaking framework release could break projects this CLI creates.

- **`aemeath generate` checks the framework version of the project.** On a release older than the one above it writes nothing and prints `go get github.com/naufalraism/aemeath@v0.17.0`, because the code it writes uses the current API (`sqlite.Open` needs v0.12.0, for example) and would not compile. Projects that `replace` the framework with a local checkout, or do not require it yet, are not checked.

- **The list route of `generate api` is paged:** `?limit=` (1 to 100, default 50) and `?offset=`; other values answer 422. It returned every row, so one request could read a whole table. The generated tests check the paging, also against a real PostgreSQL or MySQL database. The generated TypeScript hook takes the paging as optional input, so `useGetApiTasks()` keeps working.

- **`aemeath ts` and `aemeath dev --ts-out` no longer report old files as new.** The program now writes into an empty folder of its own, and only files whose content changed are copied to the output folder. Before, a program that wrote nothing (because `main` returned before `app.Listen`, for example) passed whenever the files of an earlier run were still there.

## [0.6.0] - 2026-11-25

Needs framework **v0.12.0** or later: `aemeath new` fetches the latest framework, so a project created now gets it; in an existing project run `go get github.com/naufalraism/aemeath@v0.12.0` and change `db.Open("app.db")` to `sqlite.Open("app.db")` (import `github.com/naufalraism/aemeath/db/sqlite`) before `aemeath generate`.

- **SQLite projects open the database with `sqlite.Open("app.db")`** from `github.com/naufalraism/aemeath/db/sqlite`, instead of `db.Open`, which framework v0.12.0 removed so that PostgreSQL and MySQL projects no longer compile SQLite. Generated tests use it too. `aemeath generate api --db` still recognises a project that opens SQLite with the old `db.Open` and does not register a second database plugin.

- **The generated `main.go` says the API docs are public**, in the comment above `app.EnableDocs`, and shows how to serve them only to admins with `admin.EnableDocs` (framework v0.10.0 or later), with a link to the guide. Nothing changes in what the server does.

## [0.5.0] - 2026-11-24

Needs framework **v0.6.0** or later: `aemeath new` fetches the latest framework, so a project created now gets it; in an existing project run `go get github.com/naufalraism/aemeath@v0.6.0` before `aemeath generate`.

- **Generated handlers register routes with `aemeath.GET(group, …)`** instead of `GroupGET`, which the framework deprecated in v0.6.0. Needs framework v0.6.0 or later.

- **How to contribute:** `CONTRIBUTING.md`, `SECURITY.md` (report vulnerabilities privately), `CODE_OF_CONDUCT.md`, and templates for issues and pull requests. The new-issue page lists the private security report once (GitHub adds it).

## [0.4.4] - 2026-11-23

- **`aemeath dev` on Windows lets the server shut down** on reload and on exit, so its `OnShutdown` hooks run (closing the database, flushing traces). It used to kill the process because Windows has no SIGTERM; the server now runs in its own process group and receives Ctrl+Break, which Go programs see as `os.Interrupt`. Without a console it still falls back to killing the process.

- **README: the CLI's logo** (the framework's A with a terminal cursor `_` in place of the caret) and links to the website, the CLI documentation and the framework.

## [0.4.3] - 2026-11-22

- **`generate docker`: `# env_file: .env` was inside the `environment:` block.** Removing the `#`, as the comment says, made `env_file` an environment variable instead of the service option, so `.env` was never read and its secrets never reached the app, without any error (proven with `docker compose config` on Windows). The two comment lines are now at the service level.

## [0.4.2] - 2026-11-22

- **`aemeath new` ends with `💡 All commands: aemeath help`**, for every template. Nothing in the output or the README told a new user that `aemeath help` lists the commands.
- **README: "Help and tab completion"**: `aemeath help`, `aemeath <command> --help`, and how to turn on tab completion in PowerShell (including the `$PROFILE` line that keeps it).

## [0.4.1] - 2026-11-18

- **`GET /` of the api and db templates answers `/` only** (`"/{$}"`). It was registered as `"/"`, which net/http reads as "every path": `/does-not-exist`, `/admin/.env` and `/tasks/` answered `200 Hello from Aemeath!` instead of `404`.
- **The api and db templates have a `main_test.go`**, so `go test ./...` in a new api project tests something (it printed `[no test files]`): `GET /` answers the greeting and other paths `404`. The route moved into a `hello(app)` function that the test can register.
- **Handler tests of PostgreSQL and MySQL projects can run against that database:** with `TEST_DATABASE_URL` set they open it with `postgres.Open` or `mysql.Open` instead of the temporary SQLite file, so the code path the app uses in production is tested too (before, `go test` only ever used SQLite). The tests use the id of the row they created instead of assuming `1`, so they pass on a database that already has rows, and never delete rows they did not create. CI runs the generated projects against PostgreSQL 16 and MySQL 8.4 services.

## [0.4.0] - 2026-11-18

- `internal/cli` now tests the claims the README makes about the commands (every documented flag, the name forms, that nothing is written when a destination exists, that `main.go` is never edited twice or left half-edited, and what `dev` watches).
- Corrected in the README: `dev` cannot send `SIGTERM` on Windows, where it kills the server process at once.
- Generated jobs have a time limit of 5 minutes per attempt (`Timeout()`), so a job that hangs no longer keeps its worker forever. It takes effect with the Aemeath release that has `jobs.Timeouter`; older releases ignore the method.
- `main.go` of a new project shows where to add `aemeath.RealIP` (behind a proxy) and `app.SetShutdownDelay` (behind a load balancer).
- `docker-compose.yml` has a health check on `/readyz` for the app.
- New projects no longer ignore `web/src/api/client.ts`.
- A test builds, vets and tests a project from every template against a checkout of the framework (`AEMEATH_FRAMEWORK_DIR`), and CI runs it.

## [0.3.0] - 2026-11-12

### Added (databases)

- **`aemeath new --database sqlite|postgres|mysql`** for the `db` and `fullstack` templates. PostgreSQL and MySQL register `postgres.Open(os.Getenv("DATABASE_URL"))` / `mysql.Open(...)`, add a `DATABASE_URL` example to `.env.example`, a health-checked database service and volume to `docker-compose.yml`, and a matching README section. The generated handler tests still use a temporary SQLite file, so they need no server.
- `generate api --db` keeps using the database `main.go` already registers (it never adds a second plugin) and accepts `--database` for a project that has none yet; `generate docker` detects it too.
- **Requires a framework release that contains `aemeath/db/postgres` and `aemeath/db/mysql`** (publish the framework first).

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
