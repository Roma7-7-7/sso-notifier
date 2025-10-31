# Migration v3: Add CreatedAt Timestamp to Subscriptions

## Date
2025-10-31

## Description
This migration adds a `CreatedAt` timestamp field to all user subscriptions. This field tracks when each user first subscribed to the notification service.

**Purpose:**
- Enable analytics on user acquisition over time
- Allow filtering/sorting subscriptions by creation date
- Provide audit trail for subscription creation

**Status:** Prepared but not yet enabled. Will be activated in a future PR.

## Schema Changes

### Before (v2)
```go
type Subscription struct {
    ChatID int64             `json:"chat_id"`
    Groups map[string]string `json:"groups"` // group_id -> schedule_hash
}
```

**Example data:**
```json
{
  "chat_id": 123456789,
  "groups": {
    "5": "hash_abc123"
  }
}
```

### After (v3)
```go
type Subscription struct {
    ChatID    int64             `json:"chat_id"`
    Groups    map[string]string `json:"groups"`
    CreatedAt time.Time         `json:"created_at"` // NEW FIELD
}
```

**Example data:**
```json
{
  "chat_id": 123456789,
  "groups": {
    "5": "hash_abc123"
  },
  "created_at": "2025-10-31T14:23:45Z"
}
```

## Data Transformation

### For Existing Subscriptions
All existing subscriptions will have `CreatedAt` set to the migration execution time. This means:
- We cannot know the actual subscription date for existing users
- All existing users will have the same `CreatedAt` timestamp (when migration runs)
- This is acceptable as it provides a baseline for future analytics

### For New Subscriptions
After this migration, all new subscriptions created through the application will have `CreatedAt` set to the actual subscription time.

### Idempotency
The migration checks if a subscription already has a non-zero `CreatedAt` value before migrating it. This ensures the migration can be safely run multiple times without overwriting already-migrated data.

### Edge Cases
- **Empty subscriptions bucket:** If no subscriptions exist, the migration completes successfully without any action
- **Already migrated:** If a subscription already has `CreatedAt`, it is skipped
- **Invalid data:** If a subscription cannot be unmarshaled, the migration fails and rolls back the transaction

## Rollback Strategy

### Option 1: Restore from Backup (Recommended)
1. Stop the application
2. Restore database from pre-migration backup
3. Remove v3 migration from code if needed
4. Restart application

### Option 2: Manual Field Removal (Advanced)
If you need to rollback without losing new data:
1. Stop the application
2. Create a reverse migration (v4) that removes the `CreatedAt` field
3. Restart application

**Example reverse migration logic:**
```go
// Read all subscriptions as V3
// Transform to V2 (drop CreatedAt field)
// Write back as V2
```

## Testing Notes

### Test Cases
1. **Empty database:** Migration should succeed with no errors
2. **Database with subscriptions:** All subscriptions should have `CreatedAt` added
3. **Run twice:** Second run should skip already-migrated subscriptions
4. **New subscription after migration:** Should have actual creation time, not migration time

### Manual Testing
```bash
# Before migration
$ boltbrowser data/sso-notifier.db
# Check subscriptions bucket - should NOT have created_at field

# Run application with migration
$ ./bin/sso-notifier

# After migration
$ boltbrowser data/sso-notifier.db
# Check subscriptions bucket - should have created_at field
```

### Performance Considerations
- **Small databases (<10,000 users):** Migration completes in milliseconds
- **Large databases (>100,000 users):** May take several seconds
- **Downtime:** Application should not start until migration completes
- **Transaction safety:** All changes rolled back if migration fails

## Application Code Changes Required

After this migration is deployed, update the following files:

### 1. `internal/dal/bolt.go`
Update the `Subscription` struct to include `CreatedAt`:
```go
type Subscription struct {
    ChatID    int64             `json:"chat_id"`
    Groups    map[string]string `json:"groups"`
    CreatedAt time.Time         `json:"created_at"` // ADD THIS
}
```

### 2. `internal/service/subscriptions.go`
Update subscription creation to set `CreatedAt`:
```go
func (s *Subscriptions) SubscribeToGroup(chatID int64, groupNum string) error {
    sub := dal.Subscription{
        ChatID:    chatID,
        Groups:    map[string]string{groupNum: ""},
        CreatedAt: time.Now(), // ADD THIS
    }
    return s.store.PutSubscription(sub)
}
```

### 3. `internal/dal/migrations/migrations.go`
Enable v3 migration:
```go
func init() {
    registerMigration(v1.New())
    registerMigration(v2.New())
    registerMigration(v3.New())  // Uncomment this line
}
```

### 4. Optional: Add Analytics Queries
You can now query subscriptions by date:
```go
func (s *BoltDB) GetSubscriptionsAfter(date time.Time) ([]Subscription, error) {
    // Implementation to filter by CreatedAt
}
```

## Deployment Checklist

- [ ] Code reviewed and approved
- [ ] Database backed up
- [ ] Migration tested on copy of production database
- [ ] Migration verified to be idempotent
- [ ] Application code updated to use new `CreatedAt` field
- [ ] Deployment scheduled during low-traffic period
- [ ] Monitoring in place to detect migration failures
- [ ] Rollback plan documented and communicated

## Success Criteria

Migration is considered successful when:
- ✅ All existing subscriptions have `CreatedAt` field
- ✅ All `CreatedAt` values are valid timestamps
- ✅ Application starts successfully after migration
- ✅ New subscriptions get correct `CreatedAt` timestamp
- ✅ No errors in application logs
- ✅ All existing functionality works as before

## Related Migrations

- **v1:** Creates migrations bucket (bootstrap)
- **v2:** Creates shutdowns and subscriptions buckets
- **v3:** Adds CreatedAt to subscriptions (this migration)

---

**Status:** ⏸️ Prepared - Not Yet Enabled
**Dependencies:** v1 and v2 must be applied first
**Activation:** Uncomment in migrations.go when ready to deploy
