# SSO Notifier - EC2 Deployment Guide

This directory contains scripts and configuration files for automated deployment of the SSO Notifier bot to AWS EC2 instances.

## Overview

The deployment setup provides:
- **Automated releases** via GitHub Actions on every push to `main`
- **One-time EC2 setup** with `setup-ec2.sh`
- **Automated deployments** via hourly cron job
- **Zero-downtime updates** with automatic rollback capability
- **Secure configuration** via AWS Systems Manager Parameter Store

## Architecture

```
┌─────────────────┐
│  GitHub Push    │
│   to main       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ GitHub Actions  │
│  - Run tests    │
│  - Build binary │
│  - Create       │
│    release      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ GitHub Release  │
│  - sso-notifier │
│  - VERSION      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  EC2 Instance   │
│  - Hourly cron  │
│  - deploy.sh    │
│  - Download new │
│    release      │
│  - Restart      │
│    service      │
└─────────────────┘
```

## Prerequisites

### AWS Setup

1. **EC2 Instance**
   - Amazon Linux 2023 or Amazon Linux 2
   - Instance type: t3.micro or larger
   - Security group: Outbound HTTPS (443) access

2. **IAM Role** (attach to EC2 instance)
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": [
           "ssm:GetParameter"
         ],
         "Resource": "arn:aws:ssm:*:*:parameter/sso-notifier-bot/prod/*"
       },
       {
         "Effect": "Allow",
         "Action": [
           "kms:Decrypt"
         ],
         "Resource": "*"
       }
     ]
   }
   ```

3. **SSM Parameter Store**
   ```bash
   # Create the Telegram bot token parameter in eu-central-1
   aws ssm put-parameter \
     --name "/sso-notifier-bot/prod/telegram-token" \
     --value "YOUR_TELEGRAM_BOT_TOKEN_FROM_BOTFATHER" \
     --type "SecureString" \
     --region eu-central-1
   ```

   **Note:** The region is configured in the systemd service file. If using a different region, update the `AWS_REGION` environment variable in `/etc/systemd/system/sso-notifier.service`.

## Initial Setup

### Step 1: Connect to EC2

```bash
ssh ec2-user@<your-ec2-ip>
```

### Step 2: Run Setup Script

```bash
# Download and run the setup script
curl -L -o setup-ec2.sh \
  https://raw.githubusercontent.com/Roma7-7-7/sso-notifier/main/deployment/setup-ec2.sh

chmod +x setup-ec2.sh
sudo ./setup-ec2.sh
```

The setup script will:
1. Create directory structure at `/opt/sso-notifier/`
2. Download and install `deploy.sh`
3. Install systemd service
4. Perform initial deployment
5. Enable auto-start on boot
6. Set up hourly cron job for automated deployments
7. Start the service

### Step 3: Verify Installation

```bash
# Check service status
sudo systemctl status sso-notifier.service

# View logs
sudo journalctl -u sso-notifier.service -f

# Check current version
cat /opt/sso-notifier/current_version
```

## Directory Structure

After setup, the following structure is created:

```
/opt/sso-notifier/
├── bin/
│   └── sso-notifier           # Current binary
├── data/
│   └── sso-notifier.db        # BoltDB database
├── backups/
│   ├── backup_20250130_120000/
│   ├── backup_20250130_130000/
│   └── ...                     # Last 5 backups kept
├── deploy.sh                   # Deployment script
├── current_version             # Current release tag
└── deployment.log              # Deployment history
```

## Manual Operations

### Manual Deployment

Force a deployment check:

```bash
sudo /opt/sso-notifier/deploy.sh
```

### View Deployment History

```bash
cat /opt/sso-notifier/deployment.log
```

### Service Management

```bash
# Start
sudo systemctl start sso-notifier.service

# Stop
sudo systemctl stop sso-notifier.service

# Restart
sudo systemctl restart sso-notifier.service

# Status
sudo systemctl status sso-notifier.service

# Enable auto-start
sudo systemctl enable sso-notifier.service

# Disable auto-start
sudo systemctl disable sso-notifier.service
```

### View Logs

```bash
# Follow logs in real-time
sudo journalctl -u sso-notifier.service -f

# View last 100 lines
sudo journalctl -u sso-notifier.service -n 100

# View logs from today
sudo journalctl -u sso-notifier.service --since today

# View logs with specific time range
sudo journalctl -u sso-notifier.service --since "2025-01-30 10:00:00" --until "2025-01-30 12:00:00"
```

### Rollback to Previous Version

```bash
# List available backups
ls -lh /opt/sso-notifier/backups/

# Stop service
sudo systemctl stop sso-notifier.service

# Restore from backup
sudo cp /opt/sso-notifier/backups/backup_YYYYMMDD_HHMMSS/sso-notifier \
        /opt/sso-notifier/bin/sso-notifier

# Update version file
sudo cp /opt/sso-notifier/backups/backup_YYYYMMDD_HHMMSS/current_version \
        /opt/sso-notifier/current_version

# Start service
sudo systemctl start sso-notifier.service
```

## Configuration

### Environment Variables

Configuration is managed through the systemd service file at `/etc/systemd/system/sso-notifier.service`.

Available variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `AWS_REGION` | `eu-central-1` | AWS region for SSM Parameter Store |
| `TELEGRAM_TOKEN` | *(from SSM)* | Telegram bot token (automatically fetched from SSM) |
| `DB_PATH` | `/opt/sso-notifier/data/sso-notifier.db` | Database file path |
| `GROUPS_COUNT` | `12` | Number of power outage groups |
| `REFRESH_SHUTDOWNS_INTERVAL` | `5m` | How often to fetch schedule |
| `NOTIFY_INTERVAL` | `5m` | How often to check for updates |
| `DEV` | `false` | Development mode (use `true` for text logs) |

### Modifying Configuration

1. Edit the service file:
   ```bash
   sudo nano /etc/systemd/system/sso-notifier.service
   ```

2. Add or modify environment variables in the `[Service]` section:
   ```ini
   Environment="REFRESH_SHUTDOWNS_INTERVAL=10m"
   Environment="NOTIFY_INTERVAL=10m"
   ```

3. Reload systemd and restart service:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart sso-notifier.service
   ```

## Automated Deployments

### How It Works

1. **GitHub Actions** creates a new release when code is pushed to `main`
2. **Cron job** runs hourly: `0 * * * * /opt/sso-notifier/deploy.sh`
3. **deploy.sh** checks for new releases
4. If a new version is found:
   - Downloads the new binary
   - Stops the service
   - Backs up the current version
   - Installs the new version
   - Restarts the service
   - Verifies health

### Cron Job Management

The deployment cron job runs under **root's crontab** (since deploy.sh requires sudo to restart services).

```bash
# View current cron jobs (as root)
sudo crontab -l

# Edit cron jobs (as root)
sudo crontab -e

# Disable automated deployments (comment out the line)
# 0 * * * * /opt/sso-notifier/deploy.sh >> /opt/sso-notifier/deployment.log 2>&1

# Change frequency (e.g., every 30 minutes)
*/30 * * * * /opt/sso-notifier/deploy.sh >> /opt/sso-notifier/deployment.log 2>&1
```

## Monitoring

### Health Check

Create a simple health check script:

```bash
#!/bin/bash
if systemctl is-active --quiet sso-notifier.service; then
    echo "OK: Service is running"
    exit 0
else
    echo "CRITICAL: Service is not running"
    exit 2
fi
```

### Metrics to Monitor

- Service uptime: `systemctl status sso-notifier.service`
- Memory usage: Check systemd logs or use `ps aux | grep sso-notifier`
- Disk usage: `du -sh /opt/sso-notifier/data/`
- Deployment history: `tail -f /opt/sso-notifier/deployment.log`

## Troubleshooting

### Service Fails to Start

1. Check logs:
   ```bash
   sudo journalctl -u sso-notifier.service -n 50 --no-pager
   ```

2. Common issues:
   - **SSM token not found**: Verify SSM parameter exists and IAM role is attached
   - **Permission denied**: Check file ownership: `sudo chown -R ec2-user:ec2-user /opt/sso-notifier`
   - **Database locked**: Ensure only one instance is running

### Deployment Failures

1. Check deployment log:
   ```bash
   tail -f /opt/sso-notifier/deployment.log
   ```

2. Common issues:
   - **Network timeout**: Check security group allows outbound HTTPS
   - **GitHub rate limit**: Wait and retry (rate limits reset hourly)
   - **Invalid binary**: Check GitHub Actions build succeeded

### SSM Token Issues

1. Test SSM access manually:
   ```bash
   aws ssm get-parameter \
     --name "/sso-notifier-bot/prod/telegram-token" \
     --with-decryption \
     --region eu-central-1
   ```

2. Verify IAM role is attached to EC2 instance:
   ```bash
   curl http://169.254.169.254/latest/meta-data/iam/info
   ```

## Security Best Practices

1. **Never commit secrets** to Git
2. **Use SecureString** in SSM Parameter Store
3. **Restrict IAM permissions** to specific SSM parameters
4. **Run as non-root user** (ec2-user)
5. **Enable security hardening** in systemd (NoNewPrivileges, PrivateTmp)
6. **Keep backups** of previous versions (automatic)
7. **Monitor logs** for suspicious activity

## Uninstallation

To completely remove the bot:

```bash
# Stop and disable service
sudo systemctl stop sso-notifier.service
sudo systemctl disable sso-notifier.service

# Remove systemd service file
sudo rm /etc/systemd/system/sso-notifier.service
sudo systemctl daemon-reload

# Remove cron job (from root's crontab)
sudo crontab -l | grep -v "sso-notifier" | sudo crontab -

# Remove installation directory
sudo rm -rf /opt/sso-notifier

# Remove SSM parameter
aws ssm delete-parameter \
  --name "/sso-notifier-bot/prod/telegram-token" \
  --region eu-central-1
```

## Support

For issues, questions, or contributions:
- GitHub Issues: https://github.com/Roma7-7-7/sso-notifier/issues
- Repository: https://github.com/Roma7-7-7/sso-notifier
