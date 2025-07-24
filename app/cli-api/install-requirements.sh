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

# Parse arguments
INSTALL_CADDY=false
SILENT=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --caddy)
            INSTALL_CADDY=true
            shift
            ;;
        --silent|-y|--yes)
            SILENT=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  --caddy        Install Caddy web server"
            echo "  --silent, -y   Run without prompts"
            echo "  --help, -h     Show this help"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo -e "${BLUE}🔧 Installing Plandex CLI API Requirements${NC}"
echo

# Detect OS
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    if [ -f /etc/debian_version ]; then
        OS="debian"
    elif [ -f /etc/redhat-release ]; then
        OS="redhat"
    else
        OS="linux"
    fi
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
else
    print_error "Unsupported operating system: $OSTYPE"
    exit 1
fi

print_info "Detected OS: $OS"

# Install Go if not present
if ! command -v go &> /dev/null; then
    print_info "Installing Go..."
    
    case $OS in
        debian)
            if ! $SILENT; then
                read -p "Install Go 1.21? (Y/n): " -n 1 -r
                echo
                [[ $REPLY =~ ^[Nn]$ ]] && { print_info "Skipping Go installation"; }
            fi
            
            if $SILENT || [[ ! $REPLY =~ ^[Nn]$ ]]; then
                # Install Go
                cd /tmp
                wget -q https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
                sudo rm -rf /usr/local/go 2>/dev/null || true
                sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
                
                # Add to PATH
                if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
                    echo 'export PATH=/usr/local/go/bin:$PATH' >> ~/.bashrc
                fi
                export PATH=/usr/local/go/bin:$PATH
                print_success "Go installed successfully"
            fi
            ;;
        redhat)
            sudo yum install -y golang || sudo dnf install -y golang
            ;;
        macos)
            if command -v brew &> /dev/null; then
                brew install go
            else
                print_error "Please install Homebrew first or install Go manually from https://golang.org/dl/"
                exit 1
            fi
            ;;
    esac
else
    print_success "Go is already installed: $(go version)"
fi

# Install system dependencies
print_info "Installing system dependencies..."

case $OS in
    debian)
        sudo apt-get update -qq
        sudo apt-get install -y curl jq git build-essential
        ;;
    redhat)
        sudo yum install -y curl jq git gcc || sudo dnf install -y curl jq git gcc
        ;;
    macos)
        if command -v brew &> /dev/null; then
            brew install curl jq git
        else
            print_warning "Homebrew not found. Please install curl, jq, and git manually."
        fi
        ;;
esac

print_success "System dependencies installed"

# Install Caddy if requested
if $INSTALL_CADDY; then
    print_info "Installing Caddy web server..."
    
    case $OS in
        debian)
            curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
            curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
            sudo apt-get update -qq
            sudo apt-get install -y caddy
            ;;
        redhat)
            sudo yum install -y yum-plugin-copr || sudo dnf install -y dnf-plugins-core
            sudo yum copr enable @caddy/caddy epel-7 || sudo dnf copr enable @caddy/caddy
            sudo yum install -y caddy || sudo dnf install -y caddy
            ;;
        macos)
            if command -v brew &> /dev/null; then
                brew install caddy
            else
                print_warning "Homebrew not found. Please install Caddy manually."
            fi
            ;;
    esac
    
    print_success "Caddy installed successfully"
fi

# Verify installations
echo
print_info "Verification:"
echo "  Go: $(go version 2>/dev/null || echo 'Not found')"
echo "  Git: $(git version 2>/dev/null || echo 'Not found')"
echo "  curl: $(curl --version 2>/dev/null | head -1 || echo 'Not found')"
echo "  jq: $(jq --version 2>/dev/null || echo 'Not found')"

if $INSTALL_CADDY; then
    echo "  Caddy: $(caddy version 2>/dev/null || echo 'Not found')"
fi

echo
print_success "Requirements installation complete!"

if ! $SILENT; then
    echo
    print_info "Next steps:"
    echo "  ./install.sh     # Build and configure CLI API"
    echo "  ./deploy.sh --help  # See deployment options"
fi 