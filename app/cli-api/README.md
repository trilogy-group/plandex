# Plandex CLI API Wrapper

A secure REST API wrapper that exposes Plandex CLI functionality with job management, authentication, and webhook support.

## Features

- 🔐 **API Key Authentication** - Secure access control
- 🔄 **Async Job Management** - Request/response pattern with job IDs  
- 📡 **Webhook Support** - Get notified when jobs complete
- ⚙️ **Path-Agnostic Deployment** - Works from any directory
- 🚀 **Auto-Start on Boot** - Survives reboots with systemd user services
- 🛡️ **Full CLI Access** - Same environment as manual CLI usage

## Quick Start

### 1. Complete Setup
```bash
./setup.sh                     # Install dependencies + build + configure
```

### 2. Deploy
```bash
# For testing (runs in foreground)
./deploy.sh local

# For production (auto-start on boot)  
./deploy.sh autostart
```

### 3. Test API
```bash
# Health check
curl http://localhost:8080/api/v1/health

# With API key (get from plandex-api.json)
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/v1/health
```

## Configuration

Edit `plandex-api.json`:

```json
{
  "server": {
    "port": 8080,
    "host": "localhost"
  },
  "auth": {
    "api_keys": ["your-generated-api-key"],
    "require_auth": true
  },
  "cli": {
    "project_path": ".",
    "working_dir": "."
  },
  "webhooks": {
    "enabled": true
  }
}
```

## API Usage

### Submit a Job
```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "command": "tell",
    "args": ["Add a login form to the homepage"],
    "webhook_url": "https://your-app.com/webhooks"
  }'
```

### Check Job Status
```bash
curl -H "X-API-Key: your-api-key" \
  http://localhost:8080/api/v1/jobs/JOB_ID
```

### Available Commands
- `new` - Create new plan
- `load` - Load files into context
- `tell` - Send instructions
- `apply` - Apply changes
- `plans` - List plans
- `branches` - Manage branches

## Deployment Modes

### Local Testing
```bash
./deploy.sh local
```
- Runs in foreground
- Shows detailed logs
- Easy to stop with Ctrl+C

### Auto-Start Production  
```bash
./deploy.sh autostart        # Enable auto-start
./deploy.sh status           # Check status
./deploy.sh disable          # Disable auto-start
```
- Survives system reboots
- Automatic restart on crash
- Full CLI environment access
- Uses systemd user services (no root required)

## Reverse Proxy Setup

### Caddy Example
```caddyfile
your-domain.com {
    reverse_proxy /plandex/* localhost:8080 {
        rewrite * /api{uri}
    }
}
```

### nginx Example  
```nginx
location /api/ {
    proxy_pass http://localhost:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

## Management

### Install/Uninstall
```bash
./install.sh                 # Quick install only
./setup.sh                   # Complete setup with dependencies
./uninstall-local.sh         # Remove local installation
./uninstall-system.sh        # Remove system installation
```

### Service Management (Auto-Start Mode)
```bash
systemctl --user status plandex-cli-api
systemctl --user restart plandex-cli-api
journalctl --user -u plandex-cli-api -f
```

## Security

- **API Keys**: Generated automatically during install
- **User-Level Service**: Runs as your user with your permissions
- **Environment Access**: Full access to CLI configurations and auth tokens
- **HTTPS**: Use reverse proxy (Caddy/nginx) for SSL

## Troubleshooting

### API Not Responding
```bash
./deploy.sh status           # Check deployment status
./deploy.sh local            # Test in foreground
```

### CLI Access Issues
The API automatically loads your environment from:
- `~/.bashrc` and `~/.profile`
- Current PATH and environment variables
- Plandex CLI configurations (`~/.plandex-home-v2/`)

### Port Conflicts
Edit `plandex-api.json` to change port:
```json
{
  "server": {
    "port": 8081
  }
}
```

## Development

### Build
```bash
go mod tidy
go build -o plandex-cli-api
```

### Environment Variables
- `LOG_LEVEL`: Set logging level (debug, info, warn, error)
- `PLANDEX_ENV`: Environment mode for Plandex CLI

## Files Structure

```
app/cli-api/
├── README.md                 # This file
├── deploy.sh                 # Main deployment script
├── install.sh                # Quick installation  
├── setup.sh                  # Complete setup
├── main.go                   # Main application
├── plandex-api.json          # Configuration
├── uninstall-*.sh            # Cleanup scripts
├── config/                   # Configuration management
├── executor/                 # CLI command execution
├── jobs/                     # Job management
├── server/                   # HTTP server
└── webhooks/                 # Webhook handling
```

## License

Same as Plandex project license. 