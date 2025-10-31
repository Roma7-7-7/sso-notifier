#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
GITHUB_REPO="Roma7-7-7/sso-notifier"
INSTALL_DIR="/opt/sso-notifier"
BIN_DIR="${INSTALL_DIR}/bin"
BACKUP_DIR="${INSTALL_DIR}/backups"
VERSION_FILE="${INSTALL_DIR}/current_version"
LOG_FILE="${INSTALL_DIR}/deployment.log"
SERVICE_NAME="sso-notifier"

# Logging function
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "${LOG_FILE}"
}

log_error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR:${NC} $1" | tee -a "${LOG_FILE}"
}

log_warning() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING:${NC} $1" | tee -a "${LOG_FILE}"
}

# Check if ec2-user can run systemctl commands
if ! sudo -n systemctl status sso-notifier.service &>/dev/null && [ $? -ne 3 ]; then
    log_warning "This script requires passwordless sudo for systemctl commands"
    log_warning "Please ensure /etc/sudoers.d/ is configured properly"
fi

# Get latest release info from GitHub
log "Fetching latest release information from GitHub..."
LATEST_RELEASE=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest")
LATEST_VERSION=$(echo "${LATEST_RELEASE}" | grep -o '"tag_name": *"[^"]*"' | sed 's/"tag_name": *"\(.*\)"/\1/')

if [ -z "${LATEST_VERSION}" ]; then
    log_error "Failed to fetch latest version from GitHub"
    exit 1
fi

log "Latest version available: ${LATEST_VERSION}"

# Check current version
if [ -f "${VERSION_FILE}" ]; then
    CURRENT_VERSION=$(cat "${VERSION_FILE}")
    log "Current version: ${CURRENT_VERSION}"

    if [ "${CURRENT_VERSION}" = "${LATEST_VERSION}" ]; then
        log "Already running the latest version. Nothing to do."
        exit 0
    fi
else
    log_warning "No version file found. This might be a fresh installation."
    CURRENT_VERSION="none"
fi

# Create temporary directory for download
TMP_DIR=$(mktemp -d)
trap "rm -rf ${TMP_DIR}" EXIT

log "Downloading release ${LATEST_VERSION}..."

# Download binaries
DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${LATEST_VERSION}"

cd "${TMP_DIR}"

# Download sso-notifier binary
if ! curl -L -o sso-notifier "${DOWNLOAD_URL}/sso-notifier"; then
    log_error "Failed to download sso-notifier binary"
    exit 1
fi

# Download VERSION file
if ! curl -L -o VERSION "${DOWNLOAD_URL}/VERSION"; then
    log_error "Failed to download VERSION file"
    exit 1
fi

# Make binary executable
chmod +x sso-notifier

log "Downloaded files successfully"

# Stop the service
log "Stopping ${SERVICE_NAME} service..."
sudo systemctl stop "${SERVICE_NAME}.service" || true

# Backup current installation if it exists
if [ -f "${BIN_DIR}/sso-notifier" ]; then
    BACKUP_NAME="backup_$(date +'%Y%m%d_%H%M%S')"
    BACKUP_PATH="${BACKUP_DIR}/${BACKUP_NAME}"

    log "Creating backup: ${BACKUP_NAME}"
    mkdir -p "${BACKUP_PATH}"
    cp "${BIN_DIR}/sso-notifier" "${BACKUP_PATH}/"
    [ -f "${VERSION_FILE}" ] && cp "${VERSION_FILE}" "${BACKUP_PATH}/"

    # Keep only the last 5 backups
    cd "${BACKUP_DIR}"
    ls -t | tail -n +6 | xargs -r rm -rf
    log "Old backups cleaned up (keeping last 5)"
fi

# Install new version
log "Installing new version..."
mkdir -p "${BIN_DIR}"

cp "${TMP_DIR}/sso-notifier" "${BIN_DIR}/"
echo "${LATEST_VERSION}" > "${VERSION_FILE}"

# Start the service
log "Starting ${SERVICE_NAME} service..."
sudo systemctl start "${SERVICE_NAME}.service"

# Wait a moment for the service to initialize
sleep 2

# Check service status
if sudo systemctl is-active --quiet "${SERVICE_NAME}.service"; then
    log "✓ Service ${SERVICE_NAME} is running"
    log "Deployment completed successfully!"
    exit 0
else
    log_error "✗ Service ${SERVICE_NAME} failed to start"
    log_error "Check logs with: journalctl -u ${SERVICE_NAME}.service -n 50"
    exit 1
fi
