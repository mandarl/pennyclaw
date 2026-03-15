#!/usr/bin/env bash
# PennyClaw Teardown Script v0.1.0
# Removes all GCP resources created by deploy.sh
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m'

echo -e "${BOLD}PennyClaw Teardown${NC}"
echo ""

PROJECT=$(gcloud config get-value project 2>/dev/null)
echo -e "Project: ${PROJECT}"

# Find PennyClaw instances
INSTANCES=$(gcloud compute instances list \
    --filter="name~pennyclaw" \
    --format="value(name,zone)" \
    --project="$PROJECT" 2>/dev/null)

if [[ -z "$INSTANCES" ]]; then
    echo -e "${GREEN}No PennyClaw instances found. Nothing to clean up.${NC}"
    exit 0
fi

echo -e "\n${YELLOW}Found PennyClaw instance(s):${NC}"
echo "$INSTANCES" | while read -r line; do
    echo "  → $line"
done

echo ""
echo -ne "${RED}Delete all PennyClaw instances and firewall rules? (y/N) ${NC}"
read -r REPLY
if [[ ! "$REPLY" =~ ^[Yy]$ ]]; then
    echo "Cancelled."
    exit 0
fi

# Delete instances
echo "$INSTANCES" | while read -r NAME ZONE; do
    echo "Deleting instance: ${NAME} (${ZONE})..."
    gcloud compute instances delete "$NAME" --zone="$ZONE" --project="$PROJECT" --quiet
done

# Delete firewall rules
echo "Removing firewall rules..."
gcloud compute firewall-rules delete pennyclaw-allow-web --project="$PROJECT" --quiet 2>/dev/null || true

echo ""
echo -e "${GREEN}${BOLD}Teardown complete.${NC} All PennyClaw resources have been removed."
echo "No further charges will be incurred."
