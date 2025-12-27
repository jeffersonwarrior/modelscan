# ModelScan Routing Examples

This directory contains examples for using ModelScan's routing layer with different modes.

## Modes

### 1. Direct Mode
- Routes requests directly to SDK clients
- No proxy overhead
- Best for single-provider applications

```bash
# Uses direct_config.yaml
go run main.go
```

### 2. Plano Proxy Mode
- Routes through an external Plano instance
- Centralized policy management
- Best for production/Kubernetes deployments

```bash
# Start Plano externally first
docker run -d \
  -p 12000:12000 \
  -v $(pwd)/plano_config.yaml:/app/plano_config.yaml:ro \
  -e OPENAI_API_KEY=${OPENAI_API_KEY} \
  -e ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY} \
  katanemo/plano:0.4.0

# Then run the example
go run main.go
```

### 3. Plano Embedded Mode
- Automatically manages a Plano Docker container
- Self-contained deployment
- Best for development and testing

**Requirements:**
- Docker installed and running
- API keys in environment variables

```bash
export OPENAI_API_KEY=your-key
export ANTHROPIC_API_KEY=your-key
export DEEPSEEK_API_KEY=your-key
export GROQ_API_KEY=your-key

go run main.go
```

## Configuration Files

- `direct_config.yaml` - Direct mode configuration
- `proxy_config.yaml` - Proxy mode configuration
- `embedded_config.yaml` - Embedded mode configuration
- `plano_config.yaml` - Plano routing policies

## Policy-Based Routing

The `plano_config.yaml` demonstrates intelligent routing based on task descriptions:

- **OpenAI GPT-4o**: General conversation, data analysis
- **Anthropic Claude**: Code generation, complex reasoning
- **DeepSeek Coder**: Code review, refactoring
- **Groq Llama**: Quick responses, batch processing

Plano's 1.5B router model automatically selects the best provider based on your prompt.

## Testing

```bash
# Test direct mode
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# Test proxy/embedded mode (Plano handles routing)
curl -X POST http://localhost:10000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "none",
    "messages": [{"role": "user", "content": "Write a Python function"}]
  }'
```

## Performance

| Mode | Latency | Overhead | Use Case |
|------|---------|----------|----------|
| Direct | ~100ms | None | Single provider |
| Proxy | ~120ms | ~20ms | Shared instance |
| Embedded | ~150ms | ~50ms | Development |

## Fallback Behavior

When `fallback: true` is set:
1. Primary routing method is attempted
2. On failure, falls back to direct SDK calls
3. Ensures reliability even if Plano is unavailable

## Environment Variables

Required for embedded/proxy modes:

```bash
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
DEEPSEEK_API_KEY=...
GROQ_API_KEY=gsk_...
```

## Troubleshooting

### Docker not available
```
Error: docker not available: exec: "docker": executable file not found in $PATH
```
**Solution**: Install Docker and ensure it's running

### Config file not found
```
Error: config file not found: stat ./plano_config.yaml: no such file or directory
```
**Solution**: Copy `plano_config.yaml` to your working directory

### Container health check failed
```
Error: container failed health check
```
**Solution**: Check Docker logs: `docker logs <container-id>`

### Connection refused
```
Error: dial tcp 127.0.0.1:12000: connect: connection refused
```
**Solution**: Ensure Plano is running and listening on the correct port
