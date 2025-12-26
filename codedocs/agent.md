# Agent Package Documentation

## Package Overview

**Package**: `sdk/agent`  
**Purpose**: Core agent definitions, memory management, and workflow orchestration  
**Stability**: Alpha  
**Test Coverage**: ~60% (team tests only)

---

## Architecture

The agent package implements a modular agent system with memory, workflows, and tool integration:

```
┌─────────────────────────────────────────────┐
│           Agent Definition                  │
├─────────────────────────────────────────────┤
│           Memory Store                      │
├─────────────────────────────────────────────┤
│           Workflow Engine                   │
├─────────────────────────────────────────────┤
│           Tool Registry                     │
└─────────────────────────────────────────────┘
```

**Key Concepts**:
- **Agent**: Independent entity with capabilities and configuration
- **Team**: Collection of agents working together
- **Memory**: Thread-safe conversation history
- **Workflow**: Multi-step task orchestration
- **Tool**: External capability invocation

---

## Core Types

### 1. Agent

**File**: `agent.go`

Represents an AI agent with capabilities, configuration, and runtime state.

```go
type Agent struct {
    ID            string
    Name          string
    Mode          Mode
    Provider      string
    Capabilities  []string
    Config        map[string]interface{}
    Metadata      map[string]interface{}
    Status        Status
    MaxIterations int
    Timeout       time.Duration
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

**Agent Modes**:
```go
const (
    ModeLLM       Mode = "llm"        // Language model only
    ModeTool      Mode = "tool"       // Tool use only
    ModeWorkflow  Mode = "workflow"   // Multi-step workflows
    ModeHybrid    Mode = "hybrid"     // LLM + tools
)
```

**Agent Statuses**:
```go
const (
    StatusIdle      Status = "idle"
    StatusActive    Status = "active"
    StatusBusy      Status = "busy"
    StatusError     Status = "error"
    StatusDisabled  Status = "disabled"
)
```

**Key Methods**:
- `New(name string, mode Mode) *Agent` - Factory function
- `AddCapability(cap string)` - Add capability
- `RemoveCapability(cap string)` - Remove capability
- `SetConfig(key string, value interface{})` - Update configuration
- `Clone() *Agent` - Create deep copy

**JSON Serialization**:
```go
// Custom marshaling for metadata preservation
func (a *Agent) MarshalJSON() ([]byte, error)
func (a *Agent) UnmarshalJSON(data []byte) error
```

**Notes**:
- Agents are immutable after creation (use clone for modifications)
- Capabilities determine what the agent can do
- Provider specifies which LLM provider to use
- Config contains provider-specific settings

---

### 2. Team

**File**: `team.go`

Represents a collection of agents working together toward a common goal.

```go
type Team struct {
    ID          string
    Name        string
    Description string
    Members     []*Agent
    Metadata    map[string]interface{}
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

**Key Methods**:
- `NewTeam(name string) *Team` - Create new team
- `AddMember(agent *Agent, role string) error` - Add agent
- `RemoveMember(agentID string) error` - Remove agent
- `GetMember(agentID string) *Agent` - Find agent by ID
- `ListMembersByRole(role string) []*Agent` - Filter by role
- `GetLeader() *Agent` - Get team leader
- `GetOrchestrator() *Agent` - Get workflow orchestrator

**Team Roles**:
- `leader` - Decision maker
- `orchestrator` - Workflow coordinator
- `member` - Regular participant
- `observer` - Read-only access

**Team Communication**:
- Broadcast messages to all members
- Direct messaging between specific agents
- Shared memory for context preservation

**Example**:
```go
team := NewTeam("research-team")
team.Description = "Multi-agent research team"

// Add members
llmAgent := agent.New("researcher", ModeLLM)
team.AddMember(llmAgent, "member")

// Add orchestrator
orchAgent := agent.New("coordinator", ModeWorkflow)
team.AddMember(orchAgent, "orchestrator")

// Broadcast message
team.Broadcast("New research goal: Climate change analysis")
```

---

### 3. Memory

**File**: `memory.go`

Thread-safe conversation history and context store.

```go
type Memory struct {
    mu      sync.RWMutex
    store   map[string][]Message
    maxSize int
}
```

**Key Methods**:
- `NewMemory(maxMessages int) *Memory` - Create memory store
- `Add(conversationID string, msg Message)` - Add message
- `Get(conversationID string, limit int) []Message` - Retrieve history
- `GetSince(conversationID string, since time.Time) []Message` - Time-based filter
- `Clear(conversationID string)` - Clear conversation
- `ClearAll()` - Clear everything
- `Size(conversationID string) int` - Get message count

**Message Structure**:
```go
type Message struct {
    ID             string
    ConversationID string
    SenderID       string
    ReceiverID     string
    ContentType    string
    Content        string
    Metadata       map[string]interface{}
    Timestamp      time.Time
}
```

**Eviction Policy**:
- When max size is reached, oldest messages removed
- Per-conversation limits supported
- Priority messages can be preserved

**Concurrency Safety**:
- All operations protected by `sync.RWMutex`
- Read-heavy workload optimized
- No lock contention in typical usage

**Example**:
```go
memory := agent.NewMemory(100) // Keep last 100 messages

// Add message
msg := agent.Message{
    ConversationID: "conv-123",
    SenderID:       "agent-1",
    ContentType:    "text/plain",
    Content:        "Hello, team!",
    Timestamp:      time.Now(),
}
memory.Add("conv-123", msg)

// Retrieve conversation
history := memory.Get("conv-123", 50)
```

---

### 4. Workflow

**File**: `workflow.go`

Multi-step task orchestration engine.

```go
type Workflow struct {
    ID           string
    Name         string
    Description  string
    Steps        []Step
    Variables    map[string]interface{}
    Status       WorkflowStatus
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

**Workflow Statuses**:
```go
const (
    WorkflowPending   WorkflowStatus = "pending"
    WorkflowRunning   WorkflowStatus = "running"
    WorkflowPaused    WorkflowStatus = "paused"
    WorkflowCompleted WorkflowStatus = "completed"
    WorkflowFailed    WorkflowStatus = "failed"
    WorkflowCancelled WorkflowStatus = "cancelled"
)
```

**Step Structure**:
```go
type Step struct {
    ID          string
    Name        string
    Type        StepType
    AgentID     string
    Input       map[string]interface{}
    Output      map[string]interface{}
    Condition   string  // Expression to evaluate
    RetryPolicy RetryPolicy
    Timeout     time.Duration
    OnSuccess   string  // Next step ID
    OnFailure   string  // Failure handler step ID
}
```

**Step Types**:
- `StepLLM` - LLM generation
- `StepTool` - Tool execution
- `StepHuman` - Human input
- `StepCondition` - Conditional branch
- `StepParallel` - Parallel execution
- `StepLoop` - Loop/iteration

**Key Methods**:
- `NewWorkflow(name string) *Workflow` - Create workflow
- `AddStep(step Step) error` - Add step to workflow
- `Start(ctx context.Context) error` - Begin execution
- `Pause()` - Pause execution
- `Resume()` - Resume execution
- `Cancel()` - Cancel workflow
- `GetStatus() WorkflowStatus` - Get current status
- `GetVariable(name string) interface{}` - Get workflow variable
- `SetVariable(name string, value interface{})` - Set workflow variable

**Step Execution Flow**:
1. Validate step inputs
2. Check preconditions
3. Execute step (with timeout)
4. Capture outputs
5. Evaluate next step based on success/failure
6. Apply retry policy if needed

**Example**:
```go
workflow := agent.NewWorkflow("research-paper")

// Add data collection step
step1 := agent.Step{
    ID:      "collect-data",
    Name:    "Collect Research Data",
    Type:    agent.StepTool,
    AgentID: "researcher-1",
    Input: map[string]interface{}{
        "query": "climate change impacts 2024",
    },
    Timeout: 5 * time.Minute,
}
workflow.AddStep(step1)

// Add analysis step
step2 := agent.Step{
    ID:      "analyze-data",
    Name:    "Analyze Data",
    Type:    agent.StepLLM,
    AgentID: "analyst-1",
    Condition: "previous.output.data_count > 10",
}
workflow.AddStep(step2)

// Execute workflow
err := workflow.Start(context.Background())
```

---

### 5. Tool Registry

**File**: `tools.go`

Manages external tool definitions and invocations.

```go
type ToolRegistry struct {
    mu    sync.RWMutex
    tools map[string]Tool
}

type Tool struct {
    Name        string
    Description string
    Parameters  []Parameter
    Handler     func(ctx context.Context, input map[string]interface{}) (interface{}, error)
    Timeout     time.Duration
    Category    string
}
```

**Tool Categories**:
- `search` - Web/search tools
- `compute` - Calculation tools
- `data` - Data manipulation
- `api` - External API calls
- `system` - System operations

**Key Methods**:
- `NewToolRegistry() *ToolRegistry` - Create registry
- `Register(tool Tool) error` - Add tool
- `Get(name string) (Tool, bool)` - Retrieve tool
- `List(category string) []Tool` - List tools by category
- `Execute(ctx context.Context, name string, input map[string]interface{}) (interface{}, error)` - Run tool

**Example**:
```go
registry := agent.NewToolRegistry()

// Define calculator tool
calcTool := agent.Tool{
    Name:        "calculator",
    Description: "Perform mathematical calculations",
    Category:    "compute",
    Parameters: []agent.Parameter{
        {Name: "expression", Type: "string", Required: true},
    },
    Handler: func(ctx context.Context, input map[string]interface{}) (interface{}, error) {
        expr := input["expression"].(string)
        return evalExpression(expr), nil
    },
    Timeout: 10 * time.Second,
}
registry.Register(calcTool)

// Execute tool
result, err := registry.Execute(ctx, "calculator", map[string]interface{}{
    "expression": "2 + 2 * 2",
})
```

---

### 6. Errors

**File**: `errors.go`

Custom error types for agent operations.

```go
var (
    ErrAgentNotFound    = errors.New("agent not found")
    ErrTeamNotFound     = errors.New("team not found")
    ErrInvalidMode      = errors.New("invalid agent mode")
    ErrInvalidWorkflow  = errors.New("invalid workflow definition")
    ErrStepFailed       = errors.New("workflow step failed")
    ErrToolNotFound     = errors.New("tool not found")
    ErrMemoryFull       = errors.New("memory store full")
    ErrDuplicateAgent   = errors.New("duplicate agent name")
    ErrDuplicateTeam    = errors.New("duplicate team name")
)
```

**Error Handling Patterns**:
```go
// Check specific error
if errors.Is(err, agent.ErrAgentNotFound) {
    // Handle not found
}

// Wrap with context
return fmt.Errorf("workflow %s: step %s: %w", wf.ID, step.ID, err)
```

---

## Usage Patterns

### Creating an Agent

```go
import "github.com/jeffersonwarrior/modelscan/sdk/agent"

// Create basic LLM agent
llmAgent := agent.New("research-assistant", agent.ModeLLM)
llmAgent.Provider = "openai"
llmAgent.Capabilities = []string{"text-generation", "analysis", "chat"}
llmAgent.Config = map[string]interface{}{
    "model":      "gpt-4",
    "temperature": 0.7,
    "max_tokens": 4000,
}
llmAgent.Metadata = map[string]interface{}{
    "version": "1.0",
    "domain":  "general-research",
}

// Create tool agent
toolAgent := agent.New("web-scraper", agent.ModeTool)
toolAgent.Capabilities = []string{"web-scraping", "data-extraction"}
```

### Team Collaboration

```go
// Create team
team := agent.NewTeam("data-analysis-team")
team.Description = "Multi-agent system for data analysis"

// Add specialized agents
researcher := agent.New("researcher", agent.ModeLLM)
analyst := agent.New("analyst", agent.ModeLLM)
scraper := agent.New("scraper", agent.ModeTool)
orchestrator := agent.New("coordinator", agent.ModeWorkflow)

team.AddMember(researcher, "member")
team.AddMember(analyst, "member")
team.AddMember(scraper, "member")
team.AddMember(orchestrator, "orchestrator")

// Coordinate work
orchestrator.SendMessage("Start analysis on dataset X")
```

### Workflow Execution

```go
// Define workflow
wf := agent.NewWorkflow("customer-support")

// Add intake step
intakeStep := agent.Step{
    ID:       "intake",
    Type:     agent.StepLLM,
    AgentID:  receptionist.ID,
    Input: map[string]interface{}{
        "prompt": "Classify customer request: {{.message}}",
    },
}

// Add routing step
routeStep := agent.Step{
    ID:        "route",
    Type:      agent.StepCondition,
    Condition: "contains(output.category, 'technical')",
    OnSuccess: "support-agent",
    OnFailure: "sales-agent",
}

// Execute
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

err := wf.Start(ctx)
if err != nil {
    log.Fatalf("Workflow failed: %v", err)
}
```

---

## Testing

### Current Coverage

- Team operations: 80%
- Agent creation: 60%
- Memory operations: 70%
- Workflows: 0%
- Tools: 0%

**Overall**: ~60% (Target: 100%)

### Test Examples

```go
func TestTeamAddMember(t *testing.T) {
    team := NewTeam("test-team")
    agent := New("test-agent", ModeLLM)
    
    err := team.AddMember(agent, "member")
    assert.NoError(t, err)
    assert.Equal(t, 1, len(team.Members))
}

func TestMemoryConcurrency(t *testing.T) {
    mem := NewMemory(100)
    
    // Concurrent writes
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            msg := Message{
                ConversationID: "test",
                Content:        fmt.Sprintf("Message %d", id),
            }
            mem.Add("test", msg)
        }(i)
    }
    wg.Wait()
    
    assert.Equal(t, 10, mem.Size("test"))
}
```

---

## Performance Considerations

### Memory Management

**Current**: Memory store grows unbounded per conversation  
**Recommendation**: Implement LRU eviction

```go
type Memory struct {
    store   map[string]*lru.Cache
    maxSize int
    
}
```

### Workflow Execution

**Current**: Synchronous step execution  
**Recommendation**: Parallel step support

```go
// For StepParallel
for _, parallelStep := range step.ParallelSteps {
    go func(s Step) {
        defer wg.Done()
        executeStep(ctx, s)
    }(parallelStep)
}
```

---

## Security Considerations

### Agent Isolation

**Current**: No sandboxing between agents  
**Recommendation**: Namespace isolation for:
- File system access
- Network access
- Memory allocation

### Tool Security

**Current**: Tools run with agent permissions  
**Recommendations**:
1. Whitelist allowed tools per agent
2. Input validation
3. Timeout enforcement
4. Resource limits (CPU, memory)
5. Sandboxed execution

```go
type ToolSecurityPolicy struct {
    AllowedTools []string
    MaxExecutionTime time.Duration
    MaxMemoryMB int
    NetworkAccess bool
}
```

---

## Known Issues

1. **Memory Leaks**: Memory store not pruned automatically
2. **Workflow Loops**: No detection for infinite loops
3. **Tool Errors**: No retry logic with exponential backoff
4. **State Management**: Workflow state persistence incomplete
5. **Agent Recovery**: No crash recovery mechanism

---

## Future Enhancements

1. **Agent Migration**: Move agents between teams
2. **Hierarchical Teams**: Nested team structures
3. **Agent Cloning**: Clone with modified configuration
4. **Workflow Templates**: Reusable workflow patterns
5. **Tool Marketplace**: Share tools between agents
6. **Agent Versioning**: Track agent evolution
7. **Performance Metrics**: Agent execution statistics

---

**Last Updated**: December 18, 2025  
**Package Version**: v0.2.0  
**Maintainer**: ModelScan Team