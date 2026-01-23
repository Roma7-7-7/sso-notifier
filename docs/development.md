# Development Workflow

## Build

```bash
make build
# Produces: bin/sso-notifier
```

## Run Locally

```bash
DEV=true TELEGRAM_TOKEN=your_token ./bin/sso-notifier
```

## Linting

**IMPORTANT**: When making code changes, only lint the files you've modified, not the entire codebase.

```bash
# Lint only changed files (recommended during development)
make lint:changed

# Lint entire codebase (use sparingly)
make lint
```

## Dependencies

All vendored (no network required for builds):
```bash
go mod vendor
```

## Testing Considerations

### Testable Components

All services use interfaces, making them mockable:

```go
type MockStore struct {
    shutdowns dal.Shutdowns
    subs      []dal.Subscription
}

func (m *MockStore) GetShutdowns() (dal.Shutdowns, bool, error) {
    return m.shutdowns, true, nil
}
```

### Critical Test Cases

1. **HTML Parsing**
   - Malformed HTML
   - Missing elements
   - Invalid time formats
   - Edge case: "23:0000:00"

2. **Notification Logic**
   - Hash changes correctly detected
   - Past periods filtered
   - Consecutive periods joined
   - Timezone handling

3. **Subscription Management**
   - Multiple subscriptions per user
   - Blocked user cleanup
   - Concurrent access

4. **Error Handling**
   - Network failures
   - Telegram API errors
   - Database corruption

## Performance Characteristics

### Memory Usage
- BoltDB: Memory-mapped file (efficient)
- Vendor directory: ~3MB (libraries included)
- Runtime: Minimal (single binary, no GC pressure)

### Network Usage
- HTTP: One request per 5 minutes
- Telegram: Variable (depends on subscriber count)
- Average: Very low bandwidth

### Scalability
- **Current Design**: Single instance
- **Bottleneck**: Notification loop processes subscriptions serially
- **Improvement**: Parallel notification processing with goroutine pool

### Database
- BoltDB: Single file, no maintenance required
- Backup: Simple file copy (when not running)
- Growth: ~1KB per subscriber + schedule (~5KB)

## Known Limitations

1. **No Retry Logic**: HTTP failures are logged but not retried
2. **No Rate Limiting**: No protection against Telegram API rate limits
3. **Timezone Hardcoded**: `Europe/Kyiv` hardcoded in notifications.go

## Resources

- Telegram Bot API: https://core.telegram.org/bots/api
- goquery docs: https://pkg.go.dev/github.com/PuerkitoBio/goquery
- BoltDB: https://github.com/etcd-io/bbolt
- Schedule source: https://oblenergo.cv.ua/shutdowns/
