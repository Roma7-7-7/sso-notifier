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
2. **Refresh Thread**: Fetches schedule every 5 minutes
3. **Notification Thread**: Checks for updates every 5 seconds

## Code Structure

### `/cmd/bot/main.go`

Entry point with three main responsibilities:

1. **Initialization** (lines 19-32)
   - Creates BoltDB store at `data/app.db`
   - Initializes logger (JSON for prod, text for dev)
   - Creates Telegram bot builder
   - Wires up services

2. **Goroutine Management** (lines 34-44)
   - Spawns refresh schedule goroutine
   - Spawns notification goroutine
   - Both use sync.WaitGroup for graceful shutdown

3. **Interval Functions**
   - `refreshShutdowns()` (line 53): Fetches new schedule every 5 minutes
   - `notifyShutdownUpdates()` (line 80): Checks and sends notifications every 5 seconds

**Key Constants:**
- `refreshTableInterval = 5 * time.Minute`
- `notifyUpdatesInterval = 5 * time.Second`

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
type SSOBot struct {
    bot     *tb.Bot
    markups *markups  // Inline keyboard layouts
    subscriptionService SubscriptionService
}
```

**Handlers:**

1. **StartHandler** (lines 62-73)
   - Shows main menu
   - Different markup for subscribed/unsubscribed users

2. **ChooseGroupHandler** (lines 75-77)
   - Shows group selection (1-12 buttons)

3. **SetGroupHandler** (lines 79-89)
   - Subscribes user to selected group
   - Returns success message

4. **UnsubscribeHandler** (lines 91-97)
   - Removes all subscriptions
   - Shows unsubscribed state

**Markup Generation** (lines 173-222):

Creates inline keyboards:
- Main menu: Subscribe/Unsubscribe buttons
- Group selection: 12 numbered buttons (5 per row)
- Navigation: Back button

**Message Sender** (lines 245-263):

Handles Telegram API errors:
- Catches `ErrBlockedByUser`
- Calls `blockedHandler` to purge user data
- Prevents errors for blocked users

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

1. `refreshShutdowns()` runs every 5 minutes
2. Fetches HTML from oblenergo.cv.ua
3. Parses and stores in BoltDB
4. `notifyShutdownUpdates()` runs every 5 seconds
5. Detects hash change for group 5
6. Finds all subscriptions with group 5
7. Renders message with emoji indicators
8. Sends to each subscriber
9. Updates subscription hashes

### User Blocks Bot

1. Bot tries to send message
2. Telegram API returns `ErrBlockedByUser`
3. `messageSender.SendMessage()` catches error
4. Calls `blockedHandler(chatID)`
5. `PurgeSubscriptions()` removes all user data
6. No error logged (graceful handling)

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

### 3. Builder Pattern

Telegram bot uses builder for configuration:
```go
bb := telegram.NewBotBuilder()
sender := bb.Sender(purgeSubscriber(store))
bot := bb.Build(subscriptionsSvc)
```

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

### Environment Variables

- `TOKEN` (required): Telegram bot token
- `ENV` (optional): Set to "dev" for text logging

### Timeouts

- HTTP request: 1 minute (line 39, shutdowns.go)
- Refresh interval: 5 minutes (line 16, main.go)
- Notification check: 5 seconds (line 17, main.go)

### Telegram Constants

- `GroupsCount = 12` (line 16, telegram.go)

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
