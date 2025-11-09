# SSO Notifier - Architecture Overview

Essential context for AI assistants working with this codebase.

## Project Purpose

Telegram bot monitoring electricity outage schedules in Chernivtsi, Ukraine. Scrapes HTML from power provider, detects changes via hash comparison, notifies subscribers with:
- Change notifications (when schedule updates)
- 10-minute advance alerts (before power status changes)

## Core Concepts

**Power Outage Structure**: 12 groups, 30-minute intervals, three statuses: OFF (`В`), ON (`З`), MAYBE (`МЗ`)

**Change Detection**: Hash-based (`hash(date + status_sequence)`) stored per user/group. Hash mismatch → notification → update hash.

## Architecture

```
Telegram Bot → Service Layer → DAL (BoltDB) → Provider (HTML scraping)
```

**Concurrency Model**:
- Main thread: Telegram bot event loop
- Background: Single `Scheduler` service spawns 3 goroutines:
  - Refresh schedule (default: 5m)
  - Notify on changes (default: 5m) 
  - Alert 10min before outages (default: 1m)
- Built-in: heartbeat logging, panic recovery, graceful shutdown

## Code Organization

### `/cmd/bot/main.go`
Entry point: configuration (envconfig), initialization (DB, logger, services), lifecycle (spawns Scheduler, handles shutdown).

**Key Design**: Single `Scheduler` service manages all background goroutines instead of spawning them individually in main.

### `/internal/dal/`
Data Access Layer using BoltDB (embedded key-value database).

**BoltDB Buckets:**
- `shutdowns`: Current schedule (key: date string)
- `subscriptions`: User subscriptions (key: chatID, value: groups map + hashes + settings)
- `alerts`: Deduplication for 10-min alerts (key: `{chatID}_{date}_{time}_{status}_{group}`)
- `migrations`: Migration tracking (key: version, value: timestamp)

See `dal/bolt.go` for type definitions. See `dal/migrations/README.md` for schema details.

### `/internal/providers/chernivtsi.go`
HTML scraper for https://oblenergo.cv.ua/shutdowns/ (configurable via `SCHEDULE_URL` env var).

Extracts: date, time periods (30-min intervals), 12 groups, statuses (В/З/МЗ → OFF/ON/MAYBE). Handles edge case "23:0000:00" → ["23:00", "24:00"].

### `/internal/service/shutdowns.go`
Fetches schedules from provider and stores in DB. `Refresh()` uses mutex for thread safety, 1-minute timeout, fetches today + tomorrow (if available).

### `/internal/service/schedules.go`
Centralized scheduler managing all background goroutines. `Start()` spawns 3 processes with configurable intervals. `run()` provides: heartbeat logging (every 5min), panic recovery, error handling, clean shutdown. Prevents duplicated scheduling code in main.

### `/internal/service/notifications.go`
Change detection and notifications. Compares hashes per user/group, on change: filters past periods (`cutByKyivTime()`), joins consecutive periods (`join()`), renders message, sends, updates hash.

**Critical**: Uses `Europe/Kyiv` timezone for filtering. Messages rendered via templates in `messages.go`.

> **Template Changes**: When modifying templates in `messages.go`, update `internal/service/TEMPLATES.md` documentation.

### `/internal/service/subscriptions.go`
Multi-group subscription management. `ToggleGroupSubscription()` adds if not exists, removes if exists. Deletes entire subscription when last group removed.

### `/internal/service/alerts.go`
10-minute advance alerts before status changes. Flow: check time window (6AM-11PM), calculate target time (+10min), find matching period, check if outage start (`isOutageStart()`), send per user settings, deduplicate via alerts bucket.

**Key Algorithms:**
- `findPeriodIndex()`: Finds period containing time (works at any minute, not just 30-min boundaries)
- `isOutageStart()`: Detects if period is START of status change (prevents duplicate notifications)

**Settings**: `notify_off_10min`, `notify_maybe_10min`, `notify_on_10min` (all default false).

### `/internal/service/upcoming_messages.go`
Template-based rendering for 10-min alerts. Groups alerts by status+time, sorts groups, renders with emoji/labels.

> **Template Changes**: When modifying, update `internal/service/TEMPLATES.md` documentation.

### `/internal/telegram/telegram.go`
Bot UI using telebot.v3. Centralized callback router for all inline buttons. Handlers: `/start`, manage groups (toggle with ✅), unsubscribe. Dynamic markup with configurable group count. Context-aware shutdown.

## Key Data Flows

**Schedule Update Notification:**
1. Scheduler → Shutdowns.Refresh() → fetch HTML → store in DB
2. Scheduler → Notifications.NotifyShutdownUpdates() → compare hashes → filter/join periods → send → update hashes

**10-Minute Alert:**
1. Scheduler (every 1min) → Alerts.NotifyUpcomingShutdowns() → check time window (6-23h)
2. Calculate target time (+10min) → find period → check isOutageStart()
3. Filter by user settings → deduplicate (alerts bucket) → send merged message

**User Subscription:**
User → `/start` → "Підписатись" → ManageGroupsHandler → tap group → ToggleGroupHandler → DB update → show ✅

## Key Design Patterns & Conventions

**Repository Pattern**: Services depend on interfaces (ShutdownsStore, SubscriptionsStore), not concrete BoltDB.

**Dependency Injection**: All dependencies via constructors. Enables testing with mocks.

**Thread Safety**: All services use mutexes for concurrent access protection.

**Error Handling**: Constructors return errors (not panic). Context cancellation respected everywhere.

## Database Migrations

**Location**: `internal/dal/migrations/` - See `migrations/README.md` for complete documentation.

**Key Principle**: Migrations are self-contained. NEVER import from `internal/dal`. Copy-paste old and new type definitions into migration code.

**Creating New Migration**:
1. Create `vN/` directory with `migration.go` and `README.md`
2. Copy old types, define new types (both in migration file)
3. Implement transformation logic
4. Test on production DB copy
5. Update `dal/bolt.go` types AFTER migration is tested

**Critical Rules**:
- ❌ Never import from `internal/dal`
- ❌ Never modify existing migrations
- ✅ Document everything in README
- ✅ Test on copy, not live DB

## Configuration

All via environment variables (envconfig):

**Required**: `TELEGRAM_TOKEN`

**Optional** (defaults): `DEV` (false), `GROUPS_COUNT` (12), `DB_PATH` (data/sso-notifier.db), `REFRESH_SHUTDOWNS_INTERVAL` (5m), `NOTIFY_INTERVAL` (5m), `NOTIFY_UPCOMING_INTERVAL` (1m), `SCHEDULE_URL` (https://oblenergo.cv.ua/shutdowns/)

**Timeouts**: HTTP 1min, Telegram polling 5s

## Known Limitations

- No retry logic for HTTP failures (logged only)
- No rate limiting protection for Telegram API
- `Europe/Kyiv` timezone hardcoded (notifications.go)

## Testing & Performance

**Testable**: All services use interfaces for mocking. Key test areas: HTML parsing (edge case: "23:0000:00"), hash detection, period filtering/joining, timezone handling.

**Performance**: BoltDB memory-mapped, minimal runtime overhead. Network: ~1 HTTP req/5min + Telegram messages. Current bottleneck: serial notification processing (could parallelize).

## Deployment

**Binary**: Static (`CGO_ENABLED=0`), single file, no dependencies.

**Storage**: BoltDB at `data/sso-notifier.db`, logs to stdout (JSON/text).

**Backups**: Automated S3 backups via `deployment/backup.sh` (cron daily 8PM). Local safety backups during setup. See `deployment/README.md`.

**Monitoring**: Structured logging (slog) with context: service, chatID, group, errors.

## Development

**Build**: `make build` → `bin/sso-notifier`

**Run**: `ENV=dev TOKEN=xxx ./bin/sso-notifier`

**Linting**: `make lint:changed` (only check modified files). Don't run `make lint` or fix unrelated files.

**Dependencies**: All vendored (`go mod vendor`).

## Code Style & Conventions

**Logging**: Use `slog`, not `log`. Error wrapping: `fmt.Errorf("context: %w", err)`

**Context**: Pass to all I/O operations.

**Messages**: Ukrainian for user-facing text.

**Comments**: Only explain WHY or non-obvious behavior. Never repeat what code obviously does.
- ✅ Good: `// Check notification window (6 AM - 11 PM)` (business context)
- ❌ Bad: `// Save to store` before `store.Put()` (obvious)

## Common Tasks

**Change templates**: Update `messages.go` or `upcoming_messages.go`, then update `TEMPLATES.md` docs.

**Add bot command**: Handler in `telegram.go`, register in `Start()`, update markups.

**Add scheduled task**: Method in service, spawn in `Scheduler.Start()`, add env config.

**Schema change**: Create migration in `migrations/vN/`, copy old/new types, test on DB copy, update `dal/bolt.go` after.

**Deployment scripts**: `setup-ec2.sh` (initial), `deploy.sh` (binary only), `backup.sh` (S3). Never touch DB in scripts.

## Critical Gotchas

- HTML parsing handles edge case: "23:0000:00" → ["23:00", "24:00"]
- Timezone is `Europe/Kyiv` (hardcoded in `notifications.go`)
- Alerts dedupe by period START time, not target time
- Migration types NEVER import from `internal/dal`
- Template changes require updating `TEMPLATES.md`

## Where to Find Things

- Bot commands & UI: `telegram.go`
- Message templates: `messages.go`, `upcoming_messages.go`, `TEMPLATES.md`
- Schedule parsing: `providers/chernivtsi.go`
- Change detection: `notifications.go` (hash comparison)
- 10-min alerts: `alerts.go` (time window, isOutageStart)
- Scheduling: `schedules.go` (heartbeat, panic recovery)
- Data access: `dal/bolt.go`, `dal/migrations/README.md`
