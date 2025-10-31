# Migration v2: Create Application Buckets

## Date
2025-10-31

## Description
This migration creates the core application buckets needed for the SSO Notifier to function:
- `shutdowns` - stores power outage schedules
- `subscriptions` - stores user subscriptions

Previously, these buckets were created in `NewBoltDB()` constructor. Moving this logic to migrations ensures:
1. All schema changes are tracked and versioned
2. Database initialization is consistent
3. Migration system has full control over schema

## Schema Changes

### Before
- Only `migrations` bucket exists (created by v1)

### After
Database now has three buckets:
- `migrations` - migration tracking (from v1)
- `shutdowns` - power outage schedules (NEW)
- `subscriptions` - user subscriptions (NEW)

### Bucket Structures

#### shutdowns
- **Key Format:** Date string `"YYYY-MM-DD"` (e.g., `"2025-10-31"`)
- **Value Format:** JSON-encoded `Shutdowns` struct
- **Purpose:** Store fetched power outage schedules

#### subscriptions
- **Key Format:** Chat ID as string (e.g., `"123456789"`)
- **Value Format:** JSON-encoded `Subscription` struct
- **Purpose:** Store user subscriptions to power outage groups

## Data Transformation
No data transformation is performed. This migration only creates empty buckets if they don't already exist.

### Idempotency
Uses `CreateBucketIfNotExists`, so running multiple times is safe and will not cause errors.

### Edge Cases
- **Buckets already exist:** Migration succeeds without changes
- **Fresh database:** Creates both buckets successfully
- **Partial state (one bucket exists):** Creates only the missing bucket

## Rollback Strategy

### Option 1: Restore from Backup (Recommended)
1. Stop the application
2. Restore database from pre-migration backup
3. Restart application (v2 will not be applied)

### Option 2: Manual Bucket Deletion (Not Recommended)
**WARNING:** This will delete all data!

1. Stop the application
2. Open database with BoltDB tool
3. Delete `shutdowns` and `subscriptions` buckets
4. Remove v2 migration record from `migrations` bucket
5. Restart application

**Note:** Only do this on test databases. Production databases should be restored from backup.

## Testing Notes

### Test Cases
1. **Fresh database:** Both buckets created successfully
2. **Existing buckets:** Migration succeeds without errors
3. **Run twice:** Second run completes successfully (idempotent)
4. **Application functionality:** App works normally after migration

### Manual Testing
```bash
# Before migration - only migrations bucket should exist
$ bolt buckets data/sso-notifier.db
migrations

# Run application with v2 enabled
$ ./bin/sso-notifier

# After migration - all three buckets should exist
$ bolt buckets data/sso-notifier.db
migrations
shutdowns
subscriptions
```

### Performance Considerations
- **Execution Time:** <1ms (just creates empty buckets)
- **Downtime:** None (part of startup sequence)
- **Resource Usage:** Negligible

## Application Code Changes Required

### 1. `internal/dal/bolt.go`
Remove bucket creation from `NewBoltDB()`:

**Before:**
```go
func NewBoltDB(db *bbolt.DB) (*BoltDB, error) {
    mustBucket(db, shutdownsBucket)      // REMOVE
    mustBucket(db, subscriptionsBucket)  // REMOVE
    return &BoltDB{db: db}, nil
}
```

**After:**
```go
func NewBoltDB(db *bbolt.DB) (*BoltDB, error) {
    // Buckets are now created by migrations
    return &BoltDB{db: db}, nil
}
```

The `mustBucket` function can be removed entirely if not used elsewhere.

### 2. `internal/dal/migrations/migrations.go`
Enable v2 migration:

**Before:**
```go
func init() {
    registerMigration(v1.New())
    // registerMigration(v2.New())  // Commented out
}
```

**After:**
```go
func init() {
    registerMigration(v1.New())
    registerMigration(v2.New())  // Uncommented
}
```

## Deployment Checklist

- [ ] Code reviewed and approved
- [ ] Database backed up
- [ ] Migration tested on copy of production database
- [ ] Verified application starts successfully after migration
- [ ] Verified v2 is registered in `migrations.go`
- [ ] Verified bucket creation removed from `NewBoltDB()`
- [ ] Deployment scheduled
- [ ] Monitoring in place

## Success Criteria

Migration is considered successful when:
- ✅ `shutdowns` bucket exists
- ✅ `subscriptions` bucket exists
- ✅ Application starts successfully
- ✅ All existing functionality works
- ✅ No errors in application logs
- ✅ v2 recorded in migrations bucket

## Impact on Existing Databases

### New Databases (First Install)
- v1 creates `migrations` bucket
- v2 creates `shutdowns` and `subscriptions` buckets
- Ready to use immediately

### Existing Databases
- v1 already applied (skipped)
- v2 checks if buckets exist before creating
- **Expected:** Migration succeeds because buckets already exist (created by old `NewBoltDB`)
- **Result:** No data loss, seamless upgrade

## Why This Change?

Moving bucket creation to migrations provides several benefits:

1. **Consistency:** All schema changes tracked in one place
2. **Visibility:** Clear history of what buckets were created and when
3. **Control:** Migrations have full ownership of schema
4. **Simplicity:** `NewBoltDB` constructor simplified (no side effects)
5. **Testing:** Easier to test migration system independently

## Related Migrations

- **v1:** Creates migrations bucket (bootstrap)
- **v3:** (Future) Add CreatedAt to Subscription (example, commented)

---

**Status:** ✅ Ready for Production
**Dependencies:** v1 must be applied first
**Backwards Compatible:** Yes (idempotent bucket creation)
