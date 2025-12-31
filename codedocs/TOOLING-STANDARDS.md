# Tool Calling Standardization

## Problem

Every provider has different tool calling formats:
- **Anthropic**: JSON with `tool_use` blocks
- **OpenAI**: JSON with `tool_calls` array
- **xAI**: Custom `<x.ai:tool_call>` XML tags
- **DeepSeek**: Generic XML `<tool>...</tool>`
- **Others**: Bizarre variations like `<tool>ls -la</tool>`

We need ONE canonical format internally, then transform to Anthropic/OpenAI standards on output.

## Design Goals

- Single source of truth for tool calls
- Provider-agnostic internal representation
- Dual output: Anthropic + OpenAI compatible
- Auto-detect unknown formats
- Extensible parser registry
- Stream-safe (handle partial calls)

## Strategy 1: Canonical Tool Call Format

**Location**: `internal/tooling/types.go`

```go
// ToolCall is the internal canonical representation
type ToolCall struct {
    ID       string                 `json:"id"`        // Unique call identifier
    Name     string                 `json:"name"`      // Function/tool name
    Args     map[string]interface{} `json:"arguments"` // Parsed arguments
    Type     string                 `json:"type"`      // function, bash, etc
    RawInput string                 `json:"raw_input,omitempty"` // Original text
}

// ToolResult is the canonical result format
type ToolResult struct {
    CallID  string `json:"call_id"`
    Content string `json:"content"`
    IsError bool   `json:"is_error"`
}
```

## Strategy 2: Provider-Specific Parsers Registry

**Location**: `internal/tooling/parsers.go`

```go
// ToolParser converts provider-specific formats to canonical
type ToolParser interface {
    Parse(response string) ([]ToolCall, error)
    Format() string // "json", "xml", "custom"
    ProviderID() string
}

// Registry maps provider IDs to parsers
var parsers = map[string]ToolParser{
    "anthropic": &AnthropicParser{},
    "openai":    &OpenAIParser{},
    "xai":       &XAICustomParser{},
    "deepseek":  &XMLParser{},
    "google":    &GoogleParser{},
}

// GetParser returns parser for provider
func GetParser(providerID string) (ToolParser, error) {
    p, ok := parsers[providerID]
    if !ok {
        return nil, fmt.Errorf("no parser for provider %s", providerID)
    }
    return p, nil
}

// RegisterParser adds custom parser
func RegisterParser(providerID string, parser ToolParser) {
    parsers[providerID] = parser
}
```

## Strategy 3: Dual Output Transformers

**Location**: `internal/tooling/transformers.go`

```go
// ToAnthropic converts canonical to Anthropic format
func (t *ToolCall) ToAnthropic() map[string]interface{} {
    return map[string]interface{}{
        "type": "tool_use",
        "id":   t.ID,
        "name": t.Name,
        "input": t.Args,
    }
}

// ToOpenAI converts canonical to OpenAI format
func (t *ToolCall) ToOpenAI() map[string]interface{} {
    argsJSON, _ := json.Marshal(t.Args)
    return map[string]interface{}{
        "id":   t.ID,
        "type": "function",
        "function": map[string]interface{}{
            "name":      t.Name,
            "arguments": string(argsJSON),
        },
    }
}

// ToGeneric converts for non-standard clients
func (t *ToolCall) ToGeneric() map[string]interface{} {
    return map[string]interface{}{
        "tool_call_id": t.ID,
        "tool_name":    t.Name,
        "parameters":   t.Args,
    }
}
```

## Strategy 4: Auto-Detection via Regex Patterns

**Location**: `internal/tooling/detection.go`

```go
var toolPatterns = []struct{
    pattern *regexp.Regexp
    parser  ToolParser
}{
    {regexp.MustCompile(`<x\.ai:tool_call>`), &XAIParser{}},
    {regexp.MustCompile(`<tool>.*</tool>`), &XMLParser{}},
    {regexp.MustCompile(`"tool_calls":\s*\[`), &OpenAIParser{}},
    {regexp.MustCompile(`"tool_use"`), &AnthropicParser{}},
}

// DetectFormat auto-detects tool call format
func DetectFormat(response string) (ToolParser, error) {
    for _, tp := range toolPatterns {
        if tp.pattern.MatchString(response) {
            return tp.parser, nil
        }
    }
    return nil, fmt.Errorf("unknown tool call format")
}
```

## Strategy 5: Schema Translation Layer

**Location**: `internal/tooling/schema.go`

```go
// SchemaTranslator converts tool schemas between formats
type SchemaTranslator struct{}

// AnthropicToOpenAI converts Anthropic tool schema to OpenAI functions
func (s *SchemaTranslator) AnthropicToOpenAI(tools []map[string]interface{}) []map[string]interface{} {
    var funcs []map[string]interface{}
    for _, tool := range tools {
        funcs = append(funcs, map[string]interface{}{
            "type": "function",
            "function": map[string]interface{}{
                "name":        tool["name"],
                "description": tool["description"],
                "parameters":  tool["input_schema"],
            },
        })
    }
    return funcs
}

// OpenAIToAnthropic converts OpenAI functions to Anthropic tools
func (s *SchemaTranslator) OpenAIToAnthropic(funcs []map[string]interface{}) []map[string]interface{} {
    var tools []map[string]interface{}
    for _, fn := range funcs {
        if funcDef, ok := fn["function"].(map[string]interface{}); ok {
            tools = append(tools, map[string]interface{}{
                "name":         funcDef["name"],
                "description":  funcDef["description"],
                "input_schema": funcDef["parameters"],
            })
        }
    }
    return tools
}
```

## Strategy 6: Streaming Token Accumulator

**Location**: `internal/tooling/stream.go`

```go
// ToolCallAccumulator handles partial tool calls in streams
type ToolCallAccumulator struct {
    buffer      strings.Builder
    detector    FormatDetector
    completed   []ToolCall
    inProgress  map[string]*ToolCall
}

func (a *ToolCallAccumulator) AddChunk(chunk string) {
    a.buffer.WriteString(chunk)

    // Try to parse completed tool calls
    if calls, err := a.detector.Parse(a.buffer.String()); err == nil {
        a.completed = append(a.completed, calls...)
        a.buffer.Reset()
    }
}

func (a *ToolCallAccumulator) GetCompleted() []ToolCall {
    return a.completed
}
```

## Strategy 7: Tool Call Validation & Sanitization

**Location**: `internal/tooling/validator.go`

```go
// ValidateToolCall checks and sanitizes tool calls
func ValidateToolCall(tc *ToolCall) error {
    // Check required fields
    if tc.Name == "" {
        return fmt.Errorf("tool call missing name")
    }
    if tc.ID == "" {
        tc.ID = generateToolCallID()
    }

    // Parse arguments if raw input exists
    if tc.RawInput != "" && tc.Args == nil {
        // Try JSON first
        if err := json.Unmarshal([]byte(tc.RawInput), &tc.Args); err != nil {
            // Fallback to XML
            if err := parseXML(tc.RawInput, &tc.Args); err != nil {
                return fmt.Errorf("failed to parse tool arguments: %w", err)
            }
        }
    }

    // Sanitize string arguments
    for k, v := range tc.Args {
        if s, ok := v.(string); ok {
            tc.Args[k] = sanitizeString(s)
        }
    }

    return nil
}

func sanitizeString(s string) string {
    // Remove control characters, null bytes, etc
    return strings.Map(func(r rune) rune {
        if r < 32 && r != '\n' && r != '\t' {
            return -1
        }
        return r
    }, s)
}
```

## Strategy 8: Provider Capability Matrix

**Location**: `internal/tooling/capabilities.go`

```go
// Capabilities describes provider tool support
type Capabilities struct {
    Format           string   // json|xml|custom
    SupportsParallel bool     // Multiple tools in one response
    MaxToolsPerCall  int      // Max tool calls per response
    RequiresSchema   bool     // Must send tool schemas
    StreamSupport    bool     // Supports streaming tool calls
    Quirks           []string // Known issues
}

var capabilities = map[string]Capabilities{
    "anthropic": {
        Format:           "json",
        SupportsParallel: true,
        MaxToolsPerCall:  20,
        RequiresSchema:   true,
        StreamSupport:    true,
    },
    "openai": {
        Format:           "json",
        SupportsParallel: true,
        MaxToolsPerCall:  10,
        RequiresSchema:   true,
        StreamSupport:    true,
    },
    "xai": {
        Format:           "custom",
        SupportsParallel: false,
        MaxToolsPerCall:  1,
        RequiresSchema:   false,
        StreamSupport:    false,
        Quirks:           []string{"uses <x.ai:tool_call> tags"},
    },
    "deepseek": {
        Format:           "xml",
        SupportsParallel: true,
        MaxToolsPerCall:  5,
        RequiresSchema:   false,
        StreamSupport:    false,
    },
}

func GetCapabilities(providerID string) Capabilities {
    if cap, ok := capabilities[providerID]; ok {
        return cap
    }
    // Return conservative defaults for unknown providers
    return Capabilities{
        Format:           "unknown",
        SupportsParallel: false,
        MaxToolsPerCall:  1,
        RequiresSchema:   false,
        StreamSupport:    false,
    }
}
```

## Strategy 9: Middleware Chain Pattern

**Location**: `internal/tooling/middleware.go`

```go
// Request Flow: Client -> [Normalize] -> [Transform Schema] -> Provider
// Response Flow: Provider -> [Parse] -> [Validate] -> [Transform] -> Client

type ToolMiddleware interface {
    ProcessRequest(req *Request) error
    ProcessResponse(resp *Response) error
}

type ToolingMiddleware struct {
    parser     ToolParser
    translator *SchemaTranslator
    validator  *Validator
}

func (m *ToolingMiddleware) ProcessRequest(req *Request) error {
    // Translate tool schemas to provider format
    if req.Tools != nil {
        req.Tools = m.translator.ToProviderFormat(req.Tools, req.ProviderID)
    }
    return nil
}

func (m *ToolingMiddleware) ProcessResponse(resp *Response) error {
    // Parse provider-specific tool calls
    calls, err := m.parser.Parse(resp.Body)
    if err != nil {
        return err
    }

    // Validate each call
    for _, call := range calls {
        if err := m.validator.Validate(&call); err != nil {
            return err
        }
    }

    // Store canonical format
    resp.ToolCalls = calls

    // Transform to requested output format (Anthropic or OpenAI)
    if resp.OutputFormat == "anthropic" {
        resp.ToolCallsFormatted = transformToAnthropic(calls)
    } else {
        resp.ToolCallsFormatted = transformToOpenAI(calls)
    }

    return nil
}
```

## Strategy 10: Discovery-Driven Parser Selection

**Location**: `internal/discovery/agent.go` (extend DiscoveryResult)

```go
// Add tooling info to discovery results
type ToolingInfo struct {
    Format      string   `json:"format"`       // json|xml|custom
    Example     string   `json:"example"`      // Sample tool call
    Parser      string   `json:"parser"`       // Parser to use
    Quirks      []string `json:"quirks"`       // Known issues
    SchemaStyle string   `json:"schema_style"` // openai|anthropic|custom
}

type DiscoveryResult struct {
    Provider ProviderInfo `json:"provider"`
    SDK      SDKInfo      `json:"sdk"`
    Tooling  ToolingInfo  `json:"tooling"` // NEW
}

// LLM discovers tooling format during provider analysis
// Example synthesis prompt addition:
// "Analyze the provider's tool calling format. Determine:
//  1. Format type (json/xml/custom)
//  2. Example tool call syntax
//  3. Schema requirements
//  4. Any quirks or limitations"
```

## Implementation Roadmap

### Phase 1: Foundation (Week 1)
1. Create `internal/tooling/` package
2. Implement canonical `ToolCall` struct (#1)
3. Build parser registry (#2)
4. Add Anthropic + OpenAI parsers

### Phase 2: Transformation (Week 2)
5. Implement dual transformers (#3)
6. Add schema translator (#5)
7. Create validation layer (#7)

### Phase 3: Auto-Detection (Week 3)
8. Build format detector (#4)
9. Add capability matrix (#8)
10. Create middleware chain (#9)

### Phase 4: Integration (Week 4)
11. Extend discovery to detect tooling format (#10)
12. Add streaming accumulator (#6)
13. Update all SDK clients to use tooling layer

## Usage Example

```go
// Request with tools
req := &Request{
    Model:      "claude-sonnet-4.5",
    ProviderID: "anthropic",
    Messages:   []Message{{Role: "user", Content: "What's the weather?"}},
    Tools: []Tool{
        {Name: "get_weather", Description: "Get weather", InputSchema: schema},
    },
}

// Middleware processes request (translates schema to Anthropic format)
middleware.ProcessRequest(req)

// Send to provider
resp := provider.Send(req)

// Middleware processes response (parses tool calls, validates, transforms)
middleware.ProcessResponse(resp)

// Get tool calls in desired format
anthropicCalls := resp.GetToolCalls("anthropic")
openaiCalls := resp.GetToolCalls("openai")
canonicalCalls := resp.ToolCalls // Internal format
```

## Testing Strategy

```go
// Test each parser with real provider responses
func TestAnthropicParser(t *testing.T) {
    parser := &AnthropicParser{}
    response := `{"content": [{"type": "tool_use", "id": "123", "name": "calc", "input": {"x": 5}}]}`

    calls, err := parser.Parse(response)
    require.NoError(t, err)
    require.Len(t, calls, 1)
    assert.Equal(t, "calc", calls[0].Name)
    assert.Equal(t, 5.0, calls[0].Args["x"])
}

// Test transformers round-trip correctly
func TestTransformRoundTrip(t *testing.T) {
    canonical := &ToolCall{ID: "1", Name: "test", Args: map[string]interface{}{"x": 5}}

    // Canonical -> Anthropic -> Canonical
    anthropic := canonical.ToAnthropic()
    parsed1, _ := AnthropicParser{}.Parse(anthropic)
    assert.Equal(t, canonical, parsed1)

    // Canonical -> OpenAI -> Canonical
    openai := canonical.ToOpenAI()
    parsed2, _ := OpenAIParser{}.Parse(openai)
    assert.Equal(t, canonical, parsed2)
}
```

## Provider-Specific Notes

### Anthropic
- Format: JSON `tool_use` blocks in content array
- Supports parallel calls
- Requires tool schemas in request

### OpenAI
- Format: JSON `tool_calls` array
- Supports parallel calls
- Arguments are JSON string (need to parse)

### xAI
- Format: Custom `<x.ai:tool_call>` XML tags
- Single tool per response
- No schema required

### DeepSeek
- Format: Generic XML `<tool>...</tool>`
- Supports parallel calls
- Arguments in XML attributes or nested tags

### Google (Gemini)
- Format: JSON `functionCall` objects
- Different schema format (OpenAPI-like)
- Requires special parameter handling
