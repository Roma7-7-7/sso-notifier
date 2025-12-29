#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
GITHUB_REPO="Roma7-7-7/sso-notifier"
INSTALL_DIR="/opt/sso-notifier"
BIN_DIR="${INSTALL_DIR}/bin"
DATA_DIR="${INSTALL_DIR}/data"
BACKUP_DIR="${INSTALL_DIR}/backups"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}SSO Notifier Simple Setup Script${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}Please run as root or with sudo${NC}"
    exit 1
fi

# Determine which user to run the service as
if [ -n "$SUDO_USER" ]; then
    SERVICE_USER="$SUDO_USER"
else
    echo -e "${YELLOW}Enter the username to run the service as (default: current user):${NC}"
    read -r SERVICE_USER
    if [ -z "$SERVICE_USER" ]; then
        SERVICE_USER=$(whoami)
    fi
fi

echo -e "${GREEN}Service will run as user: ${SERVICE_USER}${NC}"
echo ""

# Verify user exists
if ! id "$SERVICE_USER" &>/dev/null; then
    echo -e "${RED}User $SERVICE_USER does not exist${NC}"
    exit 1
fi

# Check if already installed
if [ -d "${INSTALL_DIR}" ]; then
    echo -e "${YELLOW}Warning: Installation directory ${INSTALL_DIR} already exists${NC}"

    # Check if database exists
    DB_FILE="${DATA_DIR}/sso-notifier.db"
    if [ -f "$DB_FILE" ]; then
        echo -e "${YELLOW}Database found at: ${DB_FILE}${NC}"

        # Create backup of existing database
        BACKUP_TIMESTAMP=$(date +'%Y%m%d_%H%M%S')
        DB_BACKUP_FILE="${BACKUP_DIR}/sso-notifier.db.backup.${BACKUP_TIMESTAMP}"

        echo -e "${GREEN}Creating safety backup of database...${NC}"
        mkdir -p "${BACKUP_DIR}"
        cp "$DB_FILE" "$DB_BACKUP_FILE"
        echo -e "${GREEN}✓ Database backed up to: ${DB_BACKUP_FILE}${NC}"
        echo ""
    fi

    echo -e "${YELLOW}This will update the installation (binary and scripts only).${NC}"
    echo -e "${GREEN}Your database and existing data will NOT be affected.${NC}"
    read -p "Do you want to continue? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Setup cancelled."
        exit 1
    fi
fi

echo -e "${GREEN}[1/6] Creating directory structure...${NC}"
mkdir -p "${BIN_DIR}"
mkdir -p "${DATA_DIR}"
mkdir -p "${BACKUP_DIR}"
chown -R "${SERVICE_USER}:${SERVICE_USER}" "${INSTALL_DIR}"
echo "✓ Directories created"

echo ""
echo -e "${GREEN}[2/6] Downloading deploy.sh script...${NC}"
curl -L -o "${INSTALL_DIR}/deploy.sh" \
    "https://raw.githubusercontent.com/${GITHUB_REPO}/main/deployment/deploy.sh"
chmod +x "${INSTALL_DIR}/deploy.sh"
chown "${SERVICE_USER}:${SERVICE_USER}" "${INSTALL_DIR}/deploy.sh"
echo "✓ Deploy script installed"

echo ""
echo -e "${GREEN}[3/6] Installing systemd service...${NC}"
# Download the simple service file and replace placeholders
curl -L -s "https://raw.githubusercontent.com/${GITHUB_REPO}/main/deployment/systemd/sso-notifier-simple.service" | \
    sed "s/{{SERVICE_USER}}/${SERVICE_USER}/g" > /etc/systemd/system/sso-notifier.service
systemctl daemon-reload
echo "✓ Systemd service installed"

echo ""
echo -e "${GREEN}[4/6] Configuring sudoers for passwordless service management...${NC}"
# Create sudoers file for the service user
cat > /etc/sudoers.d/sso-notifier <<EOF
${SERVICE_USER} ALL=(ALL) NOPASSWD: /bin/systemctl start sso-notifier.service
${SERVICE_USER} ALL=(ALL) NOPASSWD: /bin/systemctl stop sso-notifier.service
${SERVICE_USER} ALL=(ALL) NOPASSWD: /bin/systemctl restart sso-notifier.service
${SERVICE_USER} ALL=(ALL) NOPASSWD: /bin/systemctl status sso-notifier.service
${SERVICE_USER} ALL=(ALL) NOPASSWD: /bin/systemctl is-active sso-notifier.service
EOF
chmod 0440 /etc/sudoers.d/sso-notifier
echo "✓ Sudoers configuration installed"

echo ""
echo -e "${GREEN}[5/6] Setting up environment file...${NC}"
ENV_FILE="${INSTALL_DIR}/.env"

if [ -f "$ENV_FILE" ]; then
    echo -e "${YELLOW}Environment file already exists at: ${ENV_FILE}${NC}"
    echo -e "${YELLOW}Skipping environment file creation to preserve existing configuration${NC}"
else
    echo -e "${BLUE}Enter your Telegram bot token (from @BotFather):${NC}"
    read -r TELEGRAM_TOKEN

    if [ -z "$TELEGRAM_TOKEN" ]; then
        echo -e "${YELLOW}⚠ No token provided. You'll need to configure it manually later.${NC}"
        cat > "$ENV_FILE" <<EOF
# Telegram bot token (required)
TELEGRAM_TOKEN=

# Optional: Uncomment and modify these to override defaults
# DB_PATH=/opt/sso-notifier/data/sso-notifier.db
# GROUPS_COUNT=12
# REFRESH_SHUTDOWNS_INTERVAL=5m
# NOTIFY_INTERVAL=5m
# NOTIFY_UPCOMING_INTERVAL=1m
# DEV=false
EOF
    else
        cat > "$ENV_FILE" <<EOF
# Telegram bot token
TELEGRAM_TOKEN=${TELEGRAM_TOKEN}

# Optional: Uncomment and modify these to override defaults
# DB_PATH=/opt/sso-notifier/data/sso-notifier.db
# GROUPS_COUNT=12
# REFRESH_SHUTDOWNS_INTERVAL=5m
# NOTIFY_INTERVAL=5m
# NOTIFY_UPCOMING_INTERVAL=1m
# DEV=false
EOF
    fi

    chmod 600 "$ENV_FILE"
    chown "${SERVICE_USER}:${SERVICE_USER}" "$ENV_FILE"
    echo "✓ Environment file created at: ${ENV_FILE}"
fi

echo ""
echo -e "${GREEN}[6/6] Running initial deployment...${NC}"
sudo -u "${SERVICE_USER}" "${INSTALL_DIR}/deploy.sh"
echo "✓ Initial deployment completed"

echo ""
echo -e "${GREEN}[7/7] Enabling service auto-start...${NC}"
systemctl enable sso-notifier.service
echo "✓ Service will start automatically on boot"

echo ""
echo -e "${GREEN}[8/8] Verifying installation...${NC}"

# Check service status
if systemctl is-active --quiet sso-notifier.service; then
    echo "✓ Service is running"
else
    echo -e "${YELLOW}⚠ Service is not running${NC}"
    echo -e "${YELLOW}Check configuration and logs${NC}"
fi

# Check version
if [ -f "${INSTALL_DIR}/current_version" ]; then
    VERSION=$(cat "${INSTALL_DIR}/current_version")
    echo "✓ Installed version: ${VERSION}"
fi

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}Setup completed successfully!${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Show database backup info if one was created
if [ -n "${DB_BACKUP_FILE}" ] && [ -f "${DB_BACKUP_FILE}" ]; then
    echo -e "${GREEN}Database Safety Info:${NC}"
    echo "  A backup of your existing database was created at:"
    echo "  ${DB_BACKUP_FILE}"
    echo ""
fi

echo -e "${YELLOW}Important Information:${NC}"
echo ""
echo "Configuration file: ${ENV_FILE}"
echo "  Edit this file to configure the bot (requires service restart)"
echo ""
echo -e "${YELLOW}Useful commands:${NC}"
echo "  Status:  sudo systemctl status sso-notifier.service"
echo "  Logs:    sudo journalctl -u sso-notifier.service -f"
echo "  Deploy:  ${INSTALL_DIR}/deploy.sh"
echo "  Stop:    sudo systemctl stop sso-notifier.service"
echo "  Start:   sudo systemctl start sso-notifier.service"
echo "  Restart: sudo systemctl restart sso-notifier.service"
echo ""
echo -e "${YELLOW}Manual Backups:${NC}"
echo "  Database location: ${DATA_DIR}/sso-notifier.db"
echo "  Backup with: scp ${SERVICE_USER}@your-server:${DATA_DIR}/sso-notifier.db ~/backups/"
echo ""
