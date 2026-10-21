# Aemeath CLI 🚀

The official Command Line Interface for the [Aemeath Go Framework](https://github.com/naufalraism/aemeath).

## 📦 Installation

Since the CLI is still in early development, you can install it directly via `go install`:

```bash
go install github.com/naufalraism/aemeath-cli@latest
```
*(Note: Make sure your `$GOPATH/bin` is in your system's PATH).*

## 🛠️ Commands

### 1. Create a new project
```bash
aemeath new my-app
```
Scaffolds a new Aemeath project with a ready-to-run `main.go` and `go.mod`.

### 2. Generate API Handlers
```bash
aemeath generate api Product
```
Automatically creates a `handlers/product.go` file containing the Struct, In/Out models, and CRUD routes for `Product`.

### 3. Start Dev Server with Hot Reload
```bash
aemeath dev
```
Starts your Aemeath server. Any changes you make to `.go` files will automatically restart the server without hitting `bind: address already in use` errors.

## 📝 License

MIT License.