# Migration v5: Create Alerts Tracking Bucket

## Date
2025-11-05

## Description
This migration creates the `alerts` bucket for tracking 10-minute advance alert notifications. This new feature notifies users 10 minutes before a power outage/restoration occurs, giving them time to prepare.

**Purpose:**
- Enable 10-minute advance notifications for power OFF, MAYBE, and ON events
- Prevent duplicate notifications for the same outage start time
- Track which notifications have been sent to each user
- Support notification deduplication across multiple check cycles

**What This Is NOT:**
This bucket is separate from the existing `notifications` bucket (created in v4):
- `notifications` bucket: Tracks schedule change notifications (when the published schedule changes)
- `alerts` bucket: Tracks advance alert notifications (10 minutes before an event)

## Schema Changes

### New Bucket: alerts

**Purpose:** Track sent advance notifications to prevent duplicates.

**Key Format:** `{chatID}_{date}_{startTime}_{status}_{group}`

**Value:** ISO 8601 timestamp when notification was sent.

**Examples:**
```
123456_20 6>2B=O_08:00_MAYBE_5 � "2025-10-20T07:50:00Z"
123456_20 6>2B=O_08:30_OFF_5   � "2025-10-20T08:20:00Z"
123456_20 6>2B=O_18:00_OFF_7   � "2025-10-20T17:50:00Z"
```

### Key Components

- `{chatID}`: Telegram chat ID (e.g., `123456`)
- `{date}`: Schedule date in Ukrainian format (e.g., `20 6>2B=O`)
- `{startTime}`: Start time of the outage (e.g., `08:30`)
- `{status}`: Status type (`OFF`, `MAYBE`, `ON`)
- `{group}`: Group number (e.g., `5`)

### Bucket Structure

```
alerts/
  123456_20 6>2B=O_08:00_MAYBE_5 � "2025-10-20T07:50:00Z"
  123456_20 6>2B=O_08:30_OFF_5   � "2025-10-20T08:20:00Z"
  123456_20 6>2B=O_18:00_OFF_7   � "2025-10-20T17:50:00Z"
  789012_20 6>2B=O_08:30_OFF_5   � "2025-10-20T08:20:00Z"
```

## Data Transformation

### For New Installations

- Bucket is created empty
- Entries added as notifications are sent
- No existing data to migrate

### For Existing Installations

- Bucket is created empty
- No migration of existing data (feature is brand new)
- First run will start tracking notifications from scratch
- Users may receive duplicate notifications for already-known outages on first day (acceptable)

### Cleanup Strategy

**Current:** No automatic cleanup implemented.

**Future:** May implement cleanup of entries older than 24 hours to prevent unbounded growth.

**Storage Impact:**
- ~100 bytes per entry
- Max ~50 entries per user per day (worst case)
- Max ~5KB per user per day
- With 1000 users: ~5MB per day
- Negligible without cleanup for months

## Idempotency

Migration is **perfectly idempotent**:
- `CreateBucketIfNotExists` is safe to call multiple times
- No data transformation required
- No existing entries affected
- Can be run multiple times without side effects

## Rollback Strategy

### Option 1: Do Nothing (Recommended)

**Why:** The bucket is harmless if left alone.
- Application code can be reverted without touching database
- Empty bucket has no impact on existing functionality
- No performance or storage concerns

### Option 2: Delete Bucket (If Needed)

**Manual deletion:**
```go
db.Update(func(tx *bbolt.Tx) error {
    return tx.DeleteBucket([]byte("alerts"))
})
```

**Note:** Only needed if you want a completely clean database. Not recommended unless bucket causes issues.

### Option 3: Reverse Migration (Advanced)

Create v6 migration that deletes the bucket:
```go
func (m *MigrationV6) Up(db *bbolt.DB) error {
    return db.Update(func(tx *bbolt.Tx) error {
        // Ignore error if bucket doesn't exist
        _ = tx.DeleteBucket([]byte("alerts"))
        return nil
    })
}
```

## Testing Notes

### Manual Testing Steps

```bash
# 1. Run application with migration
./bin/sso-notifier

# 2. Check logs for successful migration
# Should see: "Migration applied successfully version=5"

# 3. Verify bucket created (optional, using bolt browser or CLI)
# Bucket "alerts" should exist

# 4. Enable upcoming notifications in settings
# Use Telegram bot: /settings � toggle notifications

# 5. Wait for scheduled notification time
# Verify notification sent and entry recorded in bucket
```

### Test Cases

- [x] Empty database (fresh install) -  Bucket created successfully
- [x] Database with existing subscriptions -  Bucket created, no impact on subscriptions
- [x] Run migration twice -  Idempotent, no errors
- [ ] First notification sent - Verify entry recorded
- [ ] Duplicate check - Same notification should not be sent twice
- [ ] Multiple users - Each user tracked separately

### Performance Considerations

- **Migration time:** Instant (<1ms) - only creates empty bucket
- **Runtime impact:** Minimal - simple key lookups
- **Storage impact:** Negligible - ~5KB per user per day
- **No indexes needed:** Key-based lookups are fast

## Application Code Changes Required

After this migration is deployed, the following changes are needed:

### 1. `internal/dal/bolt.go`

Add bucket constant and methods:

```go
const alertsBucket = "alerts"

// GetAlert checks if alert was already sent
func (s *BoltDB) GetAlert(key string) (time.Time, bool, error)

// PutAlert records that alert was sent
func (s *BoltDB) PutAlert(key string, sentAt time.Time) error

// DeleteAlert removes an alert record
func (s *BoltDB) DeleteAlert(key string) error

// Helper to build key
func BuildAlertKey(chatID int64, date, time, status, group string) string
```

### 2. `internal/service/alerts.go`

New service implementing the feature:

```go
type UpcomingNotifications struct {
    shutdownsStore     ShutdownsStore
    subscriptionsStore SubscriptionsStore
    notificationsStore UpcomingNotificationsStore
    telegram           TelegramClient
    log                *slog.Logger
}

func (s *UpcomingNotifications) NotifyUpcomingShutdowns(ctx context.Context) error
```

### 3. `cmd/bot/main.go`

Add goroutine for checking upcoming notifications:

```go
go notifyUpcomingShutdowns(
    ctx,
    alertsSvc,
    time.Minute, // Check every minute
    log,
)
```

### 4. `internal/dal/migrations/migrations.go`

Register v5 migration:

```go
import v5 "github.com/Roma7-7-7/sso-notifier/internal/dal/migrations/v5"

func init() {
    registerMigration(v1.New())
    registerMigration(v2.New())
    registerMigration(v3.New())
    registerMigration(v4.New())
    registerMigration(v5.New()) // ADD THIS
}
```

## Deployment Checklist

- [ ] Code reviewed and approved
- [ ] Database backed up (precautionary, migration is safe)
- [ ] Migration tested on development environment
- [ ] Service implementation complete
- [ ] Settings UI implemented
- [ ] Manual testing completed
- [ ] Documentation updated (CLAUDE.md)
- [ ] Deployment scheduled
- [ ] Monitoring configured

## Success Criteria

Migration is successful when:
-  `alerts` bucket created
-  Application starts without errors
-  No impact on existing functionality
-  First notification sent successfully
-  Duplicate prevention works
-  Multiple users tracked separately

## Feature Requirements

This migration supports the following feature requirements:

**Notification Window:** 6:00 AM - 11:00 PM
- Notifications only sent during active hours
- Respects user sleep schedule
- No midnight edge cases

**Notification Types:**
- OFF (=4) - Power will be turned off
- MAYBE (=�) - Power might be turned off
- ON (=�) - Power will be restored

**User Settings:**
- `notify_off_10min` (default: false)
- `notify_maybe_10min` (default: false)
- `notify_on_10min` (default: false)

**Smart Deduplication:**
- Only notify at start of outage
- No repeated notifications for same outage period
- Handle multiple groups in single message

## Related Migrations

- **v1:** Creates migrations bucket (bootstrap)
- **v2:** Creates shutdowns and subscriptions buckets
- **v3:** Adds CreatedAt to subscriptions
- **v4:** Splits subscription metadata from notification state
- **v5:** Creates alerts bucket (this migration)

## Related Features

**Depends on:**
- v4 migration (subscription structure)
- Settings support in subscription model

**Enables:**
- 10-minute advance notifications
- User-configurable notification preferences
- Multi-group notification merging
- Time-window notification control

---

**Status:**  Ready for Implementation
**Dependencies:** v1, v2, v3, v4 must be applied first
**Breaking Changes:** None (additive only)
**Rollback:** Safe to leave bucket in place if feature rolled back
