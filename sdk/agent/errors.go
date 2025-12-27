package agent

// ToolError represents an error from tool execution
type ToolError struct {
	Message string
}

// Error implements the error interface
func (e *ToolError) Error() string {
	return e.Message
}

// NewToolError creates a new ToolError
func NewToolError(message string) *ToolError {
	return &ToolError{Message: message}
}
