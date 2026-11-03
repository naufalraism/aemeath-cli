# Aemeath CLI 🚀

The official command line interface for the [Aemeath Go framework](https://github.com/naufalraism/aemeath): create projects, generate code, and run a hot-reload dev server.

## 📦 Installation

```bash
go install github.com/naufalraism/aemeath-cli/cmd/aemeath@latest
```

This installs a command named **`aemeath`** into `$(go env GOPATH)/bin` (on Windows, `%USERPROFILE%\go\bin`). Make sure that folder is in your `PATH`, then check:

```bash
aemeath --version
```

No Go installed? Download a binary for Windows, macOS or Linux (amd64 and arm64) from the [Releases page](https://github.com/naufalraism/aemeath-cli/releases). The CLI still needs the Go toolchain to build your project, and Node.js 20.19+ or 22.12+ for the `fullstack` template.

## 🛠️ Commands

### `aemeath new <project>`

Creates a project, registers its first resource and runs `go mod tidy`.

```bash
aemeath new my-app                       # a minimal API (default)
aemeath new my-app --template db         # + SQLite and a Task CRUD resource
aemeath new my-app --template fullstack  # + a React + TypeScript app in web/
aemeath new my-app --module github.com/me/my-app   # custom module path
aemeath new my-app --no-tidy             # skip 'go mod tidy'
aemeath new my-app --replace ../aemeath  # use a local framework checkout
```

| Template | What you get |
| :--- | :--- |
| `api` | `main.go` with health checks, docs and a hello route; no dependencies besides Aemeath |
| `db` | `api` + the SQLite plugin, a `Task` model, CRUD handlers under `/tasks` and tests that run against a real database |
| `fullstack` | `db` with the routes under `/api`, plus `web/`: Vite + React + TypeScript + TanStack Query. The typed client, Zod schemas and hooks are generated into `web/src/api`, so it works right away |

For `fullstack`, run two terminals:

```bash
cd my-app && aemeath dev --ts-out web/src/api     # the API, and the TypeScript files kept in sync
cd my-app/web && npm install && npm run dev       # http://localhost:5173, proxies /api to Go
```

`npm run build` fills `web/dist`, which the Go server serves at `/`.

### `aemeath generate <kind> <Name>`

`<Name>` may be written as `Product`, `product-item`, `product_item` or `ProductItem`.

| Command | Creates | Notes |
| :--- | :--- | :--- |
| `generate api Product` | `handlers/product.go`, `handlers/product_test.go` | Full CRUD (list, create, get, update, delete) on `/products`. Data is kept in memory, so it works immediately |
| `generate api Product --db` | the above backed by GORM, plus `models/product.go` | Registers the database plugin too. Each handler creates its own table at startup |
| `generate api Product --prefix /api` | routes under `/api/products` | |
| `generate job SendEmail` | `jobs/sendemail.go` | A job with `Run(ctx)`; registers the retrying job runner |
| `generate test Product` | `handlers/product_test.go` | Add `--db` and `--prefix` to match the handlers |
| `generate model Product` | `models/product.go` | GORM model |
| `generate middleware Auth` | `middleware/auth.go` | You add it to `app.Use` yourself: the order of middlewares matters |
| `generate docker` | `Dockerfile`, `docker-compose.yml`, `.dockerignore` | Multi-stage, non-root; builds `web/` too when it exists |

Flags: `--force` overwrites existing files, `--no-wire` leaves `main.go` alone, `--no-test` skips the test file.

**How `main.go` is edited.** `generate` registers what it creates right after the comments `// aemeath:plugins` and `// aemeath:handlers` that a new project has (or before `app.Listen` if you removed them). It only inserts lines, only into a `main.go` that parses, and runs the result through `gofmt`. If it cannot do that safely (no `func main`, no `aemeath.New()`, nowhere to put the lines) it leaves the file exactly as it was and prints the lines to add. Running a command twice never registers anything twice, and **nothing is written if any of the files already exists**.

### `aemeath ts`

```bash
aemeath ts --out web/src/api
```

Writes `client.ts`, `schemas.ts` (Zod) and `hooks.ts` (React Query) from your Go routes. The CLI builds your program and runs it with `AEMEATH_TS_OUT=<dir>`; `app.Listen` writes the files and exits. Files that did not change are left alone, so a bundler does not reload for nothing.

If it reports that the program "did not write" the files, check that `main` calls `app.Listen` and that your Aemeath version is recent. If it warns about `log.Fatal`, write `if err := app.Listen(addr); err != nil { log.Fatal(err) }`: `log.Fatal(app.Listen(addr))` exits with status 1 even after a clean shutdown.

### `aemeath dev`

```bash
aemeath dev
aemeath dev --pkg ./cmd/server --delay 500ms
aemeath dev --ts-out web/src/api    # also rewrite the TypeScript files after every successful build
```

- It runs the compiled binary directly and stops it with `SIGTERM` (then `SIGKILL` after 5 s), so the port is always free again.
- If a build fails, the previous version keeps running until you fix the error.
- Changes are debounced, new directories are watched, and `_test.go` files, hidden folders, `vendor`, `node_modules`, `tmp` and `uploads` are ignored.

## 🧪 Development

```bash
go test -race ./...
go test -short ./...   # skips the tests that build Go programs
```

Releases are built by [GoReleaser](https://goreleaser.com) when a `v*` tag is pushed (`.goreleaser.yaml`, `.github/workflows/release.yml`).

## 📝 License

MIT License.
