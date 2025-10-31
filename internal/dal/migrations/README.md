# BoltDB Migrations

This directory contains the database migration system for SSO Notifier.

## Overview

The migration system manages schema changes to the BoltDB database in a versioned, trackable way. Each migration is self-contained and includes its own type definitions to prevent coupling with the DAL package.

## Current Database Schema

### Latest Schema Version: v2 (active in production)

**Note:** v3 is prepared but not yet enabled. It will be activated in a future release.

### Buckets

#### 1. `shutdowns`
Stores power outage schedules fetched from the provider.

- **Key Format:** Date string in format `"YYYY-MM-DD"` (e.g., `"2025-10-31"`)
- **Value Format:** JSON-encoded `Shutdowns` struct
- **Structure:**
  ```go
  type Shutdowns struct {
      Date    string                   `json:"date"`      // e.g., "20 жовтня"
      Periods []Period                 `json:"periods"`   // Time intervals
      Groups  map[string]ShutdownGroup `json:"groups"`    // Power groups
  }

  type Period struct {
      From string `json:"from"` // e.g., "00:00"
      To   string `json:"to"`   // e.g., "00:30"
  }

  type ShutdownGroup struct {
      Number int      // Group number (1-12)
      Items  []Status // Status for each period
  }

  type Status string // "Y" (ON), "N" (OFF), "M" (MAYBE)
  ```

- **Example Entry:**
  - Key: `"2025-10-31"`
  - Value:
    ```json
    {
      "date": "31 жовтня",
      "periods": [
        {"from": "00:00", "to": "00:30"},
        {"from": "00:30", "to": "01:00"}
      ],
      "groups": {
        "1": {
          "Number": 1,
          "Items": ["N", "Y"]
        }
      }
    }
    ```

#### 2. `subscriptions`
Stores user subscriptions to power outage groups.

- **Key Format:** Chat ID as string (e.g., `"123456789"`)
- **Value Format:** JSON-encoded `Subscription` struct
- **Current Structure (v2):**
  ```go
  type Subscription struct {
      ChatID int64             `json:"chat_id"`
      Groups map[string]string `json:"groups"` // group_id -> schedule_hash
  }
  ```

- **Future Structure (v3 - if activated):**
  ```go
  type Subscription struct {
      ChatID    int64             `json:"chat_id"`
      Groups    map[string]string `json:"groups"`
      CreatedAt time.Time         `json:"created_at"` // Would be added in v3
  }
  ```

- **Example Entry (v2):**
  - Key: `"123456789"`
  - Value:
    ```json
    {
      "chat_id": 123456789,
      "groups": {
        "5": "abc123def456"
      }
    }
    ```

- **Groups Map Explanation:**
  - **Key:** Group number (as string, e.g., `"5"`)
  - **Value:** Hash of the last notified schedule for that group
  - **Purpose:** Detect changes in schedule to trigger notifications

#### 3. `migrations`
Tracks applied database migrations.

- **Key Format:** Version string (e.g., `"v1"`, `"v2"`)
- **Value Format:** RFC3339 timestamp (e.g., `"2025-10-31T14:23:45Z"`)
- **Example Entry:**
  - Key: `"v1"`
  - Value: `"2025-10-31T14:23:45Z"`

## Migration System Architecture

### Principles

1. **Independence:** Migrations never import from `internal/dal`
2. **Immutability:** Once deployed, migrations are never modified
3. **Documentation:** Every migration has a detailed README
4. **Self-Tracking:** Migration status stored in the database itself
5. **Fail-Fast:** Application exits if migration fails

### Migration Lifecycle

```
Application Start
      ↓
RunMigrations()
      ↓
Ensure migrations bucket exists
      ↓
Load applied migrations from DB
      ↓
Load registered migrations from code
      ↓
Filter out applied migrations
      ↓
Sort by version (ascending)
      ↓
For each pending migration:
  - Log start
  - Execute Up()
  - Record timestamp
  - Log success
      ↓
Continue with application startup
```

## Migration List

| Version | Description | Status | Date Applied |
|---------|-------------|--------|--------------|
| v1 | Bootstrap migration system | ✅ Active | 2025-10-31 |
| v2 | Create shutdowns and subscriptions buckets | ✅ Active | 2025-10-31 |
| v3 | Add CreatedAt to subscriptions | ⏸️ Prepared | Not yet enabled |

## How to Create a New Migration

### Step 1: Create Directory Structure

```bash
mkdir internal/dal/migrations/vN
touch internal/dal/migrations/vN/migration.go
touch internal/dal/migrations/vN/README.md
```

### Step 2: Implement Migration

```go
// internal/dal/migrations/vN/migration.go
package vN

import (
    "go.etcd.io/bbolt"
)

// Copy OLD type definitions from dal/bolt.go
type OldStructV{N-1} struct {
    // ...
}

// Define NEW type definitions
type NewStructVN struct {
    // ... with new fields
}

type MigrationVN struct{}

func (m *MigrationVN) Version() int {
    return N
}

func (m *MigrationVN) Description() string {
    return "Brief description"
}

func (m *MigrationVN) Up(db *bbolt.DB) error {
    return db.Update(func(tx *bbolt.Tx) error {
        // Transformation logic here
        // Read old data
        // Transform to new structure
        // Write back
        return nil
    })
}

func New() *MigrationVN {
    return &MigrationVN{}
}
```

### Step 3: Register Migration

Edit `internal/dal/migrations/migrations.go`:

```go
import (
    "github.com/rsav/sso-notifier/internal/dal/migrations/vN"
)

func init() {
    registerMigration(v1.New())
    registerMigration(v2.New())
    registerMigration(vN.New()) // ADD THIS
}
```

### Step 4: Document Migration

Create `vN/README.md` using this template:

```markdown
# Migration vN: Brief Title

## Date
YYYY-MM-DD

## Description
Detailed explanation of what this migration does and why.

## Schema Changes
### Before
{old structure}

### After
{new structure}

## Data Transformation
How existing data is migrated.

## Rollback Strategy
How to rollback if needed.

## Testing Notes
Special considerations for testing.
```

### Step 5: Update This README

Add migration to the Migration List table above.

### Step 6: Test Migration

```bash
# Copy production database
cp data/sso-notifier.db data/test.db

# Test migration on copy
DB_PATH=data/test.db ./bin/sso-notifier

# Verify schema changes
boltbrowser data/test.db

# Test idempotency - run again
DB_PATH=data/test.db ./bin/sso-notifier
```

### Step 7: Update DAL

After migration is tested, update `internal/dal/bolt.go`:

```go
type Subscription struct {
    // Update structure to match new schema
}
```

### Step 8: Deploy

Deploy application - migration runs automatically on startup.

## Migration Best Practices

### DO
- ✅ Copy-paste type definitions into migration package
- ✅ Write comprehensive README for each migration
- ✅ Test on production data snapshot before deploying
- ✅ Use transactions for data integrity
- ✅ Handle edge cases (empty buckets, nil values)
- ✅ Make migrations idempotent when possible
- ✅ Document rollback strategy

### DON'T
- ❌ Import types from `internal/dal`
- ❌ Modify existing migrations after deployment
- ❌ Skip documentation
- ❌ Test only on empty databases
- ❌ Assume migration succeeds
- ❌ Deploy without backup strategy

## Troubleshooting

### Migration Fails on Startup

**Symptoms:** Application exits with migration error

**Solutions:**
1. Check logs for specific error message
2. Verify database file permissions
3. Ensure database is not corrupted
4. Restore from backup if needed

### Migration Marked as Applied but Data Unchanged

**Symptoms:** Migration recorded in DB but schema unchanged

**Solutions:**
1. Check migration logic for bugs
2. Verify bucket names are correct
3. Ensure transaction committed
4. May need to manually remove migration record and rerun

### Need to Rollback Migration

**Process:**
1. Stop application
2. Restore database from pre-migration backup
3. Remove migration registration from `migrations.go`
4. Fix migration code
5. Test thoroughly
6. Redeploy

## Tools

### View Database Contents
```bash
# Using boltbrowser (if installed)
boltbrowser data/sso-notifier.db

# Using bolt CLI
bolt buckets data/sso-notifier.db
bolt keys data/sso-notifier.db migrations
```

### Check Applied Migrations
```bash
# Using bolt CLI
bolt get data/sso-notifier.db migrations v1
bolt get data/sso-notifier.db migrations v2
```

### Backup Database
```bash
# Simple file copy (when app is stopped)
cp data/sso-notifier.db data/backup-$(date +%Y%m%d-%H%M%S).db

# Or use BoltDB backup (while app is running)
# Requires custom backup command
```

## Testing

### Unit Tests
```bash
go test ./internal/dal/migrations/...
```

### Integration Tests
```bash
# Test full migration chain
go test ./internal/dal/migrations -integration
```

## Migration History

### v1 - Bootstrap (2025-10-31)
- Created migrations bucket
- Initialized migration tracking system
- Status: ✅ Deployed

### v2 - Create Application Buckets (2025-10-31)
- Created shutdowns and subscriptions buckets
- Moved bucket creation from NewBoltDB to migrations
- Status: ✅ Deployed

### v3 - Add CreatedAt to Subscriptions (2025-10-31)
- Adds CreatedAt timestamp field to Subscription
- Enables user acquisition analytics
- Status: ⏸️ Prepared, not yet enabled

## Resources

- [BoltDB Documentation](https://github.com/etcd-io/bbolt)
- [Project CLAUDE.md](../../CLAUDE.md#database-migrations) - Comprehensive migration guide
- [MIGRATIONS_PLAN.md](../../../MIGRATIONS_PLAN.md) - Original implementation plan

## Support

If you encounter issues with migrations:
1. Check this README
2. Check individual migration READMEs
3. Review logs for error messages
4. Restore from backup if needed
5. Contact maintainer

---

**Last Updated:** 2025-10-31
**Schema Version:** v2 (active), v3 (prepared)
**Maintainer:** @rsav
