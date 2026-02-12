# Architecture & Code Structure

## Layered Architecture

```
Presentation Layer (Telegram Bot)
    ↓
Service Layer (Business Logic)
    ↓
Data Access Layer (BoltDB)
    ↓
External Provider Layer (HTML Scraping)
```

## Concurrency Model

Five concurrent goroutines (calendar sync runs only when configured):

1. **Main Thread**: Telegram bot event loop
2. **Refresh Thread**: Fetches schedule (configurable, default: 5 minutes)
3. **Notification Thread**: Checks for schedule updates (configurable, default: 5 minutes)
4. **Alerts Thread**: Checks for upcoming outages (configurable, default: 1 minute)
5. **Calendar Sync Thread**: Syncs power outage schedule (today + tomorrow) to Google Calendar at configurable interval (default: 15m). Started only when `CALENDAR_EMAIL` and `CALENDAR_CREDENTIALS_PATH` are set. Personal-only feature; not exposed to bot users.

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

3. **Goroutine Management & Lifecycle** (lines 80-95)
   - Spawns refresh schedule goroutine
   - Spawns notification goroutine
   - Spawns alerts goroutine (for upcoming outage notifications)
   - Uses context cancellation for graceful shutdown
   - Listens for SIGINT/SIGTERM signals
   - Waits for all goroutines to complete

4. **Interval Functions**
   - `refreshShutdowns()`: Fetches new schedule at configured interval
   - `notifyShutdownUpdates()`: Checks and sends notifications at configured interval
   - `notifyUpcomingShutdowns()`: Checks for upcoming outages and sends 10-minute advance alerts
   - Calendar sync (optional): When calendar env is set, syncs one group’s schedule to Google Calendar (delete-then-recreate)

**Configuration Struct:**
```go
type Config struct {
    Dev                      bool          // Development mode flag
    GroupsCount              int           // Number of groups (default: 12)
    DBPath                   string        // Database path
    RefreshShutdownsInterval time.Duration // Schedule fetch interval
    NotifyInterval           time.Duration // Notification check interval
    NotifyUpcomingInterval   time.Duration // Upcoming alerts check interval (default: 1m)
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
    ChatID   int64                  // Telegram chat ID
    Groups   map[string]string      // {"1": "hash123", "2": "hash456"}
    Settings map[string]interface{} // User preferences (e.g., "notify_off_10min": true)
}
```

**BoltDB Buckets:**
- `shutdowns`: Stores current schedule (single key: "table")
- `subscriptions`: Stores user subscriptions (key: chatID)
- `alerts`: Tracks sent 10-minute advance notifications (key: "{chatID}_{date}_{time}_{status}_{group}")

**Key Methods:**
- `GetShutdowns()` / `PutShutdowns()`: Schedule CRUD
- `GetSubscription()` / `PutSubscription()`: User subscription CRUD
- `GetAllSubscriptions()`: Fetch all users (for notifications)
- `PurgeSubscriptions()`: Remove user (when blocked)

### `/internal/providers/chernivtsi.go`

HTML scraper for https://oblenergo.cv.ua/shutdowns/

**Provider Structure:**

```go
type ChernivtsiProvider struct {
    baseURL string  // Configurable schedule URL
}

func NewChernivtsiProvider(baseURL string) *ChernivtsiProvider
```

**Public Methods:**
- `Shutdowns(ctx)` - Fetches current schedule, returns (schedule, nextDayAvailable, error)
- `ShutdownsNext(ctx)` - Fetches next day schedule by appending `?next` to base URL

**Parsing Logic:**

1. **Date Extraction** (line 68) - Selector: `div#gsv ul p`
2. **Time Periods** (lines 147-183) - Selector: `div > p u`
3. **Groups** (lines 121-144) - Selector: `ul > li[data-id]`
4. **Status Items** (lines 186-208) - Selector: `div[data-id='N']` for each group
   - Text mapping: `В` → OFF, `З` → ON, Other → MAYBE

### `/internal/service/shutdowns.go`

Service for refreshing schedule from external provider.

**Key Method: `Refresh()` (lines 44-85)**
- Uses mutex to prevent concurrent refreshes
- 1-minute timeout for HTTP request
- Fetches today's schedule, optionally tomorrow's if available

### `/internal/service/notifications.go`

Core notification logic with sophisticated time handling.

**Main Method: `NotifyShutdownUpdates()` (lines 49-78)**
1. Fetch current schedule from DB
2. Get all subscriptions
3. For each subscription, call `processSubscription()`

**Key Functions:**
- `cutByKyivTime()` (lines 157-169): Filters out past periods using `Europe/Kyiv` timezone
- `join()` (lines 133-154): Merges consecutive periods with same status

**Message Templates** - See `internal/service/TEMPLATES.md` for detailed documentation.

### `/internal/service/subscriptions.go`

Subscription management service with multi-group subscription support.

**Key Methods:**
1. `IsSubscribed(chatID)` - Check if user exists
2. `GetSubscribedGroups(chatID)` - Returns list of group numbers
3. `ToggleGroupSubscription(chatID, groupNum)` - Add or remove a group subscription
4. `SubscribeToGroup(chatID, groupNum)` - Add subscription to a group
5. `Unsubscribe(chatID)` - Remove all user data

### `/internal/service/alerts.go`

Service for sending 10-minute advance notifications for upcoming power outages.

**Main Method: `NotifyUpcomingShutdowns()` (lines 70-140)**
1. Check if within notification window (6 AM - 11 PM)
2. Calculate target time (now + 10 minutes)
3. Fetch current schedule from DB
4. For each group, find period containing target time
5. Check if period is start of outage/restoration
6. Send merged notifications and mark as sent

**Key Algorithms:**
- `findPeriodIndex()` (lines 220-245): Finds which period contains a given time
- `isOutageStart()` (lines 201-219): Detects start of a new status change

**Settings Integration:**
- `notify_off_10min` - Notify before OFF (default: false)
- `notify_maybe_10min` - Notify before MAYBE (default: false)
- `notify_on_10min` - Notify before ON (default: false)

### `/internal/telegram/telegram.go`

Telegram bot integration using telebot.v3 library.

**Bot Structure:**
```go
type Bot struct {
    svc     SubscriptionService
    bot     *tb.Bot
    markups *markups
    log     *slog.Logger
}
```

**Callback Routing Architecture:**
Uses centralized callback router pattern:
```go
b.bot.Handle(tb.OnCallback, b.handleCallbackRouter)
```

**Handlers:**
1. **StartHandler** - Shows main menu with personalized message
2. **ManageGroupsHandler** - Shows group selection interface with checkmarks
3. **ToggleGroupHandler** - Toggles subscription for selected group
4. **UnsubscribeHandler** - Removes all subscriptions

## Key Design Patterns

### 1. Repository Pattern
Services depend on interfaces, not concrete implementations:
```go
type ShutdownsStore interface {
    GetShutdowns() (dal.Shutdowns, bool, error)
    PutShutdowns(s dal.Shutdowns) error
}
```

### 2. Dependency Injection
All dependencies injected via constructors.

### 3. Constructor Pattern
Returns errors instead of panicking for better error handling.

### 4. Mutex for Thread Safety
All services use mutexes to prevent concurrent access.
