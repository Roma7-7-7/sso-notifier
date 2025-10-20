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

**Message Templates** (lines 172-183):

```go
messageTemplate = `
Графік стабілізаційних відключень на {{.Date}}:

{{range .Msgs}} {{.}}
{{end}}
`

groupMessageTemplate = `Група {{.GroupNum}}:
  🟢 Заживлено:  {{range .On}} {{.From}} - {{.To}}; {{end}}
  🟡 Можливо заживлено: {{range .Maybe}} {{.From}} - {{.To}}; {{end}}
  🔴 Відключено: {{range .Off}} {{.From}} - {{.To}}; {{end}}
`
```

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

**Add new data field:**
1. Update structs in `dal/bolt.go`
2. Update parsing in `providers/chernivtsi.go`
3. Update templates in `notifications.go`

## Resources

- Telegram Bot API: https://core.telegram.org/bots/api
- goquery docs: https://pkg.go.dev/github.com/PuerkitoBio/goquery
- BoltDB: https://github.com/etcd-io/bbolt
- Schedule source: https://oblenergo.cv.ua/shutdowns/
