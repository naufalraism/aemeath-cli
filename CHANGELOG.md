# Changelog

## [0.1.1] - 2026-10-29

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
