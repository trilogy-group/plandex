#!/usr/bin/env bash

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

print_step() {
    echo -e "${BLUE}→ $1${NC}"
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

echo -e "${BLUE}🚀 Plandex CLI API Installer${NC}"
echo

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

print_step "Building Plandex CLI API..."
go mod tidy
go build -o plandex-cli-api -ldflags="-w -s"

if [ ! -f "plandex-cli-api" ]; then
    print_error "Build failed - binary not created"
    exit 1
fi

print_success "Binary built successfully"

# Generate API key if config doesn't exist
if [ ! -f "plandex-api.json" ]; then
    print_step "Creating configuration file..."
    
    API_KEY=$(openssl rand -hex 32)
    
    cat > plandex-api.json << EOF
{
  "server": {
    "port": 8080,
    "host": "127.0.0.1",
    "read_timeout": "30s",
    "write_timeout": "30s",
    "idle_timeout": "60s"
  },
  "auth": {
    "api_keys": ["${API_KEY}"],
    "require_auth": true,
    "token_lifetime": "24h"
  },
  "cli": {
    "working_dir": ".",
    "environment": {
      "PLANDEX_ENV": "production"
    },
    "timeout": "10m"
  },
  "jobs": {
    "max_concurrent": 5,
    "default_ttl": "24h",
    "cleanup_interval": "1h",
    "max_history_size": 1000
  },
  "webhooks": {
    "enabled": false,
    "secret": "",
    "max_retries": 3,
    "retry_backoff": "30s"
  },
  "security": {
    "enable_cors": true,
    "allowed_origins": ["*"],
    "rate_limit": 100,
    "trusted_proxies": []
  }
}
EOF
    
    print_success "Configuration created: plandex-api.json"
    print_warning "Your API Key: ${API_KEY}"
    echo -e "${YELLOW}   Save this key securely!${NC}"
else
    print_success "Using existing configuration: plandex-api.json"
fi

# Make binary executable
chmod +x plandex-cli-api

print_step "Testing installation..."
if ./plandex-cli-api --help > /dev/null 2>&1; then
    print_success "Binary is working correctly"
else
    print_error "Binary test failed"
    exit 1
fi

echo
print_success "Installation completed!"
echo
echo -e "${BLUE}📋 Quick Start - Local Testing:${NC}"
echo "  1. Background:       ./run-background.sh start  # Persistent background"
echo "  2. Test & config:    ./test-config.sh           # Interactive testing menu"
echo "  3. Foreground:       ./run-local.sh             # Start with helpful info"
echo "  4. Manual start:     ./plandex-cli-api --config plandex-api.json"
echo
echo -e "${BLUE}📝 Files created:${NC}"
echo "  - plandex-cli-api    (binary)"
echo "  - plandex-api.json   (configuration)"
echo
echo -e "${BLUE}🚀 For production deployment:${NC}"
echo "  - Run: sudo ./deploy.sh"
echo "  - Or use the service scripts for systemd integration"
echo
echo -e "${BLUE}🌐 For Caddy setup:${NC}"
echo "  - See EXISTING-SERVER.md for path-based setup (/plandex)"
echo "  - See CADDY.md for full domain setup"
echo

# Show API key again
if [ -f "plandex-api.json" ]; then
    API_KEY=$(grep -o '"api_keys": \["[^"]*"' plandex-api.json | cut -d'"' -f4)
    echo -e "${YELLOW}🔑 Your API Key: ${API_KEY}${NC}"
    echo
    echo -e "${BLUE}📋 Example usage on existing server:${NC}"
    echo "  # Add to existing Caddyfile:"
    echo "  reverse_proxy /plandex/* localhost:8080 { rewrite * /api{uri} }"
    echo
    echo "  # Then test:"
    echo "  curl https://your-domain.com/plandex/health"
    echo "  curl -H \"X-API-Key: ${API_KEY}\" \\"
    echo "       -X POST https://your-domain.com/plandex/jobs \\"
    echo "       -d '{\"command\": \"plans\"}'"
fi

echo
echo -e "${BLUE}📚 Next steps:${NC}"
echo "  - Add your model provider API keys to plandex-api.json"
echo "  - Use ./deploy.sh local for testing"
echo "  - Use ./deploy.sh autostart for auto-start on boot"
echo "  - See EXISTING-SERVER.md for adding to your current server" 