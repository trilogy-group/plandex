#!/usr/bin/env bash

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

echo -e "${BLUE}🗑️  Plandex CLI API Local Uninstall${NC}"
echo

# Show what will be removed
print_info "Files that will be removed from $(pwd):"
echo

LOCAL_FILES=(
    "plandex-cli-api"
    "plandex-api.json"
    "plandex-api.log"
    "plandex-api.pid"
    "start-with-environment.sh"
)

FOUND_FILES=()

for file in "${LOCAL_FILES[@]}"; do
    if [ -f "$file" ]; then
        echo "  📄 $file"
        FOUND_FILES+=("$file")
    fi
done

if [ ${#FOUND_FILES[@]} -eq 0 ]; then
    print_warning "No Plandex CLI API files found in current directory."
    echo "Nothing to uninstall."
    exit 0
fi

echo

# Confirmation
print_warning "This will remove ${#FOUND_FILES[@]} file(s) from the current directory."
echo "The following will NOT be removed:"
echo "  - System dependencies (Go, curl, jq, etc.)"
echo "  - Caddy web server"
echo "  - Development scripts and documentation"
echo

read -p "Are you sure you want to uninstall? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Uninstall cancelled."
    exit 0
fi

echo

# Stop any running processes and disable autostart
print_info "Stopping any running API processes and disabling autostart..."

# Disable autostart if configured
if [ -f "deploy.sh" ]; then
    ./deploy.sh disable 2>/dev/null || print_warning "Could not disable autostart"
fi

# Fallback to manual process termination
if pgrep -f "plandex-cli-api" > /dev/null; then
    pkill -f "plandex-cli-api" && print_success "Stopped running API processes"
else
    print_info "No running API processes found"
fi

# Remove files
print_info "Removing files..."
for file in "${FOUND_FILES[@]}"; do
    if rm -f "$file" 2>/dev/null; then
        print_success "Removed $file"
    else
        print_error "Failed to remove $file"
    fi
done

echo

print_success "Local uninstall completed!"
echo
print_info "What was removed:"
echo "  ✅ API binary (plandex-cli-api)"
echo "  ✅ Configuration file (plandex-api.json)"
echo "  ✅ Log files (if any)"
echo

print_info "What remains:"
echo "  📂 All scripts and documentation"
echo "  🔧 System dependencies (Go, curl, jq, etc.)"
echo "  🌐 Caddy web server (if installed)"
echo

print_info "To reinstall:"
echo "  ./install.sh          # Just the API"
echo "  ./setup.sh            # Complete setup"
echo

print_info "To remove system dependencies:"
echo "  ./uninstall-system.sh # Removes Go, dependencies, Caddy" 