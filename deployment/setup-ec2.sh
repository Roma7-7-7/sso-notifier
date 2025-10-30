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
echo -e "${BLUE}SSO Notifier EC2 Setup Script${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}Please run as root or with sudo${NC}"
    exit 1
fi

# Check if already installed
if [ -d "${INSTALL_DIR}" ]; then
    echo -e "${YELLOW}Warning: Installation directory ${INSTALL_DIR} already exists${NC}"
    read -p "Do you want to continue? This will overwrite the installation. (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Setup cancelled."
        exit 1
    fi
fi

echo -e "${GREEN}[1/7] Creating directory structure...${NC}"
mkdir -p "${BIN_DIR}"
mkdir -p "${DATA_DIR}"
mkdir -p "${BACKUP_DIR}"
chown -R ec2-user:ec2-user "${INSTALL_DIR}"
echo "✓ Directories created"

echo ""
echo -e "${GREEN}[2/7] Downloading deploy.sh script...${NC}"
curl -L -o "${INSTALL_DIR}/deploy.sh" \
    "https://raw.githubusercontent.com/${GITHUB_REPO}/main/deployment/deploy.sh"
chmod +x "${INSTALL_DIR}/deploy.sh"
echo "✓ Deploy script installed"

echo ""
echo -e "${GREEN}[3/7] Installing systemd service...${NC}"
curl -L -o /etc/systemd/system/sso-notifier.service \
    "https://raw.githubusercontent.com/${GITHUB_REPO}/main/deployment/systemd/sso-notifier.service"
systemctl daemon-reload
echo "✓ Systemd service installed"

echo ""
echo -e "${GREEN}[4/7] Running initial deployment...${NC}"
"${INSTALL_DIR}/deploy.sh"
echo "✓ Initial deployment completed"

echo ""
echo -e "${GREEN}[5/7] Enabling service auto-start...${NC}"
systemctl enable sso-notifier.service
echo "✓ Service will start automatically on boot"

echo ""
echo -e "${GREEN}[6/7] Setting up automated deployment cron job...${NC}"

# Check if cron job already exists
if crontab -u ec2-user -l 2>/dev/null | grep -q "${INSTALL_DIR}/deploy.sh"; then
    echo "Cron job already exists, skipping..."
else
    # Add cron job to check for updates every hour
    (crontab -u ec2-user -l 2>/dev/null; echo "0 * * * * ${INSTALL_DIR}/deploy.sh >> ${INSTALL_DIR}/deployment.log 2>&1") | crontab -u ec2-user -
    echo "✓ Cron job added (runs hourly)"
fi

echo ""
echo -e "${GREEN}[7/7] Verifying installation...${NC}"

# Check service status
if systemctl is-active --quiet sso-notifier.service; then
    echo "✓ Service is running"
else
    echo -e "${YELLOW}⚠ Service is not running (this might be expected if SSM token is not configured)${NC}"
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
echo -e "${YELLOW}Important next steps:${NC}"
echo ""
echo "1. Configure AWS SSM Parameter Store:"
echo "   aws ssm put-parameter \\"
echo "     --name '/sso-notifier-bot/prod/telegram-token' \\"
echo "     --value 'YOUR_TELEGRAM_BOT_TOKEN' \\"
echo "     --type 'SecureString' \\"
echo "     --region eu-central-1"
echo ""
echo "2. Ensure the EC2 instance has an IAM role with SSM permissions:"
echo "   - ssm:GetParameter"
echo "   - kms:Decrypt"
echo ""
echo "3. Optional: Configure environment variables in the systemd service file:"
echo "   Edit: /etc/systemd/system/sso-notifier.service"
echo "   Available variables:"
echo "     - DB_PATH (default: ${DATA_DIR}/sso-notifier.db)"
echo "     - GROUPS_COUNT (default: 12)"
echo "     - REFRESH_SHUTDOWNS_INTERVAL (default: 5m)"
echo "     - NOTIFY_INTERVAL (default: 5m)"
echo "   Then run: sudo systemctl daemon-reload && sudo systemctl restart sso-notifier"
echo ""
echo "4. Start/restart the service after SSM configuration:"
echo "   sudo systemctl restart sso-notifier.service"
echo ""
echo -e "${YELLOW}Useful commands:${NC}"
echo "  Status:  sudo systemctl status sso-notifier.service"
echo "  Logs:    sudo journalctl -u sso-notifier.service -f"
echo "  Deploy:  sudo ${INSTALL_DIR}/deploy.sh"
echo "  Stop:    sudo systemctl stop sso-notifier.service"
echo "  Start:   sudo systemctl start sso-notifier.service"
echo ""
echo -e "${GREEN}Automated deployments are enabled! New releases will be deployed hourly.${NC}"
echo ""
