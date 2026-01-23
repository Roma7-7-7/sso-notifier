# Task Completion Checklist

## After Completing Any Code Change

### 1. Lint Changed Files Only
```bash
make lint:changed
```
**IMPORTANT**: Do NOT run `make lint` on entire codebase unless specifically fixing all issues.
Do NOT fix linting issues in files you didn't modify.

### 2. Run Tests
```bash
make test
```

### 3. Verify Build (if applicable)
```bash
make build
```

## Common Tasks

### Add New Bot Command
1. Add handler method to `SSOBot` struct in `internal/telegram/telegram.go`
2. Register handler in `Start()` method
3. Update keyboard markups if needed
4. Test the command manually

### Modify Message Templates
1. Edit templates in `internal/service/messages.go` or `upcoming_messages.go`
2. Update documentation in `internal/service/TEMPLATES.md`

### Add New Data Field (Requires Migration)
1. Create new migration version in `internal/dal/migrations/vN/`
2. Copy-paste old and new structs to migration package
3. Implement transformation logic
4. Write `vN/README.md`
5. Update types in `internal/dal/bolt.go` (after migration is tested)
6. See `docs/migrations.md` for full checklist

### Change Configuration
1. Update struct in `internal/config/`
2. Update defaults in `cmd/bot/main.go`
3. Update documentation in `CLAUDE.md` and README

## Scope Discipline
- Only make changes that are directly requested
- Don't add features beyond what was asked
- Don't refactor code that wasn't part of the task
- Don't add docstrings/comments to unchanged code
- A bug fix doesn't need surrounding code cleaned up
