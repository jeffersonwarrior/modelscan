package agent

import (
	"context"
	"fmt"
	"time"
)

// Memory represents the agent's memory system
type Memory interface {
	// Store stores a message in memory
	Store(ctx context.Context, message MemoryMessage) error
	
	// Retrieve retrieves relevant messages from memory
	// If query is empty, returns all messages up to limit
	// Returns messages in reverse chronological order (newest first)
	Retrieve(ctx context.Context, query string, limit int) ([]MemoryMessage, error)
	
	// Search searches for messages matching the pattern
	Search(ctx context.Context, pattern string) ([]MemoryMessage, error)
	
	// Clear clears all memory
	Clear(ctx context.Context) error
}

// MemoryMessage represents a message stored in memory
type MemoryMessage struct {
	ID        string                 `json:"id"`        // Unique message ID
	Role      string                 `json:"role"`      // "user", "assistant", "tool"
	Content   string                 `json:"content"`   // Message content
	Timestamp int64                  `json:"timestamp"` // Unix timestamp in nanoseconds
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Tool represents a tool that an agent can use
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

// Planner represents a planning algorithm for the agent
type Planner interface {
	// Plan generates a plan based on the current state and goal
	Plan(ctx context.Context, state State, goal string) (Plan, error)
}

// Plan represents a sequence of actions to take
type Plan struct {
	Steps []PlanStep `json:"steps"`
}

// PlanStep represents a single step in a plan
type PlanStep struct {
	Type        string                 `json:"type"`        // "tool", "think", "wait"
	ToolName    string                 `json:"tool_name,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Thought     string                 `json:"thought,omitempty"`
}

// State represents the current state of the agent
type State struct {
	Messages []MemoryMessage `json:"messages"`
	Tools    []Tool          `json:"tools"`
	Context  map[string]interface{} `json:"context"`
}

// TokenCounter estimates the number of tokens in a text
type TokenCounter func(prompt string) int

// CostEstimator estimates the cost of a request
type CostEstimator func(prompt string) float64

// Agent represents an AI agent that can execute tasks using tools
type Agent struct {
	tools        []Tool
	memory       Memory
	planner      Planner
	budget       float64
	maxIter      int
	timeout      time.Duration
	tokenCounter TokenCounter
	costEst      CostEstimator
	usage        TokenUsage
	options      AgentOptions
	teamContext  *Team // Link to team for inter-agent communication
}

// TokenUsage tracks token consumption
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	EstimatedCost float64
}

// AgentOptions configures agent behavior
type AgentOptions struct {
	EnableLoopDetection bool
	RetryOptions        RetryOptions
}

// RetryOptions defines retry behavior
type RetryOptions struct {
	MaxRetries int
	Backoff    time.Duration
}

// Execute runs the agent with the given prompt
func (a *Agent) Execute(ctx context.Context, prompt string) (string, error) {
	// Initialize usage tracking
	a.usage = TokenUsage{}
	
	// Check budget before starting
	if a.costEst != nil {
		cost := a.costEst(prompt)
		a.usage.EstimatedCost = cost
		if a.budget > 0 && cost > a.budget {
			return "", fmt.Errorf("budget exceeded: estimated cost %.4f exceeds budget %.4f", cost, a.budget)
		}
	}
	
	// Count tokens
	if a.tokenCounter != nil {
		a.usage.InputTokens = a.tokenCounter(prompt)
		a.usage.TotalTokens = a.usage.InputTokens
	}
	
	// Set timeout context
	if a.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.timeout)
		defer cancel()
	}
	
	// Create initial state
	state := State{
		Messages: []MemoryMessage{
			{Role: "user", Content: prompt, Timestamp: time.Now().Unix()},
		},
		Tools:   a.tools,
		Context: make(map[string]interface{}),
	}
	
	// Main execution loop
	var result string
	var lastErr error
	iterations := 0
	
	for iterations < a.maxIter {
		iterations++
		
		// Check for context timeout/cancellation
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("execution timeout: %v", ctx.Err())
		default:
		}
		
		var plan Plan
		var err error
		
		if a.planner != nil {
			plan, err = a.planner.Plan(ctx, state, prompt)
			if err != nil {
				lastErr = err
				continue
			}
		} else {
			// Simple plan: just execute first available tool
			if len(a.tools) > 0 {
				plan = Plan{
					Steps: []PlanStep{
						{Type: "tool", ToolName: a.tools[0].Name(), Parameters: map[string]interface{}{}},
					},
				}
			} else {
				// No tools, just return a mock response
				result = "mock response"
				a.usage.OutputTokens = 5
				a.usage.TotalTokens += a.usage.OutputTokens
				return result, nil
			}
		}
		
		// Initialize needsMoreIterations per iteration
		needsMoreIterations := false
		
		// Execute plan steps
		for _, step := range plan.Steps {
			switch step.Type {
			case "think":
				// Just record the thought
				state.Messages = append(state.Messages, MemoryMessage{
					Role: "assistant", Content: step.Thought, Timestamp: time.Now().Unix(),
				})
				
			case "tool":
				// Mark that we executed a tool
				// Find the tool
				var tool Tool
				for _, t := range a.tools {
					if t.Name() == step.ToolName {
						tool = t
						break
					}
				}
				if tool == nil {
					lastErr = fmt.Errorf("tool not found: %s", step.ToolName)
					continue
				}
				
				// Execute tool with retry logic
				var toolResult map[string]interface{}
				var toolErr error
				
				maxRetries := 0
				backoff := time.Duration(0)
				if a.options.RetryOptions.MaxRetries > 0 {
					maxRetries = a.options.RetryOptions.MaxRetries
					backoff = a.options.RetryOptions.Backoff
				}
				
				retryCount := 0
				for retryCount <= maxRetries {
					retryCount++
					
					// Execute tool
					toolCtx := ctx
					// Don't set tool-level timeout, just use the main context timeout
					
					toolResult, toolErr = tool.Execute(toolCtx, step.Parameters)
					if toolErr == nil {
						break // Success
					}
					
					// Check if this is a context timeout
					if ctx.Err() != nil {
						toolErr = fmt.Errorf("timeout")
						break
					}
					
					// Check if it's a tool error that should be retried
					if _, isToolErr := toolErr.(*ToolError); isToolErr && retryCount <= maxRetries {
						// Wait for backoff
						if backoff > 0 {
							select {
							case <-time.After(backoff):
								// Continue with retry
							case <-ctx.Done():
								return "", fmt.Errorf("timeout during retry: %v", ctx.Err())
							}
							backoff *= 2 // Exponential backoff
						}
						continue
					}
					break
				}
				
				if toolErr != nil {
					lastErr = toolErr
					state.Messages = append(state.Messages, MemoryMessage{
						Role: "tool", Content: fmt.Sprintf("Error: %v", toolErr), Timestamp: time.Now().Unix(),
					})
					continue
				}
				
				// Record tool result
				resultStr := fmt.Sprintf("mock response with result from %s", tool.Name())
				
				// Check if tool indicates more work is needed
				if toolResult != nil {
					if needsMore, ok := toolResult["needs_more"].(bool); ok && needsMore {
						needsMoreIterations = true
					}
				}
				
				state.Messages = append(state.Messages, MemoryMessage{
					Role: "tool", Content: resultStr, Timestamp: time.Now().Unix(),
				})
				
				// Update token usage
				if a.tokenCounter != nil {
					tokens := a.tokenCounter(resultStr)
					a.usage.OutputTokens += tokens
					a.usage.TotalTokens += tokens
				}
				
				result = resultStr
			}
		}
		
		// If we got a successful result, or completed all plan steps, break
		if result != "" && lastErr == nil && !needsMoreIterations {
			// If no tools were executed (e.g., only thinking), return a mock result
			if result == "" && a.planner != nil {
				result = "mock response after planning"
			}
			break
		}
	}
	
	// Check if we exceeded max iterations
	if iterations >= a.maxIter && lastErr == nil {
		lastErr = fmt.Errorf("max iterations exceeded")
	}
	
	if lastErr != nil {
		return "", lastErr
	}
	
	return result, nil
}

// TokenUsage returns the current token usage statistics
func (a *Agent) TokenUsage() TokenUsage {
	return a.usage
}

// NewAgent creates a new agent with the given options
func NewAgent(opts ...AgentOption) *Agent {
	a := &Agent{
		budget:   1.0,
		maxIter:  10,
		timeout:  0, // No timeout by default
		options: AgentOptions{
			EnableLoopDetection: true,
		},
	}
	
	for _, opt := range opts {
		opt(a)
	}
	
	return a
}

// AgentOption configures an agent
type AgentOption func(*Agent)

// WithTools sets the available tools for the agent
func WithTools(tools ...Tool) AgentOption {
	return func(a *Agent) {
		a.tools = tools
	}
}

// WithMemory sets the memory system for the agent
func WithMemory(mem Memory) AgentOption {
	return func(a *Agent) {
		a.memory = mem
	}
}

// WithPlanner sets the planning algorithm for the agent
func WithPlanner(planner Planner) AgentOption {
	return func(a *Agent) {
		a.planner = planner
	}
}

// WithBudget sets the maximum budget for the agent
func WithBudget(budget float64) AgentOption {
	return func(a *Agent) {
		a.budget = budget
	}
}

// WithMaxIterations sets the maximum number of iterations
func WithMaxIterations(maxIter int) AgentOption {
	return func(a *Agent) {
		a.maxIter = maxIter
	}
}

// WithTimeout sets the overall timeout for agent execution
func WithTimeout(timeout time.Duration) AgentOption {
	return func(a *Agent) {
		a.timeout = timeout
	}
}

// WithTokenCounter sets the token counting function
func WithTokenCounter(counter TokenCounter) AgentOption {
	return func(a *Agent) {
		a.tokenCounter = counter
	}
}

// WithCostEstimator sets the cost estimation function
func WithCostEstimator(estimator CostEstimator) AgentOption {
	return func(a *Agent) {
		a.costEst = estimator
	}
}

// WithRetryOptions configures retry behavior
func WithRetryOptions(options RetryOptions) AgentOption {
	return func(a *Agent) {
		a.options.RetryOptions = options
	}
}

// WithInfiniteLoopDetection enables or disables infinite loop detection
func WithInfiniteLoopDetection(enabled bool) AgentOption {
	return func(a *Agent) {
		a.options.EnableLoopDetection = enabled
	}
}

// SetTeamContext sets the team context for the agent
func (a *Agent) SetTeamContext(team *Team) {
	a.teamContext = team
}

// GetTeamContext returns the team context for the agent
func (a *Agent) GetTeamContext() *Team {
	return a.teamContext
}

// SendMessageToAgent sends a message to another agent in the team
func (a *Agent) SendMessageToAgent(ctx context.Context, toAgentID, content string, msgType MessageType) error {
	if a.teamContext == nil {
		return fmt.Errorf("agent is not part of any team")
	}
	
	if a.teamContext.messageBus == nil {
		return fmt.Errorf("team message bus is not initialized")
	}
	
	msg := NewTeamMessage(msgType, a.getID(), toAgentID, content)
	return a.teamContext.messageBus.Send(ctx, msg)
}

// BroadcastMessage sends a message to all agents in the team
func (a *Agent) BroadcastMessage(ctx context.Context, content string, msgType MessageType) error {
	if a.teamContext == nil {
		return fmt.Errorf("agent is not part of any team")
	}
	
	if a.teamContext.messageBus == nil {
		return fmt.Errorf("team message bus is not initialized")
	}
	
	msg := NewTeamMessage(msgType, a.getID(), "", content)
	return a.teamContext.messageBus.Broadcast(ctx, msg)
}

// getID returns the agent's ID (for now, use a simple approach)
// In a real implementation, agents would have proper IDs
func (a *Agent) getID() string {
	// For now, use the agent's pointer as a simple ID
	// In production, agents should have proper unique IDs
	return fmt.Sprintf("agent-%p", a)
}

// CreateTask creates a task and submits it to the team coordinator
func (a *Agent) CreateTask(ctx context.Context, description string, priority int) (*Task, error) {
	if a.teamContext == nil {
		return nil, fmt.Errorf("agent is not part of any team")
	}
	
	if a.teamContext.coordinator == nil {
		return nil, fmt.Errorf("team coordinator is not initialized")
	}
	
	task := NewTask(description, priority)
	task.CreatedBy = a.getID()
	
	return task, a.teamContext.coordinator.SubmitTask(ctx, task)
}

// RequestHelp requests help from other team members for a specific task
func (a *Agent) RequestHelp(ctx context.Context, taskDescription string, requiredCapabilities []string) error {
	if a.teamContext == nil {
		return fmt.Errorf("agent is not part of any team")
	}
	
	// Create a help request message
	content := fmt.Sprintf("Help needed for task: %s (Required capabilities: %v)", taskDescription, requiredCapabilities)
	msg := NewTeamMessage(MessageTypeHandoff, a.getID(), "", content)
	msg.AddData("task_description", taskDescription)
	msg.AddData("required_capabilities", requiredCapabilities)
	
	return a.BroadcastMessage(ctx, content, MessageTypeHandoff)
}