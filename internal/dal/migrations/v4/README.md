# Migration v4: Split Subscription Metadata from Notification State

## Date
2025-10-31

## Description
This migration separates subscription metadata from notification state into two distinct buckets. Previously, the `subscriptions` bucket stored both user subscription information (which groups they're interested in) and notification tracking data (what we last sent them). This violated the Single Responsibility Principle and made multi-group subscriptions difficult to implement.

**Purpose:**
- Separate concerns: subscription metadata vs. notification tracking
- Enable clean multi-group subscription support
- Allow notification history tracking per date
- Improve data model clarity and maintainability
- Prepare for future features (notification history, analytics)

**Impact:**
- Creates new `notifications` bucket
- Changes `Groups` field from `map[string]string` to `map[string]struct{}` (set semantics)
- Moves hash tracking to separate notification state records

## Schema Changes

### Before (v3)

**Single bucket with mixed concerns:**

```go
type Subscription struct {
    ChatID    int64             `json:"chat_id"`
    Groups    map[string]string `json:"groups"` // ← MIXED: group_id -> hash
    CreatedAt time.Time         `json:"created_at"`
}
```

**Bucket structure:**
```
subscriptions/
  <chatID> → {
    "chat_id": 123456789,
    "groups": {
      "5": "hash_abc123",    // ← Both subscription + notification state
      "7": "hash_def456"
    },
    "created_at": "2025-10-31T14:23:45Z"
  }
```

**Problems:**
- Can't tell when notification was sent
- Can't track notification history
- Hard to support multiple groups cleanly
- Mixed metadata and transient state

### After (v4)

**Two buckets with clear separation:**

```go
// Subscription stores only metadata
type Subscription struct {
    ChatID    int64              `json:"chat_id"`
    Groups    map[string]struct{} `json:"groups"` // ← Just group IDs (set)
    CreatedAt time.Time          `json:"created_at"`
}

// NotificationState tracks what we sent
type NotificationState struct {
    ChatID int64             `json:"chat_id"`
    Date   string            `json:"date"`   // "2024-10-31" (YYYY-MM-DD)
    SentAt time.Time         `json:"sent_at"`
    Hashes map[string]string `json:"hashes"` // group_id -> hash
}
```

**Bucket structure:**
```
subscriptions/
  <chatID> → {
    "chat_id": 123456789,
    "groups": {
      "5": {},           // ← Just group membership (set semantics)
      "7": {}
    },
    "created_at": "2025-10-31T14:23:45Z"
  }

notifications/
  <chatID>_<date> → {
    "chat_id": 123456789,
    "date": "2024-10-31",
    "sent_at": "2025-10-31T14:30:00Z",
    "hashes": {
      "5": "hash_abc123",    // ← Notification tracking only
      "7": "hash_def456"
    }
  }
```

**Key Format:** `<chatID>_<YYYY-MM-DD>`
**Example:** `123456789_2024-10-31`

**Benefits:**
- Clear separation of concerns
- Can track "when did we notify?"
- Supports notification history (one record per date)
- Set semantics for groups (no duplicate handling needed)
- Easy to add more dates in future

## Data Transformation

### For Existing Subscriptions

For each subscription in v3 format:

1. **Extract group numbers** from `Groups` map keys → convert to set (`map[string]struct{}`)
2. **Extract hashes** from `Groups` map values → create notification state
3. **Use today's date** for initial notification record
4. **Set SentAt** to migration time (conservative assumption)

### Example Transformation

**Before (v3):**
```json
{
  "chat_id": 123456789,
  "groups": {
    "5": "hash_abc123",
    "7": ""
  },
  "created_at": "2025-10-30T12:00:00Z"
}
```

**After (v4):**

*Subscription:*
```json
{
  "chat_id": 123456789,
  "groups": {
    "5": {},
    "7": {}
  },
  "created_at": "2025-10-30T12:00:00Z"
}
```

*Notification State (key: `123456789_2024-10-31`):*
```json
{
  "chat_id": 123456789,
  "date": "2024-10-31",
  "sent_at": "2025-10-31T14:30:00Z",
  "hashes": {
    "5": "hash_abc123"
  }
}
```

**Note:** Group "7" has empty hash, so not included in notification state (no notification sent yet).

### Edge Cases

1. **Empty groups map (new subscriber):**
   - Creates subscription with empty set
   - No notification state created (nothing sent yet)

2. **All hashes are empty:**
   - Creates subscription with groups
   - No notification state created (no notifications sent)

3. **Missing CreatedAt:**
   - Should not happen (v3 migration ensures it exists)
   - Migration will preserve whatever is there

4. **Empty subscriptions bucket:**
   - Migration completes successfully
   - Only creates `notifications` bucket

### Idempotency

The migration is **not perfectly idempotent** because:
- Running twice will overwrite notification state with migration time
- Original `SentAt` timestamps will be lost

**Mitigation:**
- Migration system tracks applied migrations
- v4 won't run twice under normal circumstances
- If manually rerun, only affects `SentAt` field (not critical)

### Retention Policy

**Current:** Keep all notification states (no cleanup implemented)

**Future:** May implement 7-day retention (configured in requirements)
- Cleanup will be manual or via separate process
- Data volume is minimal (~1KB per user per day)

## Rollback Strategy

### Option 1: Restore from Backup (Recommended)

**Before migration:**
```bash
# Create backup
cp data/sso-notifier.db data/sso-notifier.db.backup-pre-v4
```

**To rollback:**
```bash
# Stop application
# Restore backup
cp data/sso-notifier.db.backup-pre-v4 data/sso-notifier.db
# Restart application with v4 migration disabled
```

**Data loss:** None (complete restore)

### Option 2: Reverse Migration (Advanced)

**Not recommended** because:
- Would need to merge `notifications` bucket back into `subscriptions`
- Would lose `SentAt` information (can't preserve in v3 schema)
- Complex logic with edge cases
- Not worth the effort given backup strategy

**If absolutely necessary:**
1. Create v5 migration that reverses v4
2. Read from both buckets
3. Merge back into v3 format
4. Accept data loss for `SentAt` field

## Testing Notes

### Manual Testing Steps

```bash
# 1. Create test database with v3 schema
# 2. Add test subscriptions
# 3. Run migration
./bin/sso-notifier

# 4. Verify subscriptions bucket updated
# 5. Verify notifications bucket created
# 6. Verify key format: <chatID>_<date>
# 7. Verify hashes preserved correctly
```

### Test Cases

- [ ] Empty database (no subscriptions)
- [ ] Single subscription with one group
- [ ] Single subscription with multiple groups
- [ ] Multiple subscriptions
- [ ] Subscription with empty hash
- [ ] Subscription with all empty hashes
- [ ] Verify `created_at` preserved
- [ ] Verify notification key format
- [ ] Verify `sent_at` set to migration time
- [ ] Check notifications bucket exists

### Performance Considerations

- **Small databases (<1,000 users):** Milliseconds
- **Medium databases (<10,000 users):** <1 second
- **Large databases (>10,000 users):** Few seconds
- **Transaction safety:** All changes atomic

## Application Code Changes Required

### 1. `internal/dal/bolt.go`

Update types and add new methods:

```go
// Update Subscription type
type Subscription struct {
    ChatID    int64              `json:"chat_id"`
    Groups    map[string]struct{} `json:"groups"` // CHANGED
    CreatedAt time.Time          `json:"created_at"`
}

// Add NotificationState type
type NotificationState struct {
    ChatID int64             `json:"chat_id"`
    Date   string            `json:"date"`
    SentAt time.Time         `json:"sent_at"`
    Hashes map[string]string `json:"hashes"`
}

// Add bucket constant
const notificationsBucket = "notifications"

// Add helper function
func notificationKey(chatID int64, date string) string {
    return fmt.Sprintf("%d_%s", chatID, date)
}

// Add new methods
func (s *BoltDB) GetNotificationState(chatID int64, date string) (NotificationState, bool, error)
func (s *BoltDB) PutNotificationState(state NotificationState) error
func (s *BoltDB) DeleteNotificationStates(chatID int64) error

// Update existing method
func (s *BoltDB) PurgeSubscriptions(chatID int64) error {
    // Also delete from notifications bucket
}
```

### 2. `internal/service/notifications.go`

Update to use notification state:

```go
// Before: Read hash from subscription.Groups[groupNum]
// After: Read hash from NotificationState.Hashes[groupNum]

// Before: Update subscription.Groups[groupNum] = newHash
// After: Update NotificationState.Hashes[groupNum] = newHash, PutNotificationState()
```

### 3. `internal/service/subscriptions.go`

Update to use set semantics:

```go
func (s *Subscriptions) SubscribeToGroup(chatID int64, groupNum string) error {
    sub := dal.Subscription{
        ChatID: chatID,
        Groups: map[string]struct{}{
            groupNum: {}, // CHANGED: was map[string]string{groupNum: ""}
        },
        CreatedAt: time.Now(),
    }
    return s.store.PutSubscription(sub)
}
```

### 4. `internal/telegram/telegram.go`

Minimal changes to adapt to set:

```go
// Before: Check if key exists in map[string]string
// After: Check if key exists in map[string]struct{}

// No functional changes needed, just type adaptation
```

### 5. `internal/dal/migrations/migrations.go`

Register v4 migration:

```go
import v4 "github.com/Roma7-7-7/sso-notifier/internal/dal/migrations/v4"

func init() {
    registerMigration(v1.New())
    registerMigration(v2.New())
    registerMigration(v3.New())
    registerMigration(v4.New()) // ADD THIS
}
```

## Deployment Checklist

- [ ] Code reviewed and approved
- [ ] Database backed up before migration
- [ ] Migration tested on copy of production database
- [ ] All test cases passed
- [ ] Application code updated to use new schema
- [ ] Service layer updated (notifications.go, subscriptions.go)
- [ ] Telegram layer updated (minimal changes)
- [ ] Deployment scheduled
- [ ] Monitoring in place
- [ ] Rollback plan documented

## Success Criteria

Migration is successful when:
- ✅ `notifications` bucket created
- ✅ All subscriptions have `Groups` as `map[string]struct{}`
- ✅ Notification states created for subscriptions with hashes
- ✅ Notification keys follow `<chatID>_<YYYY-MM-DD>` format
- ✅ All hashes preserved correctly
- ✅ Application starts without errors
- ✅ Users receive notifications as before
- ✅ No data loss

## Related Migrations

- **v1:** Creates migrations bucket (bootstrap)
- **v2:** Creates shutdowns and subscriptions buckets
- **v3:** Adds CreatedAt to subscriptions
- **v4:** Splits subscription metadata from notification state (this migration)

---

**Status:** ✅ Ready for Review
**Dependencies:** v1, v2, and v3 must be applied first
**Breaking Changes:** Yes (schema change)
**Rollback:** Restore from backup only
