# ModelScan v0.3 - Quick Reference

## Build & Run

```bash
# Build
go build -o modelscan ./cmd/modelscan/

# Initialize
./modelscan --init

# Run
./modelscan
```

## API Quick Reference

### Add Provider
```bash
curl -X POST http://localhost:8080/api/providers/add \
  -H "Content-Type: application/json" \
  -d '{"identifier": "openai/gpt-4", "api_key": "sk-..."}'
```

### List Providers
```bash
curl http://localhost:8080/api/providers
```

### Add API Key
```bash
curl -X POST http://localhost:8080/api/keys/add \
  -H "Content-Type: application/json" \
  -d '{"provider_id": "openai", "api_key": "sk-..."}'
```

### Get Usage Stats
```bash
curl http://localhost:8080/api/stats?model=gpt-4
```

### Health Check
```bash
curl http://localhost:8080/health
```

## Component Architecture

```
Integration Layer
    ├── Database (SQLite)
    ├── Discovery Agent (Claude 4.5)
    ├── SDK Generator (Templates)
    ├── Key Manager (Round-robin)
    ├── Admin API (REST)
    └── Routing Layer (Plano)
```

## File Locations

- Config: `config.yaml`
- Database: `modelscan.db`
- Generated SDKs: `generated/*.go`
- Logs: stdout/stderr

## Environment Variables

```bash
export MODELSCAN_DB_PATH=/var/lib/modelscan/data.db
export MODELSCAN_HOST=0.0.0.0
export MODELSCAN_PORT=8080
export MODELSCAN_AGENT_MODEL=claude-sonnet-4-5
```

## Key Management

- Automatic round-robin selection
- Rate limit tracking (RPM, TPM)
- Degradation on errors (15 min)
- Auto-recovery after degradation period

## Discovery Pipeline

1. Scrape 4 sources (models.dev, GPUStack, ModelScope, HuggingFace)
2. LLM synthesis (Claude/GPT)
3. TDD validation (3 retries)
4. Generate SDK
5. Compile & verify
6. Store in database
7. Hot-reload into router

## Testing

```bash
# Run all tests
go test ./internal/...

# Run specific package
cd internal/keymanager && go test -v

# Build validation
go build ./...
go vet ./...
```

## Documentation

- `V0.3_ARCHITECTURE.md` - Complete architecture
- `USAGE.md` - User guide with examples
- `INTEGRATION_COMPLETE.md` - Build summary
- `config.example.yaml` - Sample config

## Support

Issues: https://github.com/jeffersonwarrior/modelscan/issues
