# BoltDB Migration System - Implementation Plan

**Status:** Planning Complete - Ready for Implementation
**Date:** 2025-10-31

## Overview

This document outlines the complete plan for implementing a schema migration system for the SSO Notifier's BoltDB database.

## Goals

1. Enable safe, versioned database schema changes
2. Track applied migrations in the database itself
3. Maintain independence from DAL package to enable parallel development
4. Provide clear documentation for every schema change
5. Ensure fail-fast behavior on migration errors

## Architecture Decisions

### Independence from DAL

**Decision:** Migrations MUST NOT import `internal/dal` package.

**Rationale:**
- Allows parallel development: DAL can be refactored without breaking migrations
- Each migration is a snapshot in time, self-contained
- Prevents circular dependencies
- Makes migrations truly immutable

**Implementation:** Copy-paste type definitions into migration packages.

### Storage Strategy

**Decision:** Store migration status in BoltDB itself, in a `migrations` bucket.

**Rationale:**
- No external state needed
- Migration status travels with the database
- Simple backup/restore (single file)
- No additional infrastructure

**Format:**
- Bucket: `migrations`
- Key: `"v1"`, `"v2"`, etc. (string version number)
- Value: RFC3339 timestamp of when migration was applied

### Execution Strategy

**Decision:** Sequential execution, fail-fast, no rollback.

**Rationale:**
- Simple to reason about
- Easier to debug
- Rollback is complex and error-prone for data migrations
- Better to restore from backup and fix migration code

**Failure Handling:**
- If migration fails, stop immediately
- Log error with full context
- Exit application (prevent running with wrong schema)
- Admin must: restore backup, fix migration, redeploy

## Package Structure

```
internal/dal/migrations/
├── README.md                    # Latest schema + migration system docs
├── migrations.go                # Core runner and interfaces
├── v1/
│   ├── README.md               # Bootstrap migration docs
│   └── migration.go            # Creates migrations bucket
├── v2/
│   ├── README.md               # Example migration docs
│   └── migration.go            # Example: Add CreatedAt to Subscription
└── v3/
    ├── README.md               # Future migration
    └── migration.go            # Future migration
```

## Core Interfaces

### Migration Interface

```go
// internal/dal/migrations/migrations.go
package migrations

import "go.etcd.io/bbolt"

// Migration represents a single database migration
type Migration interface {
    // Version returns the migration version number (1, 2, 3, ...)
    Version() int

    // Description returns a human-readable description of what this migration does
    Description() string

    // Up performs the migration on the provided database
    Up(db *bbolt.DB) error
}
```

### Runner Function

```go
// RunMigrations executes all pending migrations in order
func RunMigrations(db *bbolt.DB) error {
    // 1. Ensure migrations bucket exists
    // 2. Get list of applied migrations from DB
    // 3. Get list of registered migrations
    // 4. Filter out applied migrations
    // 5. Sort by version
    // 6. Execute each migration
    // 7. Record timestamp after each success
    // 8. Return error on first failure
}
```

## Implementation Steps

### Phase 1: Core Infrastructure

1. **Create package structure**
   ```bash
   mkdir -p internal/dal/migrations/v1
   touch internal/dal/migrations/migrations.go
   touch internal/dal/migrations/v1/migration.go
   touch internal/dal/migrations/README.md
   touch internal/dal/migrations/v1/README.md
   ```

2. **Implement core interfaces in `migrations.go`**
   - Define `Migration` interface
   - Implement `RunMigrations(db *bbolt.DB) error` function
   - Implement helper: `ensureMigrationsBucket(db *bbolt.DB) error`
   - Implement helper: `getAppliedMigrations(db *bbolt.DB) (map[int]time.Time, error)`
   - Implement helper: `recordMigration(db *bbolt.DB, version int) error`
   - Implement migration registry: `var registeredMigrations []Migration`

3. **Add logging support**
   - Accept `*slog.Logger` in `RunMigrations`
   - Log start/completion of each migration
   - Log errors with full context

### Phase 2: Bootstrap Migration (v1)

4. **Implement v1 migration**
   ```go
   // v1/migration.go
   package v1

   import "go.etcd.io/bbolt"

   type MigrationV1 struct{}

   func (m *MigrationV1) Version() int {
       return 1
   }

   func (m *MigrationV1) Description() string {
       return "Bootstrap migration system - create migrations bucket"
   }

   func (m *MigrationV1) Up(db *bbolt.DB) error {
       return db.Update(func(tx *bbolt.Tx) error {
           _, err := tx.CreateBucketIfNotExists([]byte("migrations"))
           return err
       })
   }

   func New() *MigrationV1 {
       return &MigrationV1{}
   }
   ```

5. **Document v1 in `v1/README.md`**
   - Date: 2025-10-31
   - Description: Creates migrations bucket
   - Schema changes: Adds `migrations` bucket
   - Rollback: Delete migrations bucket

### Phase 3: Integration

6. **Expose raw DB access in `internal/dal/bolt.go`**
   ```go
   // DB returns the underlying BoltDB instance for migrations
   func (s *BoltDB) DB() *bbolt.DB {
       return s.db
   }
   ```

7. **Integrate into `cmd/bot/main.go`**
   ```go
   // After: store, err := dal.NewBoltDB(cfg.DBPath)
   // Before: services initialization

   log.Info("Running database migrations")
   if err := migrations.RunMigrations(store.DB(), log); err != nil {
       log.Error("Failed to run database migrations", "error", err)
       os.Exit(1)
   }
   log.Info("Database migrations completed successfully")
   ```

8. **Register migrations in `migrations.go`**
   ```go
   func init() {
       registerMigration(v1.New())
   }
   ```

### Phase 4: Example Migration (v2)

9. **Create example migration v2**
   - Purpose: Demonstrate real schema change
   - Change: Add `CreatedAt time.Time` to `Subscription`
   - Copy old `Subscription` struct from `dal/bolt.go`
   - Define new `SubscriptionV2` struct
   - Implement transformation logic

10. **Document v2 in `v2/README.md`**
    - Follow template from CLAUDE.md
    - Include before/after schemas
    - Explain data transformation

### Phase 5: Documentation

11. **Write `migrations/README.md`**
    - Current database schema (all buckets)
    - Migration system overview
    - How to create new migration
    - Troubleshooting guide

12. **Update CLAUDE.md** ✅ (Already completed)
    - Add Database Migrations section
    - Update "Common Operations" section
    - Add migration creation checklist

## Current Database Schema

### Buckets

1. **shutdowns**
   - **Purpose:** Store power outage schedules
   - **Key Format:** Date string (e.g., "2025-10-31")
   - **Value Format:** JSON-encoded `Shutdowns` struct
   - **Structure:**
     ```go
     type Shutdowns struct {
         Date    string                   `json:"date"`
         Periods []Period                 `json:"periods"`
         Groups  map[string]ShutdownGroup `json:"groups"`
     }

     type Period struct {
         From string `json:"from"`
         To   string `json:"to"`
     }

     type ShutdownGroup struct {
         Number int
         Items  []Status  // "Y" (ON), "N" (OFF), "M" (MAYBE)
     }
     ```

2. **subscriptions**
   - **Purpose:** Store user subscriptions
   - **Key Format:** Chat ID as string (e.g., "123456789")
   - **Value Format:** JSON-encoded `Subscription` struct
   - **Structure:**
     ```go
     type Subscription struct {
         ChatID int64             `json:"chat_id"`
         Groups map[string]string `json:"groups"` // group_id -> schedule_hash
     }
     ```

3. **migrations** (to be added by v1)
   - **Purpose:** Track applied migrations
   - **Key Format:** Version string (e.g., "v1", "v2")
   - **Value Format:** RFC3339 timestamp (e.g., "2025-10-31T14:23:45Z")

## Migration Creation Workflow

### Step-by-Step Process

1. **Identify Need for Migration**
   - New field in existing struct
   - New bucket needed
   - Data transformation required
   - Bucket rename/restructure

2. **Plan Migration**
   - Document what changes and why
   - Identify old and new structures
   - Plan data transformation
   - Consider rollback strategy

3. **Create Migration Package**
   ```bash
   mkdir internal/dal/migrations/vN
   touch internal/dal/migrations/vN/migration.go
   touch internal/dal/migrations/vN/README.md
   ```

4. **Implement Migration**
   - Copy old type definitions from `dal/bolt.go`
   - Define new type definitions
   - Implement `Migration` interface
   - Implement `Up(db *bbolt.DB) error` with transformation logic
   - Add constructor function `New() *MigrationVN`

5. **Document Migration**
   - Fill out `vN/README.md` using template
   - Update `migrations/README.md` with new schema
   - Document any breaking changes

6. **Test Migration**
   - Copy production database to test environment
   - Run migration
   - Verify data integrity
   - Test application with new schema
   - Verify idempotency (run twice)

7. **Register Migration**
   - Add to `migrations.go`: `registerMigration(vN.New())`
   - Verify migrations are sorted by version

8. **Update DAL**
   - Update types in `dal/bolt.go` to match new schema
   - Update queries if needed
   - Update services if needed

9. **Deploy**
   - Application will run migrations on startup
   - Monitor logs for success
   - Verify no errors in production

## Testing Strategy

### Unit Tests

```go
// migrations_test.go
func TestRunMigrations_EmptyDB(t *testing.T) {
    // Test: migrations run on fresh DB
}

func TestRunMigrations_AlreadyApplied(t *testing.T) {
    // Test: already-applied migrations are skipped
}

func TestRunMigrations_Ordering(t *testing.T) {
    // Test: migrations run in correct order
}

func TestMigrationV1(t *testing.T) {
    // Test: v1 creates migrations bucket
}
```

### Integration Tests

```go
func TestMigrationChain_V1ToV3(t *testing.T) {
    // Test: full migration chain works
}

func TestMigrationIdempotency(t *testing.T) {
    // Test: running migrations twice doesn't break
}
```

### Manual Testing Checklist

- [ ] Create test database with production-like data
- [ ] Run migrations
- [ ] Verify all buckets exist
- [ ] Verify data structure matches new schema
- [ ] Run application and test all features
- [ ] Check logs for warnings/errors
- [ ] Run migrations again (test idempotency)
- [ ] Restore from backup and retry (test repeatability)

## Error Handling

### Migration Failures

**Scenario:** Migration returns error during execution

**Handling:**
1. Stop migration chain immediately
2. Log error with:
   - Migration version
   - Migration description
   - Error message
   - Stack trace (if applicable)
3. Do NOT mark migration as applied
4. Exit application with non-zero code

**Recovery:**
1. Restore database from backup
2. Fix migration code
3. Redeploy application

### Database Corruption

**Scenario:** Cannot open migrations bucket or read migration status

**Handling:**
1. Log error with full context
2. Exit application (fail-fast)
3. Alert operators

**Recovery:**
1. Restore from backup
2. Investigate root cause

### Partial Application

**Scenario:** Migration partially succeeds (some records migrated, some failed)

**Prevention:**
- Use BoltDB transactions for all migrations
- Transaction ensures atomicity (all-or-nothing)

**If Still Occurs:**
- Restore from backup
- Fix migration logic to handle edge cases

## Rollback Strategy

**Default Position:** No automatic rollback.

**Rationale:**
- Data migrations are complex
- Automatic rollback can make things worse
- Safer to restore from backup
- Gives time to analyze and fix issue

**Rollback Process:**
1. Stop application
2. Restore database from backup
3. Analyze migration failure
4. Fix migration code
5. Test in staging environment
6. Redeploy with fixed migration

**Future Enhancement:** Consider adding `Down()` method to Migration interface for explicit rollback support.

## Monitoring & Observability

### Logging

Log at these points:
- Before migration chain starts
- Before each migration
- After each migration success
- On migration failure
- After all migrations complete

**Log Fields:**
- `migration_version`: int
- `migration_description`: string
- `error`: error (if failed)
- `duration`: time.Duration
- `applied_migrations_count`: int

### Metrics (Future)

Consider adding:
- `migrations_applied_total`: counter
- `migrations_failed_total`: counter
- `migration_duration_seconds`: histogram

## Security Considerations

1. **Database File Access**
   - Ensure proper file permissions (0600)
   - Migrations run with same permissions as application

2. **Migration Code Review**
   - All migrations must be code-reviewed
   - Check for SQL injection (N/A for BoltDB)
   - Verify data transformation logic

3. **Backup Before Migration**
   - Always backup database before deploying migrations
   - Automate backup in deployment pipeline

## Performance Considerations

### Large Databases

**Problem:** Migrating millions of records can take time.

**Solutions:**
1. **Batch Processing:**
   ```go
   func (m *Migration) Up(db *bbolt.DB) error {
       batchSize := 1000
       // Process in batches
   }
   ```

2. **Progress Logging:**
   ```go
   log.Info("Migration progress", "processed", count, "total", total)
   ```

3. **Consider Downtime:**
   - Schedule migrations during low-traffic periods
   - Communicate downtime window to users

### Database Size

**Current:** ~1KB per subscriber + 5KB for schedule = minimal

**At Scale:** 10,000 users = ~10MB database

**Conclusion:** Performance not a concern for current scale. Monitor as user base grows.

## Open Questions & Decisions Needed

1. **Should migrations have mandatory unit tests?**
   - Recommendation: Yes, at least basic tests
   - Can be enforced via CI

2. **Should we add a dry-run mode?**
   - Recommendation: Nice-to-have, not critical for MVP
   - Can add in future iteration

3. **Should migrations log to separate file?**
   - Recommendation: No, use same structured logging
   - Operators can filter by component

4. **Version numbering: strict sequential or allow gaps?**
   - Recommendation: Strict sequential (v1, v2, v3...)
   - Simpler to reason about
   - Gaps can indicate issues

## Success Criteria

The migration system is successful if:

- ✅ Can add new fields to structs without downtime
- ✅ Migration status survives application restarts
- ✅ Failed migrations prevent application startup
- ✅ All migrations are documented
- ✅ Developers can create new migrations without confusion
- ✅ Database schema is always in consistent state
- ✅ Rollback via backup is straightforward

## Future Enhancements

### Phase 2 (Future)

1. **Down() Migrations**
   - Add `Down(db *bbolt.DB) error` to interface
   - Allow explicit rollback

2. **Dry-Run Mode**
   - Validate migrations without applying
   - Useful for testing

3. **Migration Validation**
   - Verify migration numbers are sequential
   - Check for duplicate versions
   - Validate that all migrations have documentation

4. **Schema Version Metadata**
   - Store current schema version in separate bucket
   - Quick version check without iterating migrations

5. **Migration Hooks**
   - Pre-migration hook
   - Post-migration hook
   - For custom logic (backups, notifications, etc.)

## References

- CLAUDE.md: Database Migrations section (comprehensive guide)
- internal/dal/bolt.go: Current database schema
- BoltDB docs: https://github.com/etcd-io/bbolt

## Appendix: Code Examples

### Complete Migration Runner Example

```go
// internal/dal/migrations/migrations.go
package migrations

import (
    "fmt"
    "log/slog"
    "sort"
    "time"

    "go.etcd.io/bbolt"
    "github.com/yourusername/sso-notifier/internal/dal/migrations/v1"
)

// Migration represents a database migration
type Migration interface {
    Version() int
    Description() string
    Up(db *bbolt.DB) error
}

var registeredMigrations []Migration

func init() {
    registerMigration(v1.New())
    // Future: registerMigration(v2.New())
}

func registerMigration(m Migration) {
    registeredMigrations = append(registeredMigrations, m)
}

// RunMigrations executes all pending migrations
func RunMigrations(db *bbolt.DB, log *slog.Logger) error {
    log.Info("Starting database migrations")

    // Ensure migrations bucket exists
    if err := ensureMigrationsBucket(db); err != nil {
        return fmt.Errorf("ensure migrations bucket: %w", err)
    }

    // Get applied migrations
    applied, err := getAppliedMigrations(db)
    if err != nil {
        return fmt.Errorf("get applied migrations: %w", err)
    }

    // Sort migrations by version
    sort.Slice(registeredMigrations, func(i, j int) bool {
        return registeredMigrations[i].Version() < registeredMigrations[j].Version()
    })

    // Run pending migrations
    for _, migration := range registeredMigrations {
        version := migration.Version()

        if _, alreadyApplied := applied[version]; alreadyApplied {
            log.Info("Skipping already-applied migration",
                "version", version,
                "description", migration.Description())
            continue
        }

        log.Info("Applying migration",
            "version", version,
            "description", migration.Description())

        start := time.Now()
        if err := migration.Up(db); err != nil {
            return fmt.Errorf("migration v%d failed: %w", version, err)
        }
        duration := time.Since(start)

        if err := recordMigration(db, version); err != nil {
            return fmt.Errorf("record migration v%d: %w", version, err)
        }

        log.Info("Migration applied successfully",
            "version", version,
            "duration", duration)
    }

    log.Info("All migrations completed successfully")
    return nil
}

func ensureMigrationsBucket(db *bbolt.DB) error {
    return db.Update(func(tx *bbolt.Tx) error {
        _, err := tx.CreateBucketIfNotExists([]byte("migrations"))
        return err
    })
}

func getAppliedMigrations(db *bbolt.DB) (map[int]time.Time, error) {
    applied := make(map[int]time.Time)

    err := db.View(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte("migrations"))
        if b == nil {
            return nil
        }

        return b.ForEach(func(k, v []byte) error {
            var version int
            if _, err := fmt.Sscanf(string(k), "v%d", &version); err != nil {
                return fmt.Errorf("parse version from key %s: %w", k, err)
            }

            timestamp, err := time.Parse(time.RFC3339, string(v))
            if err != nil {
                return fmt.Errorf("parse timestamp for v%d: %w", version, err)
            }

            applied[version] = timestamp
            return nil
        })
    })

    return applied, err
}

func recordMigration(db *bbolt.DB, version int) error {
    return db.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte("migrations"))
        key := []byte(fmt.Sprintf("v%d", version))
        value := []byte(time.Now().Format(time.RFC3339))
        return b.Put(key, value)
    })
}
```

## Timeline

**Estimated Implementation Time:** 4-6 hours

- Phase 1 (Core): 2 hours
- Phase 2 (Bootstrap): 30 minutes
- Phase 3 (Integration): 1 hour
- Phase 4 (Example): 1 hour
- Phase 5 (Documentation): 1 hour
- Testing: 1-2 hours

**Total:** 1 day of focused work

---

## Approval & Next Steps

**Status:** ✅ Plan Approved

**Next Actions:**
1. Begin Phase 1 implementation
2. Create initial package structure
3. Implement core migration runner
4. Implement v1 bootstrap migration
5. Test on development environment
6. Deploy to production

**Point of Contact:** @rsav

**Last Updated:** 2025-10-31
