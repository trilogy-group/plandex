# Plandex CLI API

A HTTP API wrapper around the Plandex CLI that allows you to use existing Plandex functionality via REST endpoints.

## Overview

The Plandex CLI API exposes core CLI functionality via HTTP endpoints, allowing you to:

- **Chat**: Ask questions and have conversations without making changes
- **Tell**: Send implementation prompts that can modify your code
- **Switch & List Plans**: Change between existing plans and list available plans
- **Work Remotely**: Use Plandex from any HTTP client or integrate with other tools

**Important**: The API requires full CLI setup and only works with existing plans and configurations. It does not create plans or configure models - that must be done via the CLI first.

## Prerequisites

Before using the API, you must have:

1. **Plandex CLI configured**: Run `plandex auth` to authenticate
2. **Project initialized**: Run `plandex new` to create a project  
3. **At least one plan**: Create and configure plans via CLI
4. **Current plan set**: Use `plandex cd <plan-name>` to select a plan
5. **Models configured**: Set up your AI model credentials via CLI

The API will **refuse to start** if these prerequisites are not met.

## Quick Start

### 1. Setup via CLI First

```bash
# Authenticate with Plandex
plandex auth

# Create a new project and plan
plandex new

# Load context 
plandex load .

# Test CLI is working
plandex chat "Hello, is everything configured?"
```

### 2. Install API

```bash
# Install and build the CLI API
./install-cli-api.sh

# This creates:
# - plandex-cli-api (binary)  
# - plandex-api.json (configuration)
```

### 3. Test API

```bash
# Run in foreground for testing
./deploy-cli-api.sh local
```

### 4. Production Deployment

```bash
# Setup auto-start on boot
./deploy-cli-api.sh autostart
```

## Configuration

The API is configured via `plandex-api.json`:

```json
{
  "server": {
    "port": 8080,
    "host": "127.0.0.1",
    "read_timeout": "30s",
    "write_timeout": "30s", 
    "idle_timeout": "60s"
  },
  "auth": {
    "api_keys": ["your-api-key-here"],
    "require_auth": true
  },
  "cli": {
    "working_dir": ".",
    "environment": {
      "PLANDEX_ENV": "production"
    }
  },
  "security": {
    "enable_cors": true,
    "allowed_origins": ["*"]
  }
}
```

## API Endpoints

### Health & Status

```bash
# Health check
GET /api/v1/health

# Current status (shows auth, project, current plan)
GET /api/v1/status
```

### Chat (No Changes)

```bash
POST /api/v1/chat
Content-Type: application/json
X-API-Key: your-api-key

{
  "prompt": "What is the purpose of this codebase?",
  "plan_name": "optional-plan-name"
}
```

### Tell (Implementation)

```bash
POST /api/v1/tell  
Content-Type: application/json
X-API-Key: your-api-key

{
  "prompt": "Add error handling to the user login function",
  "plan_name": "optional-plan-name",
  "auto_apply": false,
  "auto_context": true
}
```

### Plan Management (Read-Only + Switch)

```bash
# List existing plans
GET /api/v1/plans
X-API-Key: your-api-key

# Get current plan
GET /api/v1/plans/current
X-API-Key: your-api-key

# Switch to existing plan
POST /api/v1/plans/current
Content-Type: application/json
X-API-Key: your-api-key

{
  "name": "existing-plan-name"
}
```

**Note**: Plan creation is NOT available via API - use `plandex new` in CLI.

## Usage Examples

### Basic Chat Example

```bash
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "prompt": "Explain how authentication works in this project"
  }'
```

### Implementation Example

```bash
curl -X POST http://localhost:8080/api/v1/tell \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "prompt": "Add input validation to the registration form",
    "auto_context": true
  }'
```

### Plan Switching Example

```bash
# List available plans
curl -H "X-API-Key: your-api-key" \
  http://localhost:8080/api/v1/plans

# Switch to a different plan
curl -X POST http://localhost:8080/api/v1/plans/current \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"name": "feature-user-dashboard"}'

# Work on the plan
curl -X POST http://localhost:8080/api/v1/tell \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "prompt": "Create a user dashboard with profile editing capabilities"
  }'
```

## Authentication

The API uses API key authentication via the `X-API-Key` header. API keys are configured in `plandex-api.json`.

**Security Note**: Keep your API keys secure and never commit them to version control.

## Working Directory

The API operates from the configured working directory (default: current directory). This should be:

1. A directory with an existing Plandex project (configured via CLI)
2. Have proper authentication set up for your Plandex account
3. Contain any context files you want to work with

## Deployment Options

### Option 1: Local Testing
- Run in foreground 
- Easy debugging
- Manual start/stop

```bash
./deploy-cli-api.sh local
```

### Option 2: Auto-start Service
- Starts automatically on boot
- Runs in background
- Automatic restart on crash
- User-level systemd service (no root required)

```bash
./deploy-cli-api.sh autostart
```

### Option 3: Manual
```bash
./plandex-cli-api --server --config plandex-api.json
```

## Integration with CLI

The API and CLI work together - CLI for setup, API for remote work:

1. **Setup via CLI**: `plandex auth`, `plandex new`, `plandex load .`
2. **Work via API**: Send chat/tell requests remotely  
3. **Review via CLI**: `plandex diff`, `plandex apply`, `plandex commit`
4. **Continue via API**: More implementation work remotely
5. **Manage via CLI**: Create new plans, configure models, etc.

Both use the same plan state and project configuration.

## API Restrictions

The API is **read-only for configuration** and **write-only for content**:

### ✅ Allowed via API:
- Chat with existing plans
- Send tell prompts to existing plans  
- Switch between existing plans
- Auto-apply changes
- Use auto-context

### ❌ NOT allowed via API:
- Create new plans
- Configure AI models  
- Set up authentication
- Initialize projects
- Modify plan configurations
- Load context files

These operations must be done via CLI first.

## Troubleshooting

### API Won't Start

**Error: "CLI setup required"**
- Run `plandex auth` to authenticate
- Run `plandex new` to create a project
- Run `plandex cd <plan-name>` to select a plan
- Verify with `plandex current`

**Error: "No plans exist"**
- Create a plan: `plandex new` 
- Verify: `plandex plans`

**Error: "No current plan set"**
- Select a plan: `plandex cd <plan-name>`
- Verify: `plandex current`

### Authentication Issues
- Verify API key in configuration
- Check if authentication is enabled in config
- Ensure CLI authentication is working: `plandex auth status`

### Plan Issues
- List available plans: `plandex plans`
- Switch to valid plan: `plandex cd <plan-name>`  
- Create plan if needed: `plandex new`

### Context Issues
- Load context via CLI: `plandex load .`
- Check context: `plandex ls`
- Use `auto_context: true` in tell requests

## Development

### Building from Source
```bash
# Build binary
go build -o plandex-cli-api -ldflags="-w -s" .

# Or use install script
./install-cli-api.sh --force
```

### Configuration Changes
After modifying `plandex-api.json`, restart the service:

```bash
# Autostart mode
systemctl --user restart plandex-cli-api

# Local mode
# Stop with Ctrl+C and restart
```

## Security Considerations

1. **API Keys**: Use strong, unique API keys
2. **Network Access**: Bind to localhost (127.0.0.1) for local use only
3. **CORS**: Configure `allowed_origins` appropriately for your use case
4. **File Access**: API has same file system access as the user running it
5. **Environment**: Runs with same environment as the user (access to CLI credentials)

## Advanced Usage

### Custom Working Directory
```json
{
  "cli": {
    "working_dir": "/path/to/your/project"
  }
}
```

### Environment Variables
```json
{
  "cli": {
    "environment": {
      "PLANDEX_ENV": "production",
      "OPENAI_API_KEY": "your-key",
      "ANTHROPIC_API_KEY": "your-key"
    }
  }
}
```

### Multiple API Keys
```json
{
  "auth": {
    "api_keys": [
      "key-for-development",
      "key-for-ci-cd", 
      "key-for-scripts"
    ]
  }
}
```

## Support

For issues and questions:
1. Check this documentation
2. Verify CLI setup is complete: `plandex auth status`, `plandex current`
3. Review CLI documentation (same underlying functionality)
4. Check server logs
5. Test CLI independently first 