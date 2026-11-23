## What does this change?

## Why?

## Checklist

- [ ] `gofmt -l .` prints nothing, `go vet ./...` and `go test -race ./...` pass
- [ ] A test covers the change (a bug fix has a test that fails without it)
- [ ] `CHANGELOG.md` has an entry under *Unreleased* (breaking changes marked)
- [ ] If a template changed: `TestGeneratedProjectsBuild` passes (it needs `AEMEATH_FRAMEWORK_DIR`, see CONTRIBUTING.md)
- [ ] If processes, signals or paths changed: it works on Windows as well as macOS and Linux
