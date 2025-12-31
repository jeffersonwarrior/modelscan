package tooling

// ToolCall is the internal canonical representation of a tool call.
// All provider-specific formats are parsed into this unified structure.
type ToolCall struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Args     map[string]interface{} `json:"arguments"`
	Type     string                 `json:"type"` // function, bash, etc
	RawInput string                 `json:"raw_input,omitempty"`
}

// ToolResult represents the result of executing a tool call.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
	IsError    bool   `json:"is_error"`
}

// Tool defines a tool's schema for provider compatibility.
// Used when sending tool definitions to providers.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ToolFormat identifies which format a provider uses
type ToolFormat string

const (
	FormatAnthropicJSON ToolFormat = "anthropic_json" // Anthropic native JSON
	FormatOpenAIJSON    ToolFormat = "openai_json"    // OpenAI function calling
	FormatXAIXML        ToolFormat = "xai_xml"        // xAI XML format
	FormatDeepSeekXML   ToolFormat = "deepseek_xml"   // DeepSeek XML format
	FormatGoogleJSON    ToolFormat = "google_json"    // Google Gemini JSON
)

// ProviderCapabilities tracks what tool calling features a provider supports
type ProviderCapabilities struct {
	SupportsToolCalling bool
	SupportsParallel    bool // Multiple tool calls in single response
	SupportsStreaming   bool // Streaming tool call tokens
	Format              ToolFormat
}
