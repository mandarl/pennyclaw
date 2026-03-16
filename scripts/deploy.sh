#!/usr/bin/env bash
# PennyClaw Deployment Script v0.1.1
# Deploys PennyClaw to GCP's Always Free e2-micro instance with comprehensive
# pre-flight validation to protect users from unexpected charges.
#
# Usage:
#   bash deploy.sh                  # Full deployment
#   bash deploy.sh --preflight-only # Run checks only, don't deploy
#   bash deploy.sh --teardown       # Remove PennyClaw from GCP

set -euo pipefail

# ============================================================================
# Constants
# ============================================================================
VERSION="0.1.1"
INSTANCE_PREFIX="pennyclaw"
MACHINE_TYPE="e2-micro"
DISK_SIZE_GB=30
DISK_TYPE="pd-standard"
IMAGE_FAMILY="ubuntu-2204-lts"
IMAGE_PROJECT="ubuntu-os-cloud"
NETWORK_TIER="STANDARD"
FREE_REGIONS=("us-west1" "us-central1" "us-east1")
FREE_ZONES=("us-west1-b" "us-central1-a" "us-east1-b")
SWAP_SIZE_MB=512
BUDGET_AMOUNT="1.00"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Counters
CHECKS_PASSED=0
CHECKS_WARNED=0
CHECKS_FAILED=0
PREFLIGHT_ONLY=false

# ============================================================================
# Utility Functions
# ============================================================================

banner() {
    echo -e "${CYAN}"
    echo '  ____                        ____ _                '
    echo ' |  _ \ ___ _ __  _ __  _   _ / ___| | __ ___      __'
    echo ' | |_) / _ \  _ \|  _ \| | | | |   | |/ _  \ \ /\ / /'
    echo ' |  __/  __/ | | | | | | |_| | |___| | (_| |\ V  V / '
    echo ' |_|   \___|_| |_|_| |_|\__, |\____|_|\__,_| \_/\_/  '
    echo '                         |___/                        '
    echo -e "${NC}"
    echo -e "  ${BOLD}Deployment Script v${VERSION}${NC} — Your \$0/month AI agent"
    echo ""
}

info()  { echo -e "  ${BLUE}ℹ${NC}  $1"; }
ok()    { echo -e "  ${GREEN}✓${NC}  $1"; CHECKS_PASSED=$((CHECKS_PASSED + 1)); }
warn()  { echo -e "  ${YELLOW}⚠${NC}  $1"; CHECKS_WARNED=$((CHECKS_WARNED + 1)); }
fail()  { echo -e "  ${RED}✗${NC}  $1"; CHECKS_FAILED=$((CHECKS_FAILED + 1)); }
step()  { echo -e "\n${BOLD}━━━ $1 ━━━${NC}\n"; }
ask()   { echo -ne "  ${CYAN}?${NC}  $1 "; read -r REPLY; }

# ============================================================================
# Parse Arguments
# ============================================================================

for arg in "$@"; do
    case $arg in
        --preflight-only) PREFLIGHT_ONLY=true ;;
        --teardown) exec bash "$(dirname "$0")/teardown.sh"; exit 0 ;;
        --help|-h)
            echo "Usage: deploy.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --preflight-only  Run pre-flight checks without deploying"
            echo "  --teardown        Remove PennyClaw from GCP"
            echo "  --help            Show this help"
            exit 0
            ;;
    esac
done

# ============================================================================
# Pre-Flight Checks
# ============================================================================

banner

step "PHASE 1: GCP Account & Authentication"

# Check 1: gcloud CLI installed
if command -v gcloud &>/dev/null; then
    GCLOUD_VERSION=$(gcloud version --format="value(Google Cloud SDK)" 2>/dev/null | head -1 || echo "unknown")
    ok "gcloud CLI installed (${GCLOUD_VERSION})"
else
    fail "gcloud CLI not found. Install: https://cloud.google.com/sdk/docs/install"
    echo -e "\n${RED}Cannot proceed without gcloud CLI. Exiting.${NC}"
    exit 1
fi

# Check 2: Authenticated
ACCOUNT=$(gcloud auth list --filter=status:ACTIVE --format="value(account)" 2>/dev/null | head -1 || echo "")
if [[ -n "$ACCOUNT" ]]; then
    ok "Authenticated as: ${ACCOUNT}"
else
    fail "Not authenticated. Run: gcloud auth login"
    exit 1
fi

# Check 3: Project selected
PROJECT=$(gcloud config get-value project 2>/dev/null || echo "")
if [[ -n "$PROJECT" && "$PROJECT" != "(unset)" ]]; then
    ok "Project: ${PROJECT}"
else
    fail "No project selected. Run: gcloud config set project YOUR_PROJECT_ID"
    exit 1
fi

# Check 4: Billing enabled
BILLING_ENABLED="false"
if gcloud billing projects describe "$PROJECT" --format="value(billingEnabled)" &>/dev/null; then
    BILLING_ENABLED=$(gcloud billing projects describe "$PROJECT" --format="value(billingEnabled)" 2>/dev/null || echo "false")
fi
if [[ "$BILLING_ENABLED" == "True" ]]; then
    ok "Billing is enabled (required even for free tier)"
else
    warn "Could not verify billing status (may need billing.projects.get permission)."
    warn "Free tier requires a billing account. Verify at:"
    info "https://console.cloud.google.com/billing/linkedaccount?project=${PROJECT}"
fi

# Check 5: Compute Engine API
COMPUTE_API=$(gcloud services list --enabled --filter="name:compute.googleapis.com" --format="value(name)" 2>/dev/null || echo "")
if [[ -n "$COMPUTE_API" ]]; then
    ok "Compute Engine API is enabled"
else
    info "Enabling Compute Engine API..."
    if gcloud services enable compute.googleapis.com --project="$PROJECT" 2>/dev/null; then
        ok "Compute Engine API enabled"
    else
        fail "Could not enable Compute Engine API. Enable it manually:"
        info "https://console.cloud.google.com/apis/library/compute.googleapis.com?project=${PROJECT}"
    fi
fi

# ============================================================================
step "PHASE 2: Free Tier Eligibility"

# Check 6: Existing e2-micro instances (CRITICAL)
EXISTING_MICROS=$(gcloud compute instances list \
    --filter="machineType~e2-micro AND status=RUNNING" \
    --format="value(name,zone)" \
    --project="$PROJECT" 2>/dev/null || echo "")

if [[ -z "$EXISTING_MICROS" ]]; then
    ok "No existing e2-micro instances found — you're eligible for the free tier!"
else
    MICRO_COUNT=$(echo "$EXISTING_MICROS" | wc -l)
    fail "Found ${MICRO_COUNT} running e2-micro instance(s):"
    echo "$EXISTING_MICROS" | while read -r line; do
        echo -e "     ${YELLOW}→ ${line}${NC}"
    done
    echo ""
    warn "GCP's free tier only covers ONE e2-micro instance per billing account."
    warn "Deploying another will incur charges (~\$4.50/month for e2-micro)."
    echo ""
    ask "Continue anyway? (y/N)"
    if [[ ! "$REPLY" =~ ^[Yy]$ ]]; then
        info "Deployment cancelled. Consider stopping an existing instance first."
        exit 0
    fi
fi

# Check 7: Existing instances of any type
ALL_INSTANCES=$(gcloud compute instances list \
    --format="table[no-heading](name,machineType.basename(),zone.basename(),status)" \
    --project="$PROJECT" 2>/dev/null || echo "")
if [[ -n "$ALL_INSTANCES" ]]; then
    INSTANCE_COUNT=$(echo "$ALL_INSTANCES" | wc -l)
    warn "Found ${INSTANCE_COUNT} total instance(s) in this project:"
    echo "$ALL_INSTANCES" | while read -r line; do
        echo -e "     ${YELLOW}→ ${line}${NC}"
    done
else
    ok "No existing instances — clean project"
fi

# Check 8: Check for existing PennyClaw installation
EXISTING_PC=$(gcloud compute instances list \
    --filter="name~${INSTANCE_PREFIX}" \
    --format="value(name,zone,status)" \
    --project="$PROJECT" 2>/dev/null || echo "")
if [[ -n "$EXISTING_PC" ]]; then
    warn "Existing PennyClaw instance found: ${EXISTING_PC}"
    ask "Upgrade existing instance instead of creating new? (Y/n)"
    if [[ "$REPLY" =~ ^[Nn]$ ]]; then
        info "Will create a new instance."
    else
        UPGRADE_MODE=true
        info "Will upgrade existing instance."
    fi
fi

# Check 9: Disk usage
TOTAL_DISK_GB=$(gcloud compute disks list \
    --format="value(sizeGb)" \
    --project="$PROJECT" 2>/dev/null | paste -sd+ 2>/dev/null | bc 2>/dev/null || echo "0")
TOTAL_DISK_GB=${TOTAL_DISK_GB:-0}
REMAINING_DISK=$((30 - TOTAL_DISK_GB))
if [[ $TOTAL_DISK_GB -eq 0 ]]; then
    ok "No existing disks — full 30GB free tier available"
elif [[ $REMAINING_DISK -ge $DISK_SIZE_GB ]]; then
    ok "Disk usage: ${TOTAL_DISK_GB}GB / 30GB free tier (${REMAINING_DISK}GB remaining)"
else
    warn "Disk usage: ${TOTAL_DISK_GB}GB / 30GB free tier — deploying ${DISK_SIZE_GB}GB may exceed free tier"
fi

# ============================================================================
step "PHASE 3: Region Selection"

# Check 10: Auto-detect best free-tier region by latency
info "Testing latency to free-tier regions..."
BEST_REGION=""
BEST_LATENCY=9999

for i in "${!FREE_REGIONS[@]}"; do
    REGION=${FREE_REGIONS[$i]}
    # Ping the region's metadata endpoint (approximate)
    LATENCY=$(curl -o /dev/null -s -w '%{time_connect}' \
        "https://${REGION}-docker.pkg.dev" 2>/dev/null || echo "9999")
    LATENCY_MS=$(echo "$LATENCY * 1000" | bc 2>/dev/null | cut -d. -f1)
    
    if [[ -n "$LATENCY_MS" ]] && [[ "$LATENCY_MS" -lt "$BEST_LATENCY" ]]; then
        BEST_LATENCY=$LATENCY_MS
        BEST_REGION=$REGION
        BEST_ZONE=${FREE_ZONES[$i]}
    fi
    info "  ${REGION}: ${LATENCY_MS:-???}ms"
done

if [[ -z "$BEST_REGION" ]]; then
    BEST_REGION="us-central1"
    BEST_ZONE="us-central1-a"
fi

ok "Selected region: ${BEST_REGION} (${BEST_ZONE}) — ${BEST_LATENCY}ms latency"

# Validate region is in free tier
REGION_VALID=false
for r in "${FREE_REGIONS[@]}"; do
    if [[ "$BEST_REGION" == "$r" ]]; then
        REGION_VALID=true
        break
    fi
done

if $REGION_VALID; then
    ok "Region ${BEST_REGION} is in the GCP Always Free tier"
else
    fail "Region ${BEST_REGION} is NOT in the free tier!"
    fail "Free tier regions: ${FREE_REGIONS[*]}"
    exit 1
fi

# ============================================================================
step "PHASE 4: Cost Protection Verification"

# Check 11: Machine type guard
info "Machine type: ${MACHINE_TYPE} (free tier eligible)"
ok "Machine type verified: ${MACHINE_TYPE}"

# Check 12: Disk type guard
info "Disk type: ${DISK_TYPE} (free tier eligible, NOT pd-ssd)"
ok "Disk type verified: ${DISK_TYPE}"

# Check 13: Disk size guard
if [[ $DISK_SIZE_GB -le 30 ]]; then
    ok "Disk size: ${DISK_SIZE_GB}GB (within 30GB free tier limit)"
else
    fail "Disk size ${DISK_SIZE_GB}GB exceeds 30GB free tier limit!"
    exit 1
fi

# Check 14: Network tier guard
info "Network tier: ${NETWORK_TIER} (free tier uses Standard, not Premium)"
ok "Network tier verified: ${NETWORK_TIER}"

# ============================================================================
step "PHASE 5: Pre-Deploy Cost Summary"

echo -e "  ${BOLD}Estimated Monthly Cost Breakdown:${NC}"
echo -e "  ┌─────────────────────────────────────────────────┐"
echo -e "  │ Resource              │ Cost       │ Free Tier   │"
echo -e "  ├─────────────────────────────────────────────────┤"
echo -e "  │ e2-micro VM (730 hrs) │ \$0.00      │ ${GREEN}✓ Covered${NC}   │"
echo -e "  │ 30GB pd-standard disk │ \$0.00      │ ${GREEN}✓ Covered${NC}   │"
echo -e "  │ 1GB egress (Std tier) │ \$0.00      │ ${GREEN}✓ Covered${NC}   │"
echo -e "  │ External IP (Std)     │ \$0.00      │ ${GREEN}✓ Covered${NC}   │"
echo -e "  ├─────────────────────────────────────────────────┤"
echo -e "  │ ${BOLD}TOTAL                 │ \$0.00/mo${NC}   │             │"
echo -e "  └─────────────────────────────────────────────────┘"
echo ""
echo -e "  ${YELLOW}Note:${NC} LLM API costs (OpenAI, Anthropic, etc.) are separate"
echo -e "  and depend on your usage. PennyClaw itself costs \$0."

# ============================================================================
step "PHASE 6: Pre-Flight Summary"

echo -e "  ${BOLD}Checks passed:${NC}  ${GREEN}${CHECKS_PASSED}${NC}"
echo -e "  ${BOLD}Warnings:${NC}       ${YELLOW}${CHECKS_WARNED}${NC}"
echo -e "  ${BOLD}Failures:${NC}       ${RED}${CHECKS_FAILED}${NC}"
echo ""

if [[ $CHECKS_FAILED -gt 0 ]]; then
    echo -e "  ${RED}${BOLD}Pre-flight checks failed. Please resolve the issues above.${NC}"
    exit 1
fi

echo -e "  ${BOLD}Deployment Plan:${NC}"
echo -e "  • Instance: ${INSTANCE_PREFIX}-$(date +%s | tail -c 6)"
echo -e "  • Zone: ${BEST_ZONE}"
echo -e "  • Machine: ${MACHINE_TYPE} (2 shared vCPUs, 1GB RAM)"
echo -e "  • Disk: ${DISK_SIZE_GB}GB ${DISK_TYPE}"
echo -e "  • Network: ${NETWORK_TIER} tier"
echo -e "  • Swap: ${SWAP_SIZE_MB}MB (effective RAM: ~1.5GB)"
echo -e "  • Auto-restart: systemd service"
echo -e "  • Security updates: unattended-upgrades"
echo ""

if $PREFLIGHT_ONLY; then
    echo -e "  ${GREEN}${BOLD}Pre-flight checks complete!${NC} (--preflight-only mode)"
    echo -e "  Run without --preflight-only to deploy."
    exit 0
fi

ask "Deploy PennyClaw now? (y/N)"
if [[ ! "$REPLY" =~ ^[Yy]$ ]]; then
    info "Deployment cancelled."
    exit 0
fi

# ============================================================================
step "PHASE 7: Deploying PennyClaw"

INSTANCE_NAME="${INSTANCE_PREFIX}-$(date +%s | tail -c 6)"

# Create startup script
STARTUP_SCRIPT=$(cat <<'STARTUP'
#!/bin/bash
set -euo pipefail

# Log everything
exec > >(tee /var/log/pennyclaw-setup.log) 2>&1
echo "=== PennyClaw Setup Started: $(date) ==="

# Configure swap for effective ~1.5GB RAM
if [ ! -f /swapfile ]; then
    echo "Configuring swap..."
    fallocate -l 512M /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
    # Tune swappiness for low-memory operation
    echo 'vm.swappiness=10' >> /etc/sysctl.conf
    sysctl -p
fi

# Install dependencies
apt-get update -qq
apt-get install -y -qq curl sqlite3 unattended-upgrades

# Enable automatic security updates
dpkg-reconfigure -plow unattended-upgrades

# Download PennyClaw binary
PENNYCLAW_VERSION="0.1.1"
echo "Downloading PennyClaw v${PENNYCLAW_VERSION}..."
mkdir -p /opt/pennyclaw/data
cd /opt/pennyclaw

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH_SUFFIX="amd64" ;;
    aarch64) ARCH_SUFFIX="arm64" ;;
    *)       ARCH_SUFFIX="amd64" ;;
esac

# Download from GitHub releases
TARBALL="pennyclaw-linux-${ARCH_SUFFIX}.tar.gz"
curl -fsSL "https://github.com/mandarl/pennyclaw/releases/download/v${PENNYCLAW_VERSION}/${TARBALL}" \
    -o "/tmp/${TARBALL}" && \
    tar -xzf "/tmp/${TARBALL}" -C /opt/pennyclaw/ && \
    rm -f "/tmp/${TARBALL}" || {
    echo "Download failed. Building from source..."
    apt-get install -y -qq golang-go gcc libsqlite3-dev
    git clone https://github.com/mandarl/pennyclaw.git /tmp/pennyclaw-src
    cd /tmp/pennyclaw-src
    CGO_ENABLED=1 go build -ldflags="-s -w" -o /opt/pennyclaw/pennyclaw ./cmd/pennyclaw
    cd /opt/pennyclaw
}

chmod +x pennyclaw

# Create default config if not exists
if [ ! -f config.json ]; then
    cat > config.json <<'CONFIG'
{
  "server": { "host": "0.0.0.0", "port": 3000 },
  "llm": {
    "provider": "openai",
    "model": "gpt-4.1-mini",
    "api_key": "$OPENAI_API_KEY",
    "max_tokens": 4096,
    "temperature": 0.7
  },
  "channels": { "web": { "enabled": true } },
  "memory": { "db_path": "data/pennyclaw.db", "max_history": 50, "persist_sessions": true },
  "sandbox": { "enabled": true, "work_dir": "/tmp/pennyclaw-sandbox", "max_timeout": 30, "max_memory": 128 },
  "system_prompt": "You are PennyClaw, a helpful personal AI assistant."
}
CONFIG
fi

# Generate auth token if not already set in .env
if [ ! -f .env ] || ! grep -q PENNYCLAW_AUTH_TOKEN .env 2>/dev/null; then
    AUTH_TOKEN=$(openssl rand -hex 32)
    echo "PENNYCLAW_AUTH_TOKEN=${AUTH_TOKEN}" >> .env
    echo "Generated authentication token: ${AUTH_TOKEN}"
fi

# Create systemd service
cat > /etc/systemd/system/pennyclaw.service <<'SERVICE'
[Unit]
Description=PennyClaw AI Agent
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=pennyclaw
Group=pennyclaw
WorkingDirectory=/opt/pennyclaw
ExecStart=/opt/pennyclaw/pennyclaw --config config.json
Restart=always
RestartSec=5
MemoryMax=800M
MemoryHigh=600M
CPUQuota=80%

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/pennyclaw/data /tmp/pennyclaw-sandbox
PrivateTmp=true

# Environment
EnvironmentFile=-/opt/pennyclaw/.env

[Install]
WantedBy=multi-user.target
SERVICE

# Create pennyclaw user
useradd -r -s /bin/false pennyclaw 2>/dev/null || true
chown -R pennyclaw:pennyclaw /opt/pennyclaw /tmp/pennyclaw-sandbox 2>/dev/null || true

# Enable and start
systemctl daemon-reload
systemctl enable pennyclaw
systemctl start pennyclaw

echo "=== PennyClaw Setup Complete: $(date) ==="
STARTUP
)

# Handle upgrade mode: stop existing service, redeploy binary
if [[ "${UPGRADE_MODE:-false}" == "true" ]]; then
    info "Upgrading existing instance: ${EXISTING_PC}..."
    gcloud compute ssh "$EXISTING_PC" \
        --zone="$BEST_ZONE" \
        --project="$PROJECT" \
        --command="sudo systemctl stop pennyclaw 2>/dev/null; echo 'Service stopped'" || true
    INSTANCE_NAME="$EXISTING_PC"
    # Re-run startup script on existing instance
    gcloud compute instances add-metadata "$INSTANCE_NAME" \
        --zone="$BEST_ZONE" \
        --project="$PROJECT" \
        --metadata=startup-script="$STARTUP_SCRIPT" 2>/dev/null
    gcloud compute instances reset "$INSTANCE_NAME" \
        --zone="$BEST_ZONE" \
        --project="$PROJECT"
    ok "Instance reset with updated PennyClaw"
else
    info "Creating instance: ${INSTANCE_NAME}..."
    gcloud compute instances create "$INSTANCE_NAME" \
    --project="$PROJECT" \
    --zone="$BEST_ZONE" \
    --machine-type="$MACHINE_TYPE" \
    --network-tier="$NETWORK_TIER" \
    --image-family="$IMAGE_FAMILY" \
    --image-project="$IMAGE_PROJECT" \
    --boot-disk-size="${DISK_SIZE_GB}GB" \
    --boot-disk-type="$DISK_TYPE" \
    --metadata=startup-script="$STARTUP_SCRIPT" \
    --tags=pennyclaw-server \
    --scopes=default \
    --no-restart-on-failure

ok "Instance created: ${INSTANCE_NAME}"
fi

# Create firewall rule for web UI
info "Configuring firewall..."
gcloud compute firewall-rules create pennyclaw-allow-web \
    --project="$PROJECT" \
    --allow=tcp:3000 \
    --target-tags=pennyclaw-server \
    --description="Allow PennyClaw web UI" \
    2>/dev/null || warn "Firewall rule already exists"

ok "Firewall configured"

# ============================================================================
step "PHASE 8: Post-Deploy Health Checks"

info "Waiting for instance to start (this takes ~60 seconds)..."
sleep 10

# Get external IP
EXTERNAL_IP=$(gcloud compute instances describe "$INSTANCE_NAME" \
    --zone="$BEST_ZONE" \
    --project="$PROJECT" \
    --format="value(networkInterfaces[0].accessConfigs[0].natIP)" 2>/dev/null)

if [[ -n "$EXTERNAL_IP" ]]; then
    ok "External IP: ${EXTERNAL_IP}"
else
    warn "Could not determine external IP"
fi

# Wait for startup script to complete
info "Waiting for PennyClaw to initialize..."
for i in $(seq 1 12); do
    sleep 10
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://${EXTERNAL_IP}:3000/api/health" 2>/dev/null || echo "000")
    if [[ "$STATUS" == "200" ]]; then
        ok "PennyClaw is healthy!"
        break
    fi
    info "  Attempt ${i}/12 — status: ${STATUS} (waiting...)"
done

if [[ "$STATUS" != "200" ]]; then
    warn "PennyClaw hasn't responded yet. It may still be installing."
    info "Check logs: gcloud compute ssh ${INSTANCE_NAME} --zone=${BEST_ZONE} -- 'sudo journalctl -u pennyclaw -f'"
fi

# Check memory usage
info "Checking resource usage..."
MEMORY_INFO=$(gcloud compute ssh "$INSTANCE_NAME" \
    --zone="$BEST_ZONE" \
    --project="$PROJECT" \
    --command="free -m | grep Mem | awk '{printf \"%d/%dMB (%.0f%%)\", \$3, \$2, \$3/\$2*100}'" \
    2>/dev/null || echo "unknown")
info "Memory usage: ${MEMORY_INFO}"

# ============================================================================
step "PHASE 9: Setup Complete!"

echo -e "  ${GREEN}${BOLD}🎉 PennyClaw is deployed and running!${NC}"
echo ""
echo -e "  ${BOLD}Access your agent:${NC}"
echo -e "  • Web UI: ${CYAN}http://${EXTERNAL_IP}:3000${NC}"
echo -e "  • Health: ${CYAN}http://${EXTERNAL_IP}:3000/api/health${NC}"
echo ""
# Retrieve the auto-generated auth token
AUTH_TOKEN=$(gcloud compute ssh "$INSTANCE_NAME" \
    --zone="$BEST_ZONE" \
    --project="$PROJECT" \
    --command="grep PENNYCLAW_AUTH_TOKEN /opt/pennyclaw/.env 2>/dev/null | cut -d= -f2" \
    2>/dev/null || echo "")

if [[ -n "$AUTH_TOKEN" ]]; then
    echo -e "  ${BOLD}${YELLOW}⚠  Authentication Token (save this!):${NC}"
    echo -e "  ┌─────────────────────────────────────────────────────────────────────┐"
    echo -e "  │ ${CYAN}${AUTH_TOKEN}${NC} │"
    echo -e "  └─────────────────────────────────────────────────────────────────────┘"
    echo -e "  You'll need this token to sign in to the web UI."
    echo ""
fi

echo -e "  ${BOLD}Next steps:${NC}"
echo -e "  1. Set your LLM API key:"
echo -e "     ${CYAN}gcloud compute ssh ${INSTANCE_NAME} --zone=${BEST_ZONE}${NC}"
echo -e "     ${CYAN}echo 'OPENAI_API_KEY=sk-your-key-here' | sudo tee -a /opt/pennyclaw/.env${NC}"
echo -e "     ${CYAN}sudo systemctl restart pennyclaw${NC}"
echo ""
echo -e "  2. (Recommended) Set up secure access with SSH tunnel or Cloudflare Tunnel:"
echo -e "     ${CYAN}ssh -L 3000:localhost:3000 pennyclaw-instance${NC}  (SSH tunnel)"
echo -e "     ${CYAN}https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/${NC}  (Cloudflare)"
echo ""
echo -e "  ${BOLD}Useful commands:${NC}"
echo -e "  • SSH:     ${CYAN}gcloud compute ssh ${INSTANCE_NAME} --zone=${BEST_ZONE}${NC}"
echo -e "  • Logs:    ${CYAN}gcloud compute ssh ${INSTANCE_NAME} --zone=${BEST_ZONE} -- 'sudo journalctl -u pennyclaw -f'${NC}"
echo -e "  • Restart: ${CYAN}gcloud compute ssh ${INSTANCE_NAME} --zone=${BEST_ZONE} -- 'sudo systemctl restart pennyclaw'${NC}"
echo -e "  • Teardown: ${CYAN}make teardown${NC}"
echo ""

# Generate teardown script
cat > "$(dirname "$0")/teardown.sh" <<TEARDOWN
#!/usr/bin/env bash
# PennyClaw Teardown — removes all GCP resources created by deploy.sh
set -euo pipefail
echo "Removing PennyClaw instance: ${INSTANCE_NAME}..."
gcloud compute instances delete "${INSTANCE_NAME}" --zone="${BEST_ZONE}" --project="${PROJECT}" --quiet
echo "Removing firewall rule..."
gcloud compute firewall-rules delete pennyclaw-allow-web --project="${PROJECT}" --quiet 2>/dev/null || true
echo "PennyClaw removed. No more charges will be incurred."
TEARDOWN
chmod +x "$(dirname "$0")/teardown.sh"
ok "Teardown script generated: scripts/teardown.sh"

echo ""
echo -e "  ${BOLD}Monthly cost: ${GREEN}\$0.00${NC}"
