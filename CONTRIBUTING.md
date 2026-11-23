# Contributing to the Aemeath CLI

Thanks for helping! This guide keeps changes easy to review.

The CLI creates and runs projects for the [Aemeath framework](https://github.com/naufalraism/aemeath). A change to the framework itself (routing, validation, middleware, plugins) belongs in that repository.

## Setup

```bash
git clone https://github.com/naufalraism/aemeath-cli
cd aemeath-cli
go test -race ./...
go run ./cmd/aemeath --help
```

You need the Go version in `go.mod`. `go test -short ./...` skips the slow tests that build Go programs.

## Ground rules

- Every behavior change comes with a test. A bug fix comes with a test that fails without the fix.
- Run `gofmt -l .` (it must print nothing), `go vet ./...` and `go test -race ./...`. CI also runs golangci-lint.
- Add an entry under **Unreleased** in `CHANGELOG.md`, and mark breaking changes clearly.
- Keep comments, messages and docs in English.
- The CLI runs on Windows, macOS and Linux. Anything that touches processes, signals or paths needs to work on all three (see `proc_windows.go` and `proc_other.go`).

## Where things live

| Path | What |
| :--- | :--- |
| `cmd/aemeath` | the `aemeath` command |
| `internal/cli/new.go` | `aemeath new` and the project it creates |
| `internal/cli/scaffold.go` | renders the templates (Go output goes through gofmt) |
| `internal/cli/generate.go`, `wire.go`, `register.go` | `aemeath generate` and how new code and plugins are wired into `main.go` |
| `internal/cli/dev.go`, `proc_*.go` | `aemeath dev`: watching, rebuilding and restarting the server |
| `internal/cli/ts.go` | `aemeath ts`: writing the TypeScript client |
| `internal/cli/templates` | the files the CLI writes into a project |

## Changing a template

Code the CLI generates ends up in other people's projects, so it must build, pass `go vet` and be secure by default. The other tests only look at the text of the generated files. `TestGeneratedProjectsBuild` creates a project from every template and database, runs the generators in it, and compiles, vets and tests the result against a checkout of the framework. CI runs it on every pull request. Locally it is skipped unless `AEMEATH_FRAMEWORK_DIR` is the full path of a framework checkout:

```bash
git clone https://github.com/naufalraism/aemeath ../aemeath
AEMEATH_FRAMEWORK_DIR=$(cd ../aemeath && pwd) go test ./internal/cli -run TestGeneratedProjectsBuild -v
```

On Windows PowerShell: `$env:AEMEATH_FRAMEWORK_DIR = (Resolve-Path ..\aemeath).Path` before `go test`.

## Pull requests

1. Open an issue first for anything larger than a bug fix, so we can agree on the design.
2. Keep a pull request to one change.
3. Fill in the template, including the checklist.

## Security

Do not report vulnerabilities in public issues; see [SECURITY.md](SECURITY.md).

## Code of conduct

Everyone taking part is expected to follow the [code of conduct](CODE_OF_CONDUCT.md).
