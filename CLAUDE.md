# SSO Notifier - Codebase Overview

Telegram bot that monitors electricity power outage schedules in Chernivtsi, Ukraine. Scrapes HTML from oblenergo.cv.ua, detects schedule changes, and notifies subscribers via Telegram.

## Quick Reference

| Topic | Documentation |
|-------|---------------|
| Architecture & Code Structure | [docs/architecture.md](docs/architecture.md) |
| Database Migrations | [docs/migrations.md](docs/migrations.md) |
| Deployment | [docs/deployment.md](docs/deployment.md) |
| Data Flow Examples | [docs/data-flows.md](docs/data-flows.md) |
| Development Workflow | [docs/development.md](docs/development.md) |
| Message Templates | [internal/service/TEMPLATES.md](internal/service/TEMPLATES.md) |

## Core Concepts

### Power Outage Schedule Structure

- **City Division**: Chernivtsi is divided into 12 groups
- **Time Intervals**: Schedule uses 30-minute intervals (00:00, 00:30, 01:00, etc.)
- **Status Types**:
  - `В` (Ukrainian) → OFF (power is off)
  - `З` (Ukrainian) → ON (power is on)
  - `МЗ` (Ukrainian) → MAYBE (power might be off)

### Change Detection

Hash-based system to detect schedule changes:
- Each group's schedule is hashed (date + status sequence)
- Hash is stored per user subscription
- When hash changes → notification sent
- Hash updated after successful notification

## Architecture Overview

```
Presentation Layer (Telegram Bot)
    ↓
Service Layer (Business Logic)
    ↓
Data Access Layer (BoltDB)
    ↓
External Provider Layer (HTML Scraping)
```

**Four concurrent goroutines:**
1. Main Thread: Telegram bot event loop
2. Refresh Thread: Fetches schedule (default: 5 minutes)
3. Notification Thread: Checks for schedule updates (default: 5 minutes)
4. Alerts Thread: Checks for upcoming outages (default: 1 minute)

## Rules for AI Assistants

### Code Style

- Uses `slog` for structured logging (not `log`)
- Error wrapping with `fmt.Errorf("context: %w", err)`
- **Always wrap errors from public methods** - never return bare errors from public function calls
- Context passed to all I/O operations
- Ukrainian strings for user-facing messages

### Comments Policy

Only add comments that explain WHY or provide context. Never add obvious comments.

**Good:**
```go
// use !defaultValue because we inverse it below
// Check if we're within notification window (6 AM - 11 PM)
```

**Bad:**
```go
// Check if user is subscribed  (obvious from code)
// Save  (completely useless)
// Toggle it  (obvious from code)
```

### Linting Policy

**IMPORTANT**: Only lint files you've modified, not the entire codebase.

```bash
# Lint only changed files (recommended)
make lint:changed

# NOT: make lint (unless fixing all issues)
```

- Do NOT fix linting issues in files you didn't modify
- This prevents scope creep and keeps changes focused

### Common Operations

**Add new bot command:**
1. Add handler method to `SSOBot`
2. Register in `Start()` method
3. Update markups if needed

**Change refresh interval:**
1. Edit constants in `cmd/bot/main.go`
2. Consider impact on server load

**Add new data field (requires migration):**
1. Create new migration version in `internal/dal/migrations/vN/`
2. Copy-paste old and new structs to migration package
3. Implement transformation logic
4. Write vN/README.md
5. Update `dal/bolt.go` types (after migration is tested)
6. See [docs/migrations.md](docs/migrations.md) for full checklist

**Modify message templates:**
1. Edit templates in `internal/service/messages.go` or `upcoming_messages.go`
2. Update `internal/service/TEMPLATES.md`

### Key Files Quick Reference

| Purpose | File |
|---------|------|
| Entry point | `cmd/bot/main.go` |
| Data types & storage | `internal/dal/bolt.go` |
| HTML scraping | `internal/providers/chernivtsi.go` |
| Schedule refresh | `internal/service/shutdowns.go` |
| Notifications | `internal/service/notifications.go` |
| Upcoming alerts | `internal/service/alerts.go` |
| Subscriptions | `internal/service/subscriptions.go` |
| Telegram bot | `internal/telegram/telegram.go` |
| Message templates | `internal/service/messages.go` |

## Configuration

**Required:**
- `TELEGRAM_TOKEN`: Telegram bot token from @BotFather

**Optional (with defaults):**
- `DEV` (false): Text logging instead of JSON
- `GROUPS_COUNT` (12): Number of power outage groups
- `DB_PATH` (data/sso-notifier.db): Database file path
- `REFRESH_SHUTDOWNS_INTERVAL` (5m): Schedule fetch frequency
- `NOTIFY_INTERVAL` (5m): Notification check frequency
- `NOTIFY_UPCOMING_INTERVAL` (1m): Upcoming alerts frequency
- `SCHEDULE_URL` (https://oblenergo.cv.ua/shutdowns/): Schedule provider URL
