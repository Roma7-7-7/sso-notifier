# Suggested Commands

## Build & Run

```bash
# Build the bot binary
make build

# Build for CI (multi-arch Linux binaries)
make ci-build

# Build test server (for local development)
make testserver
```

## Testing

```bash
# Run all tests
make test

# Run tests with coverage report
make coverage
```

## Linting

```bash
# IMPORTANT: Lint only changed files (recommended)
make lint:changed

# Lint entire codebase (use sparingly)
make lint
```

## Go Commands

```bash
# Download dependencies
go mod download

# Generate mocks (if needed)
go generate ./...

# Run specific test
go test -v ./internal/service/... -run TestFunctionName
```

## Git (Darwin/macOS)

```bash
# Standard git commands work normally
git status
git diff
git add <file>
git commit -m "message"

# View recent commits
git log --oneline -10
```

## File System (Darwin/macOS)

```bash
# List files
ls -la

# Find files
find . -name "*.go" -type f

# Search in files
grep -r "pattern" ./internal
```
