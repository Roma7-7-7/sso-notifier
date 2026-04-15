# Deployment

## Docker (recommended)

The bot runs as a Docker container, with images automatically built and pushed to GHCR on every push to `main`.

### Image

```
ghcr.io/roma7-7-7/sso-notifier:latest
ghcr.io/roma7-7-7/sso-notifier:<short-sha>
```

Multi-architecture: `linux/amd64` and `linux/arm64`.

### Production

```bash
docker pull ghcr.io/roma7-7-7/sso-notifier:latest
docker run -d --restart unless-stopped \
  --name sso-notifier \
  -v ./data/sso-notifier:/app/data \
  --env-file .env.sso-notifier \
  ghcr.io/roma7-7-7/sso-notifier:latest
```

### Update

```bash
docker pull ghcr.io/roma7-7-7/sso-notifier:latest
docker stop sso-notifier && docker rm sso-notifier
docker run -d --restart unless-stopped \
  --name sso-notifier \
  -v ./data/sso-notifier:/app/data \
  --env-file .env.sso-notifier \
  ghcr.io/roma7-7-7/sso-notifier:latest
```

### Local Development

```bash
# Copy template and fill in your token
cp .env.dist .env

# Start
make docker-up

# Stop
make docker-down
```

## Storage

- Database: `/app/data/sso-notifier.db` inside the container (mounted from host via volume)
- Logs: stdout (JSON or text depending on `DEV`)

## Configuration

All configuration via environment variables:

### Required

- `TELEGRAM_TOKEN`: Telegram bot token from @BotFather

### Optional (with defaults)

| Variable | Default | Description |
|----------|---------|-------------|
| `DEV` | false | Text logging instead of JSON |
| `GROUPS_COUNT` | 12 | Number of power outage groups |
| `DB_PATH` | data/sso-notifier.db | Database file path |
| `REFRESH_SHUTDOWNS_INTERVAL` | 5m | Schedule fetch frequency |
| `NOTIFY_INTERVAL` | 5m | Notification check frequency |
| `NOTIFY_UPCOMING_INTERVAL` | 1m | Upcoming alerts check frequency |
| `SCHEDULE_URL` | https://oblenergo.cv.ua/shutdowns/ | Schedule provider URL |

### Calendar (optional)

Sync runs only when both `CALENDAR_EMAIL` and `CALENDAR_CREDENTIALS_PATH` are set. Mount the credentials file into the container:

```yaml
volumes:
  - ./data/sso-notifier:/app/data
  - ./gcloud.json:/app/gcloud.json:ro
environment:
  CALENDAR_CREDENTIALS_PATH: /app/gcloud.json
  CALENDAR_EMAIL: your@gmail.com
```

| Variable | Default | Description |
|----------|---------|-------------|
| `CALENDAR_EMAIL` | - | Target calendar ID |
| `CALENDAR_CREDENTIALS_PATH` | data/gcloud.json | Path to Google service account JSON |
| `CALENDAR_SYNC_INTERVAL` | 15m | Sync frequency |
| `CALENDAR_GROUP` | 4 | Group number to sync (1-12) |
| `CALENDAR_SYNC_OFF` | true | Create events for OFF status |
| `CALENDAR_SYNC_MAYBE` | false | Create events for MAYBE status |
| `CALENDAR_SYNC_ON` | false | Create events for ON status |
| `CALENDAR_CLEANUP_INTERVAL` | 6h | Stale event cleanup frequency |
| `CALENDAR_CLEANUP_LOOKBACK_DAYS` | 7 | Days to look back for cleanup |

## CI/CD

GitHub Actions workflow (`.github/workflows/docker.yml`):
- **Trigger**: Push to `main` when Go source, Dockerfile, or workflow changes
- **Steps**: Run tests, build multi-arch image, push to GHCR
- **Tags**: `:latest` + `:<short-sha>`

## Backup

```bash
# Copy database from container volume
cp ./data/sso-notifier/sso-notifier.db ~/backups/

# Or from inside the container
docker exec sso-notifier cat /app/data/sso-notifier.db > ~/backups/sso-notifier.db
```
