# Migration v1: Bootstrap Migration System

## Date
2025-10-31

## Description
This is the bootstrap migration that initializes the migration tracking system. It verifies that the `migrations` bucket exists in BoltDB, which is used to track all applied migrations.

The `migrations` bucket is actually created by the migration runner's `ensureMigrationsBucket()` function before any migrations are executed. This migration exists to:
1. Mark that the migration system is initialized
2. Verify the migrations bucket is properly created
3. Serve as the first migration in the chain

## Schema Changes

### Before
Database may or may not have the following buckets:
- `shutdowns` - stores power outage schedules
- `subscriptions` - stores user subscriptions

No migration tracking exists.

### After
Database now has:
- `shutdowns` - stores power outage schedules (unchanged)
- `subscriptions` - stores user subscriptions (unchanged)
- `migrations` - NEW bucket for tracking applied migrations

### Migrations Bucket Structure
- **Bucket Name:** `migrations`
- **Key Format:** `"v1"`, `"v2"`, `"v3"`, etc. (version string)
- **Value Format:** RFC3339 timestamp (e.g., `"2025-10-31T14:23:45Z"`)
- **Example Entry:**
  - Key: `v1`
  - Value: `2025-10-31T14:23:45Z`

## Data Transformation
No data transformation is performed. This migration only verifies the migrations bucket exists.

## Rollback Strategy
**Not Recommended** - This is the foundational migration for the system.

If absolutely necessary:
1. Stop the application
2. Open the database with a BoltDB viewer
3. Delete the `migrations` bucket
4. All migration history will be lost
5. Next application start will reapply all migrations

## Testing Notes
- This migration is safe to run multiple times (idempotent)
- Should always succeed if the database is accessible
- If this migration fails, it indicates a critical database issue

## Implementation Notes
The migration runner creates the `migrations` bucket before running any migrations. This migration simply verifies that this happened correctly. This design ensures:
- The migrations bucket always exists before v1 runs
- v1 can be tracked in the migrations bucket itself
- No chicken-and-egg problem with tracking the first migration
