# ModelScan v0.3 Usage Guide

Complete guide for using ModelScan's auto-discovering SDK service.

## Quick Start

### 1. Installation

```bash
# Clone repository
git clone https://github.com/jeffersonwarrior/modelscan.git
cd modelscan

# Build the binary
go build -o modelscan ./cmd/modelscan/

# Or install globally
go install ./cmd/modelscan/
```

### 2. Initialize Database

```bash
# Initialize database with schema
./modelscan --init

# Or specify custom path
MODELSCAN_DB_PATH=/var/lib/modelscan/data.db ./modelscan --init
```

### 3. Configure

Create `config.yaml`:

```yaml
database:
  path: modelscan.db

server:
  host: 127.0.0.1
  port: 8080

api_keys:
  openai:
    - sk-...
  anthropic:
    - sk-ant-...

discovery:
  agent_model: claude-sonnet-4-5
  parallel_batch: 5
  cache_days: 7
```

Or use environment variables:

```bash
export MODELSCAN_DB_PATH=/var/lib/modelscan/data.db
export MODELSCAN_HOST=0.0.0.0
export MODELSCAN_PORT=8080
export MODELSCAN_AGENT_MODEL=gpt-4o
```

### 4. Start Service

```bash
# Start with config file
./modelscan --config config.yaml

# Or with defaults
./modelscan

# Check version
./modelscan --version
```

## Admin API

### Health Check

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "ok",
  "time": "2025-12-27T23:30:00Z"
}
```

### List Providers

```bash
curl http://localhost:8080/api/providers
```

Response:
```json
{
  "providers": [
    {
      "id": "openai",
      "name": "OpenAI",
      "base_url": "https://api.openai.com/v1",
      "status": "online"
    }
  ],
  "count": 1
}
```

### Add New Provider

This triggers the full discovery pipeline:

```bash
curl -X POST http://localhost:8080/api/providers/add \
  -H "Content-Type: application/json" \
  -d '{
    "identifier": "deepseek/deepseek-coder",
    "api_key": "sk-..."
  }'
```

What happens:
1. Scrapes models.dev → pricing data
2. Scrapes GPUStack → deployment specs
3. Scrapes ModelScope → model card
4. Scrapes HuggingFace → capabilities
5. Claude 4.5 synthesizes information
6. Validates with TDD (3 retries)
7. Generates Go SDK (OpenAI-compatible detected)
8. Compiles and verifies
9. Hot-reloads into router
10. Stores in database

Response:
```json
{
  "provider_id": "deepseek",
  "success": true,
  "message": "Provider added successfully",
  "sdk_type": "openai-compatible",
  "validated": true
}
```

### List API Keys

```bash
curl http://localhost:8080/api/keys?provider=openai
```

Response:
```json
{
  "keys": [
    {
      "id": 1,
      "provider_id": "openai",
      "key_prefix": "sk-proj-...",
      "requests_count": 150,
      "tokens_count": 75000,
      "active": true,
      "degraded": false
    }
  ],
  "count": 1
}
```

### Add API Key

```bash
curl -X POST http://localhost:8080/api/keys/add \
  -H "Content-Type: application/json" \
  -d '{
    "provider_id": "openai",
    "api_key": "sk-proj-..."
  }'
```

Response:
```json
{
  "id": 2,
  "provider_id": "openai",
  "key_prefix": "sk-proj-...",
  "active": true,
  "degraded": false
}
```

### Trigger Discovery

Manually trigger discovery for a provider:

```bash
curl -X POST http://localhost:8080/api/discover \
  -H "Content-Type: application/json" \
  -d '{
    "identifier": "anthropic/claude-sonnet-4-5",
    "api_key": "sk-ant-..."
  }'
```

### List Generated SDKs

```bash
curl http://localhost:8080/api/sdks
```

Response:
```json
{
  "sdks": [
    "openai_generated.go",
    "anthropic_generated.go",
    "deepseek_generated.go"
  ],
  "count": 3
}
```

### Get Usage Statistics

```bash
curl http://localhost:8080/api/stats?model=gpt-4
```

Response:
```json
{
  "total_requests": 1000,
  "total_tokens_in": 250000,
  "total_tokens_out": 50000,
  "total_cost": 10.50,
  "avg_latency_ms": 1500,
  "successful_requests": 985,
  "success_rate": 0.985
}
```

## Using as a Client

### OpenAI-Compatible Endpoint

Once a provider is added, use standard OpenAI client libraries:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "deepseek-coder",
    "messages": [
      {"role": "user", "content": "Write a hello world in Go"}
    ]
  }'
```

### Python Client

```python
import openai

openai.api_base = "http://localhost:8080/v1"
openai.api_key = "YOUR_API_KEY"

response = openai.ChatCompletion.create(
    model="deepseek-coder",
    messages=[
        {"role": "user", "content": "Write a hello world in Go"}
    ]
)

print(response.choices[0].message.content)
```

### Go Client

```go
package main

import (
    "context"
    "fmt"
    "github.com/sashabaranov/go-openai"
)

func main() {
    config := openai.DefaultConfig("YOUR_API_KEY")
    config.BaseURL = "http://localhost:8080/v1"
    client := openai.NewClientWithConfig(config)

    resp, err := client.CreateChatCompletion(
        context.Background(),
        openai.ChatCompletionRequest{
            Model: "deepseek-coder",
            Messages: []openai.ChatCompletionMessage{
                {
                    Role:    openai.ChatMessageRoleUser,
                    Content: "Write a hello world in Go",
                },
            },
        },
    )
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.Choices[0].Message.Content)
}
```

## Key Management

### Multiple Keys

Add multiple keys for load balancing:

```bash
# Add 3 keys
curl -X POST http://localhost:8080/api/keys/add \
  -d '{"provider_id": "openai", "api_key": "sk-key1"}'
curl -X POST http://localhost:8080/api/keys/add \
  -d '{"provider_id": "openai", "api_key": "sk-key2"}'
curl -X POST http://localhost:8080/api/keys/add \
  -d '{"provider_id": "openai", "api_key": "sk-key3"}'
```

System automatically:
- Round-robin selects key with lowest usage
- Tracks RPM, TPM, daily limits
- Degrades keys on errors (15 min timeout)
- Re-enables after degradation period
- Resets counters on configured intervals

### Key Rotation

Keys are automatically rotated based on usage. No manual intervention needed.

## Advanced Features

### Provider Discovery from HuggingFace URL

```bash
curl -X POST http://localhost:8080/api/providers/add \
  -d '{
    "identifier": "https://huggingface.co/deepseek-ai/DeepSeek-Coder",
    "api_key": "sk-..."
  }'
```

### Custom Provider

For providers not in public catalogs:

```bash
curl -X POST http://localhost:8080/api/providers/add \
  -d '{
    "identifier": "custom-llm",
    "api_key": "custom-key",
    "base_url": "https://api.custom-llm.com/v1"
  }'
```

### Batch Provider Setup

```bash
#!/bin/bash
# setup-providers.sh

providers=(
  "openai/gpt-4o:sk-proj-..."
  "anthropic/claude-sonnet-4-5:sk-ant-..."
  "deepseek/deepseek-coder:sk-..."
  "google/gemini-2.0-flash:..."
)

for provider in "${providers[@]}"; do
  IFS=':' read -r id key <<< "$provider"
  curl -X POST http://localhost:8080/api/providers/add \
    -d "{\"identifier\": \"$id\", \"api_key\": \"$key\"}"
  echo ""
done
```

## Monitoring

### Logs

Service logs show:
- Discovery progress
- SDK generation status
- Key selection decisions
- Error degradation
- Request routing

```bash
tail -f modelscan.log
```

Example log output:
```
2025/12/27 23:30:00 Adding provider: deepseek/deepseek-coder
2025/12/27 23:30:01   1. Discovering metadata from sources...
2025/12/27 23:30:05   2. Generating SDK code...
2025/12/27 23:30:06   3. Storing provider in database...
2025/12/27 23:30:06   4. Storing API key...
2025/12/27 23:30:06 Provider deepseek added successfully
2025/12/27 23:30:10 Routing request: provider=deepseek, model=deepseek-coder
2025/12/27 23:30:10 Selected key ID 5 (usage: 50 requests, 25000 tokens)
```

### Metrics

Check usage stats for any model:

```bash
# Last 7 days
curl http://localhost:8080/api/stats?model=gpt-4

# Last 30 days
curl http://localhost:8080/api/stats?model=gpt-4&days=30
```

## Troubleshooting

### Database locked

```bash
# Stop service
kill $(pgrep modelscan)

# Reinitialize
./modelscan --init
```

### Discovery failed

Check logs for specific error. Common issues:
- Network timeout → Increase timeout in config
- Invalid API key → Verify key is correct
- Rate limited → Add more keys
- Unsupported provider → Use custom provider flow

### SDK compilation failed

```bash
# Check generated code
ls -la generated/

# View specific SDK
cat generated/deepseek_generated.go

# Manually compile
cd generated && go build .
```

## Configuration Reference

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MODELSCAN_DB_PATH` | `modelscan.db` | Database file path |
| `MODELSCAN_HOST` | `127.0.0.1` | Server bind address |
| `MODELSCAN_PORT` | `8080` | Server port |
| `MODELSCAN_AGENT_MODEL` | `claude-sonnet-4-5` | Discovery agent model |
| `MODELSCAN_PARALLEL_BATCH` | `5` | Concurrent discovery tasks |
| `MODELSCAN_CACHE_DAYS` | `7` | Cache duration |

### Config File Schema

```yaml
database:
  path: string              # Database path

server:
  host: string              # Bind address
  port: integer             # HTTP port

api_keys:
  provider_id:
    - key1                  # API keys
    - key2

discovery:
  agent_model: string       # claude-sonnet-4-5, gpt-4o
  parallel_batch: integer   # Concurrent tasks (1-10)
  cache_days: integer       # Cache duration (1-30)
```

## Security

### API Key Storage

- Keys are SHA256 hashed before storage
- Only prefix (first 10 chars) stored in plaintext
- Actual keys never logged

### Network Security

- Default: localhost only (127.0.0.1)
- LAN access: set `MODELSCAN_HOST=0.0.0.0`
- Production: Use reverse proxy (nginx) with TLS

### Rate Limiting

- Automatic per-key rate limiting
- Degradation on repeated errors
- Prevents API key exhaustion

## Next Steps

- Add more providers via `/api/providers/add`
- Set up monitoring dashboards
- Configure automatic key rotation
- Integrate with Plano routing layer
- Deploy to production

## Support

Issues: https://github.com/jeffersonwarrior/modelscan/issues
Docs: https://github.com/jeffersonwarrior/modelscan/wiki
