# Code Style & Conventions

## Logging
- Use `slog` for structured logging (NOT `log` package)
- Example: `slog.Info("message", "key", value)`

## Error Handling
- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- Always include meaningful context in error messages

## Context
- Pass `context.Context` to all I/O operations
- First parameter in function signatures

## Language
- User-facing messages must be in **Ukrainian**
- Code, comments, and logs in English

## Comments Policy

**ONLY** add comments that explain WHY or provide context. Never add obvious comments.

**Good examples:**
```go
// use !defaultValue because we inverse it below
// Check if we're within notification window (6 AM - 11 PM)
```

**Bad examples (don't do this):**
```go
// Check if user is subscribed  (obvious from code)
// Save  (completely useless)
// Toggle it  (obvious from code)
```

## Imports Order (enforced by gci)
1. Standard library
2. External packages
3. `github.com/Roma7-7-7` packages
4. `github.com/Roma7-7-7/sso-notifier` packages

## Linting Configuration
- Uses golangci-lint v2 with extensive linter list
- Max line length: 175 characters
- Max function length: 100 lines / 50 statements
- Max cyclomatic complexity: 30
- Linters are strict but have sensible exclusions for tests

## Testing
- Use testify for assertions
- Use uber/mock for mocking
- Test files excluded from some strict linters
