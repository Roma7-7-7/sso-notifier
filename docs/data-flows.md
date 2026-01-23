# Data Flow Examples

## New User Subscribes to Multiple Groups

1. User sends `/start` → `StartHandler`
2. Bot shows "Підписатись на оновлення" button
3. User clicks → routed to `ManageGroupsHandler` via callback router
4. Bot shows groups 1-12 (no checkmarks yet)
5. User clicks "5" → routed to `ToggleGroupHandler("5")` via callback router
6. Service creates subscription: `{ChatID: 123, Groups: {"5": struct{}{}}}`
7. Bot shows updated view with "5 ✅" and feedback message
8. User clicks "7" → `ToggleGroupHandler("7")`
9. Service updates subscription: `{ChatID: 123, Groups: {"5": struct{}{}, "7": struct{}{}}}`
10. Bot shows both "5 ✅" and "7 ✅"
11. User can toggle any group to remove it
12. User clicks "Назад" to return to main menu showing "Ви підписані на групи: 5, 7"

## Schedule Update Notification

1. `refreshShutdowns()` runs at configured interval (default: 5 minutes)
2. Fetches HTML from oblenergo.cv.ua
3. Parses and stores in BoltDB
4. `notifyShutdownUpdates()` runs at configured interval (default: 5 minutes)
5. Detects hash change for group 5
6. Finds all subscriptions with group 5
7. Renders message with emoji indicators
8. Sends via separate Telegram client to each subscriber
9. Updates subscription hashes

## Upcoming Outage Alert

1. `notifyUpcomingShutdowns()` runs at configured interval (default: 1 minute)
2. Checks if within notification window (6 AM - 11 PM)
3. Calculates target time (now + 10 minutes), e.g., 8:20 → checks for 8:30
4. Fetches schedule from DB
5. For each group, finds period containing target time using `findPeriodIndex()`
6. Checks if period is start of outage using `isOutageStart()`
7. Gets all subscriptions and filters by:
   - User subscribed to group
   - User enabled notification type (OFF/MAYBE/ON)
   - Alert not already sent (checks alerts bucket)
8. Groups alerts by user, status, and time
9. Renders merged message (e.g., "Групи 5, 7: 🔴 Відключення об 08:30")
10. Sends via Telegram client
11. Marks as sent in alerts bucket using period start time for deduplication

### Example Timeline

- 8:00 AM: Schedule shows group 5 OFF from 8:30-11:00
- 8:20 AM: Goroutine runs, finds period 8:30-9:00 contains target time 8:30
- 8:20 AM: Detects this is start of OFF (previous period was ON)
- 8:20 AM: Sends notification "⚠️ Увага! Через 10 хвилин: Група 5: 🔴 Відключення об 08:30"
- 8:21 AM: Goroutine runs again, finds same period, but alert key already exists → skipped
- 8:30 AM: Power actually goes off

## User Blocks Bot

1. External Telegram client tries to send notification
2. Telegram API returns "Forbidden: bot was blocked by the user"
3. Client handles error and purges subscription
4. User data removed from database
5. No further messages sent to that user
