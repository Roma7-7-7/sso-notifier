#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
DB_PATH="${DB_PATH:-/opt/sso-notifier/data/sso-notifier.db}"
S3_BACKUP_URI="${S3_BACKUP_URI:-}"
TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
BACKUP_FILENAME="sso-notifier-db-${TIMESTAMP}.db"

# Validate S3_BACKUP_URI is set
if [ -z "$S3_BACKUP_URI" ]; then
    echo -e "${RED}Error: S3_BACKUP_URI environment variable is not set${NC}"
    echo "Please set it to your S3 backup location, e.g.: s3://my-bucket/backups/"
    exit 1
fi

# Validate DB_PATH exists
if [ ! -f "$DB_PATH" ]; then
    echo -e "${RED}Error: Database file not found at: ${DB_PATH}${NC}"
    exit 1
fi

# Remove trailing slash from S3_BACKUP_URI if present
S3_BACKUP_URI="${S3_BACKUP_URI%/}"

# Full S3 path
S3_FULL_PATH="${S3_BACKUP_URI}/${BACKUP_FILENAME}"

echo -e "${GREEN}Starting BoltDB backup...${NC}"
echo "Database: ${DB_PATH}"
echo "Destination: ${S3_FULL_PATH}"
echo ""

# Upload to S3
if aws s3 cp "$DB_PATH" "$S3_FULL_PATH"; then
    echo ""
    echo -e "${GREEN}✓ Backup completed successfully${NC}"
    echo "Backup location: ${S3_FULL_PATH}"

    # Get file size for confirmation
    DB_SIZE=$(du -h "$DB_PATH" | cut -f1)
    echo "Backup size: ${DB_SIZE}"
else
    echo ""
    echo -e "${RED}✗ Backup failed${NC}"
    exit 1
fi
