# SSO Notifier

A Telegram bot that monitors electricity power outage schedules in Chernivtsi, Ukraine and sends notifications to subscribers when the schedule changes.

## Overview

During power outages in Chernivtsi, the local electricity provider publishes schedules on their website (https://oblenergo.cv.ua/shutdowns/). The city is divided into 12 groups, and the schedule shows 30-minute intervals throughout the day with three possible states:

- **В (OFF)**: Power is off
- **З (ON)**: Power is on
- **МЗ (MAYBE)**: Power might be off

This bot automatically:
1. Fetches and parses the HTML schedule every 5 minutes
2. Detects changes in the schedule
3. Notifies subscribed users about updates for their group(s)
4. Filters out past time periods, showing only future events

## Features

- Subscribe to one or more power outage groups (1-12)
- Automatic notifications when schedules change
- Messages in Ukrainian with emoji indicators:
  - 🟢 Power is on
  - 🟡 Power might be off
  - 🔴 Power is off
- Smart time filtering (only shows future periods)
- Handles user blocking/unblocking gracefully
- Persistent storage using BoltDB

## Architecture

### Components

```
cmd/bot/main.go                  - Application entry point
internal/
  ├── dal/bolt.go               - Data access layer (BoltDB)
  ├── providers/chernivtsi.go   - HTML parser for power outage schedule
  ├── service/
  │   ├── shutdowns.go          - Schedule refresh logic
  │   ├── notifications.go      - Notification logic
  │   └── subscriptions.go      - Subscription management
  └── telegram/telegram.go      - Telegram bot handlers
```

### Data Flow

1. **Schedule Refresh** (configurable, default: 5 minutes)
   - `providers.ChernivtsiShutdowns()` fetches and parses HTML
   - Stores schedule in BoltDB via `service.Shutdowns`

2. **Notification Check** (configurable, default: 5 minutes)
   - `service.Notifications` compares current schedule with stored hashes
   - Generates messages for changed groups
   - Sends via Telegram to affected subscribers
   - Updates subscription hashes to prevent duplicate notifications

3. **User Interaction**
   - Users interact with bot via Telegram commands
   - Bot manages subscriptions through `service.Subscriptions`
   - Data persisted in BoltDB

## Setup

### Prerequisites

- Go 1.25.3 or higher
- Telegram Bot Token (from [@BotFather](https://t.me/botfather))

### Installation

1. Clone the repository:
```bash
git clone https://github.com/Roma7-7-7/sso-notifier.git
cd sso-notifier
```

2. Copy environment template:
```bash
cp .env.dist .env
```

3. Edit `.env` and configure:
```bash
DEV=true                                    # Enable development mode (optional)
TELEGRAM_TOKEN=your_telegram_bot_token_here # Required
```

4. Build the application:
```bash
make build
```

### Running

```bash
# Load environment variables
source .env

# Run the bot
./bin/sso-notifier
```

Or set environment variables directly:
```bash
TELEGRAM_TOKEN=your_token DEV=true ./bin/sso-notifier
```

## Bot Commands

- `/start` - Show main menu and subscription status
- `/subscribe` - Choose a group to subscribe to
- `/unsubscribe` - Remove all subscriptions

The bot also provides inline buttons for easy navigation.

## Configuration

All configuration is done via environment variables:

- `TELEGRAM_TOKEN` (required) - Telegram bot token from @BotFather
- `DEV` (optional) - Set to `true` for development mode (text logging instead of JSON, default: false)
- `GROUPS_COUNT` (optional) - Number of power outage groups (default: 12)
- `DB_PATH` (optional) - Database file path (default: `data/sso-notifier.db`)
- `REFRESH_SHUTDOWNS_INTERVAL` (optional) - How often to fetch schedule (default: 5m)
- `NOTIFY_INTERVAL` (optional) - How often to check for notifications (default: 5m)

## Data Storage

The bot uses BoltDB for persistent storage in `data/sso-notifier.db` (configurable via `DB_PATH`) with two buckets:

- `shutdowns` - Current power outage schedule
- `subscriptions` - User subscriptions with group hashes

Data structure:
```go
type Subscription struct {
    ChatID int64             // Telegram chat ID
    Groups map[string]string // Group number -> schedule hash
}

type Shutdowns struct {
    Date    string                   // Schedule date
    Periods []Period                 // Time periods (30-min intervals)
    Groups  map[string]ShutdownGroup // Group number -> schedule
}
```

## Development

### Project Structure

- `cmd/bot/` - Main application
- `internal/dal/` - Database layer (BoltDB operations)
- `internal/providers/` - External data providers (HTML parsing)
- `internal/service/` - Business logic layer
- `internal/telegram/` - Telegram bot integration

### Building

```bash
make build
```

This produces a static binary at `./bin/sso-notifier` (CGO_ENABLED=0 for portability).

### Dependencies

- [goquery](https://github.com/PuerkitoBio/goquery) - HTML parsing
- [bbolt](https://github.com/etcd-io/bbolt) - Embedded key-value database
- [telebot](https://gopkg.in/telebot.v3) - Telegram bot framework
- [telegram](https://github.com/Roma7-7-7/telegram) - Telegram client for notifications
- [envconfig](https://github.com/kelseyhightower/envconfig) - Environment variable configuration

## How It Works

### Schedule Parsing

The parser (`internal/providers/chernivtsi.go`) extracts:

1. **Date** - From `div#gsv ul p`
2. **Time Periods** - From `div > p u` elements (e.g., "00:00", "00:30", etc.)
3. **Groups** - From `ul > li[data-id]` attributes (1-12)
4. **Status** - From each group's items (`o`, `u`, `s` tags):
   - `В` → OFF
   - `З` → ON
   - Other → MAYBE

### Notification Logic

The notification system (`internal/service/notifications.go`):

1. Compares schedule hashes for each subscribed group
2. Detects changes (including new day with same schedule)
3. Joins consecutive periods with same status
4. Filters out past time periods using Kyiv timezone
5. Renders message with emoji indicators
6. Updates subscription hash to prevent duplicates

### Message Format

```
Графік стабілізаційних відключень на [Date]:

Група [N]:
  🟢 Заживлено:  [periods]
  🟡 Можливо заживлено: [periods]
  🔴 Відключено: [periods]
```

## Production Deployment

### AWS EC2 Automated Deployment

The recommended deployment method uses automated CI/CD to AWS EC2 instances. See the **[Deployment Guide](deployment/README.md)** for complete instructions.

**Quick Start:**
1. Set up AWS SSM Parameter Store with your Telegram token
2. Launch an EC2 instance with appropriate IAM role
3. Run the setup script:
   ```bash
   curl -L https://raw.githubusercontent.com/Roma7-7-7/sso-notifier/main/deployment/setup-ec2.sh | sudo bash
   ```

This provides:
- Automated releases on every push to `main`
- Hourly deployment checks via cron
- Zero-downtime updates with automatic rollback
- Secure configuration via AWS Systems Manager
- Systemd service management

### Manual Production Deployment

For production use without automated deployment, consider:

1. **Systemd Service** - Run as a background service
2. **Monitoring** - Log aggregation and alerting
3. **Backup** - Regular backups of `data/sso-notifier.db`
4. **Resource Limits** - Memory and CPU constraints
5. **Error Handling** - Restart on failure

Example systemd service:
```ini
[Unit]
Description=SSO Notifier Bot
After=network.target

[Service]
Type=simple
User=ssobot
WorkingDirectory=/opt/sso-notifier
Environment="TELEGRAM_TOKEN=your_token_here"
Environment="DEV=false"
Environment="DB_PATH=/opt/sso-notifier/data/sso-notifier.db"
ExecStart=/opt/sso-notifier/bin/sso-notifier
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Chernivtsi Oblenergo for providing the schedule data
- Telegram Bot API for the messaging platform
