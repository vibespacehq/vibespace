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
    echo "  - ~/.vibespace (all vibespace data)"
    echo "  - ~/.colima (Colima VM)"
    echo "  - ~/.lima (Lima VM)"
    echo "  - ~/.kube (kubectl config)"
    echo "  - app/dist (frontend build)"
    echo "  - app/src-tauri/target (Rust build)"
    echo ""
    echo "  And kill all running processes:"
    echo "  - vibespace (desktop app)"
    echo "  - colima (VM)"
    echo "  - lima (VM backend)"
    echo "  - kubectl (K8s CLI)"
    echo "  - dnsd (DNS server)"
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

# Kill Go API server (might be running as "main" from go run or as binary)
pkill -9 -f "vibespace-api" 2>/dev/null && echo -e "${GREEN}✓${NC} Killed vibespace-api processes" || echo -e "${GREEN}✓${NC} No vibespace-api processes to kill"
pkill -9 -f "go run.*cmd/server" 2>/dev/null && echo -e "${GREEN}✓${NC} Killed go run server processes" || echo -e "${GREEN}✓${NC} No go run server processes to kill"
# Kill any process listening on API port 8090
lsof -ti:8090 2>/dev/null | xargs kill -9 2>/dev/null && echo -e "${GREEN}✓${NC} Killed process on port 8090" || echo -e "${GREEN}✓${NC} No process on port 8090"
echo ""

# 2. Remove directories
echo "=== Removing Directories ==="
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

# 3. Verify cleanup
echo "=== Verification ==="

# Check processes
if ps aux | grep -E '(colima|kubectl|lima|vibespace|dnsd)' | grep -v grep > /dev/null; then
    echo -e "${RED}✗${NC} Some processes still running:"
    ps aux | grep -E '(colima|kubectl|lima|vibespace|dnsd)' | grep -v grep
else
    echo -e "${GREEN}✓${NC} No vibespace-related processes running"
fi

# Check directories
all_clean=true
for dir in "$HOME/.vibespace" "$HOME/.colima" "$HOME/.lima" "$HOME/.kube" "$REPO_ROOT/app/dist" "$REPO_ROOT/app/src-tauri/target"; do
    if [ -d "$dir" ]; then
        echo -e "${RED}✗${NC} Directory still exists: $dir"
        all_clean=false
    fi
done

if [ "$all_clean" = true ]; then
    echo -e "${GREEN}✓${NC} All directories removed"
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
