# SSO Notifier - Project Overview

## Purpose
Telegram bot that monitors electricity power outage schedules in Chernivtsi, Ukraine. Scrapes HTML from oblenergo.cv.ua, detects schedule changes, and notifies subscribers via Telegram.

## Tech Stack
- **Language**: Go 1.25.5
- **Database**: BoltDB (go.etcd.io/bbolt)
- **Telegram**: gopkg.in/telebot.v3
- **HTML Scraping**: github.com/PuerkitoBio/goquery
- **Testing**: github.com/stretchr/testify, go.uber.org/mock
- **Configuration**: github.com/kelseyhightower/envconfig
- **AWS**: AWS SDK v2 (for SSM parameter store)

## Architecture
```
Presentation Layer (Telegram Bot)
    ↓
Service Layer (Business Logic)
    ↓
Data Access Layer (BoltDB)
    ↓
External Provider Layer (HTML Scraping)
```

**Four concurrent goroutines:**
1. Main Thread: Telegram bot event loop
2. Refresh Thread: Fetches schedule (default: 5 minutes)
3. Notification Thread: Checks for schedule updates (default: 5 minutes)
4. Alerts Thread: Checks for upcoming outages (default: 1 minute)

## Project Structure
```
cmd/
  bot/           - Main entry point
  testserver/    - Test server for development
internal/
  config/        - Configuration handling
  dal/           - Data access layer (BoltDB)
  providers/     - HTML scraping from oblenergo.cv.ua
  service/       - Business logic (notifications, alerts, subscriptions)
  telegram/      - Telegram bot handlers
docs/            - Documentation
deployment/      - Deployment configs
scripts/         - Utility scripts
```

## Key Files
| Purpose | File |
|---------|------|
| Entry point | `cmd/bot/main.go` |
| Data types & storage | `internal/dal/bolt.go` |
| HTML scraping | `internal/providers/chernivtsi.go` |
| Schedule refresh | `internal/service/shutdowns.go` |
| Notifications | `internal/service/notifications.go` |
| Upcoming alerts | `internal/service/alerts.go` |
| Subscriptions | `internal/service/subscriptions.go` |
| Telegram bot | `internal/telegram/telegram.go` |
| Message templates | `internal/service/messages.go` |

## Domain Concepts

### Power Outage Schedule Structure
- **City Division**: Chernivtsi is divided into 12 groups
- **Time Intervals**: Schedule uses 30-minute intervals (00:00, 00:30, 01:00, etc.)
- **Status Types**:
  - `В` (Ukrainian) → OFF (power is off)
  - `З` (Ukrainian) → ON (power is on)
  - `МЗ` (Ukrainian) → MAYBE (power might be off)

### Change Detection
Hash-based system to detect schedule changes:
- Each group's schedule is hashed (date + status sequence)
- Hash is stored per user subscription
- When hash changes → notification sent
- Hash updated after successful notification

## Configuration

**Required:**
- `TELEGRAM_TOKEN`: Telegram bot token from @BotFather

**Optional (with defaults):**
- `DEV` (false): Text logging instead of JSON
- `GROUPS_COUNT` (12): Number of power outage groups
- `DB_PATH` (data/sso-notifier.db): Database file path
- `REFRESH_SHUTDOWNS_INTERVAL` (5m): Schedule fetch frequency
- `NOTIFY_INTERVAL` (5m): Notification check frequency
- `NOTIFY_UPCOMING_INTERVAL` (1m): Upcoming alerts frequency
- `SCHEDULE_URL` (https://oblenergo.cv.ua/shutdowns/): Schedule provider URL
