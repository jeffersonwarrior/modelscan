# Model Auto-Configuration Design

Auto-configure local AI models from public catalogs for use with ModelScan.

## Overview

Allow users to:
1. Browse AI models from multiple catalogs (models.dev, GPUStack, ModelScope)
2. Select a model with simple identifier (e.g., "gpt-4o", "claude-sonnet-4-5")
3. Automatically configure and deploy locally via GPUStack
4. Integrate seamlessly with ModelScan routing layer

## Architecture

```
User Selection
     │
     ▼
Model Discovery
     │
     ├─→ models.dev API (metadata, pricing, capabilities)
     ├─→ GPUStack Catalog (deployment specs)
     └─→ ModelScope Hub (alternative source)
     │
     ▼
Auto-Configuration
     │
     ├─→ Detect local GPU capabilities
     ├─→ Select optimal quantization
     ├─→ Choose deployment backend (vLLM/SGLang)
     └─→ Generate GPUStack config
     │
     ▼
Local Deployment
     │
     ├─→ GPUStack CLI/API
     ├─→ Download model weights
     ├─→ Start inference server
     └─→ Health check
     │
     ▼
ModelScan Integration
     │
     └─→ Register with routing layer
```

## Data Sources

### 1. models.dev
**Purpose**: Model discovery, metadata, pricing

**API**: `https://models.dev/api.json`

**Data Structure** (TOML):
```toml
name = "GPT-4o"
attachment = true
reasoning = false
tool_call = true
structured_output = true
temperature = true
knowledge = "2024-04"
release_date = "2025-02-19"
last_updated = "2025-02-19"
open_weights = false

[cost]
input = 3.00
output = 15.00

[limit]
context = 400_000
output = 8_192

[modalities]
input = ["text", "image"]
output = ["text"]
```

**Key Fields**:
- Model ID (provider/model)
- Capabilities (tool_call, reasoning, attachments)
- Pricing per million tokens
- Context limits
- Modalities

### 2. GPUStack Model Catalog
**Purpose**: Local deployment specs

**Format**: YAML

**Data Structure**:
```yaml
model_sets:
- name: Deepseek R1
  description: "..."
  categories: [llm]
  size: 671  # billion parameters
  specs:
    - mode: throughput
      quantization: FP8
      gpu_filters:
        vendor: nvidia
        compute_capability: ">=9.0"
      source: huggingface
      huggingface_repo_id: deepseek-ai/DeepSeek-R1
      backend: SGLang
      backend_parameters:
        - --enable-dp-attention
        - --context-length=32768
```

**Key Fields**:
- Deployment mode (throughput, latency, standard)
- Quantization (FP16, FP8, INT8, INT4)
- GPU requirements
- Backend (vLLM, SGLang)
- Source (huggingface, modelscope, local_path)

### 3. ModelScope
**Purpose**: Alternative model source (China-friendly)

**API**: Similar to HuggingFace

**Usage**: GPUStack supports `model_scope_model_id` as alternative to `huggingface_repo_id`

## Implementation Plan

### Phase 1: Model Discovery Package

```go
// catalog/catalog.go
package catalog

type ModelCatalog struct {
    ModelsDevAPI    string
    GPUStackCatalog string
    Cache           ModelCache
}

type Model struct {
    ID           string            // "openai/gpt-4o"
    Provider     string            // "openai"
    Name         string            // "GPT-4o"
    Description  string
    Capabilities Capabilities
    Pricing      Pricing
    Limits       Limits
    Deployment   DeploymentSpec    // from GPUStack
    Source       ModelSource       // huggingface, modelscope, local
}

type Capabilities struct {
    ToolCall          bool
    Reasoning         bool
    Attachments       bool
    StructuredOutput  bool
    Temperature       bool
    OpenWeights       bool
}

type Pricing struct {
    InputPerM      float64  // per million tokens
    OutputPerM     float64
    ReasoningPerM  float64
    CacheReadPerM  float64
    CacheWritePerM float64
}

type Limits struct {
    ContextWindow int
    MaxInput      int
    MaxOutput     int
}

type DeploymentSpec struct {
    Mode          string   // throughput, latency, standard
    Quantization  string   // FP16, FP8, INT8, INT4
    Backend       string   // vLLM, SGLang
    Parameters    []string // backend-specific flags
    GPUVendor     string   // nvidia, amd, intel
    MinCompute    string   // compute capability
    SourceType    string   // huggingface, modelscope, local_path
    SourceID      string   // repo ID or path
}

func (c *ModelCatalog) Search(query string) ([]Model, error)
func (c *ModelCatalog) Get(modelID string) (*Model, error)
func (c *ModelCatalog) List() ([]Model, error)
```

### Phase 2: GPU Detection

```go
// gpu/detect.go
package gpu

type GPUInfo struct {
    Vendor           string   // nvidia, amd, intel
    DeviceCount      int
    ComputeCapability string  // NVIDIA: 7.5, 8.0, 9.0
    TotalMemoryGB    float64
    AvailableMemoryGB float64
    Driver           string
}

func DetectGPU() (*GPUInfo, error)
func (g *GPUInfo) CanRun(spec DeploymentSpec) bool
```

### Phase 3: Auto-Configuration

```go
// autoconfig/autoconfig.go
package autoconfig

type AutoConfig struct {
    GPUInfo      *gpu.GPUInfo
    Catalog      *catalog.ModelCatalog
    GPUStackURL  string
}

type DeploymentConfig struct {
    ModelID      string
    Quantization string
    Backend      string
    Parameters   []string
    SourceType   string
    SourceID     string
    LocalPath    string  // if already downloaded
}

func (a *AutoConfig) ConfigureForLocal(modelID string) (*DeploymentConfig, error) {
    // 1. Get model metadata from catalog
    // 2. Detect GPU capabilities
    // 3. Select best quantization (FP8 > INT8 > INT4)
    // 4. Choose backend (SGLang for throughput, vLLM for standard)
    // 5. Generate deployment config
}

func (a *AutoConfig) EstimateMemory(model *catalog.Model, quant string) float64 {
    // Estimate VRAM needed based on model size and quantization
}

func (a *AutoConfig) SelectQuantization(modelSize int, gpuMemoryGB float64) string {
    // Choose optimal quantization for available VRAM
}
```

### Phase 4: GPUStack Integration

```go
// gpustack/client.go
package gpustack

type Client struct {
    BaseURL    string
    HTTPClient *http.Client
}

type DeployRequest struct {
    Name         string
    Source       string  // huggingface, modelscope
    RepoID       string
    Quantization string
    Backend      string
    Parameters   []string
}

type Deployment struct {
    ID     string
    Name   string
    Status string  // deploying, running, failed
    URL    string  // inference endpoint
}

func (c *Client) Deploy(req DeployRequest) (*Deployment, error)
func (c *Client) Status(deploymentID string) (*Deployment, error)
func (c *Client) Delete(deploymentID string) error
func (c *Client) List() ([]Deployment, error)
```

### Phase 5: CLI Interface

```bash
# List available models
modelscan catalog list
modelscan catalog search "code generation"

# Get model info
modelscan catalog info deepseek/deepseek-coder

# Auto-configure and deploy locally
modelscan deploy deepseek/deepseek-coder
# → Detects GPU: NVIDIA RTX 4090 (24GB)
# → Selects quantization: FP8 (estimated 11GB)
# → Backend: vLLM
# → Source: huggingface/deepseek-ai/DeepSeek-Coder
# → Starting deployment...
# → Deployed at http://localhost:8080

# Deploy with options
modelscan deploy deepseek/deepseek-coder \
  --quantization INT8 \
  --backend sglang \
  --gpu 0,1

# List local deployments
modelscan list

# Stop deployment
modelscan stop deepseek-coder

# Remove deployment
modelscan remove deepseek-coder
```

## Configuration Examples

### Auto-Config Flow

```yaml
# User selects model
model: openai/gpt-4o

# Auto-config detects and generates:
deployment:
  name: gpt-4o-local
  source: huggingface
  repo_id: openai/gpt-4o  # (if open weights)
  quantization: FP8
  backend: vLLM
  gpu_filters:
    vendor: nvidia
    min_memory_gb: 40
  parameters:
    - --max-model-len=128000
    - --tensor-parallel-size=2
```

### Manual Override

```yaml
deployment:
  model: deepseek/deepseek-r1
  quantization: INT4  # override auto-selected FP8
  backend: sglang
  source: modelscope  # use China mirror
  model_id: deepseek-ai/DeepSeek-R1
  parameters:
    - --context-length=32768
    - --enable-dp-attention
```

## Integration with Routing

After deployment, auto-register with routing layer:

```go
// Auto-register local deployment
router.RegisterClient("deepseek-coder", &LocalClient{
    BaseURL: "http://localhost:8080",
    Model:   "deepseek/deepseek-coder",
})

// Update routing config
routing:
  mode: direct
  direct:
    providers:
      - name: deepseek-coder
        type: local
        base_url: http://localhost:8080
        model: deepseek/deepseek-coder
```

## Dependencies

**Zero external Go dependencies** - use stdlib for:
- HTTP client (catalog API, GPUStack)
- JSON/YAML/TOML parsing
- GPU detection via `os/exec` (nvidia-smi, rocm-smi)

**External tools** (optional):
- GPUStack (for local deployment)
- nvidia-smi (for GPU detection)
- HuggingFace CLI (for model download)

## Workflow Example

1. **User browses catalog**:
   ```bash
   modelscan catalog search "reasoning"
   ```

2. **Auto-config analyzes**:
   - GPU: NVIDIA RTX 4090 (24GB VRAM)
   - Model: DeepSeek R1 (671B params)
   - Quantization: INT4 (fits in 24GB)
   - Backend: vLLM (standard mode)

3. **Deployment**:
   ```bash
   modelscan deploy deepseek/deepseek-r1
   ```

4. **GPUStack downloads and serves**:
   - Downloads INT4 quantized weights
   - Starts vLLM server on localhost:8080
   - Exposes OpenAI-compatible API

5. **Auto-registers with routing**:
   ```go
   router.Route(ctx, Request{
       Provider: "deepseek-r1",
       Model: "deepseek/deepseek-r1",
       Messages: [...],
   })
   ```

## Future Enhancements

1. **Multi-GPU support**: Tensor parallelism across GPUs
2. **Model quantization**: Auto-quantize FP16 to INT8/INT4
3. **Caching**: Cache downloaded models
4. **Updates**: Check for model updates
5. **Benchmarking**: Auto-benchmark deployed models
6. **Cost tracking**: Track inference costs vs cloud
7. **Model comparison**: Side-by-side comparison tool

## Security Considerations

- **Model verification**: Verify checksums of downloaded models
- **Sandboxing**: Run inference in isolated containers
- **API keys**: Never store API keys in configs
- **Network isolation**: Option to disable internet access after download

## Sources

- [models.dev](https://models.dev/) - Model catalog and pricing
- [models.dev GitHub](https://github.com/sst/models.dev) - TOML schema and API
- [GPUStack Docs](https://docs.gpustack.ai/2.0/user-guide/model-catalog/) - Deployment catalog
- [ModelScope](https://www.modelscope.ai/) - Chinese model hub
