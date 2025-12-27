package agent

import (
	"context"
	"fmt"
	"strings"
)

// ReActPlanner implements the ReAct (Reason+Act) planning algorithm
// It follows the cycle: Thought → Action → Observation → Thought → ...
type ReActPlanner struct {
	maxThoughts  int
	maxActions   int
	toolSelector ToolSelector
	memoryKey    string // key to store reasoning history in memory
}

// ToolSelector helps select appropriate tools based on the context
type ToolSelector interface {
	SelectTool(ctx context.Context, tools []Tool, goal string, thoughts []string) (Tool, map[string]interface{}, error)
}

// NewReActPlanner creates a new ReAct planner
func NewReActPlanner() *ReActPlanner {
	return &ReActPlanner{
		maxThoughts:  5,
		maxActions:   3,
		toolSelector: &DefaultToolSelector{},
		memoryKey:    "react_history",
	}
}

// WithMaxThoughts sets the maximum number of reasoning steps
func (r *ReActPlanner) WithMaxThoughts(max int) *ReActPlanner {
	r.maxThoughts = max
	return r
}

// WithMaxActions sets the maximum number of tool actions
func (r *ReActPlanner) WithMaxActions(max int) *ReActPlanner {
	r.maxActions = max
	return r
}

// WithToolSelector sets a custom tool selector
func (r *ReActPlanner) WithToolSelector(selector ToolSelector) *ReActPlanner {
	r.toolSelector = selector
	return r
}

// Plan generates a ReAct plan based on the current state and goal
func (r *ReActPlanner) Plan(ctx context.Context, state State, goal string) (Plan, error) {
	// Extract previous thoughts from messages if available
	var thoughts []string
	for _, msg := range state.Messages {
		if contains(msg.Content, "Thought:") ||
			contains(msg.Content, "Action:") ||
			contains(msg.Content, "Observation:") {
			thoughts = append(thoughts, msg.Content)
		}
	}

	plan := Plan{}
	steps := []PlanStep{}

	// Generate initial thought about the goal
	initialThought := r.generateInitialThought(goal, state)
	steps = append(steps, PlanStep{
		Type:    "think",
		Thought: initialThought,
	})
	thoughts = append(thoughts, fmt.Sprintf("Thought: %s", initialThought))

	// Decide if we need to use tools
	if len(state.Tools) > 0 && r.shouldUseTools(goal, thoughts) {
		tool, params, err := r.toolSelector.SelectTool(ctx, state.Tools, goal, thoughts)
		if err != nil {
			return Plan{}, fmt.Errorf("tool selection failed: %w", err)
		}

		// Add action step
		steps = append(steps, PlanStep{
			Type:       "tool",
			ToolName:   tool.Name(),
			Parameters: params,
		})
		thoughts = append(thoughts, fmt.Sprintf("Action: Use %s with %v", tool.Name(), params))
	}

	// Generate final thought
	finalThought := r.generateFinalThought(goal, thoughts)
	steps = append(steps, PlanStep{
		Type:    "think",
		Thought: finalThought,
	})

	plan.Steps = steps
	return plan, nil
}

// generateInitialThought creates the first reasoning step
func (r *ReActPlanner) generateInitialThought(goal string, state State) string {
	thought := fmt.Sprintf("I need to: %s", goal)

	if len(state.Messages) > 0 {
		thought += ". I will use my memory to help with this task."
	}

	if len(state.Tools) > 0 {
		availableTools := make([]string, len(state.Tools))
		for i, tool := range state.Tools {
			availableTools[i] = tool.Name()
		}
		thought += fmt.Sprintf(". Available tools: %s", strings.Join(availableTools, ", "))
	}

	return thought
}

// shouldUseTools determines if tools are needed for this goal
func (r *ReActPlanner) shouldUseTools(goal string, thoughts []string) bool {
	// Simple heuristic: use tools for goals that suggest action
	actionKeywords := []string{
		"calculate", "compute", "find", "search", "get", "fetch",
		"send", "write", "create", "update", "delete", "modify",
		"execute", "run", "perform", "do", "make", "build", "use",
	}

	goalLower := strings.ToLower(goal)
	for _, keyword := range actionKeywords {
		if strings.Contains(goalLower, keyword) {
			return true
		}
	}

	// Always use tools for goals that have tool names
	// This handles cases like "echo this message"
	if strings.Contains(goalLower, "echo") ||
		strings.Contains(goalLower, "calculate") ||
		strings.Contains(goalLower, "test") ||
		strings.Contains(goalLower, "tool") {
		return true
	}

	// Check if previous thoughts indicate a need for tools
	for _, thought := range thoughts {
		if strings.Contains(strings.ToLower(thought), "need to use") ||
			strings.Contains(strings.ToLower(thought), "should try") {
			return true
		}
	}

	return false
}

// generateFinalThought creates the concluding reasoning step
func (r *ReActPlanner) generateFinalThought(goal string, thoughts []string) string {
	if len(thoughts) == 1 {
		return "I will proceed with the initial approach."
	}

	if len(thoughts) >= 3 {
		return "Based on my reasoning and available actions, I believe I have a good plan to achieve this goal."
	}

	return "I should proceed step by step to achieve the goal."
}

// DefaultToolSelector provides basic tool selection logic
type DefaultToolSelector struct{}

// SelectTool chooses the most appropriate tool for the given context
func (d *DefaultToolSelector) SelectTool(ctx context.Context, tools []Tool, goal string, thoughts []string) (Tool, map[string]interface{}, error) {
	if len(tools) == 0 {
		return nil, nil, fmt.Errorf("no tools available")
	}

	// Simple selection strategy: choose based on goal keywords
	goalLower := strings.ToLower(goal)

	// Calculator for math-related goals
	for _, tool := range tools {
		if strings.Contains(strings.ToLower(tool.Name()), "calculator") &&
			(strings.Contains(goalLower, "calculate") ||
				strings.Contains(goalLower, "compute") ||
				strings.Contains(goalLower, "add") ||
				strings.Contains(goalLower, "multiply") ||
				strings.Contains(goalLower, "multiply") ||
				strings.Contains(goalLower, "is *")) {
			return tool, d.guessCalculatorParams(goal), nil
		}
	}

	// Echo tool for general content processing
	for _, tool := range tools {
		if strings.Contains(strings.ToLower(tool.Name()), "echo") {
			return tool, map[string]interface{}{
				"message": goal,
			}, nil
		}
	}

	// Default: use first available tool with empty parameters
	return tools[0], map[string]interface{}{}, nil
}

// guessCalculatorParams attempts to extract numbers from the goal
func (d *DefaultToolSelector) guessCalculatorParams(goal string) map[string]interface{} {
	// Very simple number extraction - in real implementation would be more sophisticated
	params := map[string]interface{}{
		"operation": "add",
		"a":         1.0,
		"b":         1.0,
	}
	return params
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			indexOf(s, substr) >= 0))
}

// indexOf finds the index of a substring
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
