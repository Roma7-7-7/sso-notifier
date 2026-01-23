# Deployment

The bot supports two deployment modes.

## Deployment Options

### 1. Simple Deployment (recommended for most users)
- Works on any Linux server (Hetzner, Contabo, OVH, etc.)
- No AWS dependencies
- Configuration via `/opt/sso-notifier/.env` file
- Manual backups (SCP from local machine)
- See `deployment/SIMPLE-DEPLOYMENT.md`

### 2. AWS EC2 Deployment
- Automated S3 backups
- AWS SSM Parameter Store for secrets
- IAM role-based authentication
- See `deployment/README.md` (EC2 section)

## Multi-Architecture Support

The build system produces binaries for both architectures:
- `sso-notifier-amd64`: For Intel/AMD x86_64 processors (most VPS providers)
- `sso-notifier-arm64`: For ARM64 processors (AWS Graviton, Apple Silicon)

The `deploy.sh` script automatically detects the server architecture using `uname -m`.

## Binary

- CGO disabled (`CGO_ENABLED=0`) for static linking
- Single binary with no dependencies
- Cross-platform compatible (Linux AMD64 and ARM64)

## Storage

- Database: `data/sso-notifier.db` (or configured via `DB_PATH`)
- Logs: stdout (JSON or text)

## Configuration

All configuration via environment variables using `envconfig`:

### Token Configuration

The `TELEGRAM_TOKEN` can be provided in three ways (priority order):

1. **Environment variable** (recommended for simple deployments):
   - Set `TELEGRAM_TOKEN=your_token` in environment or `.env` file
   - Skips AWS SSM lookup entirely

2. **AWS SSM Parameter Store** (production EC2 deployments):
   - If `TELEGRAM_TOKEN` env var is not set and `DEV=false`
   - Fetches from `/sso-notifier-bot/prod/telegram-token` parameter
   - Requires IAM permissions for `ssm:GetParameter` and `kms:Decrypt`

3. **Dev mode requirement**:
   - If `DEV=true`, token MUST be provided via environment variable

### Required Variables

- `TELEGRAM_TOKEN`: Telegram bot token from @BotFather

### Optional Variables (with defaults)

| Variable | Default | Description |
|----------|---------|-------------|
| `DEV` | false | Set to "true" for text logging |
| `GROUPS_COUNT` | 12 | Number of power outage groups |
| `DB_PATH` | data/sso-notifier.db | Database file path |
| `REFRESH_SHUTDOWNS_INTERVAL` | 5m | Schedule fetch frequency |
| `NOTIFY_INTERVAL` | 5m | Notification check frequency |
| `NOTIFY_UPCOMING_INTERVAL` | 1m | Upcoming alerts check frequency |
| `SCHEDULE_URL` | https://oblenergo.cv.ua/shutdowns/ | Schedule provider URL |

### Timeouts

- HTTP request: 1 minute
- Telegram polling: 5 seconds

## Backup System

### Simple Deployment

Manual backups recommended:
```bash
scp user@server:/opt/sso-notifier/data/sso-notifier.db ~/backups/
```

### AWS EC2 Deployment

**Scripts:**
- `deployment/backup.sh`: Uploads database to S3 with timestamp
- `deployment/setup-ec2.sh`: Configures automated backups during initial setup

**Automated S3 Backups:**
- Schedule: Daily at 8 PM (20:00) via cron job
- Naming: `sso-notifier-db-YYYY-MM-DD_HH-MM-SS.db`
- Configuration: `/opt/sso-notifier/backup.env`
- Logs: `/opt/sso-notifier/backups/backup.log`

**Environment Variables** (backup.sh):
- `DB_PATH`: Database file location
- `S3_BACKUP_URI`: S3 destination (e.g., `s3://bucket/backups`)
- `AWS_DEFAULT_REGION`: AWS region for S3

**IAM Requirements:**
```json
{
  "Effect": "Allow",
  "Action": ["s3:PutObject", "s3:PutObjectAcl"],
  "Resource": "arn:aws:s3:::your-bucket/*"
}
```

**Safety Guarantees:**
- Re-running `setup-ec2.sh` never overwrites existing database
- Local backup created before any setup modifications
- `deploy.sh` never touches the database

## Monitoring

Structured logging with slog:
```go
log.InfoContext(ctx, "refreshing shutdowns")
log.ErrorContext(ctx, "Error refreshing", "error", err)
```

Fields for observability:
- Service name
- Chat IDs
- Group numbers
- Error details

## Deployment Scripts

- `deployment/setup-ec2.sh`: Initial EC2 setup and configuration
- `deployment/deploy.sh`: Binary deployment only (never touches DB)
- `deployment/backup.sh`: S3 backup script

See `deployment/README.md` for detailed procedures.
