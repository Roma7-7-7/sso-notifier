#!/bin/bash

# Script to update the coverage badge in README.md
# Usage: ./scripts/update-coverage-badge.sh

set -e

echo "🧪 Running tests to calculate coverage..."
make coverage > /dev/null 2>&1

ACTUAL_COVERAGE=$(go tool cover -func=coverage.filtered.out | grep total | awk '{print $3}' | sed 's/%//')
BADGE_COVERAGE=$(grep -o 'coverage-[0-9.]*%25' README.md | head -1 | sed 's/coverage-//;s/%25//')

echo "Current badge coverage: ${BADGE_COVERAGE}%"
echo "Actual coverage: ${ACTUAL_COVERAGE}%"

if [ "$ACTUAL_COVERAGE" = "$BADGE_COVERAGE" ]; then
  echo "✅ Coverage badge is already up to date!"
  exit 0
fi

# Update the badge
echo "📝 Updating coverage badge from ${BADGE_COVERAGE}% to ${ACTUAL_COVERAGE}%..."

# Determine badge color based on coverage
if (( $(echo "$ACTUAL_COVERAGE >= 80" | bc -l) )); then
  COLOR="brightgreen"
elif (( $(echo "$ACTUAL_COVERAGE >= 60" | bc -l) )); then
  COLOR="green"
elif (( $(echo "$ACTUAL_COVERAGE >= 40" | bc -l) )); then
  COLOR="yellow"
elif (( $(echo "$ACTUAL_COVERAGE >= 20" | bc -l) )); then
  COLOR="orange"
else
  COLOR="red"
fi

# Update README.md
sed -i.bak "s/coverage-[0-9.]*%25-[a-z]*/coverage-${ACTUAL_COVERAGE}%25-${COLOR}/" README.md
rm README.md.bak

echo "✅ Coverage badge updated to ${ACTUAL_COVERAGE}% with color ${COLOR}"
echo "📄 Please commit the updated README.md"

