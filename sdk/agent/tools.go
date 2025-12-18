package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ToolRegistry manages a collection of tools available to agents
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewToolRegistry creates a new empty tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (tr *ToolRegistry) Register(tool Tool) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	
	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tool has empty name")
	}
	
	if _, exists := tr.tools[name]; exists {
		return fmt.Errorf("tool '%s' already registered", name)
	}
	
	tr.tools[name] = tool
	return nil
}

// Get retrieves a tool by name
func (tr *ToolRegistry) Get(name string) (Tool, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	tool, exists := tr.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool '%s' not found", name)
	}
	return tool, nil
}

// List returns all registered tool names
func (tr *ToolRegistry) List() []string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	names := make([]string, 0, len(tr.tools))
	for name := range tr.tools {
		names = append(names, name)
	}
	return names
}

// ToolExecutor handles the execution of tools with error handling and logging
type ToolExecutor struct {
	registry *ToolRegistry
	logger   ToolLogger
}

// ToolLogger logs tool execution for debugging and monitoring
type ToolLogger interface {
	LogExecution(ctx context.Context, toolName string, input, output map[string]interface{}, err error, duration int64)
}

// DefaultToolLogger provides a simple no-op logger
type DefaultToolLogger struct{}

func (d *DefaultToolLogger) LogExecution(ctx context.Context, toolName string, input, output map[string]interface{}, err error, duration int64) {
	// No-op by default
}

// NewToolExecutor creates a new tool executor
func NewToolExecutor(registry *ToolRegistry) *ToolExecutor {
	return &ToolExecutor{
		registry: registry,
		logger:   &DefaultToolLogger{},
	}
}

// WithLogger sets the logger for the tool executor
func (te *ToolExecutor) WithLogger(logger ToolLogger) *ToolExecutor {
	te.logger = logger
	return te
}

// Execute runs a tool with the given input
func (te *ToolExecutor) Execute(ctx context.Context, toolName string, input map[string]interface{}) (map[string]interface{}, error) {
	tool, err := te.registry.Get(toolName)
	if err != nil {
		return nil, fmt.Errorf("tool executor: %w", err)
	}
	
	// Validate input if tool provides validation
	if validator, ok := tool.(ToolInputValidator); ok {
		if err := validator.ValidateInput(input); err != nil {
			return nil, fmt.Errorf("input validation failed for tool '%s': %w", toolName, err)
		}
	}
	
	// Execute the tool
	startTime := time.Now()
	output, err := tool.Execute(ctx, input)
	duration := time.Since(startTime)
	
	// Log the execution
	te.logger.LogExecution(ctx, toolName, input, output, err, duration.Nanoseconds())
	
	if err != nil {
		return nil, fmt.Errorf("tool execution failed for '%s': %w", toolName, err)
	}
	
	// Validate output if tool provides validation
	if validator, ok := tool.(ToolOutputValidator); ok {
		if err := validator.ValidateOutput(output); err != nil {
			return nil, fmt.Errorf("output validation failed for tool '%s': %w", toolName, err)
		}
	}
	
	return output, nil
}

// --- Tool interfaces for extended functionality ---

// ToolInputValidator can be implemented by tools that need input validation
type ToolInputValidator interface {
	ValidateInput(input map[string]interface{}) error
}

// ToolOutputValidator can be implemented by tools that need output validation  
type ToolOutputValidator interface {
	ValidateOutput(output map[string]interface{}) error
}

// ToolWithSchema can be implemented by tools that provide JSON schemas
type ToolWithSchema interface {
	InputSchema() map[string]interface{}
	OutputSchema() map[string]interface{}
}

// ToolWithCategories can be implemented by tools that declare categories
type ToolWithCategories interface {
	Categories() []string
}

// --- Built-in utility tools ---

// EchoTool echoes back its input for testing
type EchoTool struct{}

func (e *EchoTool) Name() string {
	return "echo"
}

func (e *EchoTool) Description() string {
	return "Echoes back the input for testing purposes"
}

func (e *EchoTool) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{
		"echoed": input,
	}, nil
}

// CalculatorTool performs basic arithmetic
type CalculatorTool struct{}

func (c *CalculatorTool) Name() string {
	return "calculator"
}

func (c *CalculatorTool) Description() string {
	return "Performs basic arithmetic operations: +, -, *, /"
}

func (c *CalculatorTool) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Extract operation and operands
	op, ok := input["operation"].(string)
	if !ok {
		return nil, &ToolError{Message: "missing 'operation' parameter"}
	}
	
	a, ok := input["a"].(float64)
	if !ok {
		return nil, &ToolError{Message: "missing or invalid 'a' parameter"}
	}
	
	b, ok := input["b"].(float64)
	if !ok {
		return nil, &ToolError{Message: "missing or invalid 'b' parameter"}
	}
	
	var result float64
	switch op {
	case "+":
		result = a + b
	case "-":
		result = a - b
	case "*":
		result = a * b
	case "/":
		if b == 0 {
			return nil, &ToolError{Message: "division by zero"}
		}
		result = a / b
	default:
		return nil, &ToolError{Message: fmt.Sprintf("unsupported operation: %s", op)}
	}
	
	return map[string]interface{}{
		"result": result,
	}, nil
}

// --- Helper functions ---

// RegisterCommonTools registers common built-in tools with a registry
func RegisterCommonTools(registry *ToolRegistry) error {
	if err := registry.Register(&EchoTool{}); err != nil {
		return err
	}
	return registry.Register(&CalculatorTool{})
}