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
chown -R ec2-user:ec2-user "${INSTALL_DIR}"
echo "✓ Directories created"

echo ""
echo -e "${GREEN}[2/6] Downloading deploy.sh script...${NC}"
curl -L -o "${INSTALL_DIR}/deploy.sh" \
    "https://raw.githubusercontent.com/${GITHUB_REPO}/main/deployment/deploy.sh"
chmod +x "${INSTALL_DIR}/deploy.sh"
echo "✓ Deploy script installed"

echo ""
echo -e "${GREEN}[3/6] Installing systemd service...${NC}"
curl -L -o /etc/systemd/system/sso-notifier.service \
    "https://raw.githubusercontent.com/${GITHUB_REPO}/main/deployment/systemd/sso-notifier.service"
systemctl daemon-reload
echo "✓ Systemd service installed"

echo ""
echo -e "${GREEN}[3.1/6] Configuring sudoers for passwordless service management...${NC}"
curl -L -o /etc/sudoers.d/sso-notifier \
    "https://raw.githubusercontent.com/${GITHUB_REPO}/main/deployment/sudoers.d/sso-notifier"
chmod 0440 /etc/sudoers.d/sso-notifier
echo "✓ Sudoers configuration installed"

echo ""
echo -e "${GREEN}[4/6] Running initial deployment...${NC}"
"${INSTALL_DIR}/deploy.sh"
echo "✓ Initial deployment completed"

echo ""
echo -e "${GREEN}[5/6] Setting up automated backups...${NC}"

# Ask for S3 backup location
echo ""
echo -e "${YELLOW}Do you want to set up automated daily backups to S3? (y/n)${NC}"
read -p "Enable backups? " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo ""
    echo -e "${BLUE}Enter the S3 URI where backups should be stored${NC}"
    echo -e "${YELLOW}Example: s3://my-backup-bucket/sso-notifier-backups${NC}"
    read -p "S3 URI: " S3_BACKUP_URI

    if [ -z "$S3_BACKUP_URI" ]; then
        echo -e "${YELLOW}⚠ No S3 URI provided, skipping backup setup${NC}"
    else
        # Download backup script
        echo "Downloading backup script..."
        curl -L -o "${INSTALL_DIR}/backup.sh" \
            "https://raw.githubusercontent.com/${GITHUB_REPO}/main/deployment/backup.sh"
        chmod +x "${INSTALL_DIR}/backup.sh"
        chown ec2-user:ec2-user "${INSTALL_DIR}/backup.sh"

        # Create environment file for cron
        cat > "${INSTALL_DIR}/backup.env" <<EOF
DB_PATH=${DATA_DIR}/sso-notifier.db
S3_BACKUP_URI=${S3_BACKUP_URI}
PATH=/usr/local/bin:/usr/bin:/bin
AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION:-eu-central-1}
EOF
        chown ec2-user:ec2-user "${INSTALL_DIR}/backup.env"

        # Create cron job wrapper script to source environment
        cat > "${INSTALL_DIR}/backup-wrapper.sh" <<EOF
#!/bin/bash
set -a
source ${INSTALL_DIR}/backup.env
set +a
${INSTALL_DIR}/backup.sh >> ${INSTALL_DIR}/backups/backup.log 2>&1
EOF
        chmod +x "${INSTALL_DIR}/backup-wrapper.sh"
        chown ec2-user:ec2-user "${INSTALL_DIR}/backup-wrapper.sh"

        # Add cron job for daily backup at 8 PM
        CRON_JOB="0 20 * * * ${INSTALL_DIR}/backup-wrapper.sh"

        # Check if cron job already exists
        if crontab -u ec2-user -l 2>/dev/null | grep -q "${INSTALL_DIR}/backup-wrapper.sh"; then
            echo -e "${YELLOW}⚠ Cron job already exists, updating...${NC}"
            crontab -u ec2-user -l 2>/dev/null | grep -v "${INSTALL_DIR}/backup-wrapper.sh" | crontab -u ec2-user -
        fi

        # Add new cron job
        (crontab -u ec2-user -l 2>/dev/null; echo "$CRON_JOB") | crontab -u ec2-user -

        echo "✓ Backup script installed"
        echo "✓ Daily backups scheduled at 10 PM"
        echo "✓ Backups will be stored at: ${S3_BACKUP_URI}"
        echo "✓ Backup logs: ${INSTALL_DIR}/backups/backup.log"
    fi
else
    echo "Skipping backup setup"
fi

echo ""
echo -e "${GREEN}[6/6] Enabling service auto-start...${NC}"
systemctl enable sso-notifier.service
echo "✓ Service will start automatically on boot"

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

# Show database backup info if one was created
if [ -n "${DB_BACKUP_FILE}" ] && [ -f "${DB_BACKUP_FILE}" ]; then
    echo -e "${GREEN}Database Safety Info:${NC}"
    echo "  A backup of your existing database was created at:"
    echo "  ${DB_BACKUP_FILE}"
    echo ""
fi

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
echo "  Deploy:  ${INSTALL_DIR}/deploy.sh"
echo "  Stop:    sudo systemctl stop sso-notifier.service"
echo "  Start:   sudo systemctl start sso-notifier.service"

# Show backup commands if backups are configured
if [ -f "${INSTALL_DIR}/backup.sh" ]; then
    echo ""
    echo -e "${YELLOW}Backup commands:${NC}"
    echo "  Manual backup:   ${INSTALL_DIR}/backup.sh"
    echo "  View cron jobs:  crontab -l"
    echo "  Backup logs:     tail -f ${INSTALL_DIR}/backups/backup.log"
fi

echo ""
echo -e "${YELLOW}Note: Automated deployments are NOT enabled for security reasons.${NC}"
echo -e "${YELLOW}Run ${INSTALL_DIR}/deploy.sh manually to update to the latest release.${NC}"
echo ""
