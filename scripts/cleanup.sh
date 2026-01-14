#!/bin/bash

# vibespace Fresh Install Cleanup Script
# Removes all vibespace-related directories, processes, and build artifacts
# Usage: ./scripts/cleanup.sh [--force]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if --force flag is provided
FORCE=false
if [[ "$1" == "--force" ]]; then
    FORCE=true
fi

echo "========================================="
echo "  vibespace Fresh Install Cleanup"
echo "========================================="
echo ""

# Warn user
if [ "$FORCE" = false ]; then
    echo -e "${YELLOW}WARNING:${NC} This will remove:"
    echo ""
    echo "  Directories:"
    echo "  - ~/.vibespace (all vibespace data)"
    echo "  - ~/.colima (Colima VM)"
    echo "  - ~/.lima (Lima VM)"
    echo "  - ~/.kube (kubectl config)"
    echo "  - ~/Library/Application Support/mkcert (CA files)"
    echo "  - app/dist (frontend build)"
    echo "  - app/src-tauri/target (Rust build)"
    echo ""
    echo "  System configurations (requires sudo):"
    echo "  - /Library/LaunchDaemons/space.vibe.portfwd.plist (launchd service)"
    echo "  - /etc/resolver/vibe.space (DNS resolver)"
    echo "  - /etc/hosts vibespace-managed entries"
    echo "  - mkcert CA from System keychain"
    echo ""
    echo "  Processes:"
    echo "  - vibespace, colima, lima, kubectl, dnsd, portfwd"
    echo ""
    read -p "Are you sure you want to continue? (y/N): " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Cleanup cancelled."
        exit 0
    fi
fi

echo ""
echo "Starting cleanup..."
echo ""

# Function to kill processes safely
kill_process() {
    local process_name=$1
    if killall -9 "$process_name" 2>/dev/null; then
        echo -e "${GREEN}✓${NC} Killed $process_name processes"
    else
        echo -e "${GREEN}✓${NC} No $process_name processes to kill"
    fi
}

# Function to remove directory safely
remove_dir() {
    local dir=$1
    local display_name=$2
    if [ -d "$dir" ]; then
        rm -rf "$dir"
        echo -e "${GREEN}✓${NC} Removed $display_name"
    else
        echo -e "${GREEN}✓${NC} $display_name (already removed)"
    fi
}

# 1. Kill running processes
echo "=== Killing Processes ==="
kill_process "vibespace"
kill_process "colima"
kill_process "limactl"
kill_process "kubectl"
kill_process "dnsd"
kill_process "portfwd"

# Kill Go API server (might be running as "main" from go run or as binary)
pkill -9 -f "vibespace-api" 2>/dev/null && echo -e "${GREEN}✓${NC} Killed vibespace-api processes" || echo -e "${GREEN}✓${NC} No vibespace-api processes to kill"
pkill -9 -f "go run.*cmd/server" 2>/dev/null && echo -e "${GREEN}✓${NC} Killed go run server processes" || echo -e "${GREEN}✓${NC} No go run server processes to kill"
# Kill any process listening on API port 8090
lsof -ti:8090 2>/dev/null | xargs kill -9 2>/dev/null && echo -e "${GREEN}✓${NC} Killed process on port 8090" || echo -e "${GREEN}✓${NC} No process on port 8090"
echo ""

# 2. Remove system configurations (requires sudo)
echo "=== Removing System Configurations ==="
echo "Note: This section requires administrator privileges"
echo ""

# Unload and remove launchd portfwd service
LAUNCHD_PLIST="/Library/LaunchDaemons/space.vibe.portfwd.plist"
if [ -f "$LAUNCHD_PLIST" ]; then
    sudo launchctl unload "$LAUNCHD_PLIST" 2>/dev/null || true
    sudo rm -f "$LAUNCHD_PLIST"
    echo -e "${GREEN}✓${NC} Removed launchd portfwd service"
else
    echo -e "${GREEN}✓${NC} launchd portfwd service (not installed)"
fi

# Remove DNS resolver configuration
DNS_RESOLVER="/etc/resolver/vibe.space"
if [ -f "$DNS_RESOLVER" ]; then
    sudo rm -f "$DNS_RESOLVER"
    echo -e "${GREEN}✓${NC} Removed DNS resolver ($DNS_RESOLVER)"
else
    echo -e "${GREEN}✓${NC} DNS resolver (not configured)"
fi

# Clean vibespace-managed entries from /etc/hosts
if grep -q "vibespace-managed" /etc/hosts 2>/dev/null; then
    # Create backup
    sudo cp /etc/hosts /etc/hosts.vibespace-backup
    # Remove vibespace-managed lines
    sudo sed -i '' '/vibespace-managed/d' /etc/hosts
    # Also remove any standalone vibe.space entries
    sudo sed -i '' '/\.vibe\.space$/d' /etc/hosts
    echo -e "${GREEN}✓${NC} Cleaned /etc/hosts (backup at /etc/hosts.vibespace-backup)"
else
    echo -e "${GREEN}✓${NC} /etc/hosts (no vibespace entries)"
fi

# Remove ALL mkcert CAs from System keychain (may be multiple from different hostnames)
mkcert_removed=0
while security find-certificate -c "mkcert" /Library/Keychains/System.keychain >/dev/null 2>&1; do
    sudo security delete-certificate -c "mkcert" /Library/Keychains/System.keychain 2>/dev/null || break
    mkcert_removed=$((mkcert_removed + 1))
done
if [ $mkcert_removed -gt 0 ]; then
    echo -e "${GREEN}✓${NC} Removed $mkcert_removed mkcert CA(s) from System keychain"
else
    echo -e "${GREEN}✓${NC} mkcert CA (not in System keychain)"
fi

# Remove mkcert CA files
MKCERT_DIR="$HOME/Library/Application Support/mkcert"
if [ -d "$MKCERT_DIR" ]; then
    rm -rf "$MKCERT_DIR"
    echo -e "${GREEN}✓${NC} Removed mkcert CA files"
else
    echo -e "${GREEN}✓${NC} mkcert CA files (not present)"
fi

echo ""

# 3. Remove user directories
echo "=== Removing User Directories ==="
remove_dir "$HOME/.vibespace" "~/.vibespace"
remove_dir "$HOME/.colima" "~/.colima"
remove_dir "$HOME/.lima" "~/.lima"
remove_dir "$HOME/.kube" "~/.kube"

# Build directories (relative to script location)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"

remove_dir "$REPO_ROOT/app/dist" "app/dist"
remove_dir "$REPO_ROOT/app/src-tauri/target" "app/src-tauri/target"

# Remove API binary if it exists
if [ -f "$REPO_ROOT/api/vibespace-api" ]; then
    rm -f "$REPO_ROOT/api/vibespace-api"
    echo -e "${GREEN}✓${NC} Removed api/vibespace-api binary"
else
    echo -e "${GREEN}✓${NC} api/vibespace-api binary (already removed)"
fi
echo ""

# 4. Verify cleanup
echo "=== Verification ==="

# Check processes
if ps aux | grep -E '(colima|kubectl|lima|vibespace|dnsd|portfwd)' | grep -v grep > /dev/null; then
    echo -e "${RED}✗${NC} Some processes still running:"
    ps aux | grep -E '(colima|kubectl|lima|vibespace|dnsd|portfwd)' | grep -v grep
else
    echo -e "${GREEN}✓${NC} No vibespace-related processes running"
fi

# Check system configurations
all_clean=true

if [ -f "/Library/LaunchDaemons/space.vibe.portfwd.plist" ]; then
    echo -e "${RED}✗${NC} launchd plist still exists"
    all_clean=false
fi

if [ -f "/etc/resolver/vibe.space" ]; then
    echo -e "${RED}✗${NC} DNS resolver still exists"
    all_clean=false
fi

if grep -q "vibespace-managed" /etc/hosts 2>/dev/null; then
    echo -e "${RED}✗${NC} /etc/hosts still has vibespace entries"
    all_clean=false
fi

if security find-certificate -c "mkcert" /Library/Keychains/System.keychain >/dev/null 2>&1; then
    echo -e "${RED}✗${NC} mkcert CA still in System keychain"
    all_clean=false
fi

# Check directories
for dir in "$HOME/.vibespace" "$HOME/.colima" "$HOME/.lima" "$HOME/.kube" "$HOME/Library/Application Support/mkcert" "$REPO_ROOT/app/dist" "$REPO_ROOT/app/src-tauri/target"; do
    if [ -d "$dir" ]; then
        echo -e "${RED}✗${NC} Directory still exists: $dir"
        all_clean=false
    fi
done

if [ "$all_clean" = true ]; then
    echo -e "${GREEN}✓${NC} All cleanup verified"
fi

echo ""
echo "========================================="
if [ "$all_clean" = true ]; then
    echo -e "${GREEN}✓ Cleanup complete! Ready for fresh install.${NC}"
else
    echo -e "${YELLOW}⚠ Cleanup incomplete. Please check errors above.${NC}"
    exit 1
fi
echo "========================================="
