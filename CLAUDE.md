# SSO Notifier - Codebase Overview for Claude

This document provides a comprehensive overview of the SSO Notifier codebase for AI assistants and developers.

## Project Purpose

SSO Notifier is a Telegram bot that monitors electricity power outage schedules in Chernivtsi, Ukraine. It scrapes HTML from the local power provider's website, detects schedule changes, and notifies subscribers via Telegram.

## Core Concepts

### Power Outage Schedule Structure

- **City Division**: Chernivtsi is divided into 12 groups
- **Time Intervals**: Schedule uses 30-minute intervals (00:00, 00:30, 01:00, etc.)
- **Status Types**:
  - `В` (Ukrainian) → OFF (power is off)
  - `З` (Ukrainian) → ON (power is on)
  - `МЗ` (Ukrainian) → MAYBE (power might be off)

### Change Detection

The bot uses a hash-based system to detect schedule changes:
- Each group's schedule is hashed (date + status sequence)
- Hash is stored per user subscription
- When hash changes → notification sent
- Hash updated after successful notification

## Architecture

### Layered Architecture

```
Presentation Layer (Telegram Bot)
    ↓
Service Layer (Business Logic)
    ↓
Data Access Layer (BoltDB)
    ↓
External Provider Layer (HTML Scraping)
```

### Concurrency Model

Three concurrent goroutines:

1. **Main Thread**: Telegram bot event loop
2. **Refresh Thread**: Fetches schedule (configurable, default: 5 minutes)
3. **Notification Thread**: Checks for updates (configurable, default: 5 minutes)

## Code Structure

### `/cmd/bot/main.go`

Entry point with configuration and lifecycle management:

1. **Configuration** (lines 20-26)
   - Uses `envconfig` to load environment variables into `Config` struct
   - Supports development mode, custom intervals, group count, DB path
   - All settings have sensible defaults
   - `TELEGRAM_TOKEN` is the only required variable

2. **Initialization** (lines 38-64)
   - Creates BoltDB store at configurable path (default: `data/sso-notifier.db`)
   - Initializes logger (JSON for prod, text for dev based on `DEV` flag)
   - Creates separate Telegram clients for bot UI and notifications
   - Wires up services with dependency injection

3. **Goroutine Management & Lifecycle** (lines 66-87)
   - Spawns refresh schedule goroutine
   - Spawns notification goroutine
   - Uses context cancellation for graceful shutdown
   - Listens for SIGINT/SIGTERM signals
   - Waits for all goroutines to complete

4. **Interval Functions**
   - `refreshShutdowns()`: Fetches new schedule at configured interval
   - `notifyShutdownUpdates()`: Checks and sends notifications at configured interval
   - Both use configurable delays passed as parameters

**Configuration Struct:**
```go
type Config struct {
    Dev                      bool          // Development mode flag
    GroupsCount              int           // Number of groups (default: 12)
    DBPath                   string        // Database path
    RefreshShutdownsInterval time.Duration // Schedule fetch interval
    NotifyInterval           time.Duration // Notification check interval
    TelegramToken            string        // Bot token (required)
}
```

### `/internal/dal/bolt.go`

Data Access Layer using BoltDB (embedded key-value database).

**Data Types:**

```go
type Status string  // "Y" (ON), "N" (OFF), "M" (MAYBE)

type Shutdowns struct {
    Date    string                   // e.g., "20 жовтня"
    Periods []Period                 // [{From: "00:00", To: "00:30"}, ...]
    Groups  map[string]ShutdownGroup // {"1": {...}, "2": {...}}
}

type Subscription struct {
    ChatID int64             // Telegram chat ID
    Groups map[string]string // {"1": "hash123", "2": "hash456"}
}
```

**BoltDB Buckets:**
- `shutdowns`: Stores current schedule (single key: "table")
- `subscriptions`: Stores user subscriptions (key: chatID)

**Key Methods:**
- `GetShutdowns()` / `PutShutdowns()`: Schedule CRUD
- `GetSubscription()` / `PutSubscription()`: User subscription CRUD
- `GetAllSubscriptions()`: Fetch all users (for notifications)
- `PurgeSubscriptions()`: Remove user (when blocked)

### `/internal/providers/chernivtsi.go`

HTML scraper for https://oblenergo.cv.ua/shutdowns/

**Parsing Logic:**

1. **Date Extraction** (line 68)
   - Selector: `div#gsv ul p`
   - Extracts date string (e.g., "20 жовтня")

2. **Time Periods** (lines 147-183)
   - Selector: `div > p u`
   - Parses time strings like "00:00", "00:30"
   - Handles edge case: "23:0000:00" → ["23:00", "24:00"]

3. **Groups** (lines 121-144)
   - Selector: `ul > li[data-id]`
   - Extracts 12 groups (data-id="1" through "12")

4. **Status Items** (lines 186-208)
   - Selector: `div[data-id='N']` for each group
   - Finds child nodes: `o`, `u`, `s` tags
   - Text mapping:
     - `В` → OFF
     - `З` → ON
     - Other → MAYBE

**Error Handling:**
- Validates all extracted data (lines 96-119)
- Ensures groups have correct number of items (matching periods)

### `/internal/service/shutdowns.go`

Service for refreshing schedule from external provider.

**Key Method: `Refresh()` (lines 34-51)**

```go
func (s *Shutdowns) Refresh(ctx context.Context) error {
    s.mx.Lock()  // Prevent concurrent refreshes
    defer s.mx.Unlock()

    // 1-minute timeout for HTTP request
    ctx, cancel := context.WithTimeout(ctx, time.Minute)
    defer cancel()

    // Fetch from provider
    table := providers.ChernivtsiShutdowns(ctx)

    // Store in database
    s.store.PutShutdowns(table)
}
```

**Thread Safety**: Uses mutex to prevent concurrent refreshes.

### `/internal/service/notifications.go`

Core notification logic with sophisticated time handling.

**Main Method: `NotifyShutdownUpdates()` (lines 49-78)**

Flow:
1. Fetch current schedule from DB
2. Get all subscriptions
3. For each subscription, call `processSubscription()`

**Key Function: `processSubscription()` (lines 80-121)**

For each subscribed group:
1. Calculate new hash: `shutdownGroupHash(group, date)`
2. Compare with stored hash
3. If changed:
   - Join consecutive periods with same status
   - Cut past time periods
   - Render message
   - Send via Telegram
   - Update subscription hash

**Time Filtering: `cutByKyivTime()` (lines 157-169)**

Critical for user experience:
- Uses `Europe/Kyiv` timezone
- Filters out past periods
- Only shows future events

Example:
```
Current time: 14:30 Kyiv
Original: [00:00-12:00 OFF, 12:00-18:00 ON, 18:00-24:00 OFF]
Filtered: [12:00-18:00 ON, 18:00-24:00 OFF]
```

**Period Joining: `join()` (lines 133-154)**

Merges consecutive periods with same status for cleaner messages:
```
Before: [00:00-00:30 OFF, 00:30-01:00 OFF, 01:00-01:30 OFF]
After:  [00:00-01:30 OFF]
```

**Message Templates** (messages.go:164-174):

> **IMPORTANT:** If you change the template or rendering logic in `messages.go`, you MUST also update:
> - `internal/service/TEMPLATES.md` - Update all examples
> - This section in CLAUDE.md

Current template structure (supports multiple dates and groups):

```go
messageTemplate = `Графік стабілізаційних відключень:
{{range .Dates}}
📅 {{.Date}}:
{{range .Groups}}Група {{.GroupNum}}:
{{range .StatusLines}}{{if .Periods}}  {{.Emoji}} {{.Label}}:{{range .Periods}} {{.From}} - {{.To}};{{end}}
{{end}}{{end}}
{{end}}{{end}}`

// Status line configuration (messages.go:185-189):
statusLines := []StatusLine{
    {Emoji: "🟢", Label: "Заживлено", Periods: grouped[dal.ON]},
    {Emoji: "🟡", Label: "Можливо заживлено", Periods: grouped[dal.MAYBE]},
    {Emoji: "🔴", Label: "Відключено", Periods: grouped[dal.OFF]},
}
```

Example output:
```
Графік стабілізаційних відключень:

📅 20 жовтня:
Група 5:
  🟢 Заживлено:  14:00 - 18:00; 20:00 - 24:00;
  🔴 Відключено:  18:00 - 20:00;
```

See `internal/service/TEMPLATES.md` for detailed documentation on the template system.

### `/internal/service/subscriptions.go`

Subscription management service.

**Key Methods:**

1. `IsSubscribed(chatID)` - Check if user exists
2. `SubscribeToGroup(chatID, groupNum)` - Add/update subscription
3. `Unsubscribe(chatID)` - Remove all user data

**Important Note** (line 62-64):
When subscribing to a new group, the entire `Groups` map is replaced:
```go
sub.Groups = map[string]string{
    groupNum: "",  // Empty hash triggers first notification
}
```
This means users can only subscribe to one group at a time (potential feature limitation).

### `/internal/telegram/telegram.go`

Telegram bot integration using telebot.v3 library.

**Bot Structure:**

```go
type Bot struct {
    svc     SubscriptionService  // Subscription management
    bot     *tb.Bot             // Telegram bot instance
    markups *markups            // Inline keyboard layouts
    log     *slog.Logger        // Structured logger
}
```

**Constructor: `NewBot()`** (lines 33-50)
- Takes token, subscription service, group count, and logger
- Returns error instead of panicking (better error handling)
- Creates bot instance with 5-second polling timeout
- Initializes markups based on configurable group count
- Adds "component: bot" to logger for context

**Lifecycle: `Start(ctx context.Context)`** (lines 52-76)
- Accepts context for graceful shutdown
- Registers all command handlers (`/start`, `/subscribe`, `/unsubscribe`)
- Uses helper method `registerButtonHandlers()` for cleaner code
- Listens for context cancellation in goroutine
- Stops bot gracefully on shutdown signal
- Returns error for better error propagation

**Handlers:**

1. **StartHandler** (lines 58-76)
   - Extracts chatID for logging
   - Shows main menu
   - Different markup for subscribed/unsubscribed users
   - Logs with chatID and subscription status
   - Structured error handling with context

2. **ChooseGroupHandler** (lines 78-81)
   - Shows group selection buttons
   - Logs chatID for debugging

3. **SetGroupHandler** (lines 83-101)
   - Subscribes user to selected group
   - Logs success at Info level with chatID and group
   - Logs errors with full context
   - Returns success message with dynamic group number

4. **UnsubscribeHandler** (lines 103-114)
   - Removes all subscriptions
   - Logs unsubscribe events at Info level
   - Returns appropriate markup based on state

**Helper Method: `registerButtonHandlers()`** (lines 116-120)
- Registers same handler for multiple buttons
- Cleaner than manual iteration
- Avoids variable capture issues

**Markup Generation** (lines 214-261):

Creates inline keyboards with configurable group count:
- Main menu: Subscribe/Unsubscribe buttons
- Group selection: Dynamic number of buttons (default 12, 5 per row)
- Navigation: Back button
- All button text and callbacks in one place

**Key Improvements in Refactor:**
- Context-aware shutdown instead of blocking `Start()`
- Structured logging with chatID throughout
- Error propagation instead of panics
- Configurable group count (not hardcoded constant)
- Cleaner handler registration pattern
- Better separation of concerns (no MessageSender in this file)

## Data Flow Examples

### New User Subscribes

1. User sends `/start` → `StartHandler`
2. Bot shows "Підписатись на оновлення" button
3. User clicks → `ChooseGroupHandler`
4. Bot shows groups 1-12
5. User clicks "5" → `SetGroupHandler("5")`
6. Service creates subscription: `{ChatID: 123, Groups: {"5": ""}}`
7. Bot confirms: "Ви підписались на групу 5"

### Schedule Update Notification

1. `refreshShutdowns()` runs at configured interval (default: 5 minutes)
2. Fetches HTML from oblenergo.cv.ua
3. Parses and stores in BoltDB
4. `notifyShutdownUpdates()` runs at configured interval (default: 5 minutes)
5. Detects hash change for group 5
6. Finds all subscriptions with group 5
7. Renders message with emoji indicators
8. Sends via separate Telegram client to each subscriber
9. Updates subscription hashes

### User Blocks Bot

1. External Telegram client tries to send notification
2. Telegram API returns "Forbidden: bot was blocked by the user"
3. Client handles error and purges subscription
4. User data removed from database
5. No further messages sent to that user

## Key Design Patterns

### 1. Repository Pattern

Services depend on interfaces, not concrete implementations:
```go
type ShutdownsStore interface {
    GetShutdowns() (dal.Shutdowns, bool, error)
    PutShutdowns(s dal.Shutdowns) error
}
```
Allows easy testing and swapping storage backends.

### 2. Dependency Injection

All dependencies injected via constructors:
```go
func NewNotifications(
    shutdowns ShutdownsStore,
    subscriptions SubscriptionsStore,
    telegram TelegramClient,
    log *slog.Logger,
) *Notifications
```

### 3. Constructor Pattern

Telegram bot and services use simple constructor functions:
```go
bot, err := telegram.NewBot(token, subscriptionsSvc, groupCount, log)
shutdownsSvc := service.NewShutdowns(store, log)
```
Returns errors instead of panicking for better error handling.

### 4. Mutex for Thread Safety

All services use mutexes to prevent concurrent access:
```go
func (s *Shutdowns) Refresh(ctx context.Context) error {
    s.mx.Lock()
    defer s.mx.Unlock()
    // ... critical section
}
```

## Database Migrations

The codebase uses a custom migration system for managing BoltDB schema changes.

### Migration System Architecture

**Location:** `internal/dal/migrations/`

**Key Principle:** Migrations are completely independent from the `dal` package. They work directly with raw `*bbolt.DB` and contain their own type definitions.

### Package Structure

```
internal/dal/migrations/
├── README.md           # Latest DB schema + migration system overview
├── migrations.go       # Core migration runner and interfaces
├── v1/
│   ├── README.md      # Bootstrap migration docs
│   └── migration.go   # Creates migrations bucket
├── v2/
│   ├── README.md      # v2 migration docs
│   └── migration.go   # Creates shutdowns and subscriptions buckets
└── v3/
    ├── README.md      # v3 migration docs
    └── migration.go   # Adds CreatedAt to subscriptions (not yet enabled)
```

### Migration Storage

Migrations are tracked in BoltDB itself:

- **Bucket:** `migrations`
- **Key Format:** `"v1"`, `"v2"`, `"v3"`, etc.
- **Value Format:** ISO 8601 timestamp (RFC3339) of when migration was applied
- **Example:** Key: `"v3"`, Value: `"2025-10-31T14:23:45Z"`

### Migration Interface

```go
type Migration interface {
    // Version returns migration version (1, 2, 3, etc.)
    Version() int

    // Description returns human-readable description
    Description() string

    // Up performs the migration
    Up(db *bbolt.DB) error
}
```

### Migration Execution Flow

1. Open/create `migrations` bucket in BoltDB
2. Load all registered migrations
3. Read applied migrations from DB
4. Filter out already-applied migrations
5. Sort remaining by version (ascending)
6. Execute each migration sequentially
7. Record execution timestamp after successful completion
8. Fail fast if any migration errors

### Creating a New Migration

**CRITICAL RULES:**

1. **Never import from `internal/dal`** - Migrations must be self-contained
2. **Copy-paste old types** - Include both old and new structures in migration code
3. **Write README first** - Document what changes and why
4. **Test on production data copy** - Never test migrations on live DB
5. **Never modify existing migrations** - Once deployed, migrations are immutable

**Step-by-Step Checklist:**

- [ ] Create `internal/dal/migrations/vN/` directory (N = next version)
- [ ] Copy old type definitions from `dal/bolt.go` to `vN/migration.go`
- [ ] Define new type structures in `vN/migration.go`
- [ ] Implement `Migration` interface with transformation logic
- [ ] Write `vN/README.md` with:
  - Date
  - Description of what changed and why
  - Schema before/after
  - Data transformation details
  - Rollback strategy (or "not possible")
- [ ] Update core `migrations/README.md` with latest schema
- [ ] Test migration on copy of production DB
- [ ] Verify idempotency (running twice doesn't break)
- [ ] Update `dal/bolt.go` types (after migration is tested)
- [ ] Register migration in `migrations.go`

### Example Migration Structure

**Scenario:** Add `CreatedAt` timestamp to subscriptions (v3 migration)

```go
// internal/dal/migrations/v3/migration.go
package v3

import (
    "encoding/json"
    "fmt"
    "time"
    "go.etcd.io/bbolt"
)

// SubscriptionV2 is the OLD structure (copy-pasted from dal at time of v2)
type SubscriptionV2 struct {
    ChatID int64             `json:"chat_id"`
    Groups map[string]string `json:"groups"`
}

// SubscriptionV3 is the NEW structure
type SubscriptionV3 struct {
    ChatID    int64             `json:"chat_id"`
    Groups    map[string]string `json:"groups"`
    CreatedAt time.Time         `json:"created_at"`
}

type MigrationV3 struct{}

func (m *MigrationV3) Version() int {
    return 3
}

func (m *MigrationV3) Description() string {
    return "Add CreatedAt timestamp to subscriptions"
}

func (m *MigrationV3) Up(db *bbolt.DB) error {
    return db.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte("subscriptions"))
        if b == nil {
            return nil // No subscriptions to migrate
        }

        c := b.Cursor()
        now := time.Now()

        for k, v := c.First(); k != nil; k, v = c.Next() {
            // Unmarshal old structure
            var oldSub SubscriptionV2
            if err := json.Unmarshal(v, &oldSub); err != nil {
                return fmt.Errorf("unmarshal old subscription: %w", err)
            }

            // Transform to new structure
            newSub := SubscriptionV3{
                ChatID:    oldSub.ChatID,
                Groups:    oldSub.Groups,
                CreatedAt: now, // Set to migration time
            }

            // Marshal and write back
            newData, err := json.Marshal(newSub)
            if err != nil {
                return fmt.Errorf("marshal new subscription: %w", err)
            }

            if err := b.Put(k, newData); err != nil {
                return fmt.Errorf("put new subscription: %w", err)
            }
        }

        return nil
    })
}
```

### Migration Best Practices

**DO:**
- ✅ Copy-paste type definitions to migration package
- ✅ Document every change in README
- ✅ Test on production data snapshot
- ✅ Handle errors gracefully
- ✅ Use transactions for data integrity
- ✅ Log migration progress
- ✅ Verify idempotency
- ✅ Consider data volume (may need batching)

**DON'T:**
- ❌ Import types from `internal/dal`
- ❌ Modify existing migrations
- ❌ Skip documentation
- ❌ Test on live database
- ❌ Assume migration succeeds
- ❌ Forget about rollback strategy
- ❌ Ignore backward compatibility during deployment

### Integration with Application

**In `cmd/bot/main.go`:**

```go
// After creating BoltDB instance, before services
if err := migrations.RunMigrations(store.DB()); err != nil {
    log.Error("Failed to run database migrations", "error", err)
    os.Exit(1)
}
```

**Expose raw DB access in `internal/dal/bolt.go`:**

```go
// DB returns the underlying BoltDB instance for migrations
func (s *BoltDB) DB() *bbolt.DB {
    return s.db
}
```

### Migration README Template

```markdown
# Migration v{N}: {Brief Title}

## Date
{YYYY-MM-DD}

## Description
{Detailed explanation of what this migration does and why it's needed}

## Schema Changes

### Before
{Old structure with field descriptions}

### After
{New structure with field descriptions}

## Data Transformation
{How existing data is migrated. Include examples.}

## Rollback Strategy
{How to rollback if needed, or explicitly state "Not possible - breaking change"}

## Testing Notes
{Any special considerations for testing this migration}
```

### Troubleshooting

**Migration fails midway:**
- Migrations run in transactions when possible
- Check logs for specific error
- Restore from backup if needed
- Fix migration code and retry

**Migration marked as applied but data unchanged:**
- Check migration logic
- Verify bucket names are correct
- Ensure transaction committed

**Need to rollback migration:**
- Restore database from backup
- Remove migration from registry
- Fix migration code

### Version 1 (Bootstrap)

The first migration (v1) is special - it creates the migrations bucket itself:

```go
func (m *MigrationV1) Up(db *bbolt.DB) error {
    return db.Update(func(tx *bbolt.Tx) error {
        _, err := tx.CreateBucketIfNotExists([]byte("migrations"))
        return err
    })
}
```

This ensures the migration system can track itself from the start.

## Configuration

All configuration via environment variables using `envconfig`:

### Required Variables

- `TELEGRAM_TOKEN`: Telegram bot token from @BotFather

### Optional Variables (with defaults)

- `DEV` (default: false): Set to "true" for text logging instead of JSON
- `GROUPS_COUNT` (default: 12): Number of power outage groups
- `DB_PATH` (default: "data/sso-notifier.db"): Database file path
- `REFRESH_SHUTDOWNS_INTERVAL` (default: 5m): Schedule fetch frequency
- `NOTIFY_INTERVAL` (default: 5m): Notification check frequency

### Timeouts

- HTTP request: 1 minute (line 39, shutdowns.go)
- Telegram polling: 5 seconds (telegram.go:65)

## Potential Issues & TODOs

### Known Limitations

1. **Single Group Subscription** (subscriptions.go:62)
   - Current implementation overwrites `Groups` map
   - Users can only subscribe to one group
   - Should append to existing map

2. **No Retry Logic**
   - HTTP failures are logged but not retried
   - Could add exponential backoff

3. **No Rate Limiting**
   - Telegram API has rate limits
   - No protection against hitting them

4. **Timezone Hardcoded**
   - `Europe/Kyiv` hardcoded (notifications.go:32)
   - Could be configurable

### TODOs in Code

- `main.go:29` - "todo use my own lib" (referring to telebot)

## Testing Considerations

### Testable Components

All services use interfaces, making them mockable:

```go
// Mock for testing
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

## Deployment

### Binary

- CGO disabled (`CGO_ENABLED=0`) for static linking
- Single binary with no dependencies
- Cross-platform compatible

### Storage

- Database: `data/app.db`
- Logs: stdout (JSON or text)

### Monitoring

Structured logging with slog:
```go
log.InfoContext(ctx, "refreshing shutdowns")
log.ErrorContext(ctx, "Error refreshing", "error", err)
```

Fields for observability:
- Service name
- Chat IDs
- Group numbers
- Error details

## Development Workflow

### Build

```bash
make build
# Produces: bin/sso-notifier
```

### Run Locally

```bash
ENV=dev TOKEN=your_token ./bin/sso-notifier
```

### Linting

**IMPORTANT**: When making code changes, only lint the files you've modified, not the entire codebase.

```bash
# Lint only changed files (recommended during development)
make lint:changed

# Lint entire codebase (use sparingly, only when fixing all issues)
make lint
```

**Best Practice for AI Assistants:**
- After modifying code, run `make lint:changed` to check only your changes
- Do NOT run `make lint` or fix linting issues in files you didn't modify
- Only address linting issues in code you've actually changed
- This prevents scope creep and keeps changes focused

### Dependencies

All vendored (no network required for builds):
```bash
go mod vendor
```

## Useful Context for AI Assistants

### When Making Changes

1. **HTML Structure Changes**: Update selectors in `chernivtsi.go`
2. **Message Format**: Edit templates in `notifications.go:172-183`
3. **Bot Commands**: Add handlers in `telegram.go`
4. **Storage Schema**: Update structs in `dal/bolt.go`

### Code Style

- Uses `slog` for structured logging (not `log`)
- Error wrapping with `fmt.Errorf("context: %w", err)`
- Context passed to all I/O operations
- Ukrainian strings for user-facing messages

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
3. Implement transformation logic in migration
4. Write vN/README.md with change description
5. Update core migrations/README.md with latest schema
6. Test migration on copy of production DB
7. Update structs in `dal/bolt.go` (after migration is ready)
8. Update parsing in `providers/chernivtsi.go` (if needed)
9. Update templates in `notifications.go` (if needed)

## Resources

- Telegram Bot API: https://core.telegram.org/bots/api
- goquery docs: https://pkg.go.dev/github.com/PuerkitoBio/goquery
- BoltDB: https://github.com/etcd-io/bbolt
- Schedule source: https://oblenergo.cv.ua/shutdowns/
