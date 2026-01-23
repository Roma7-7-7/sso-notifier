# Database Migrations

The codebase uses a custom migration system for managing BoltDB schema changes.

## Architecture

**Location:** `internal/dal/migrations/`

**Key Principle:** Migrations are completely independent from the `dal` package. They work directly with raw `*bbolt.DB` and contain their own type definitions.

## Package Structure

```
internal/dal/migrations/
├── README.md           # Latest DB schema + migration system overview
├── migrations.go       # Core migration runner and interfaces
├── v1/                 # Creates migrations bucket
├── v2/                 # Creates shutdowns and subscriptions buckets
├── v3/                 # Adds CreatedAt to subscriptions (not yet enabled)
├── v4/                 # Adds Settings map to subscriptions
└── v5/                 # Creates alerts bucket for 10-minute advance notifications
```

## Migration Storage

Migrations are tracked in BoltDB itself:
- **Bucket:** `migrations`
- **Key Format:** `"v1"`, `"v2"`, `"v3"`, etc.
- **Value Format:** ISO 8601 timestamp (RFC3339) of when migration was applied

## Migration Interface

```go
type Migration interface {
    Version() int
    Description() string
    Up(db *bbolt.DB) error
}
```

## Execution Flow

1. Open/create `migrations` bucket in BoltDB
2. Load all registered migrations
3. Read applied migrations from DB
4. Filter out already-applied migrations
5. Sort remaining by version (ascending)
6. Execute each migration sequentially
7. Record execution timestamp after successful completion
8. Fail fast if any migration errors

## Creating a New Migration

### CRITICAL RULES

1. **Never import from `internal/dal`** - Migrations must be self-contained
2. **Copy-paste old types** - Include both old and new structures in migration code
3. **Write README first** - Document what changes and why
4. **Test on production data copy** - Never test migrations on live DB
5. **Never modify existing migrations** - Once deployed, migrations are immutable

### Step-by-Step Checklist

- [ ] Create `internal/dal/migrations/vN/` directory (N = next version)
- [ ] Copy old type definitions from `dal/bolt.go` to `vN/migration.go`
- [ ] Define new type structures in `vN/migration.go`
- [ ] Implement `Migration` interface with transformation logic
- [ ] Write `vN/README.md` with:
  - Date
  - Description of what changed and why
  - Schema before/after
  - Data transformation details
  - Rollback strategy (or "not possible")
- [ ] Update core `migrations/README.md` with latest schema
- [ ] Test migration on copy of production DB
- [ ] Verify idempotency (running twice doesn't break)
- [ ] Update `dal/bolt.go` types (after migration is tested)
- [ ] Register migration in `migrations.go`

### Example Migration Structure

```go
// internal/dal/migrations/v3/migration.go
package v3

import (
    "encoding/json"
    "fmt"
    "time"
    "go.etcd.io/bbolt"
)

// SubscriptionV2 is the OLD structure (copy-pasted from dal at time of v2)
type SubscriptionV2 struct {
    ChatID int64             `json:"chat_id"`
    Groups map[string]string `json:"groups"`
}

// SubscriptionV3 is the NEW structure
type SubscriptionV3 struct {
    ChatID    int64             `json:"chat_id"`
    Groups    map[string]string `json:"groups"`
    CreatedAt time.Time         `json:"created_at"`
}

type MigrationV3 struct{}

func (m *MigrationV3) Version() int     { return 3 }
func (m *MigrationV3) Description() string { return "Add CreatedAt timestamp to subscriptions" }

func (m *MigrationV3) Up(db *bbolt.DB) error {
    return db.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte("subscriptions"))
        if b == nil {
            return nil // No subscriptions to migrate
        }

        c := b.Cursor()
        now := time.Now()

        for k, v := c.First(); k != nil; k, v = c.Next() {
            var oldSub SubscriptionV2
            if err := json.Unmarshal(v, &oldSub); err != nil {
                return fmt.Errorf("unmarshal old subscription: %w", err)
            }

            newSub := SubscriptionV3{
                ChatID:    oldSub.ChatID,
                Groups:    oldSub.Groups,
                CreatedAt: now,
            }

            newData, err := json.Marshal(newSub)
            if err != nil {
                return fmt.Errorf("marshal new subscription: %w", err)
            }

            if err := b.Put(k, newData); err != nil {
                return fmt.Errorf("put new subscription: %w", err)
            }
        }
        return nil
    })
}
```

## Best Practices

**DO:**
- Copy-paste type definitions to migration package
- Document every change in README
- Test on production data snapshot
- Handle errors gracefully
- Use transactions for data integrity
- Verify idempotency

**DON'T:**
- Import types from `internal/dal`
- Modify existing migrations
- Skip documentation
- Test on live database
- Assume migration succeeds

## Integration with Application

**In `cmd/bot/main.go`:**

```go
if err := migrations.RunMigrations(store.DB()); err != nil {
    log.Error("Failed to run database migrations", "error", err)
    os.Exit(1)
}
```

## Migration README Template

```markdown
# Migration v{N}: {Brief Title}

## Date
{YYYY-MM-DD}

## Description
{Detailed explanation}

## Schema Changes

### Before
{Old structure}

### After
{New structure}

## Data Transformation
{How existing data is migrated}

## Rollback Strategy
{How to rollback or "Not possible - breaking change"}
```

## Troubleshooting

**Migration fails midway:**
- Migrations run in transactions when possible
- Check logs for specific error
- Restore from backup if needed

**Need to rollback migration:**
- Restore database from backup
- Remove migration from registry
- Fix migration code
