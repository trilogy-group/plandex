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

# Check if running as root for system uninstall
if [[ $EUID -ne 0 ]] && ! command -v sudo &> /dev/null; then
    print_error "This script requires sudo access or root privileges for system uninstall"
    exit 1
fi

# Function to run commands with appropriate privileges
run_privileged() {
    if [[ $EUID -eq 0 ]]; then
        "$@"
    else
        sudo "$@"
    fi
}

echo -e "${BLUE}🗑️  Plandex CLI API System Uninstall${NC}"
echo

SERVICE_NAME="plandex-cli-api"
INSTALL_DIR="/opt/plandex-cli-api"
CONFIG_DIR="/etc/plandex-cli-api"
LOG_DIR="/var/log/plandex-cli-api"
SERVICE_USER="plandex"

# Check what's installed
print_info "Checking system installation..."
SYSTEM_ITEMS_FOUND=()

if systemctl list-unit-files | grep -q "^${SERVICE_NAME}.service"; then
    SYSTEM_ITEMS_FOUND+=("systemd service")
fi

if [ -d "$INSTALL_DIR" ]; then
    SYSTEM_ITEMS_FOUND+=("installation directory")
fi

if [ -d "$CONFIG_DIR" ]; then
    SYSTEM_ITEMS_FOUND+=("configuration directory")
fi

if [ -d "$LOG_DIR" ]; then
    SYSTEM_ITEMS_FOUND+=("log directory")
fi

if id "$SERVICE_USER" &>/dev/null; then
    SYSTEM_ITEMS_FOUND+=("service user")
fi

if [ ${#SYSTEM_ITEMS_FOUND[@]} -eq 0 ]; then
    print_warning "No system installation found."
    print_info "Nothing to uninstall."
    exit 0
fi

echo
print_info "System components found:"
for item in "${SYSTEM_ITEMS_FOUND[@]}"; do
    echo "  📦 $item"
done

echo
print_warning "This will remove the system-wide Plandex CLI API installation."
echo "The following will be removed:"
echo "  🗑️  Systemd service (${SERVICE_NAME})"
echo "  🗑️  Installation directory (${INSTALL_DIR})"
echo "  🗑️  Configuration directory (${CONFIG_DIR})"
echo "  🗑️  Log directory (${LOG_DIR})"
echo "  🗑️  Service user (${SERVICE_USER})"
echo

# Ask about dependencies
echo -e "${YELLOW}Optional: Remove system dependencies?${NC}"
echo "This includes Go, build tools, and other packages installed by install-requirements.sh"
echo "⚠️  Warning: This may affect other applications using these tools"
echo
read -p "Remove system dependencies? (y/N): " -n 1 -r
echo
REMOVE_DEPS=""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    REMOVE_DEPS="yes"
fi

echo
read -p "Are you sure you want to uninstall the system installation? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Uninstall cancelled."
    exit 0
fi

echo

# Stop and disable service
if systemctl list-unit-files | grep -q "^${SERVICE_NAME}.service"; then
    print_info "Stopping and disabling service..."
    
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        run_privileged systemctl stop "$SERVICE_NAME"
        print_success "Service stopped"
    fi
    
    if systemctl is-enabled --quiet "$SERVICE_NAME"; then
        run_privileged systemctl disable "$SERVICE_NAME"
        print_success "Service disabled"
    fi
    
    # Remove service file
    if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
        run_privileged rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
        run_privileged systemctl daemon-reload
        print_success "Service file removed"
    fi
fi

# Remove directories
print_info "Removing directories..."

if [ -d "$INSTALL_DIR" ]; then
    run_privileged rm -rf "$INSTALL_DIR"
    print_success "Removed installation directory"
fi

if [ -d "$CONFIG_DIR" ]; then
    run_privileged rm -rf "$CONFIG_DIR"
    print_success "Removed configuration directory"
fi

if [ -d "$LOG_DIR" ]; then
    run_privileged rm -rf "$LOG_DIR"
    print_success "Removed log directory"
fi

# Remove service user
if id "$SERVICE_USER" &>/dev/null; then
    print_info "Removing service user..."
    run_privileged userdel "$SERVICE_USER" 2>/dev/null || print_warning "Could not remove user (may have active processes)"
    print_success "Service user removed"
fi

# Remove dependencies if requested
if [ "$REMOVE_DEPS" = "yes" ]; then
    print_info "Removing system dependencies..."
    
    # Detect OS
    if command -v apt-get &> /dev/null; then
        OS="ubuntu"
    elif command -v yum &> /dev/null; then
        OS="centos"
    elif command -v dnf &> /dev/null; then
        OS="fedora"
    else
        print_warning "Unknown OS - skipping dependency removal"
        OS="unknown"
    fi
    
    if [ "$OS" != "unknown" ]; then
        # Remove Go
        if [ -d "/usr/local/go" ]; then
            run_privileged rm -rf /usr/local/go
            print_success "Removed Go installation"
        fi
        
        # Remove Caddy if installed via our script
        if command -v caddy &> /dev/null; then
            print_info "Caddy is installed. Remove it? (y/N): "
            read -p "" -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                case $OS in
                    ubuntu)
                        run_privileged apt-get remove -y caddy
                        run_privileged apt-get autoremove -y
                        ;;
                    centos|fedora)
                        run_privileged rm -f /usr/local/bin/caddy
                        run_privileged rm -f /etc/systemd/system/caddy.service
                        if id "caddy" &>/dev/null; then
                            run_privileged userdel caddy
                        fi
                        run_privileged systemctl daemon-reload
                        ;;
                esac
                print_success "Removed Caddy"
            fi
        fi
        
        print_warning "Note: Basic system packages (curl, wget, jq, build-essential) were NOT removed"
        print_warning "as they may be used by other applications."
    fi
fi

echo

print_success "System uninstall completed!"
echo
print_info "What was removed:"
echo "  ✅ Systemd service"
echo "  ✅ System directories and files"
echo "  ✅ Service user"
if [ "$REMOVE_DEPS" = "yes" ]; then
    echo "  ✅ Go programming language"
    echo "  ✅ Caddy web server (if selected)"
fi

echo
print_info "What remains:"
echo "  📂 This directory and all scripts"
echo "  📂 Any local installations (run ./uninstall-local.sh)"
if [ "$REMOVE_DEPS" != "yes" ]; then
    echo "  🔧 System dependencies (Go, curl, jq, etc.)"
fi

echo
print_info "To reinstall:"
echo "  ./setup.sh            # Complete setup"
echo "  sudo ./deploy.sh      # System service only" 