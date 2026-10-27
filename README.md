# Aemeath CLI 🚀
[![Go](https://github.com/naufalraism/aemeath-cli/actions/workflows/go.yml/badge.svg)](https://github.com/naufalraism/aemeath-cli/actions/workflows/go.yml)

The official command line interface for the [Aemeath Go framework](https://github.com/naufalraism/aemeath).

## 📦 Installation

```bash
go install github.com/naufalraism/aemeath-cli@latest
```

The binary is named after the module, so rename it or alias it to `aemeath`. Make sure `$(go env GOPATH)/bin` is in your `PATH`. The CLI requires the Go toolchain to be installed (it runs `go build` and `go mod tidy`).

## 🛠️ Commands

### `aemeath new <project>`

Creates a project with a ready-to-run `main.go`, `go.mod`, `.gitignore`, `.env.example` and `README.md`, then runs `go mod tidy`.

```bash
aemeath new my-app
aemeath new my-app --module github.com/me/my-app   # custom module path
aemeath new my-app --no-tidy                        # skip 'go mod tidy'
aemeath new my-app --replace ../aemeath             # use a local framework checkout
```

### `aemeath ts`

Generates the TypeScript client (`client.ts`), the Zod schemas (`schemas.ts`) and the React Query hooks (`hooks.ts`) from your Go routes.

```bash
aemeath ts                      # writes to ./api-client
aemeath ts --out web/src/api
```

The CLI cannot read your Go types from outside, so it builds your program and runs it with `AEMEATH_TS_OUT=<dir>`. `app.Listen` notices the variable, writes the files and exits without starting the server. Nothing to change in `main.go`, but any code that runs before `app.Listen` runs too. Files whose content did not change are left alone, so a bundler watching the directory does not reload for nothing. `schemas.ts` needs the `zod` package and `hooks.ts` needs `@tanstack/react-query` v5.

If it reports that the program "did not write" the files, check that `main` calls `app.Listen` (or `app.Serve`) and that your Aemeath version is recent enough. If it warns about `log.Fatal`, write `if err := app.Listen(addr); err != nil { log.Fatal(err) }`: `log.Fatal(app.Listen(addr))` exits with status 1 even after a clean shutdown.

### `aemeath dev`

Builds and runs your server, and rebuilds when `.go`, `go.mod`, `go.sum` or `.env` files change.

```bash
aemeath dev
aemeath dev --pkg ./cmd/server --delay 500ms
aemeath dev --ts-out web/src/api    # also rewrite the TypeScript files after every successful build
```

- It runs the compiled binary directly and stops it with `SIGTERM` (then `SIGKILL` after 5 s), so the port is always free again for the next start.
- If a build fails, the previous version keeps running until you fix the error.
- Changes are debounced, directories created later are watched, and `_test.go` files, hidden folders, `vendor`, `node_modules`, `tmp` and `uploads` are ignored.

### `aemeath generate <kind> <Name>`

`<Name>` may be written as `Product`, `product-item`, `product_item` or `ProductItem`. Existing files are never overwritten unless you pass `--force`.

| Command | Creates | Notes |
| :--- | :--- | :--- |
| `generate api Product` | `handlers/product.go` | Full CRUD (list, create, get, update, delete) on `/products`, backed by an in-memory store you replace with your database |
| `generate test Product` | `handlers/product_test.go` | Tests for the generated handlers (needs `go.mod`) |
| `generate model Product` | `models/product.go` | GORM model |
| `generate middleware Auth` | `middleware/auth.go` | `AuthMiddleware()` skeleton |
| `generate job SendEmail` | `jobs/sendemail.go` | `Handle(ctx) error` skeleton (there is no job runner yet) |
| `generate docker` | `Dockerfile`, `docker-compose.yml`, `.dockerignore` | Multi-stage build, non-root user |

After `generate api`, register the handlers in `main.go` as the command prints:

```go
import "my-app/handlers"

handlers.ProductHandler(app)
```

## 🧪 Development

```bash
go test -race ./...
go test -short ./...   # skips the hot-reload test, which builds Go programs
```

## 📝 License

MIT License.
