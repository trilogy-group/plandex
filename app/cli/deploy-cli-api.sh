#!/usr/bin/env bash

set -e

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

# Dynamic paths - works from any deployment location
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_PATH="$SCRIPT_DIR/plandex-cli-api"
CONFIG_PATH="$SCRIPT_DIR/plandex-api.json"
PID_FILE="$SCRIPT_DIR/plandex-api.pid"
LOG_FILE="$SCRIPT_DIR/plandex-api.log"
SERVICE_NAME="plandex-cli-api"
SERVICE_FILE="$HOME/.config/systemd/user/${SERVICE_NAME}.service"
CURRENT_USER="$(whoami)"

echo -e "${BLUE}🚀 Plandex CLI API Deployment${NC}"
echo "📍 Deployment directory: $SCRIPT_DIR"
echo

# Check prerequisites
check_prerequisites() {
    if [ ! -f "$BINARY_PATH" ]; then
        print_error "Binary not found at $BINARY_PATH"
        print_info "Run './install-cli-api.sh' first"
        exit 1
    fi

    if [ ! -f "$CONFIG_PATH" ]; then
        print_error "Configuration not found at $CONFIG_PATH"
        print_info "Run './install-cli-api.sh' to create configuration file first"
        exit 1
    fi
}

# Option 1: Run locally (foreground)
run_local() {
    print_info "Starting Plandex CLI API locally..."
    
    check_prerequisites
    
    # Get configuration details
    if command -v jq &> /dev/null && [ -f "$CONFIG_PATH" ]; then
        PORT=$(jq -r '.server.port // 8080' "$CONFIG_PATH" 2>/dev/null)
        API_KEY=$(jq -r '.auth.api_keys[0] // "not-configured"' "$CONFIG_PATH" 2>/dev/null)
    else
        PORT=8080
        API_KEY="not-configured"
    fi
    
    echo
    print_success "Configuration loaded:"
    echo "  📂 Working directory: $SCRIPT_DIR"
    echo "  🔧 Config file: $CONFIG_PATH"
    echo "  🚪 Port: $PORT"
    echo "  🔑 API Key: ${API_KEY:0:8}...${API_KEY: -8}"
    echo "  📝 Logs: Will display in console"
    
    echo
    print_info "API will be available at: http://localhost:$PORT"
    print_info "Health check: curl http://localhost:$PORT/api/v1/health"
    print_info "With API key: curl -H \"X-API-Key: $API_KEY\" http://localhost:$PORT/api/v1/health"
    echo
    print_info "Press Ctrl+C to stop"
    echo
    
    # Start the API
    cd "$SCRIPT_DIR"
    exec "$BINARY_PATH" --server --config "$CONFIG_PATH"
}

# Option 3: Setup autostart (systemd with user lingering)
setup_autostart() {
    print_info "Setting up auto-start on boot..."
    
    check_prerequisites
    
    # Check systemd availability
    if ! command -v systemctl &> /dev/null; then
        print_error "systemctl not found. This system doesn't support systemd user services."
        exit 1
    fi
    
    # Create systemd user directory
    mkdir -p "$(dirname "$SERVICE_FILE")"
    
    # Detect current environment
    print_info "Capturing current environment..."
    
    # Get current PATH and important environment variables
    FULL_PATH="$PATH"
    ENV_VARS=""
    for var in GOPATH GOROOT PLANDEX_ENV OPENAI_API_KEY ANTHROPIC_API_KEY AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY; do
        if [ -n "${!var}" ]; then
            ENV_VARS="${ENV_VARS}Environment=${var}=${!var}\n"
        fi
    done
    
    # Create startup wrapper script
    WRAPPER_SCRIPT="$SCRIPT_DIR/start-with-environment.sh"
    cat > "$WRAPPER_SCRIPT" << EOF
#!/usr/bin/env bash

# Auto-generated startup wrapper for Plandex CLI API
# Ensures full user environment access

set -e

# Load user environment for CLI access
if [ -f "\$HOME/.bashrc" ]; then
    source "\$HOME/.bashrc"
fi

if [ -f "\$HOME/.profile" ]; then
    source "\$HOME/.profile"
fi

# Ensure Go is in PATH
if [ -d "/usr/local/go/bin" ]; then
    export PATH="/usr/local/go/bin:\$PATH"
fi

# Set GOPATH if not set
if [ -z "\$GOPATH" ]; then
    export GOPATH="\$HOME/go"
fi

# Change to deployment directory
cd "$SCRIPT_DIR"

# Start the API
exec "$BINARY_PATH" --server --config "$CONFIG_PATH"
EOF
    
    chmod +x "$WRAPPER_SCRIPT"
    print_success "Created startup wrapper: $WRAPPER_SCRIPT"
    
    # Create systemd user service
    cat > "$SERVICE_FILE" << EOF
[Unit]
Description=Plandex CLI API Server
After=graphical-session.target
Wants=graphical-session.target

[Service]
Type=exec
User=${CURRENT_USER}
Group=${CURRENT_USER}
WorkingDirectory=${SCRIPT_DIR}
ExecStart=${WRAPPER_SCRIPT}
Restart=always
RestartSec=10
KillMode=mixed
TimeoutStopSec=30

# Environment setup for CLI access
Environment=PATH=${FULL_PATH}
Environment=HOME=${HOME}
Environment=USER=${CURRENT_USER}
Environment=LOGNAME=${CURRENT_USER}
$(echo -e "$ENV_VARS")

# User runtime directory
Environment=XDG_RUNTIME_DIR=/run/user/$(id -u)

[Install]
WantedBy=default.target
EOF
    
    print_success "Created systemd user service: $SERVICE_FILE"
    
    # Reload systemd and enable service
    systemctl --user daemon-reload
    
    if systemctl --user enable "$SERVICE_NAME"; then
        print_success "Auto-start enabled"
    else
        print_error "Failed to enable auto-start"
        exit 1
    fi
    
    # Enable user lingering (allows service to start without login)
    if command -v loginctl &> /dev/null; then
        if loginctl enable-linger "$CURRENT_USER" 2>/dev/null; then
            print_success "User lingering enabled (starts without login)"
        else
            print_warning "Could not enable user lingering - may need admin privileges"
            print_info "Service will start after user login"
        fi
    fi
    
    echo
    print_success "Auto-start configuration complete!"
    echo
    print_info "The API will now:"
    echo "  - Start automatically on system boot"
    echo "  - Restart automatically if it crashes"
    echo "  - Run with full CLI environment access"
    echo "  - Work from deployment directory: $SCRIPT_DIR"
    echo
    print_info "Management commands:"
    echo "  systemctl --user start $SERVICE_NAME"
    echo "  systemctl --user stop $SERVICE_NAME"
    echo "  systemctl --user status $SERVICE_NAME"
    echo "  systemctl --user restart $SERVICE_NAME"
    echo "  journalctl --user -u $SERVICE_NAME -f"
    echo
    print_info "To disable: systemctl --user disable $SERVICE_NAME"
}

# Disable autostart
disable_autostart() {
    print_info "Disabling auto-start..."
    
    # Stop service if running
    if systemctl --user is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        systemctl --user stop "$SERVICE_NAME"
        print_success "Stopped service"
    fi
    
    # Disable service
    if systemctl --user disable "$SERVICE_NAME" 2>/dev/null; then
        print_success "Auto-start disabled"
    else
        print_info "Service was not enabled"
    fi
    
    # Remove service file
    if [ -f "$SERVICE_FILE" ]; then
        rm -f "$SERVICE_FILE"
        print_success "Removed service file"
    fi
    
    # Remove wrapper script
    if [ -f "$SCRIPT_DIR/start-with-environment.sh" ]; then
        rm -f "$SCRIPT_DIR/start-with-environment.sh"
        print_success "Removed startup wrapper"
    fi
    
    systemctl --user daemon-reload
    print_success "Auto-start completely removed"
}

# Show status
show_status() {
    echo -e "${BLUE}📊 Deployment Status${NC}"
    echo
    print_info "Deployment directory: $SCRIPT_DIR"
    print_info "Binary: $([ -f "$BINARY_PATH" ] && echo "✅ Found" || echo "❌ Missing")"
    print_info "Config: $([ -f "$CONFIG_PATH" ] && echo "✅ Found" || echo "❌ Missing")"
    echo
    
    if [ -f "$SERVICE_FILE" ]; then
        print_success "Auto-start is configured"
        
        if systemctl --user is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
            print_success "Auto-start is enabled"
        else
            print_warning "Auto-start is configured but not enabled"
        fi
        
        if systemctl --user is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            print_success "Service is currently running"
        else
            print_info "Service is not running"
        fi
        
        echo
        print_info "Service status:"
        systemctl --user status "$SERVICE_NAME" --no-pager -l 2>/dev/null || true
    else
        print_info "Auto-start is not configured"
    fi
    
    echo
    if command -v loginctl &> /dev/null; then
        if loginctl show-user "$CURRENT_USER" --property=Linger | grep -q "Linger=yes"; then
            print_success "User lingering enabled (starts without login)"
        else
            print_info "User lingering disabled (requires login to start)"
        fi
    fi
}

# Main menu
show_menu() {
    echo "Usage: $0 {local|autostart|disable|status}"
    echo
    echo "Commands:"
    echo "  local     - Run API locally in foreground"
    echo "  autostart - Setup auto-start on boot with systemd"
    echo "  disable   - Disable auto-start"
    echo "  status    - Show deployment status"
    echo
    echo "Features:"
    echo "  ✅ Path-agnostic (works from any deployment location)"
    echo "  ✅ Full CLI environment access"
    echo "  ✅ Automatic restart on crash (autostart mode)"
    echo "  ✅ Survives system reboots (autostart mode)"
    echo "  ✅ User-level service (no root required)"
}

# Parse global arguments first
SILENT=false
while [[ $# -gt 0 ]]; do
    case $1 in
        --silent|-y|--yes)
            SILENT=true
            shift
            ;;
        local|1|autostart|3|disable|status)
            # Command found, break to handle it
            break
            ;;
        *)
            if [[ "$1" =~ ^-- ]]; then
                echo "Unknown option: $1"
                exit 1
            else
                # Assume it's a command
                break
            fi
            ;;
    esac
done

# Main command handler
case "${1:-}" in
    local|1)
        run_local
        ;;
    autostart|3)
        setup_autostart
        ;;
    disable)
        disable_autostart
        ;;
    status)
        show_status
        ;;
    *)
        show_menu
        exit 0
        ;;
esac 