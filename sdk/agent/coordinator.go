package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Coordinator manages task distribution and coordination within a team
type Coordinator struct {
	team       *Team
	taskQueue  chan Task
	active     map[string]*TaskExecution
	completed  map[string]*TaskResult
	mu         sync.RWMutex
	capability *CapabilityMatcher
	strategy   DistributionStrategy
}

// DistributionStrategy defines how tasks are distributed
type DistributionStrategy int

const (
	DistributionStrategyRoundRobin DistributionStrategy = iota
	DistributionStrategyCapabilityBased
	DistributionStrategyLoadBalance
	DistributionStrategyPriority
)

// TaskExecution tracks an active task execution
type TaskExecution struct {
	Task       Task
	AgentID    string
	StartTime  time.Time
	Deadline   time.Time
	CancelFunc context.CancelFunc
	ResultChan chan *TaskResult
}

// CapabilityMatcher matches agents to tasks based on capabilities
type CapabilityMatcher struct {
	agentCapabilities   map[string][]string
	taskRequirements    map[string][]string
	relatedCapabilities map[string][]string // Maps a capability to related capabilities
}

// NewCoordinator creates a new coordinator
func NewCoordinator(team *Team) *Coordinator {
	return &Coordinator{
		team:       team,
		taskQueue:  make(chan Task, 100),
		active:     make(map[string]*TaskExecution),
		completed:  make(map[string]*TaskResult),
		capability: NewCapabilityMatcher(),
		strategy:   DistributionStrategyRoundRobin,
	}
}

// NewCoordinatorWithStrategy creates a new coordinator with specific strategy
func NewCoordinatorWithStrategy(team *Team, strategy DistributionStrategy) *Coordinator {
	return &Coordinator{
		team:       team,
		taskQueue:  make(chan Task, 100),
		active:     make(map[string]*TaskExecution),
		completed:  make(map[string]*TaskResult),
		capability: NewCapabilityMatcher(),
		strategy:   strategy,
	}
}

// ExecuteTask executes a task using the team
func (c *Coordinator) ExecuteTask(ctx context.Context, task Task) (*TaskResult, error) {
	// Start task execution
	c.taskQueue <- task

	// For now, execute synchronously
	return c.executeTaskInternal(ctx, task)
}

// executeTaskInternal executes a single task
func (c *Coordinator) executeTaskInternal(ctx context.Context, task Task) (*TaskResult, error) {
	result := &TaskResult{
		TaskID:    task.ID,
		Status:    TaskStatusActive,
		StartTime: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Add to active tasks before selecting agent
	c.mu.Lock()
	c.active[task.ID] = &TaskExecution{
		Task:       task,
		AgentID:    "",
		StartTime:  time.Now(),
		Deadline:   time.Now().Add(30 * time.Minute), // Default 30 min deadline
		ResultChan: make(chan *TaskResult, 1),
	}
	c.mu.Unlock()

	// Select an agent for the task
	agentID, err := c.selectAgent(task)
	if err != nil {
		result.Status = TaskStatusFailed
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	result.AgentID = agentID

	// Update the active task with the selected agent
	c.mu.Lock()
	if exec, exists := c.active[task.ID]; exists {
		exec.AgentID = agentID
	}
	c.mu.Unlock()

	// Get the agent
	agent, exists := c.team.GetMember(agentID)
	if !exists {
		result.Status = TaskStatusFailed
		result.Error = fmt.Sprintf("agent %s not found", agentID)
		return result, fmt.Errorf("%s", result.Error)
	}

	// Execute the task on the agent
	task.Activate()
	resp, err := agent.Execute(ctx, task.Description)

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		result.Status = TaskStatusFailed
		result.Error = err.Error()
		task.Fail(err)
	} else {
		result.Status = TaskStatusCompleted
		result.Result = resp
		task.Complete()
	}

	// Notify team of task completion
	c.team.Broadcast(ctx, "coordinator",
		fmt.Sprintf("Task %s completed by agent %s", task.ID, agentID), MessageTypeResult)

	// Store result and clean up active task
	c.mu.Lock()
	result.Metadata = map[string]interface{}{"original_task": task} // Store original task
	c.completed[task.ID] = result
	delete(c.active, task.ID)
	c.mu.Unlock()

	return result, nil
}

// selectAgent selects the best agent for a task based on strategy
func (c *Coordinator) selectAgent(task Task) (string, error) {
	switch c.strategy {
	case DistributionStrategyRoundRobin:
		return c.selectRoundRobin()
	case DistributionStrategyCapabilityBased:
		return c.selectByCapability(task)
	case DistributionStrategyLoadBalance:
		return c.selectByLoad()
	case DistributionStrategyPriority:
		if task.Priority > 0 {
			return c.selectByCapability(task)
		}
		return c.selectByLoad()
	default:
		return c.selectRoundRobin()
	}
}

// selectRoundRobin selects the next agent in round-robin fashion
func (c *Coordinator) selectRoundRobin() (string, error) {
	members := c.team.ListMembers()
	if len(members) == 0 {
		return "", fmt.Errorf("no agents available")
	}

	// Count tasks for each agent from active and completed
	c.mu.RLock()
	taskCounts := make(map[string]int)

	// Count active tasks
	for _, exec := range c.active {
		taskCounts[exec.AgentID]++
	}

	// Count total completed tasks to simulate round-robin
	completedCounts := make(map[string]int)
	for _, result := range c.completed {
		if result.AgentID != "" {
			completedCounts[result.AgentID]++
		}
	}
	c.mu.RUnlock()

	// Total tasks per agent
	totalCounts := make(map[string]int)
	for _, agentID := range members {
		totalCounts[agentID] = taskCounts[agentID] + completedCounts[agentID]
	}

	// Find agent with minimum total tasks (ties broken by alphabetical order for consistency)
	minTasks := int(^uint(0) >> 1)
	var selectedAgent string

	for _, agentID := range members {
		if totalCounts[agentID] < minTasks ||
			(totalCounts[agentID] == minTasks && (selectedAgent == "" || agentID < selectedAgent)) {
			minTasks = totalCounts[agentID]
			selectedAgent = agentID
		}
	}

	return selectedAgent, nil
}

// selectByCapability selects agent based on capability matching
func (c *Coordinator) selectByCapability(task Task) (string, error) {
	members := c.team.ListMembers()
	if len(members) == 0 {
		return "", fmt.Errorf("no agents available")
	}

	// Get task requirements - first check explicit requirements in task data
	var requirements []string
	if reqCaps, exists := task.Data["required_capabilities"]; exists {
		if caps, ok := reqCaps.([]string); ok {
			requirements = caps
		}
	}

	// If no explicit requirements, extract from description
	if len(requirements) == 0 {
		requirements = c.capability.GetTaskRequirements(task.Description)
	}

	// Score each agent based on capability match
	type agentScore struct {
		id    string
		score float64
	}

	var scores []agentScore

	for _, agentID := range members {
		_, exists := c.team.GetMember(agentID)
		if !exists {
			continue
		}

		// Check agent capabilities
		capabilities := c.capability.GetAgentCapabilities(agentID)

		// Calculate match score
		score := c.calculateCapabilityScore(requirements, capabilities)
		scores = append(scores, agentScore{id: agentID, score: score})
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	if len(scores) == 0 {
		return "", fmt.Errorf("no suitable agent found")
	}

	return scores[0].id, nil
}

// calculateCapabilityScore calculates how well an agent matches task requirements
func (c *Coordinator) calculateCapabilityScore(requirements, capabilities []string) float64 {
	if len(requirements) == 0 {
		return 1.0 // No requirements, any agent can do it
	}

	if len(capabilities) == 0 {
		return 0.1 // Agent has no capabilities
	}

	matches := 0
	for _, req := range requirements {
		for _, cap := range capabilities {
			if req == cap {
				matches++
				break
			}
		}
	}

	return float64(matches) / float64(len(requirements))
}

// selectByLoad selects agent based on current load
func (c *Coordinator) selectByLoad() (string, error) {
	members := c.team.ListMembers()
	if len(members) == 0 {
		return "", fmt.Errorf("no agents available")
	}

	// Sort by current task count
	sort.Slice(members, func(i, j int) bool {
		loadI := c.getAgentLoad(members[i])
		loadJ := c.getAgentLoad(members[j])
		return loadI < loadJ
	})

	return members[0], nil
}

// getAgentLoad returns the current load of an agent
func (c *Coordinator) getAgentLoad(agentID string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	load := 0
	// Count active tasks
	for _, exec := range c.active {
		if exec.AgentID == agentID {
			load++
		}
	}
	// Also count completed tasks for distribution purposes
	for _, result := range c.completed {
		if result.AgentID == agentID {
			load++
		}
	}
	return load
}

// GetTaskResult returns the result of a task
func (c *Coordinator) GetTaskResult(taskID string) (*TaskResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result, exists := c.completed[taskID]
	if !exists {
		return nil, fmt.Errorf("task result not found: %s", taskID)
	}

	return result, nil
}

// GetActiveTasks returns all currently active tasks
func (c *Coordinator) GetActiveTasks() []*TaskExecution {
	c.mu.RLock()
	defer c.mu.RUnlock()

	active := make([]*TaskExecution, 0, len(c.active))
	for _, exec := range c.active {
		active = append(active, exec)
	}

	return active
}

// CancelTask cancels an active task
func (c *Coordinator) CancelTask(taskID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	exec, exists := c.active[taskID]
	if !exists {
		return fmt.Errorf("active task not found: %s", taskID)
	}

	if exec.CancelFunc != nil {
		exec.CancelFunc()
	}

	delete(c.active, taskID)

	// Store cancelled result
	result := &TaskResult{
		TaskID:    taskID,
		Status:    TaskStatusCancelled,
		StartTime: exec.StartTime,
		EndTime:   time.Now(),
		Duration:  time.Since(exec.StartTime),
		AgentID:   exec.AgentID,
		Metadata:  map[string]interface{}{"original_task": exec.Task}, // Store original task
	}
	c.completed[taskID] = result

	return nil
}

// SetStrategy sets the task distribution strategy
func (c *Coordinator) SetStrategy(strategy DistributionStrategy) {
	c.strategy = strategy
}

// SetCapabilityMatcher sets the capability matcher
func (c *Coordinator) SetCapabilityMatcher(matcher *CapabilityMatcher) {
	c.capability = matcher
}

// NewCapabilityMatcher creates a new capability matcher
func NewCapabilityMatcher() *CapabilityMatcher {
	// Initialize related capabilities mapping
	related := map[string][]string{
		"math":       {"calculator"},
		"calculator": {"math"},
		"calculate":  {"calculator"},
		"compute":    {"calculator"},
		"analysis":   {"analyze"},
		"analyze":    {"analysis"},
		"search":     {"find"},
		"find":       {"search"},
		"reader":     {"read"},
		"read":       {"reader"},
	}

	return &CapabilityMatcher{
		agentCapabilities:   make(map[string][]string),
		taskRequirements:    make(map[string][]string),
		relatedCapabilities: related,
	}
}

// RegisterAgentCapabilities registers capabilities for an agent
func (cm *CapabilityMatcher) RegisterAgentCapabilities(agentID string, capabilities []string) {
	cm.agentCapabilities[agentID] = capabilities
}

// RegisterCapability is an alias for RegisterAgentCapabilities
func (cm *CapabilityMatcher) RegisterCapability(agentID string, capabilities []string) {
	cm.RegisterAgentCapabilities(agentID, capabilities)
}

// GetAgentCapabilities returns the capabilities of an agent
func (cm *CapabilityMatcher) GetAgentCapabilities(agentID string) []string {
	return cm.agentCapabilities[agentID]
}

// FindBestAgent finds the best agent for required capabilities
func (cm *CapabilityMatcher) FindBestAgent(requiredCapabilities []string, availableAgents []string) string {
	if len(requiredCapabilities) == 0 || len(availableAgents) == 0 {
		return ""
	}

	type agentScore struct {
		id                string
		score             float64
		canPartiallyMatch bool // Uses related capabilities
	}

	var scores []agentScore

	for _, agentID := range availableAgents {
		capabilities := cm.GetAgentCapabilities(agentID)

		// Calculate match score
		score := cm.calculateScore(requiredCapabilities, capabilities)

		// Check if this agent uses related capabilities for partial matching
		canPartiallyMatch := false
		if score > 0 && score < 1.0 {
			for _, req := range requiredCapabilities {
				if _, exists := cm.relatedCapabilities[req]; exists {
					canPartiallyMatch = true
					break
				}
			}
		}

		scores = append(scores, agentScore{id: agentID, score: score, canPartiallyMatch: canPartiallyMatch})
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// First try to find perfect match (score = 1.0)
	for _, s := range scores {
		if s.score == 1.0 {
			return s.id
		}
	}

	// If no perfect match, only allow partial matches for special cases
	// and only if all requirements can be satisfied (even with related capabilities)
	for _, s := range scores {
		if s.score > 0 && s.canPartiallyMatch && s.score >= (float64(len(requiredCapabilities))/float64(len(requiredCapabilities))) {
			return s.id
		}
	}

	return ""
}

// calculateScore calculates how well capabilities match requirements
func (cm *CapabilityMatcher) calculateScore(requirements, capabilities []string) float64 {
	if len(requirements) == 0 {
		return 1.0 // No requirements, any agent can do it
	}

	if len(capabilities) == 0 {
		return 0.0 // Agent has no capabilities
	}

	matches := 0
	for _, req := range requirements {
		matched := false
		// Direct match
		for _, cap := range capabilities {
			if req == cap {
				matches++
				matched = true
				break
			}
		}
		// Check for partial match (substring) if direct match failed
		if !matched {
			for _, cap := range capabilities {
				if strings.Contains(req, cap) || strings.Contains(cap, req) {
					matches++
					matched = true
					break
				}
			}
		}
		// Check for related capabilities if still no match
		if !matched {
			if related, exists := cm.relatedCapabilities[req]; exists {
				for _, relatedCap := range related {
					for _, cap := range capabilities {
						if relatedCap == cap {
							matches++
							matched = true
							break
						}
					}
					if matched {
						break
					}
				}
			}
		}
	}

	return float64(matches) / float64(len(requirements))
}

// GetTaskRequirements extracts requirements from a task description
func (cm *CapabilityMatcher) GetTaskRequirements(description string) []string {
	// Simple keyword-based requirement extraction
	descLower := strings.ToLower(description)

	var requirements []string

	// Check for math requirements
	if strings.Contains(descLower, "calculate") || strings.Contains(descLower, "compute") ||
		strings.Contains(descLower, "add") || strings.Contains(descLower, "multiply") ||
		strings.Contains(descLower, "divide") || strings.Contains(descLower, "subtract") ||
		strings.Contains(descLower, "math") {
		requirements = append(requirements, "calculator")
	}

	// Check for file operations
	if strings.Contains(descLower, "file") || strings.Contains(descLower, "read") ||
		strings.Contains(descLower, "write") || strings.Contains(descLower, "delete") ||
		strings.Contains(descLower, "create") {
		requirements = append(requirements, "file_ops")
	}

	// Check for search requirements
	if strings.Contains(descLower, "search") || strings.Contains(descLower, "find") ||
		strings.Contains(descLower, "lookup") || strings.Contains(descLower, "query") {
		requirements = append(requirements, "search")
	}

	// Check for analysis requirements
	if strings.Contains(descLower, "analyze") || strings.Contains(descLower, "process") ||
		strings.Contains(descLower, "summarize") || strings.Contains(descLower, "review") {
		requirements = append(requirements, "analysis")
	}

	return requirements
}

// SubmitTask adds a task to the queue
func (c *Coordinator) SubmitTask(ctx context.Context, task *Task) error {
	// Store task in active map
	c.mu.Lock()
	c.active[task.ID] = &TaskExecution{
		Task:       *task,
		AgentID:    "",
		StartTime:  time.Now(),
		Deadline:   time.Now().Add(30 * time.Minute), // Default 30 min deadline
		ResultChan: make(chan *TaskResult, 1),
	}
	c.mu.Unlock()

	c.taskQueue <- *task
	return nil
}

// GetTask retrieves a task by ID
func (c *Coordinator) GetTask(taskID string) (*Task, bool) {
	// Check active tasks
	c.mu.RLock()
	defer c.mu.RUnlock()

	if exec, exists := c.active[taskID]; exists {
		return &exec.Task, true
	}

	// Check completed tasks
	if result, exists := c.completed[taskID]; exists {
		// Try to get original task from metadata
		if task, ok := result.Metadata["original_task"].(Task); ok {
			// Update the status to match the result
			task.Status = result.Status
			return &task, true
		}
		// Fallback: return a task with just the ID and status
		task := Task{
			ID:     taskID,
			Status: result.Status,
		}
		return &task, true
	}

	return nil, false
}

// ListTasks returns all tasks (active and completed)
func (c *Coordinator) ListTasks() []*Task {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var tasks []*Task

	// Add active tasks
	for _, exec := range c.active {
		tasks = append(tasks, &exec.Task)
	}

	// Add completed tasks (as placeholders)
	for taskID := range c.completed {
		tasks = append(tasks, &Task{ID: taskID})
	}

	return tasks
}

// GetStats returns coordinator statistics
func (c *Coordinator) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["active_tasks"] = len(c.active)
	stats["completed_tasks"] = len(c.completed)
	stats["total_agents"] = len(c.team.ListMembers())
	stats["strategy"] = c.strategy

	return stats
}
