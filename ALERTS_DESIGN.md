# Alerts Feature - Design Document

**Status**: 🚧 In Progress
**Created**: 2025-11-05
**Target**: PR ready for review
**Note**: This is a temporary file - will be deleted once PR is merged

---

## Table of Contents

1. [Overview](#overview)
2. [User Stories](#user-stories)
3. [Architecture](#architecture)
4. [Implementation Phases](#implementation-phases)
5. [Current Progress](#current-progress)
6. [Testing Strategy](#testing-strategy)
7. [Edge Cases](#edge-cases)

---

## Overview

### Goal

Implement 10-minute advance notifications for upcoming power outages, allowing users to prepare for:
- Power OFF (confirmed outage)
- Power MAYBE (possible outage)
- Power ON (restoration) - **bonus feature**

### Key Features

- ✅ Configurable per-status notifications (OFF/MAYBE/ON)
- ✅ Smart detection of outage starts (no duplicate notifications)
- ✅ Merged messages for multiple groups
- ✅ Respects user sleep (6 AM - 11 PM only)
- ✅ Separate settings UI
- ✅ No race conditions with user actions

### User Experience Example

**Scenario**: User subscribed to groups 5 and 7, with "notify_off_10min" enabled.

**Schedule**:
```
Group 5:
  08:30-11:00: OFF
  18:00-20:00: OFF

Group 7:
  08:30-09:00: OFF
  18:30-19:00: MAYBE
```

**Notifications Sent**:

1. **At 8:20 AM**:
   ```
   ⚠️ Увага! Через 10 хвилин:

   Групи 5, 7:
   🔴 Відключення електроенергії о 08:30
   ```

2. **At 5:50 PM**:
   ```
   ⚠️ Увага! Через 10 хвилин:

   Група 5:
   🔴 Відключення електроенергії о 18:00
   ```

**No notification** at 8:40, 8:50, 9:00, etc. (continuation of same outage)

---

## User Stories

### As a user, I want to...

1. ✅ Receive notification 10 minutes before power goes OFF
2. ✅ Receive notification 10 minutes before power MAYBE goes off
3. ✅ Receive notification 10 minutes before power comes back ON (optional)
4. ✅ Enable/disable each notification type independently
5. ✅ Access settings via `/settings` command or main menu button
6. ✅ Receive single merged message if multiple groups have simultaneous outages
7. ✅ Not be woken up at night (6 AM - 11 PM window)

---

## Architecture

### Core Algorithm: "Start of Outage" Detection

**Key Insight**: Only notify at the **beginning** of an outage period, not for every 30-minute continuation.

#### Detection Logic

```go
// isOutageStart checks if the period at index i is the START of a new outage
func isOutageStart(items []dal.ShutdownGroupItem, index int, status dal.Status) bool {
    // Validate index
    if index < 0 || index >= len(items) {
        return false
    }

    currentStatus := items[index].Status

    // Check if current period matches desired status
    if currentStatus != status {
        return false
    }

    // First period of the day
    if index == 0 {
        return true
    }

    // Check previous period - if different status, this is a START
    previousStatus := items[index-1].Status
    return previousStatus != currentStatus
}
```

#### Example Walkthrough

**Schedule**:
```
Time    Status  Previous  IsStart(OFF)?  IsStart(MAYBE)?  Notify?
07:30   ON      -         No             No               -
08:00   MAYBE   ON        No             YES ✓            7:50 (MAYBE)
08:30   OFF     MAYBE     YES ✓          No               8:20 (OFF)
09:00   OFF     OFF       No             No               -
09:30   OFF     OFF       No             No               -
10:00   OFF     OFF       No             No               -
10:30   OFF     OFF       No             No               -
11:00   ON      OFF       No             No               -
17:00   MAYBE   ON        No             YES ✓            4:50 (MAYBE)
17:30   MAYBE   MAYBE     No             No               -
18:00   OFF     MAYBE     YES ✓          No               5:50 (OFF)
18:30   OFF     OFF       No             No               -
```

**Notifications Sent** (if all enabled):
- 7:50 AM - MAYBE at 8:00
- 8:20 AM - OFF at 8:30
- 4:50 PM - MAYBE at 5:00
- 5:50 PM - OFF at 6:00

### Data Model

#### Settings in Subscription

```go
type Subscription struct {
    ChatID   int64              `json:"chat_id"`
    Groups   map[string]string  `json:"groups"`
    Settings map[string]interface{} `json:"settings,omitempty"` // nil by default
}

// Settings keys:
// "notify_on_10min"     (bool) - Notify before power ON (default: false)
// "notify_off_10min"    (bool) - Notify before power OFF (default: false)
// "notify_maybe_10min"  (bool) - Notify before power MAYBE (default: false)
// "notification_window_minutes" (int) - Time window in minutes (default: 10, UI later)
```

**No migration needed** - `Settings` defaults to `nil`, backward compatible.

#### New Bucket: alerts

**Purpose**: Track sent notifications to prevent duplicates.

**Key Format**: `{chatID}_{date}_{startTime}_{status}_{group}`

**Value**: ISO 8601 timestamp when notification was sent.

**Examples**:
```
123456_20 жовтня_08:00_MAYBE_5 → "2025-10-20T07:50:00Z"
123456_20 жовтня_08:30_OFF_5   → "2025-10-20T08:20:00Z"
123456_20 жовтня_18:00_OFF_7   → "2025-10-20T17:50:00Z"
```

**Cleanup Strategy**: (Future) Periodically delete entries older than 24 hours.

### Migration v5

```go
package v5

import "go.etcd.io/bbolt"

type MigrationV5 struct{}

func (m *MigrationV5) Version() int {
    return 5
}

func (m *MigrationV5) Description() string {
    return "Create alerts bucket for tracking 10-minute advance alerts"
}

func (m *MigrationV5) Up(db *bbolt.DB) error {
    return db.Update(func(tx *bbolt.Tx) error {
        _, err := tx.CreateBucketIfNotExists([]byte("alerts"))
        return err
    })
}
```

### Notification Flow

```
┌─────────────────────────────────────────┐
│ Goroutine: notifyUpcomingShutdowns()    │
│ Runs every 1 minute                     │
└──────────────┬──────────────────────────┘
               │
               v
┌─────────────────────────────────────────┐
│ Get current time in Kyiv timezone       │
│ targetTime = now + 10 minutes           │
└──────────────┬──────────────────────────┘
               │
               v
┌─────────────────────────────────────────┐
│ Fetch schedule from DB                  │
└──────────────┬──────────────────────────┘
               │
               v
┌─────────────────────────────────────────┐
│ For each date in schedule:              │
│   For each group in schedule:           │
│     Find period matching targetTime     │
│     Check if isOutageStart()            │
└──────────────┬──────────────────────────┘
               │
               v
┌─────────────────────────────────────────┐
│ Get all subscriptions                   │
└──────────────┬──────────────────────────┘
               │
               v
┌─────────────────────────────────────────┐
│ For each subscription:                  │
│   Check user settings (enabled?)        │
│   Check if user subscribed to group     │
│   Check if not already notified         │
└──────────────┬──────────────────────────┘
               │
               v
┌─────────────────────────────────────────┐
│ Group notifications by user + status    │
│ Render merged message                   │
│ Send via Telegram                       │
└──────────────┬──────────────────────────┘
               │
               v
┌─────────────────────────────────────────┐
│ Mark as sent in alerts  │
│ bucket (separate transaction)           │
└─────────────────────────────────────────┘
```

### Message Templates

#### Single Group, Single Status

```
⚠️ Увага! Через 10 хвилин:

Група 5:
🔴 Відключення електроенергії о 14:00
```

#### Multiple Groups, Same Time

```
⚠️ Увага! Через 10 хвилин:

Групи 5, 7, 9:
🔴 Відключення електроенергії о 14:00
```

#### Multiple Groups, Different Times/Statuses

```
⚠️ Увага! Через 10 хвилин:

Група 5:
🔴 Відключення електроенергії о 14:00

Група 7:
🟡 Можливе відключення електроенергії о 14:30
```

#### Power Restoration (ON status)

```
⚡ Гарні новини! Через 10 хвилин:

Група 5:
🟢 Відновлення електроенергії о 14:00
```

### Settings UI

#### Main Menu Changes

**Add button**: `⚙️ Налаштування`

Placement: Below "Керувати групами" / "Підписатись на оновлення"

#### Settings Screen

```
⚙️ Налаштування сповіщень

Попереджати за 10 хвилин до:

[✅ Відключення]
[❌ Можливих відключень]
[❌ Відновлення]

ℹ️ Сповіщення надсилаються з 6:00 до 23:00

[◀️ Назад]
```

**Inline Keyboard**:
- `toggle_notify_off` - Toggle OFF notifications
- `toggle_notify_maybe` - Toggle MAYBE notifications
- `toggle_notify_on` - Toggle ON notifications
- `back_from_settings` - Return to main menu

#### Settings Handler Flow

1. User clicks "⚙️ Налаштування" → `SettingsHandler()`
2. Fetch current settings from subscription
3. Render settings menu with checkmarks
4. User clicks toggle → `ToggleSettingHandler(key)`
5. Update subscription settings (map operation)
6. Refresh settings menu with new state

---

## Implementation Phases

### Phase 1: Data Layer ✅

- [x] **Migration v5**: Create `alerts` bucket
  - File: `internal/dal/migrations/v5/migration.go`
  - README: `internal/dal/migrations/v5/README.md`
  - Update: `internal/dal/migrations/README.md` (latest schema)
  - Register: `internal/dal/migrations/migrations.go`

- [x] **DAL Methods**: Add to `internal/dal/bolt.go`
  - `GetAlert(key AlertKey) (time.Time, bool, error)`
  - `PutAlert(key AlertKey, sentAt time.Time) error`
  - `DeleteAlert(key AlertKey) error`
  - `DeleteAlerts(chatID int64) error`
  - `BuildAlertKey()` helper function
  - `AlertKey` type for compile-time safety

- [x] **Settings Helpers**: Add to `internal/dal/bolt.go`
  - `GetBoolSetting(settings, key SettingKey, defaultBool bool) bool`
  - `GetIntSetting(settings, key SettingKey, defaultInt int) int`
  - `GetStringSetting(settings, key SettingKey, defaultString string) string`
  - `SettingKey` type with constants for type safety

- [x] **Additional**:
  - Updated `PurgeSubscriptions()` to delete alerts
  - Added `DB()` method for migrations access

**Files to modify**:
- `internal/dal/bolt.go`
- `internal/dal/migrations/migrations.go`
- `internal/dal/migrations/README.md`

**Files to create**:
- `internal/dal/migrations/v5/migration.go`
- `internal/dal/migrations/v5/README.md`

---

### Phase 2: Service Layer ✅

- [x] **New Service**: `internal/service/alerts.go`
  - `NotifyUpcomingShutdowns(ctx context.Context) error`
  - `isOutageStart(items []dal.Status, index int, status dal.Status) bool`
  - `findPeriodIndex(periods []dal.Period, timeStr string) int`
  - `isWithinNotificationWindow(hour int) bool` (6 AM - 11 PM check)
  - `renderUpcomingMessage(alerts []PendingAlert) string` (includes grouping logic)
  - `processSubscriptionAlert()` (processes alerts per user)
  - `getSettingKeyForStatus()`, `getEmojiForStatus()`, `getLabelForStatus()` helpers

- [x] **Subscription Service**: Extend `internal/service/subscriptions.go`
  - `GetSettings(chatID int64) (map[dal.SettingKey]interface{}, error)`
  - `ToggleSetting(chatID int64, key dal.SettingKey, defaultValue bool) error`
  - `GetBoolSetting(chatID int64, key dal.SettingKey, defaultBool bool) (bool, error)`

- [x] **Data Model**: Updated `internal/dal/subscriptions.go`
  - Added `Settings map[SettingKey]interface{}` to Subscription struct

**Files created**:
- `internal/service/alerts.go`

**Files modified**:
- `internal/service/subscriptions.go`
- `internal/dal/subscriptions.go`

---

### Phase 3: Telegram Bot UI ✅

- [x] **Settings Handlers**: Add to `internal/telegram/telegram.go`
  - `SettingsHandler(c tb.Context) error`
  - `ToggleSettingHandler(c tb.Context, key string) error`
  - Register in callback router:
    - `"settings"` → `SettingsHandler`
    - `"toggle_notify_off"` → `ToggleSettingHandler(dal.SettingNotifyOff)`
    - `"toggle_notify_maybe"` → `ToggleSettingHandler(dal.SettingNotifyMaybe)`
    - `"toggle_notify_on"` → `ToggleSettingHandler(dal.SettingNotifyOn)`
    - `"back_from_settings"` → `StartHandler`

- [x] **Markup Updates**: Extend `internal/telegram/telegram.go` (markups section)
  - Update `subscribed` main menu: Add "⚙️ Налаштування" button
  - Update `unsubscribed` main menu: ~~Add "⚙️ Налаштування" button~~ (decided NOT to show - settings only for subscribed users)
  - Create `buildSettingsMarkup(settings map[dal.SettingKey]interface{}) *tb.ReplyMarkup`

- [x] **Command Registration**: Add to `Start()` method
  - `/settings` → `SettingsHandler`
  - Added subscription check to prevent unsubscribed users from accessing settings

**Files modified**:
- `internal/telegram/telegram.go`

---

### Phase 4: Scheduler Integration ✅

- [x] **Main Entry Point**: Update `cmd/bot/main.go`
  - Add `NotifyUpcomingInterval` to `Config` struct (default: 1 minute)
  - Wire up `Alerts` service
  - Add goroutine: `notifyUpcomingShutdowns()`
  - Pass cancellation context

- [x] **Interval Function**: Add to `cmd/bot/main.go`
  ```go
  func notifyUpcomingShutdowns(
      ctx context.Context,
      svc *service.Alerts,
      delay time.Duration,
      log *slog.Logger,
  )
  ```

**Files modified**:
- `cmd/bot/main.go`
- `internal/telegram/config.go` (added `NotifyUpcomingInterval` field)

---

### Phase 5: Message Templates ✅ / ❌

- [ ] **Create Template File**: `internal/service/upcoming_messages.go`
  - Define `upcomingMessageTemplate` (text/template)
  - Implement `renderUpcomingNotification(data UpcomingData) (string, error)`
  - Support multiple groups with same/different times
  - Support ON/OFF/MAYBE status emojis and labels

- [ ] **Update TEMPLATES.md**: Document new template
  - Add section: "Upcoming Notification Template"
  - Include examples for all scenarios
  - Document data structure

**Files to create**:
- `internal/service/upcoming_messages.go`

**Files to modify**:
- `internal/service/TEMPLATES.md`

---

### Phase 6: Testing & Polish ✅ / ❌

- [ ] **Manual Testing**
  - Subscribe to multiple groups
  - Enable all notification types
  - Mock time progression (or wait for real schedule)
  - Verify notifications sent at correct times
  - Verify no duplicates
  - Test settings toggles

- [ ] **Edge Cases**
  - Test 6 AM boundary (no notification at 5:50 AM for 6:00 AM event)
  - Test 11 PM boundary (no notification at 10:55 PM for 11:05 PM event)
  - Test first period of the day (00:00) at 11:50 PM → no notification
  - Test multiple dates in schedule (if applicable)
  - Test user with no settings (all defaults to false)

- [ ] **Error Handling**
  - Telegram API failure during notification send
  - Database write failure when marking as sent
  - Malformed schedule data

- [ ] **Logging Review**
  - Ensure structured logs for debugging
  - Log: "skipped duplicate notification", "sent notification", "user setting disabled"

- [ ] **Update Documentation**
  - Update `CLAUDE.md` with new goroutine and service
  - Document settings structure
  - Document notification window (6 AM - 11 PM)
  - Update architecture diagrams

**Files to modify**:
- `CLAUDE.md`

---

## Current Progress

### Completed ✅

- [x] Design document created
- [x] Phase 1: Data Layer (Migration v5, DAL methods, type safety)
- [x] Phase 2: Service Layer (Alerts service, subscription settings methods)
- [x] Phase 3: Telegram Bot UI (Settings handlers, markups, subscription checks)
- [x] Phase 4: Scheduler Integration (Goroutine, config, lifecycle management)

### In Progress 🚧

- [ ] Phase 5: Message Templates

### Blocked 🚫

- None

---

## Testing Strategy

### Unit Testing Candidates

1. **`isOutageStart()`**
   - Test first period of day
   - Test status transition (ON → OFF)
   - Test status continuation (OFF → OFF)
   - Test invalid index

2. **`findPeriodIndex()`**
   - Test exact match
   - Test no match
   - Test edge cases (00:00, 23:30)

3. **`isWithinNotificationWindow()`**
   - Test 6 AM (in window)
   - Test 5 AM (out of window)
   - Test 11 PM (in window)
   - Test midnight (out of window)

4. **`buildNotificationKey()`**
   - Test key format
   - Test special characters in date

### Integration Testing

1. **End-to-End Flow**
   - Mock schedule with known outages
   - Mock current time
   - Run `NotifyUpcomingShutdowns()`
   - Verify correct notifications queued
   - Verify deduplication works

2. **Settings Flow**
   - Toggle settings via UI
   - Verify stored in DB
   - Verify affects notification logic

### Manual Testing Checklist

- [ ] Notification sent 10 minutes before OFF
- [ ] Notification sent 10 minutes before MAYBE
- [ ] Notification sent 10 minutes before ON
- [ ] No duplicate notifications for same outage
- [ ] Multiple groups merged into single message
- [ ] Settings toggles work correctly
- [ ] Settings persist after bot restart
- [ ] 6 AM - 11 PM window respected
- [ ] User with no settings doesn't get notifications
- [ ] Telegram errors handled gracefully

---

## Edge Cases

### 1. Notification Window (6 AM - 11 PM)

**Rule**: Only send notifications for outages/restorations between 6:00 and 23:00.

**Implementation**:
```go
func isWithinNotificationWindow(hour int) bool {
    return hour >= 6 && hour < 23
}
```

**Examples**:
- ✅ 8:30 outage → notify at 8:20 (hour=8, in window)
- ❌ 5:30 outage → no notification at 5:20 (hour=5, out of window)
- ✅ 23:00 outage → notify at 22:50 (hour=23, in window)
- ❌ 23:30 outage → no notification at 23:20 (hour=23, out of window)

**Note**: Display in settings UI: "ℹ️ Сповіщення надсилаються з 6:00 до 23:00"

### 2. Schedule Granularity vs Check Frequency

**Scenario**: Schedule has 30-minute periods, but we check every 1 minute.

**Problem**: At 8:20, 8:21, 8:22... we all match "8:30" period.

**Solution**: Deduplication via `alerts` bucket. Once key exists, skip notification.

### 3. Multiple Dates in Schedule

**Current behavior**: `dal.Shutdowns` has `map[string]ShutdownDate` where key is date.

**Decision**: For now, only check today's schedule. Future: extend to tomorrow if close to midnight.

**Rationale**: Notification window ends at 11 PM, so no need to check next day.

### 4. User Changes Settings During Notification Processing

**Problem**: User disables "notify_off_10min" while goroutine is processing.

**Solution**: Read settings at start of processing for each user. Eventual consistency is acceptable.

### 5. Telegram API Failure

**Current behavior** (from existing code): Purge subscription if user blocked bot.

**For upcoming notifications**: Log error, skip user, continue to next. Don't mark as sent if failed.

### 6. First Period of Day (00:00)

**Scenario**: Schedule starts at 00:00 with OFF status.

**Question**: Should we notify at 23:50 previous day?

**Answer**: No - notification window ends at 11 PM, so 23:50 is out of window.

### 7. Schedule Not Yet Published

**Scenario**: Notification goroutine runs but `dal.Shutdowns` is empty.

**Solution**: Log "no schedule available", skip iteration, continue.

### 8. User Subscribed to Group But Group Missing from Schedule

**Scenario**: User subscribed to group 12, but schedule only has groups 1-11.

**Solution**: Skip group, log warning, continue.

---

## Open Questions

### 1. Notification Window Setting (Future)

**Question**: Should notification window be user-configurable (e.g., 7 AM - 10 PM)?

**Decision**: Not in initial implementation. Add to `Settings` structure for future:
- `"notification_start_hour"` (int, default: 6)
- `"notification_end_hour"` (int, default: 23)

### 2. Notification Time Setting (Future)

**Question**: Should "10 minutes" be configurable (5/10/15 minutes)?

**Decision**: Add to `Settings` structure now, implement UI later:
- `"notification_window_minutes"` (int, default: 10)

### 3. Cleanup Strategy

**Question**: When to delete old entries from `alerts` bucket?

**Options**:
- A. Periodic goroutine (every 24 hours)
- B. On-access cleanup (delete if > 24 hours old)
- C. Never (entries are small)

**Decision**: Defer to later. Not critical for initial implementation.

---

## Success Criteria

### Functionality

- [x] Notifications sent 10 minutes before outage starts (logic implemented)
- [x] No duplicate notifications for same outage (alerts bucket deduplication)
- [x] Settings UI works correctly (Phase 3 complete)
- [x] 6 AM - 11 PM window enforced (isWithinNotificationWindow)
- [x] Multiple groups merged into single message (renderUpcomingMessage grouping)

### Code Quality

- [x] All new code follows existing patterns
- [x] Structured logging throughout
- [x] Error handling consistent with existing code
- [x] No race conditions (mutex in Alerts service)

### Documentation

- [x] `CLAUDE.md` updated (added comment style guidelines)
- [ ] `TEMPLATES.md` updated (Phase 5)
- [x] Migration v5 README complete
- [ ] This design doc serves as PR description (at end)

---

## Deployment Checklist

- [ ] Migration v5 tested on production DB snapshot
- [ ] Binary deployed to EC2
- [ ] Verify goroutine starts
- [ ] Monitor logs for errors
- [ ] Test with real user account
- [ ] Delete this design doc after PR merge

---

## Notes

- All timestamps in Kyiv timezone (`Europe/Kyiv`)
- Settings default to `false` (opt-in, not opt-out)
- Separate bucket prevents race conditions with user actions
- No retries for failed notifications (same as existing behavior)
- Notification window (6 AM - 11 PM) avoids sleep disruption
