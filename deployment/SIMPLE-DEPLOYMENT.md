# Simple Deployment Guide

This guide covers the simplified deployment approach for SSO Notifier, designed for running on any Linux server (VPS, dedicated server, etc.) without AWS dependencies.

## Overview

**What this deployment includes:**
- Automated binary deployment from GitHub releases
- Systemd service with auto-restart
- Simple `.env` file configuration
- Passwordless service management
- Manual backup strategy

**What this deployment does NOT include:**
- AWS SSM Parameter Store integration
- Automated backups to S3
- Cloud-specific configurations

Perfect for: Personal projects, small deployments, cost-conscious hosting (Hetzner, Contabo, OVH, etc.)

## Prerequisites

- Linux server with systemd (Ubuntu, Debian, CentOS, etc.)
- Root or sudo access
- Telegram bot token from [@BotFather](https://t.me/BotFather)
- Internet access (to download releases from GitHub)

## Quick Start

### 1. Download and run setup script

```bash
# Download setup script
curl -L -o setup-simple.sh https://raw.githubusercontent.com/Roma7-7-7/sso-notifier/main/deployment/setup-simple.sh

# Make it executable
chmod +x setup-simple.sh

# Run with sudo
sudo ./setup-simple.sh
```

The script will:
1. Ask for the user to run the service as (default: current user)
2. Create directory structure at `/opt/sso-notifier`
3. Download deployment script
4. Install systemd service
5. Configure sudoers for passwordless service management
6. Prompt for your Telegram bot token
7. Download and start the latest release

### 2. Verify installation

```bash
# Check service status
sudo systemctl status sso-notifier.service

# View logs
sudo journalctl -u sso-notifier.service -f
```

You're done! The bot should now be running and responding to `/start` in Telegram.

## Configuration

All configuration is stored in `/opt/sso-notifier/.env`

### Environment Variables

```bash
# Required: Telegram bot token
TELEGRAM_TOKEN=your_token_here

# Optional: Database path (default: /opt/sso-notifier/data/sso-notifier.db)
# DB_PATH=/opt/sso-notifier/data/sso-notifier.db

# Optional: Number of groups (default: 12)
# GROUPS_COUNT=12

# Optional: Schedule fetch interval (default: 5m)
# REFRESH_SHUTDOWNS_INTERVAL=5m

# Optional: Notification check interval (default: 5m)
# NOTIFY_INTERVAL=5m

# Optional: Upcoming alerts check interval (default: 1m)
# NOTIFY_UPCOMING_INTERVAL=1m

# Optional: Development mode (default: false)
# DEV=false
```

### Changing Configuration

1. Edit the environment file:
```bash
sudo nano /opt/sso-notifier/.env
```

2. Restart the service:
```bash
sudo systemctl restart sso-notifier.service
```

## Management Commands

All commands can be run by the service user without password (configured via sudoers).

### Service Control

```bash
# Start service
sudo systemctl start sso-notifier.service

# Stop service
sudo systemctl stop sso-notifier.service

# Restart service
sudo systemctl restart sso-notifier.service

# Check status
sudo systemctl status sso-notifier.service

# View logs (follow mode)
sudo journalctl -u sso-notifier.service -f

# View last 100 lines of logs
sudo journalctl -u sso-notifier.service -n 100
```

### Deployment

To update to the latest release:

```bash
/opt/sso-notifier/deploy.sh
```

The deployment script:
- Fetches the latest release from GitHub
- Stops the service
- Backs up the current binary
- Installs the new binary
- Starts the service
- Keeps the last 5 binary backups

## Manual Backup Strategy

### From Your Local Machine

**Daily automated backup** (add to your local crontab):

```bash
# Add to crontab (crontab -e)
0 21 * * * scp user@your-server:/opt/sso-notifier/data/sso-notifier.db ~/backups/sso-notifier/backup-$(date +\%Y\%m\%d).db
```

**One-time manual backup:**

```bash
scp user@your-server:/opt/sso-notifier/data/sso-notifier.db ~/backups/
```

### From the Server

**Create a local backup:**

```bash
cp /opt/sso-notifier/data/sso-notifier.db ~/sso-notifier-backup-$(date +%Y%m%d).db
```

**Schedule local backups** (add to crontab):

```bash
# Add to crontab (crontab -e)
0 2 * * * cp /opt/sso-notifier/data/sso-notifier.db ~/backups/sso-notifier-$(date +\%Y\%m\%d).db
```

## Directory Structure

```
/opt/sso-notifier/
├── bin/
│   └── sso-notifier          # Binary
├── data/
│   └── sso-notifier.db       # BoltDB database
├── backups/
│   ├── backup_20240315_120000/   # Binary backups (auto-created by deploy.sh)
│   └── sso-notifier.db.backup.*  # DB safety backups (created during setup)
├── .env                      # Environment configuration (IMPORTANT: contains secrets)
├── deploy.sh                 # Deployment script
├── current_version           # Current release version
└── deployment.log            # Deployment logs
```

## Troubleshooting

### Service won't start

1. Check logs:
```bash
sudo journalctl -u sso-notifier.service -n 50
```

2. Common issues:
   - Missing `TELEGRAM_TOKEN` in `.env`
   - Invalid bot token
   - Database file permissions
   - Network connectivity issues

### "Telegram token is required" error

Make sure your `.env` file has the token:
```bash
cat /opt/sso-notifier/.env
```

Should show:
```
TELEGRAM_TOKEN=your_actual_token_here
```

### Bot not responding

1. Verify the service is running:
```bash
sudo systemctl status sso-notifier.service
```

2. Check if the token is valid (test with curl):
```bash
TOKEN=$(grep TELEGRAM_TOKEN /opt/sso-notifier/.env | cut -d= -f2)
curl -s "https://api.telegram.org/bot${TOKEN}/getMe"
```

### Permission denied errors

Ensure all files are owned by the service user:
```bash
sudo chown -R your-user:your-user /opt/sso-notifier
```

### Database corrupted

Restore from backup:
```bash
# Stop service
sudo systemctl stop sso-notifier.service

# Restore from your backup
scp ~/backups/sso-notifier.db your-user@your-server:/opt/sso-notifier/data/

# Start service
sudo systemctl start sso-notifier.service
```

## Security Notes

### File Permissions

The `.env` file contains your bot token and should be protected:

```bash
# Verify permissions (should be 600)
ls -l /opt/sso-notifier/.env

# Fix if needed
sudo chmod 600 /opt/sso-notifier/.env
sudo chown your-user:your-user /opt/sso-notifier/.env
```

### Rotating the Bot Token

1. Generate new token from [@BotFather](https://t.me/BotFather)
2. Update `.env` file
3. Restart service

```bash
sudo nano /opt/sso-notifier/.env
# Update TELEGRAM_TOKEN value
sudo systemctl restart sso-notifier.service
```

### Server Access

- Use SSH keys instead of passwords
- Disable root login
- Keep your server updated: `sudo apt update && sudo apt upgrade`

## Migration from AWS EC2

If you're migrating from AWS EC2 to a simple deployment:

1. **Backup your database** from EC2:
```bash
scp ec2-user@ec2-host:/opt/sso-notifier/data/sso-notifier.db ~/
```

2. **Run setup on new server** (follow Quick Start above)

3. **Stop the service** on new server:
```bash
sudo systemctl stop sso-notifier.service
```

4. **Copy database** to new server:
```bash
scp ~/sso-notifier.db user@new-server:/opt/sso-notifier/data/
```

5. **Fix permissions**:
```bash
# On new server
sudo chown your-user:your-user /opt/sso-notifier/data/sso-notifier.db
```

6. **Start service**:
```bash
sudo systemctl start sso-notifier.service
```

All your users and subscriptions will be preserved!

## Re-running Setup

You can safely re-run `setup-simple.sh` on an existing installation:

- Existing database will be backed up automatically
- Configuration (`.env`) will NOT be overwritten
- Only binary and scripts will be updated
- Service will be restarted with new version

## Uninstallation

To completely remove SSO Notifier:

```bash
# Stop and disable service
sudo systemctl stop sso-notifier.service
sudo systemctl disable sso-notifier.service

# Remove systemd service
sudo rm /etc/systemd/system/sso-notifier.service
sudo systemctl daemon-reload

# Remove sudoers configuration
sudo rm /etc/sudoers.d/sso-notifier

# Remove installation directory
sudo rm -rf /opt/sso-notifier

# Optional: backup database first
cp /opt/sso-notifier/data/sso-notifier.db ~/sso-notifier-final-backup.db
```

## Cost Comparison

Example monthly costs for different hosting providers (as of 2024):

| Provider | Configuration | Cost/Month | Notes |
|----------|---------------|------------|-------|
| **Hetzner** | CPX11 (2 vCPU, 2GB RAM) | €4.51 (~$5) | Recommended |
| **Contabo** | VPS S (4 vCPU, 8GB RAM) | €5.99 (~$6.50) | More resources |
| **OVH** | VPS Starter (1 vCPU, 2GB RAM) | ~$7 | |
| **AWS EC2** | t3.micro (2 vCPU, 1GB RAM) | ~$10+ | Previous setup |

All configurations are more than enough for running the bot. Savings: ~$5-15/month compared to AWS.

## Support

- Issues: https://github.com/Roma7-7-7/sso-notifier/issues
- View this on GitHub: https://github.com/Roma7-7-7/sso-notifier/tree/main/deployment

## Advanced: Custom Installation Directory

To install to a different directory, modify the script variables before running:

```bash
# Edit setup-simple.sh
INSTALL_DIR="/custom/path/sso-notifier"

# Then run the modified script
sudo ./setup-simple.sh
```

Note: You'll also need to update the systemd service file manually.
