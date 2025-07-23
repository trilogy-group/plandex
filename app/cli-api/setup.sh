#!/usr/bin/env bash

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🚀 Plandex CLI API Complete Setup${NC}"
echo
echo "This script will:"
echo "  1. Install all system requirements (Go, dependencies, optionally Caddy)"
echo "  2. Build and configure the Plandex CLI API"
echo "  3. Provide usage instructions"
echo

read -p "Continue with setup? (Y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Nn]$ ]]; then
    echo "Setup cancelled."
    exit 0
fi

echo -e "${BLUE}Step 1: Installing requirements...${NC}"
echo
./install-requirements.sh

echo
echo -e "${BLUE}Step 2: Installing Plandex CLI API...${NC}"
echo
./install.sh

echo
echo -e "${GREEN}🎉 Complete setup finished!${NC}"
echo
echo -e "${BLUE}📋 What's been set up:${NC}"
echo "  ✅ All system dependencies"
echo "  ✅ Go programming language"
echo "  ✅ Plandex CLI API binary"
echo "  ✅ Configuration with API key"
echo "  ✅ Development tools"

if command -v caddy &> /dev/null; then
    echo "  ✅ Caddy web server"
fi

echo
echo -e "${BLUE}🚀 Quick Start:${NC}"
echo "  # Start the API:"
echo "  ./plandex-cli-api --config plandex-api.json"
echo
echo "  # Test it:"
echo "  curl http://localhost:8080/api/v1/health"
echo
echo "  # For production:"
echo "  sudo ./deploy.sh"
echo
echo -e "${BLUE}📚 Next Steps:${NC}"
echo "  - See EXISTING-SERVER.md to add to your current server"
echo "  - See CADDY.md for standalone Caddy setup"
echo "  - See QUICK-REFERENCE.md for configuration options" 