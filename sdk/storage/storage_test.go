package storage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates a new test database for each test

func ptr(s string) *string { return &s }

func setupTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	
	// Create temporary database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	
	// Run migrations
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE IF NOT EXISTS agents (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			capabilities TEXT,
			config TEXT,
			status VARCHAR(50) DEFAULT 'idle',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE IF NOT EXISTS teams (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			config TEXT,
			metadata TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE IF NOT EXISTS team_members (
			team_id VARCHAR(255) NOT NULL,
			agent_id VARCHAR(255) NOT NULL,
			role VARCHAR(100) DEFAULT 'member',
			joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (team_id, agent_id),
			FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		);
		
		CREATE TABLE IF NOT EXISTS tasks (
			id VARCHAR(255) PRIMARY KEY,
			agent_id VARCHAR(255) NOT NULL,
			team_id VARCHAR(255),
			type VARCHAR(100) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			priority INTEGER DEFAULT 0,
			input TEXT,
			output TEXT,
			metadata TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			completed_at DATETIME,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
			FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE SET NULL
		);
		
		CREATE TABLE IF NOT EXISTS messages (
			id VARCHAR(255) PRIMARY KEY,
			task_id VARCHAR(255) NOT NULL,
			agent_id VARCHAR(255) NOT NULL,
			team_id VARCHAR(255),
			type VARCHAR(100) NOT NULL,
			content TEXT,
			metadata TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
			FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE SET NULL
		);
		
		CREATE TABLE IF NOT EXISTS tool_executions (
			id VARCHAR(255) PRIMARY KEY,
			task_id VARCHAR(255) NOT NULL,
			agent_id VARCHAR(255) NOT NULL,
			tool_name VARCHAR(255) NOT NULL,
			tool_type VARCHAR(100),
			input TEXT,
			output TEXT,
			error TEXT,
			status VARCHAR(50) DEFAULT 'running',
			duration INTEGER DEFAULT 0,
			metadata TEXT,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		);
		
		INSERT OR IGNORE INTO schema_migrations (version) VALUES ('001_initial');
	`)
	if err != nil {
		t.Fatalf("Failed to setup test schema: %v", err)
	}
	
	return db, dbPath
}

func TestStorageLifecycle(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	storage := NewStorage(db, 1*time.Hour)
	
	// Test health check
	if err := storage.PerformHealthCheck(ctx); err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	
	// Test storage stats
	stats, err := storage.GetStorageStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get storage stats: %v", err)
	}
	
	// Verify initial stats
	if stats["agents"] != 0 {
		t.Errorf("Expected 0 agents, got %v", stats["agents"])
	}
	if stats["tasks"] != 0 {
		t.Errorf("Expected 0 tasks, got %v", stats["tasks"])
	}
	if stats["messages"] != 0 {
		t.Errorf("Expected 0 messages, got %v", stats["messages"])
	}
}

func TestAgentRepository(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	repo := NewAgentRepository(db)
	
	// Test creating an agent
	agent := &Agent{
		ID:           "test-agent-1",
		Name:         "Test Agent",
		Capabilities: []string{"text-generation", "analysis"},
		Config:       `{"model": "gpt-4"}`,
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	
	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	
	// Test getting an agent
	retrieved, err := repo.Get(ctx, "test-agent-1")
	if err != nil {
		t.Fatalf("Failed to get agent: %v", err)
	}
	
	if retrieved.Name != agent.Name {
		t.Errorf("Expected name %s, got %s", agent.Name, retrieved.Name)
	}
	
	if len(retrieved.Capabilities) != len(agent.Capabilities) {
		t.Errorf("Expected %d capabilities, got %d", len(agent.Capabilities), len(retrieved.Capabilities))
	}
	
	// Test updating an agent
	agent.Status = "active"
	agent.Capabilities = append(agent.Capabilities, "summarization")
	
	err = repo.Update(ctx, agent)
	if err != nil {
		t.Fatalf("Failed to update agent: %v", err)
	}
	
	// Verify update
	updated, err := repo.Get(ctx, "test-agent-1")
	if err != nil {
		t.Fatalf("Failed to get updated agent: %v", err)
	}
	
	if updated.Status != "active" {
		t.Errorf("Expected status active, got %s", updated.Status)
	}
	
	// Test listing agents
	agents, err := repo.ListActive(ctx, 10, 0)
	if err != nil {
		t.Fatalf("Failed to list agents: %v", err)
	}
	
	if len(agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(agents))
	}
	
	// Test List (all agents)
	allAgents, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list all agents: %v", err)
	}
	
	if len(allAgents) != 1 {
		t.Errorf("Expected 1 agent in List, got %d", len(allAgents))
	}
	
	// Test UpdateStatus
	err = repo.UpdateStatus(ctx, "test-agent-1", "busy")
	if err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}
	
	// Verify status update
	statusUpdated, err := repo.Get(ctx, "test-agent-1")
	if err != nil {
		t.Fatalf("Failed to get agent after status update: %v", err)
	}
	
	if statusUpdated.Status != "busy" {
		t.Errorf("Expected status busy, got %s", statusUpdated.Status)
	}
	
	// Test ListByStatus
	busyAgents, err := repo.ListByStatus(ctx, "busy")
	if err != nil {
		t.Fatalf("Failed to list agents by status: %v", err)
	}
	
	if len(busyAgents) != 1 {
		t.Errorf("Expected 1 busy agent, got %d", len(busyAgents))
	}
	
	// Test Delete
	err = repo.Delete(ctx, "test-agent-1")
	if err != nil {
		t.Fatalf("Failed to delete agent: %v", err)
	}
	
	// Verify deletion
	_, err = repo.Get(ctx, "test-agent-1")
	if err == nil {
		t.Error("Expected error when getting deleted agent")
	}
	
	// Test delete non-existent agent
	err = repo.Delete(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error when deleting non-existent agent")
	}
}

func TestTaskRepository(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	repo := NewTaskRepository(db)
	
	// Test creating a task
	task := &Task{
		ID:        "test-task-1",
		AgentID:   "test-agent-1",
		Type:      "text-generation",
		Status:    "pending",
		Priority:  1,
		Input:     "Write a test",
		Metadata:  map[string]interface{}{"test": true},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	err := repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}
	
	// Test getting a task
	retrieved, err := repo.Get(ctx, "test-task-1")
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}
	
	if retrieved.Type != task.Type {
		t.Errorf("Expected type %s, got %s", task.Type, retrieved.Type)
	}
	
	if retrieved.Metadata["test"] != true {
		t.Errorf("Expected metadata test=true, got %v", retrieved.Metadata["test"])
	}
	
	// Test updating task status
	err = repo.UpdateStatus(ctx, "test-task-1", "running")
	if err != nil {
		t.Fatalf("Failed to update task status: %v", err)
	}
	
	// Verify status update
	updated, err := repo.Get(ctx, "test-task-1")
	if err != nil {
		t.Fatalf("Failed to get updated task: %v", err)
	}
	
	if updated.Status != "running" {
		t.Errorf("Expected status running, got %s", updated.Status)
	}
	
	// Test listing tasks by status
	tasks, err := repo.ListByStatus(ctx, "running", 10, 0)
	if err != nil {
		t.Fatalf("Failed to list tasks by status: %v", err)
	}
	
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}
}

func TestTeamRepository(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	repo := NewTeamRepository(db)
	
	// Create a team
	team := &Team{
		ID:          "test-team-1",
		Name:        "Test Team",
		Description: "A test team",
		Config:      `{"version": "1.0"}`,
		Metadata:    map[string]interface{}{"test": true},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	err := repo.Create(ctx, team)
	if err != nil {
		t.Fatalf("Failed to create team: %v", err)
	}
	
	// Test getting a team
	retrieved, err := repo.Get(ctx, "test-team-1")
	if err != nil {
		t.Fatalf("Failed to get team: %v", err)
	}
	
	if retrieved.Name != team.Name {
		t.Errorf("Expected name %s, got %s", team.Name, retrieved.Name)
	}
	
	// Test adding team members
	err = repo.AddMember(ctx, "test-team-1", "agent-1", "lead")
	if err != nil {
		t.Fatalf("Failed to add team member: %v", err)
	}
	
	err = repo.AddMember(ctx, "test-team-1", "agent-2", "member")
	if err != nil {
		t.Fatalf("Failed to add second team member: %v", err)
	}
	
	// Test getting team members
	members, err := repo.GetMembers(ctx, "test-team-1")
	if err != nil {
		t.Fatalf("Failed to get team members: %v", err)
	}
	
	if len(members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(members))
	}
	
	// Test updating member role
	err = repo.UpdateMemberRole(ctx, "test-team-1", "agent-2", "lead")
	if err != nil {
		t.Fatalf("Failed to update member role: %v", err)
	}
	
	// Test removing team member
	err = repo.RemoveMember(ctx, "test-team-1", "agent-1")
	if err != nil {
		t.Fatalf("Failed to remove team member: %v", err)
	}
	
	// Verify removal
	members, err = repo.GetMembers(ctx, "test-team-1")
	if err != nil {
		t.Fatalf("Failed to get team members after removal: %v", err)
	}
	
	if len(members) != 1 {
		t.Errorf("Expected 1 member after removal, got %d", len(members))
	}
}

func TestMessageRepository(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	repo := NewMessageRepository(db)
	
	// Create a message
	message := &Message{
		ID:        "test-message-1",
		TaskID:    "test-task-1",
		AgentID:   "test-agent-1",
		Type:      "user_message",
		Content:   "Hello, world!",
		Metadata:  map[string]interface{}{"test": true},
		CreatedAt: time.Now(),
	}
	
	err := repo.Create(ctx, message)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}
	
	// Test getting a message
	retrieved, err := repo.Get(ctx, "test-message-1")
	if err != nil {
		t.Fatalf("Failed to get message: %v", err)
	}
	
	if retrieved.Content != message.Content {
		t.Errorf("Expected content %s, got %s", message.Content, retrieved.Content)
	}
	
	// Test listing messages by task
	messages, err := repo.ListByTask(ctx, "test-task-1", 10, 0)
	if err != nil {
		t.Fatalf("Failed to list messages by task: %v", err)
	}
	
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	
	// Test conversation thread
	thread, err := repo.GetConversationThread(ctx, "test-task-1")
	if err != nil {
		t.Fatalf("Failed to get conversation thread: %v", err)
	}
	
	if len(thread) != 1 {
		t.Errorf("Expected 1 message in thread, got %d", len(thread))
	}
}

func TestToolExecutionRepository(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	repo := NewToolExecutionRepository(db)
	
	// Create a tool execution
	execution := &ToolExecution{
		ID:        "test-exec-1",
		TaskID:    "test-task-1",
		AgentID:   "test-agent-1",
		ToolName:  "test_tool",
		ToolType:  "utility",
		Input:     "test input",
		Status:    "running",
		Duration:  0,
		Metadata:  map[string]interface{}{"test": true},
		StartedAt: time.Now(),
	}
	
	err := repo.Create(ctx, execution)
	if err != nil {
		t.Fatalf("Failed to create tool execution: %v", err)
	}
	
	// Test getting a tool execution
	retrieved, err := repo.Get(ctx, "test-exec-1")
	if err != nil {
		t.Fatalf("Failed to get tool execution: %v", err)
	}
	
	if retrieved.ToolName != execution.ToolName {
		t.Errorf("Expected tool name %s, got %s", execution.ToolName, retrieved.ToolName)
	}
	
	// Test marking as completed
	err = repo.MarkCompleted(ctx, "test-exec-1", "test output", "completed", 1000)
	if err != nil {
		t.Fatalf("Failed to mark tool execution completed: %v", err)
	}
	
	// Verify completion
	completed, err := repo.Get(ctx, "test-exec-1")
	if err != nil {
		t.Fatalf("Failed to get completed tool execution: %v", err)
	}
	
	if completed.Status != "completed" {
		t.Errorf("Expected status completed, got %s", completed.Status)
	}
	
	if completed.Duration != 1000 {
		t.Errorf("Expected duration 1000, got %d", completed.Duration)
	}
	
	// Test listing tool executions by task
	executions, err := repo.ListByTask(ctx, "test-task-1", 10, 0)
	if err != nil {
		t.Fatalf("Failed to list tool executions by task: %v", err)
	}
	
	if len(executions) != 1 {
		t.Errorf("Expected 1 execution, got %d", len(executions))
	}
}

func TestStorageIntegration(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	storage := NewStorage(db, 1*time.Hour)
	
	// Test zero-state initialization
	err := storage.InitializeZeroState(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize zero state: %v", err)
	}
	
	// Test creating with defaults
	agent := storage.NewAgentWithDefaults("Test Agent", "llm", []string{"text-generation"})
	err = storage.Agents.Create(ctx, agent)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	
	// Test cleanup
	err = storage.CleanupOldData(ctx)
	if err != nil {
		t.Fatalf("Failed to cleanup old data: %v", err)
	}
	
	// Test stats
	stats, err := storage.GetStorageStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	
	if stats["agents"] != 1 {
		t.Errorf("Expected 1 agent in stats, got %v", stats["agents"])
	}
}

func TestTaskRepository_CompleteCRUD(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	taskRepo := NewTaskRepository(db)
	agentRepo := NewAgentRepository(db)
	
	// Create an agent first
	agent := &Agent{
		ID:           "test-agent",
		Name:         "Test Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	// Test Create
	task := &Task{
		ID:        "task-1",
		AgentID:   "test-agent",
		Type:      "test",
		Status:    "pending",
		Priority:  1,
		Input:     "test input",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	err := taskRepo.Create(ctx, task)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	
	// Test Update
	task.Status = "running"
	task.Output = "test output"
	err = taskRepo.Update(ctx, task)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	
	// Verify update
	updated, err := taskRepo.Get(ctx, "task-1")
	if err != nil {
		t.Fatalf("Get after update failed: %v", err)
	}
	if updated.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", updated.Status)
	}
	
	// Test ListByAgent
	tasks, err := taskRepo.ListByAgent(ctx, "test-agent", 10, 0)
	if err != nil {
		t.Fatalf("ListByAgent failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}
	
	// Test Delete
	err = taskRepo.Delete(ctx, "task-1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	
	// Verify deletion
	_, err = taskRepo.Get(ctx, "task-1")
	if err == nil {
		t.Error("Expected error when getting deleted task")
	}
}

func TestTeamRepository_CompleteCRUD(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	teamRepo := NewTeamRepository(db)
	agentRepo := NewAgentRepository(db)
	
	// Create agents
	agent1 := &Agent{
		ID:           "agent-1",
		Name:         "Agent 1",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent1)
	
	// Test Create
	team := &Team{
		ID:          "team-1",
		Name:        "Test Team",
		Description: "A test team",
		Config:      `{"test": true}`,
		Metadata:    map[string]interface{}{"key": "value"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	err := teamRepo.Create(ctx, team)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	
	// Test Update
	team.Description = "Updated description"
	err = teamRepo.Update(ctx, team)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	
	// Test List
	teams, err := teamRepo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(teams) != 1 {
		t.Errorf("Expected 1 team, got %d", len(teams))
	}
	
	// Test GetAgentTeams
	agentTeams, err := teamRepo.GetAgentTeams(ctx, "agent-1")
	if err != nil {
		t.Fatalf("GetAgentTeams failed: %v", err)
	}
	if len(agentTeams) != 0 {
		t.Errorf("Expected 0 teams for agent, got %d", len(agentTeams))
	}
	
	// Test Delete
	err = teamRepo.Delete(ctx, "team-1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

func TestMessageRepository_CompleteCRUD(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	msgRepo := NewMessageRepository(db)
	taskRepo := NewTaskRepository(db)
	agentRepo := NewAgentRepository(db)
	
	// Create agent and task
	agent := &Agent{
		ID:           "agent-1",
		Name:         "Agent 1",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	task := &Task{
		ID:        "task-1",
		AgentID:   "agent-1",
		Type:      "test",
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	taskRepo.Create(ctx, task)
	
	// Test Create
	msg := &Message{
		ID:        "msg-1",
		TaskID:    "task-1",
		AgentID:   "agent-1",
		Type:      "text",
		Content:   "Hello",
		CreatedAt: time.Now(),
	}
	
	err := msgRepo.Create(ctx, msg)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	
	// Test ListByAgent
	messages, err := msgRepo.ListByAgent(ctx, "agent-1", 10, 0)
	if err != nil {
		t.Fatalf("ListByAgent failed: %v", err)
	}
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	
	// Test DeleteByTask
	err = msgRepo.DeleteByTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("DeleteByTask failed: %v", err)
	}
	
	// Test Delete
	msg2 := &Message{
		ID:        "msg-2",
		TaskID:    "task-1",
		AgentID:   "agent-1",
		Type:      "text",
		Content:   "Test",
		CreatedAt: time.Now(),
	}
	msgRepo.Create(ctx, msg2)
	
	err = msgRepo.Delete(ctx, "msg-2")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

func TestToolExecutionRepository_CompleteCRUD(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	toolRepo := NewToolExecutionRepository(db)
	taskRepo := NewTaskRepository(db)
	agentRepo := NewAgentRepository(db)
	
	// Create agent and task
	agent := &Agent{
		ID:           "agent-1",
		Name:         "Agent 1",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	task := &Task{
		ID:        "task-1",
		AgentID:   "agent-1",
		Type:      "test",
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	taskRepo.Create(ctx, task)
	
	// Test Create
	tool := &ToolExecution{
		ID:        "tool-1",
		TaskID:    "task-1",
		AgentID:   "agent-1",
		ToolName:  "test_tool",
		Status:    "running",
		StartedAt: time.Now(),
	}
	
	err := toolRepo.Create(ctx, tool)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	
	// Test Update
	tool.Status = "completed"
	tool.Output = "success"
	err = toolRepo.Update(ctx, tool)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	
	// Test MarkFailed
	tool2 := &ToolExecution{
		ID:        "tool-2",
		TaskID:    "task-1",
		AgentID:   "agent-1",
		ToolName:  "test_tool2",
		Status:    "running",
		StartedAt: time.Now(),
	}
	toolRepo.Create(ctx, tool2)
	
	err = toolRepo.MarkFailed(ctx, "tool-2", "Test error", 100)
	if err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}
	
	// Test ListByAgent
	tools, err := toolRepo.ListByAgent(ctx, "agent-1", 10, 0)
	if err != nil {
		t.Fatalf("ListByAgent failed: %v", err)
	}
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}
	
	// Test ListByTool
	toolsByName, err := toolRepo.ListByTool(ctx, "test_tool", 10, 0)
	if err != nil {
		t.Fatalf("ListByTool failed: %v", err)
	}
	if len(toolsByName) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(toolsByName))
	}
	
	// Test GetUsageStats
	stats, err := toolRepo.GetUsageStats(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("GetUsageStats failed: %v", err)
	}
	if len(stats) < 1 {
		t.Errorf("Expected at least 1 stat entry, got %d", len(stats))
	}
	
	// Test DeleteByTask
	err = toolRepo.DeleteByTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("DeleteByTask failed: %v", err)
	}
}

func TestStorage_HelperFunctions(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	storage := NewStorage(db, 1*time.Hour)
	
	// Test NewTaskWithDefaults
	task := storage.NewTaskWithDefaults("agent-1", "test", "test input", 1)
	if task.ID == "" {
		t.Error("Task ID should be generated")
	}
	if task.AgentID != "agent-1" {
		t.Errorf("Expected agent ID 'agent-1', got '%s'", task.AgentID)
	}
	
	// Test NewMessageWithDefaults
	msg := storage.NewMessageWithDefaults("task-1", "agent-1", "text", "Hello")
	if msg.ID == "" {
		t.Error("Message ID should be generated")
	}
	if msg.Content != "Hello" {
		t.Errorf("Expected content 'Hello', got '%s'", msg.Content)
	}
	
	// Test NewTeamWithDefaults
	team := storage.NewTeamWithDefaults("Test Team", "Description")
	if team.ID == "" {
		t.Error("Team ID should be generated")
	}
	if team.Name != "Test Team" {
		t.Errorf("Expected name 'Test Team', got '%s'", team.Name)
	}
	
	// Test NewToolExecutionWithDefaults
	tool := storage.NewToolExecutionWithDefaults("task-1", "agent-1", "test_tool", "function", "input")
	if tool.ID == "" {
		t.Error("Tool ID should be generated")
	}
	if tool.ToolName != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", tool.ToolName)
	}
	
	// Test SetAllAgentsIdle
	agent := storage.NewAgentWithDefaults("Test Agent", "llm", []string{"test"})
	agent.Status = "busy"
	storage.Agents.Create(ctx, agent)
	
	err := storage.SetAllAgentsIdle(ctx)
	if err != nil {
		t.Fatalf("SetAllAgentsIdle failed: %v", err)
	}
	
	// Test CancelAllPendingTasks
	err = storage.CancelAllPendingTasks(ctx)
	if err != nil {
		t.Fatalf("CancelAllPendingTasks failed: %v", err)
	}
	
	// Test PerformHealthCheck
	err = storage.PerformHealthCheck(ctx)
	if err != nil {
		t.Fatalf("PerformHealthCheck failed: %v", err)
	}
	// Health check passed
	
	// Test Close
	err = storage.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestDatabase_Lifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := tempDir + "/test.db"
	
	// Test NewAgentDB
	db, err := NewAgentDB(dbPath)
	if err != nil {
		t.Fatalf("NewAgentDB failed: %v", err)
	}
	
	// Test GetDB
	sqlDB := db.GetDB()
	if sqlDB == nil {
		t.Error("GetDB returned nil")
	}
	
	// Test that tables were created
	ctx := context.Background()
	var count int
	err = sqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query tables: %v", err)
	}
	if count < 5 {
		t.Errorf("Expected at least 5 tables, got %d", count)
	}
	
	// Test CleanupOldData
	err = db.CleanupOldData(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupOldData failed: %v", err)
	}
	
	// Test Close
	err = db.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestAgentRepository_SetActive(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	repo := NewAgentRepository(db)
	
	// Create agents
	agent1 := &Agent{
		ID:           "agent-1",
		Name:         "Agent 1",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo.Create(ctx, agent1)
	
	// Test SetActive
	err := repo.UpdateStatus(ctx, "agent-1", "active")
	if err != nil {
		t.Fatalf("SetActive failed: %v", err)
	}
	
	// Verify status changed to active
	updated, _ := repo.Get(ctx, "agent-1")
	if updated.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", updated.Status)
	}
	
	// Test SetActive false
	err = repo.UpdateStatus(ctx, "agent-1", "idle")
	if err != nil {
		t.Fatalf("SetActive false failed: %v", err)
	}
	
	updated, _ = repo.Get(ctx, "agent-1")
	if updated.Status != "idle" {
		t.Errorf("Expected status 'idle', got '%s'", updated.Status)
	}
}

func TestTaskRepository_ListByTeam(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	taskRepo := NewTaskRepository(db)
	agentRepo := NewAgentRepository(db)
	teamRepo := NewTeamRepository(db)
	
	// Create agent, team, and task
	agent := &Agent{
		ID:           "agent-1",
		Name:         "Agent 1",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	team := &Team{
		ID:        "team-1",
		Name:      "Team 1",
		Config:    "{}",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	teamRepo.Create(ctx, team)
	
	task := &Task{
		ID:        "task-1",
		AgentID:   "agent-1",
		TeamID:    ptr("team-1"),
		Type:      "test",
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	taskRepo.Create(ctx, task)
	
	// Test ListByTeam
	tasks, err := taskRepo.ListByTeam(ctx, "team-1", 10, 0)
	if err != nil {
		t.Fatalf("ListByTeam failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}
}

func TestMessageRepository_ListByTeam(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	msgRepo := NewMessageRepository(db)
	taskRepo := NewTaskRepository(db)
	agentRepo := NewAgentRepository(db)
	teamRepo := NewTeamRepository(db)
	
	// Create dependencies
	agent := &Agent{
		ID:           "agent-1",
		Name:         "Agent 1",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	team := &Team{
		ID:        "team-1",
		Name:      "Team 1",
		Config:    "{}",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	teamRepo.Create(ctx, team)
	
	task := &Task{
		ID:        "task-1",
		AgentID:   "agent-1",
		TeamID:    ptr("team-1"),
		Type:      "test",
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	taskRepo.Create(ctx, task)
	
	msg := &Message{
		ID:        "msg-1",
		TaskID:    "task-1",
		AgentID:   "agent-1",
		TeamID:    ptr("team-1"),
		Type:      "text",
		Content:   "Hello",
		CreatedAt: time.Now(),
	}
	msgRepo.Create(ctx, msg)
	
	// Test ListByTeam
	messages, err := msgRepo.ListByTeam(ctx, "team-1", 10, 0)
	if err != nil {
		t.Fatalf("ListByTeam failed: %v", err)
	}
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
}

func TestAgentRepository_SetActiveMultiple(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	repo := NewAgentRepository(db)
	
	// Create multiple agents
	agents := []*Agent{
		{
			ID:           "agent-1",
			Name:         "Agent 1",
			Capabilities: []string{"test"},
			Status:       "active",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           "agent-2",
			Name:         "Agent 2",
			Capabilities: []string{"test"},
			Status:       "active",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           "agent-3",
			Name:         "Agent 3",
			Capabilities: []string{"test"},
			Status:       "active",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}
	
	for _, agent := range agents {
		if err := repo.Create(ctx, agent); err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
	}
	
	// Test SetActive with specific agents
	err := repo.SetActive(ctx, []string{"agent-1", "agent-3"})
	if err != nil {
		t.Fatalf("SetActive failed: %v", err)
	}
	
	// Verify correct agents are active
	agent1, _ := repo.Get(ctx, "agent-1")
	if agent1.Status != "active" {
		t.Errorf("Expected agent-1 to be active, got %s", agent1.Status)
	}
	
	agent2, _ := repo.Get(ctx, "agent-2")
	if agent2.Status != "inactive" {
		t.Errorf("Expected agent-2 to be inactive, got %s", agent2.Status)
	}
	
	agent3, _ := repo.Get(ctx, "agent-3")
	if agent3.Status != "active" {
		t.Errorf("Expected agent-3 to be active, got %s", agent3.Status)
	}
	
	// Test SetActive with empty list (all should be inactive)
	err = repo.SetActive(ctx, []string{})
	if err != nil {
		t.Fatalf("SetActive with empty list failed: %v", err)
	}
	
	// Verify all are inactive
	for _, agentID := range []string{"agent-1", "agent-2", "agent-3"} {
		agent, _ := repo.Get(ctx, agentID)
		if agent.Status != "inactive" {
			t.Errorf("Expected %s to be inactive after empty SetActive, got %s", agentID, agent.Status)
		}
	}
}

func TestAgentDB_CleanupScheduler(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	adb, err := NewAgentDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create AgentDB: %v", err)
	}
	defer adb.Close()
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Start cleanup scheduler with short interval
	adb.StartCleanupScheduler(ctx, 100*time.Millisecond, 24*time.Hour)
	
	// Let it run for a bit
	time.Sleep(250 * time.Millisecond)
	
	// Cancel context to stop scheduler
	cancel()
	
	// Give it time to stop
	time.Sleep(100 * time.Millisecond)
	
	// If we got here without hanging, the test passes
}

func TestMessageRepository_Get(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	msgRepo := NewMessageRepository(db)
	agentRepo := NewAgentRepository(db)
	taskRepo := NewTaskRepository(db)
	
	// Create agent and task first
	agent := &Agent{
		ID:           "agent-1",
		Name:         "Test Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	task := &Task{
		ID:        "task-1",
		AgentID:   "agent-1",
		Input:     "test input",
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	taskRepo.Create(ctx, task)
	
	// Create message
	msg := &Message{
		ID:        "msg-1",
		TaskID:    "task-1",
		AgentID:   "agent-1",
		Type:      "user",
		Content:   "test message",
		CreatedAt: time.Now(),
	}
	
	err := msgRepo.Create(ctx, msg)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}
	
	// Test Get
	retrieved, err := msgRepo.Get(ctx, "msg-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	
	if retrieved.ID != "msg-1" {
		t.Errorf("Expected ID msg-1, got %s", retrieved.ID)
	}
	if retrieved.Content != "test message" {
		t.Errorf("Expected content 'test message', got %s", retrieved.Content)
	}
	
	// Test Get with non-existent ID
	_, err = msgRepo.Get(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent message")
	}
}

func TestStorage_InitializeZeroState(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	storage := NewStorage(db, 1*time.Hour)
	
	// Create some agents and tasks
	agent := &Agent{
		ID:           "agent-1",
		Name:         "Test Agent",
		Capabilities: []string{"test"},
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	storage.Agents.Create(ctx, agent)
	
	task := &Task{
		ID:        "task-1",
		AgentID:   "agent-1",
		Input:     "test",
		Status:    "running",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	storage.Tasks.Create(ctx, task)
	
	// Initialize zero state
	err := storage.InitializeZeroState(ctx)
	if err != nil {
		t.Fatalf("InitializeZeroState failed: %v", err)
	}
	
	// Verify agent is inactive
	updatedAgent, _ := storage.Agents.Get(ctx, "agent-1")
	if updatedAgent.Status != "idle" {
		t.Errorf("Expected agent to be idle, got %s", updatedAgent.Status)
	}
	
	// Verify task is cancelled
	updatedTask, _ := storage.Tasks.Get(ctx, "task-1")
	if updatedTask.Status != "cancelled" {
		t.Errorf("Expected task to be cancelled, got %s", updatedTask.Status)
	}
}

func TestStorage_CloseIdempotent(t *testing.T) {
	db, _ := setupTestDB(t)
	
	storage := NewStorage(db, 1*time.Hour)
	
	// First close
	err := storage.Close()
	if err != nil {
		t.Errorf("First Close failed: %v", err)
	}
	
	// Second close should not error (idempotent)
	err = storage.Close()
	if err != nil {
		t.Errorf("Second Close failed: %v", err)
	}
}

func TestAgentDB_CloseIdempotent(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	adb, err := NewAgentDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create AgentDB: %v", err)
	}
	
	// First close
	err = adb.Close()
	if err != nil {
		t.Errorf("First Close failed: %v", err)
	}
	
	// Second close should not error
	err = adb.Close()
	if err != nil {
		t.Errorf("Second Close failed: %v", err)
	}
}

func TestAgentDB_Migration(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_migration.db")
	
	// Create database with version 1 schema only
	adb, err := NewAgentDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create AgentDB: %v", err)
	}
	
	// Verify migrations were applied
	var version int
	err = adb.GetDB().QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version)
	if err != nil {
		t.Fatalf("Failed to check migration version: %v", err)
	}
	
	if version < 1 {
		t.Errorf("Expected at least migration version 1, got %d", version)
	}
	
	adb.Close()
	
	// Reopen database - migrations should not run again
	adb2, err := NewAgentDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen AgentDB: %v", err)
	}
	defer adb2.Close()
	
	// Verify version is still the same or higher
	var version2 int
	err = adb2.GetDB().QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version2)
	if err != nil {
		t.Fatalf("Failed to check migration version after reopen: %v", err)
	}
	
	if version2 < version {
		t.Errorf("Migration version decreased from %d to %d", version, version2)
	}
}

func TestAgentDB_TablesCreated(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_tables.db")
	
	adb, err := NewAgentDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create AgentDB: %v", err)
	}
	defer adb.Close()
	
	// Check that all expected tables exist
	expectedTables := []string{
		"schema_migrations",
		"agents",
		"teams",
		"tasks",
		"messages",
		"tool_executions",
	}
	
	for _, table := range expectedTables {
		var count int
		query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
		err := adb.GetDB().QueryRow(query, table).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to check for table %s: %v", table, err)
		}
		if count == 0 {
			t.Errorf("Expected table %s was not created", table)
		}
	}
}

func TestStorage_PerformHealthCheck_WithData(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	storage := NewStorage(db, 1*time.Hour)
	
	// Add some data
	agent := &Agent{
		ID:           "agent-health",
		Name:         "Health Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	storage.Agents.Create(ctx, agent)
	
	// Perform health check
	err := storage.PerformHealthCheck(ctx)
	if err != nil {
		t.Errorf("PerformHealthCheck failed: %v", err)
	}
}

func TestStorage_SetAllAgentsIdle(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	storage := NewStorage(db, 1*time.Hour)
	
	// Create agents with different statuses
	agents := []*Agent{
		{
			ID:           "agent-active",
			Name:         "Active Agent",
			Capabilities: []string{"test"},
			Status:       "active",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           "agent-running",
			Name:         "Running Agent",
			Capabilities: []string{"test"},
			Status:       "running",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}
	
	for _, agent := range agents {
		storage.Agents.Create(ctx, agent)
	}
	
	// Set all to idle
	err := storage.SetAllAgentsIdle(ctx)
	if err != nil {
		t.Fatalf("SetAllAgentsIdle failed: %v", err)
	}
	
	// Verify all are idle
	for _, agent := range agents {
		updated, _ := storage.Agents.Get(ctx, agent.ID)
		if updated.Status != "idle" {
			t.Errorf("Expected agent %s to be idle, got %s", agent.ID, updated.Status)
		}
	}
}

func TestStorage_CancelAllPendingTasks(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	storage := NewStorage(db, 1*time.Hour)
	
	// Create an agent first
	agent := &Agent{
		ID:           "agent-tasks",
		Name:         "Task Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	storage.Agents.Create(ctx, agent)
	
	// Create tasks with different statuses
	tasks := []*Task{
		{
			ID:        "task-pending",
			AgentID:   "agent-tasks",
			Input:     "test1",
			Status:    "pending",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "task-running",
			AgentID:   "agent-tasks",
			Input:     "test2",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "task-completed",
			AgentID:   "agent-tasks",
			Input:     "test3",
			Status:    "completed",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	
	for _, task := range tasks {
		storage.Tasks.Create(ctx, task)
	}
	
	// Cancel all pending
	err := storage.CancelAllPendingTasks(ctx)
	if err != nil {
		t.Fatalf("CancelAllPendingTasks failed: %v", err)
	}
	
	// Verify pending and running are cancelled, completed is unchanged
	pendingTask, _ := storage.Tasks.Get(ctx, "task-pending")
	if pendingTask.Status != "cancelled" {
		t.Errorf("Expected pending task to be cancelled, got %s", pendingTask.Status)
	}
	
	runningTask, _ := storage.Tasks.Get(ctx, "task-running")
	if runningTask.Status != "cancelled" {
		t.Errorf("Expected running task to be cancelled, got %s", runningTask.Status)
	}
	
	completedTask, _ := storage.Tasks.Get(ctx, "task-completed")
	if completedTask.Status != "completed" {
		t.Errorf("Expected completed task to remain completed, got %s", completedTask.Status)
	}
}

func TestAgentRepository_Update_AllFields(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	repo := NewAgentRepository(db)
	
	// Create initial agent
	agent := &Agent{
		ID:           "agent-update-test",
		Name:         "Original Name",
		Capabilities: []string{"old"},
		Config:       `{"key":"old"}`,
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo.Create(ctx, agent)
	
	// Update all fields
	agent.Name = "Updated Name"
	agent.Capabilities = []string{"new", "updated"}
	agent.Config = `{"key":"new","extra":"data"}`
	agent.Status = "active"
	
	err := repo.Update(ctx, agent)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	
	// Verify updates
	updated, err := repo.Get(ctx, "agent-update-test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	
	if updated.Name != "Updated Name" {
		t.Errorf("Name not updated: got %s", updated.Name)
	}
	if len(updated.Capabilities) != 2 {
		t.Errorf("Capabilities not updated: got %v", updated.Capabilities)
	}
	if !strings.Contains(updated.Config, "new") {
		t.Errorf("Config not updated: got %v", updated.Config)
	}
}

func TestAgentRepository_UpdateStatus_AllStatuses(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	repo := NewAgentRepository(db)
	
	// Create agent
	agent := &Agent{
		ID:           "agent-status-test",
		Name:         "Status Test",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo.Create(ctx, agent)
	
	// Test different status transitions
	statuses := []string{"active", "running", "idle", "error", "inactive"}
	
	for _, status := range statuses {
		err := repo.UpdateStatus(ctx, "agent-status-test", status)
		if err != nil {
			t.Fatalf("UpdateStatus to %s failed: %v", status, err)
		}
		
		updated, _ := repo.Get(ctx, "agent-status-test")
		if updated.Status != status {
			t.Errorf("Expected status %s, got %s", status, updated.Status)
		}
	}
}

func TestMessageRepository_Delete_Success(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	msgRepo := NewMessageRepository(db)
	agentRepo := NewAgentRepository(db)
	taskRepo := NewTaskRepository(db)
	
	// Create dependencies
	agent := &Agent{
		ID:           "agent-delete-msg",
		Name:         "Test Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	task := &Task{
		ID:        "task-delete-msg",
		AgentID:   "agent-delete-msg",
		Input:     "test",
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	taskRepo.Create(ctx, task)
	
	// Create message
	msg := &Message{
		ID:        "msg-to-delete",
		TaskID:    "task-delete-msg",
		AgentID:   "agent-delete-msg",
		Type:      "user",
		Content:   "test message",
		CreatedAt: time.Now(),
	}
	msgRepo.Create(ctx, msg)
	
	// Delete message
	err := msgRepo.Delete(ctx, "msg-to-delete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	
	// Verify deletion
	_, err = msgRepo.Get(ctx, "msg-to-delete")
	if err == nil {
		t.Error("Expected error when getting deleted message")
	}
}

func TestNewAgentDB_ErrorHandling(t *testing.T) {
		_, err := NewAgentDB("/invalid/path/that/cannot/exist/test.db")
	if err == nil {
		t.Error("Expected error with invalid path")
	}
}

func TestStorage_PerformHealthCheck_Empty(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	storage := NewStorage(db, 1*time.Hour)
	
	// Health check on empty database should succeed
	err := storage.PerformHealthCheck(ctx)
	if err != nil {
		t.Errorf("PerformHealthCheck on empty DB failed: %v", err)
	}
}

func TestAgentRepository_UpdateStatus_NonExistent(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	repo := NewAgentRepository(db)
	
	// Try to update status of non-existent agent
	err := repo.UpdateStatus(ctx, "nonexistent", "active")
	// Should not error (UPDATE with no matches is not an error in SQL)
	if err != nil {
		t.Logf("UpdateStatus on non-existent agent: %v", err)
	}
}

func TestMessageRepository_DeleteByTask_Multiple(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	msgRepo := NewMessageRepository(db)
	agentRepo := NewAgentRepository(db)
	taskRepo := NewTaskRepository(db)
	
	// Create dependencies
	agent := &Agent{
		ID:           "agent-multi-msg",
		Name:         "Multi Message Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	task := &Task{
		ID:        "task-multi-msg",
		AgentID:   "agent-multi-msg",
		Input:     "test",
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	taskRepo.Create(ctx, task)
	
	// Create multiple messages
	for i := 0; i < 5; i++ {
		msg := &Message{
			ID:        fmt.Sprintf("msg-%d", i),
			TaskID:    "task-multi-msg",
			AgentID:   "agent-multi-msg",
			Type:      "user",
			Content:   fmt.Sprintf("message %d", i),
			CreatedAt: time.Now(),
		}
		msgRepo.Create(ctx, msg)
	}
	
	// Delete all messages for task
	err := msgRepo.DeleteByTask(ctx, "task-multi-msg")
	if err != nil {
		t.Fatalf("DeleteByTask failed: %v", err)
	}
	
	// Verify all deleted
	for i := 0; i < 5; i++ {
		_, err := msgRepo.Get(ctx, fmt.Sprintf("msg-%d", i))
		if err == nil {
			t.Errorf("Message msg-%d should have been deleted", i)
		}
	}
}

func TestStorage_CleanupOldData_NoOldData(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	storage := NewStorage(db, 24*time.Hour)
	
	// Run cleanup on empty/new database
	err := storage.CleanupOldData(ctx)
	if err != nil {
		t.Errorf("CleanupOldData with no old data failed: %v", err)
	}
}

func TestTaskRepository_Update_AllFields(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	taskRepo := NewTaskRepository(db)
	agentRepo := NewAgentRepository(db)
	
	// Create agent
	agent := &Agent{
		ID:           "agent-task-update",
		Name:         "Task Update Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	// Create task
	task := &Task{
		ID:        "task-update",
		AgentID:   "agent-task-update",
		Input:     "original input",
		Output:    "",
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	taskRepo.Create(ctx, task)
	
	// Update all fields
	task.Input = "updated input"
	task.Output = "some output"
	task.Status = "completed"
	
	err := taskRepo.Update(ctx, task)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	
	// Verify updates
	updated, err := taskRepo.Get(ctx, "task-update")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	
	if updated.Input != "updated input" {
		t.Errorf("Input not updated")
	}
	if updated.Output != "some output" {
		t.Errorf("Output not updated")
	}
	if updated.Status != "completed" {
		t.Errorf("Status not updated")
	}
}

func TestAgentRepository_List_WithStatus(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	repo := NewAgentRepository(db)
	
	// Create agents with different statuses
	statuses := []string{"active", "idle", "running", "error"}
	for i, status := range statuses {
		agent := &Agent{
			ID:           fmt.Sprintf("agent-list-%d", i),
			Name:         fmt.Sprintf("Agent %d", i),
			Capabilities: []string{"test"},
			Status:       status,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		repo.Create(ctx, agent)
	}
	
	// List all agents
	agents, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	
	if len(agents) < 4 {
		t.Errorf("Expected at least 4 agents, got %d", len(agents))
	}
}

func TestTaskRepository_ListByStatus(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	taskRepo := NewTaskRepository(db)
	agentRepo := NewAgentRepository(db)
	
	// Create agent
	agent := &Agent{
		ID:           "agent-status-list",
		Name:         "Status List Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	// Create tasks with different statuses
	statuses := map[string]int{
		"pending":   2,
		"running":   1,
		"completed": 3,
	}
	
	for status, count := range statuses {
		for i := 0; i < count; i++ {
			task := &Task{
				ID:        fmt.Sprintf("task-%s-%d", status, i),
				AgentID:   "agent-status-list",
				Input:     "test",
				Status:    status,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			taskRepo.Create(ctx, task)
		}
	}
	
	// List tasks by status
	pendingTasks, err := taskRepo.ListByStatus(ctx, "pending", 100, 0)
	if err != nil {
		t.Fatalf("ListByStatus failed: %v", err)
	}
	if len(pendingTasks) != 2 {
		t.Errorf("Expected 2 pending tasks, got %d", len(pendingTasks))
	}
	
	completedTasks, err := taskRepo.ListByStatus(ctx, "completed", 100, 0)
	if err != nil {
		t.Fatalf("ListByStatus failed: %v", err)
	}
	if len(completedTasks) != 3 {
		t.Errorf("Expected 3 completed tasks, got %d", len(completedTasks))
	}
}


func TestAgentRepository_Update_NonExistent(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	repo := NewAgentRepository(db)
	
	// Try to update non-existent agent
	agent := &Agent{
		ID:           "nonexistent",
		Name:         "Ghost Agent",
		Capabilities: []string{"none"},
		Config:       "{}",
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	
	err := repo.Update(ctx, agent)
	// Should not error (UPDATE with no matches is not an SQL error)
	if err != nil {
		t.Logf("Update on non-existent agent: %v", err)
	}
}

func TestAgentRepository_Update_NilCapabilities(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	repo := NewAgentRepository(db)
	
	// Create agent
	agent := &Agent{
		ID:           "agent-nil-cap",
		Name:         "Nil Cap Agent",
		Capabilities: []string{"test"},
		Config:       "{}",
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo.Create(ctx, agent)
	
	// Update with nil capabilities
	agent.Capabilities = nil
	err := repo.Update(ctx, agent)
	if err != nil {
		t.Fatalf("Update with nil capabilities failed: %v", err)
	}
	
	// Verify update
	updated, _ := repo.Get(ctx, "agent-nil-cap")
	if updated.Capabilities == nil {
		// This is fine - nil is stored as empty JSON array
		t.Logf("Capabilities stored as nil/empty")
	}
}

func TestMessageRepository_Delete_NonExistent(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	msgRepo := NewMessageRepository(db)
	
	// Try to delete non-existent message
	err := msgRepo.Delete(ctx, "nonexistent-msg")
	// Should not error (DELETE with no matches is not an SQL error)
	if err != nil {
		t.Logf("Delete on non-existent message: %v", err)
	}
}

func TestStorage_InitializeZeroState_MultipleAgents(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	storage := NewStorage(db, 24*time.Hour)
	
	// Create multiple agents with different statuses
	statuses := []string{"active", "running", "error"}
	for i, status := range statuses {
		agent := &Agent{
			ID:           fmt.Sprintf("agent-zero-%d", i),
			Name:         fmt.Sprintf("Agent %d", i),
			Capabilities: []string{"test"},
			Status:       status,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		storage.Agents.Create(ctx, agent)
	}
	
	// Create multiple tasks with different statuses
	taskStatuses := []string{"pending", "running"}
	for i, status := range taskStatuses {
		task := &Task{
			ID:        fmt.Sprintf("task-zero-%d", i),
			AgentID:   fmt.Sprintf("agent-zero-%d", i),
			Input:     "test",
			Status:    status,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		storage.Tasks.Create(ctx, task)
	}
	
	// Initialize zero state
	err := storage.InitializeZeroState(ctx)
	if err != nil {
		t.Fatalf("InitializeZeroState failed: %v", err)
	}
	
	// Verify all agents are idle
	for i := range statuses {
		agent, _ := storage.Agents.Get(ctx, fmt.Sprintf("agent-zero-%d", i))
		if agent.Status != "idle" {
			t.Errorf("Agent %d should be idle, got %s", i, agent.Status)
		}
	}
	
	// Verify all tasks are cancelled
	for i := range taskStatuses {
		task, _ := storage.Tasks.Get(ctx, fmt.Sprintf("task-zero-%d", i))
		if task.Status != "cancelled" {
			t.Errorf("Task %d should be cancelled, got %s", i, task.Status)
		}
	}
}

func TestStorage_PerformHealthCheck_WithErrors(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	storage := NewStorage(db, 24*time.Hour)
	
	// Add some data to verify DB is working
	agent := &Agent{
		ID:           "health-agent",
		Name:         "Health Check Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	storage.Agents.Create(ctx, agent)
	
	// Perform health check
	err := storage.PerformHealthCheck(ctx)
	if err != nil {
		t.Errorf("PerformHealthCheck failed: %v", err)
	}
	
	// Close DB and try health check - should fail
	storage.Close()
	
	err = storage.PerformHealthCheck(ctx)
	if err == nil {
		t.Error("Expected PerformHealthCheck to fail on closed DB")
	}
}

func TestStorage_CleanupOldData_EmptyTables(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	storage := NewStorage(db, 1*time.Hour)
	
	// Run cleanup on empty database
	err := storage.CleanupOldData(ctx)
	if err != nil {
		t.Errorf("CleanupOldData on empty DB failed: %v", err)
	}
}

func TestStorage_CleanupOldData_MessagesAndTools(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	storage := NewStorage(db, 24*time.Hour)
	
	// Create agent
	agent := &Agent{
		ID:           "cleanup-agent",
		Name:         "Cleanup Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	storage.Agents.Create(ctx, agent)
	
	// Create task
	task := &Task{
		ID:        "cleanup-task",
		AgentID:   "cleanup-agent",
		Input:     "test",
		Status:    "completed",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	storage.Tasks.Create(ctx, task)
	
	// Create message
	msg := &Message{
		ID:        "cleanup-msg",
		TaskID:    "cleanup-task",
		AgentID:   "cleanup-agent",
		Type:      "user",
		Content:   "test message",
		CreatedAt: time.Now(),
	}
	storage.Messages.Create(ctx, msg)
	
	// Create tool execution
	tool := &ToolExecution{
		ID:       "cleanup-tool",
		TaskID:   "cleanup-task",
		AgentID:  "cleanup-agent",
		ToolName: "test-tool",
		Status:   "completed",
	}
	storage.ToolExecutions.Create(ctx, tool)
	
	// Run cleanup
	err := storage.CleanupOldData(ctx)
	if err != nil {
		t.Errorf("CleanupOldData failed: %v", err)
	}
}

func TestDatabase_CreateTables_Idempotent(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_tables_idempotent.db")
	
	// Create database twice
	adb1, err := NewAgentDB(dbPath)
	if err != nil {
		t.Fatalf("First NewAgentDB failed: %v", err)
	}
	adb1.Close()
	
	// Reopen - tables should already exist
	adb2, err := NewAgentDB(dbPath)
	if err != nil {
		t.Fatalf("Second NewAgentDB failed: %v", err)
	}
	defer adb2.Close()
	
	// Verify tables exist
	var count int
	err = adb2.GetDB().QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count tables: %v", err)
	}
	if count == 0 {
		t.Error("No tables found after second init")
	}
}

func TestDatabase_Close_ErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_close_error.db")
	
	adb, err := NewAgentDB(dbPath)
	if err != nil {
		t.Fatalf("NewAgentDB failed: %v", err)
	}
	
	// Close successfully
	err = adb.Close()
	if err != nil {
		t.Errorf("First close failed: %v", err)
	}
	
	// Close again - should not panic
	err = adb.Close()
	if err != nil {
		t.Logf("Second close returned error (may be expected): %v", err)
	}
}

func TestTaskRepository_ListByAgent_Pagination(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	taskRepo := NewTaskRepository(db)
	agentRepo := NewAgentRepository(db)
	
	// Create agent
	agent := &Agent{
		ID:           "agent-pagination",
		Name:         "Pagination Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	// Create 10 tasks
	for i := 0; i < 10; i++ {
		task := &Task{
			ID:        fmt.Sprintf("task-page-%d", i),
			AgentID:   "agent-pagination",
			Input:     fmt.Sprintf("input %d", i),
			Status:    "pending",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		taskRepo.Create(ctx, task)
	}
	
	// Test pagination
	page1, err := taskRepo.ListByAgent(ctx, "agent-pagination", 5, 0)
	if err != nil {
		t.Fatalf("ListByAgent page 1 failed: %v", err)
	}
	if len(page1) != 5 {
		t.Errorf("Expected 5 tasks in page 1, got %d", len(page1))
	}
	
	page2, err := taskRepo.ListByAgent(ctx, "agent-pagination", 5, 5)
	if err != nil {
		t.Fatalf("ListByAgent page 2 failed: %v", err)
	}
	if len(page2) != 5 {
		t.Errorf("Expected 5 tasks in page 2, got %d", len(page2))
	}
}

func TestTeamRepository_Create_WithMetadata(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	teamRepo := NewTeamRepository(db)
	
	// Create team with metadata
	team := &Team{
		ID:          "team-metadata",
		Name:        "Metadata Team",
		Description: "Team with metadata",
		Config:      `{"setting":"value"}`,
		Metadata:    map[string]interface{}{"key": "value", "count": 42},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	err := teamRepo.Create(ctx, team)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	
	// Verify creation
	retrieved, err := teamRepo.Get(ctx, "team-metadata")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	
	if retrieved.Name != "Metadata Team" {
		t.Errorf("Name mismatch: got %s", retrieved.Name)
	}
}

func TestTeamRepository_Update_AllFields(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	teamRepo := NewTeamRepository(db)
	
	// Create team
	team := &Team{
		ID:          "team-update-all",
		Name:        "Original Name",
		Description: "Original Description",
		Config:      `{"old":"value"}`,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	teamRepo.Create(ctx, team)
	
	// Update all fields
	team.Name = "Updated Name"
	team.Description = "Updated Description"
	team.Config = `{"new":"value"}`
	team.Metadata = map[string]interface{}{"updated": true}
	
	err := teamRepo.Update(ctx, team)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	
	// Verify updates
	updated, err := teamRepo.Get(ctx, "team-update-all")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	
	if updated.Name != "Updated Name" {
		t.Errorf("Name not updated")
	}
	if updated.Description != "Updated Description" {
		t.Errorf("Description not updated")
	}
}

func TestTeamRepository_Delete_Verified(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	teamRepo := NewTeamRepository(db)
	
	// Create team
	team := &Team{
		ID:          "team-delete",
		Name:        "Delete Me",
		Description: "To be deleted",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	teamRepo.Create(ctx, team)
	
	// Delete team
	err := teamRepo.Delete(ctx, "team-delete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	
	// Verify deletion
	_, err = teamRepo.Get(ctx, "team-delete")
	if err == nil {
		t.Error("Expected error when getting deleted team")
	}
}

func TestTeamRepository_List_Multiple(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	teamRepo := NewTeamRepository(db)
	
	// Create multiple teams
	for i := 0; i < 5; i++ {
		team := &Team{
			ID:          fmt.Sprintf("team-list-%d", i),
			Name:        fmt.Sprintf("Team %d", i),
			Description: fmt.Sprintf("Description %d", i),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		teamRepo.Create(ctx, team)
	}
	
	// List all teams
	teams, err := teamRepo.List(ctx, 100, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	
	if len(teams) < 5 {
		t.Errorf("Expected at least 5 teams, got %d", len(teams))
	}
}

func TestToolExecutionRepository_Create_Complete(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	toolRepo := NewToolExecutionRepository(db)
	agentRepo := NewAgentRepository(db)
	taskRepo := NewTaskRepository(db)
	
	// Create dependencies
	agent := &Agent{
		ID:           "agent-tool-exec",
		Name:         "Tool Exec Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	task := &Task{
		ID:        "task-tool-exec",
		AgentID:   "agent-tool-exec",
		Input:     "test",
		Status:    "running",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	taskRepo.Create(ctx, task)
	
	// Create tool execution with all fields
	toolExec := &ToolExecution{
		ID:       "tool-complete",
		TaskID:   "task-tool-exec",
		AgentID:  "agent-tool-exec",
		ToolName: "test-tool",
		ToolType: "function",
		Input:    `{"param":"value"}`,
		Output:   `{"result":"success"}`,
		Error:    "",
		Status:   "completed",
		Duration: 150,
	}
	
	err := toolRepo.Create(ctx, toolExec)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	
	// Verify creation
	retrieved, err := toolRepo.Get(ctx, "tool-complete")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	
	if retrieved.ToolName != "test-tool" {
		t.Errorf("ToolName mismatch")
	}
	if retrieved.Duration != 150 {
		t.Errorf("Duration mismatch")
	}
}

func TestToolExecutionRepository_Update_Complete(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	toolRepo := NewToolExecutionRepository(db)
	agentRepo := NewAgentRepository(db)
	taskRepo := NewTaskRepository(db)
	
	// Create dependencies
	agent := &Agent{
		ID:           "agent-tool-update",
		Name:         "Tool Update Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	task := &Task{
		ID:        "task-tool-update",
		AgentID:   "agent-tool-update",
		Input:     "test",
		Status:    "running",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	taskRepo.Create(ctx, task)
	
	// Create tool execution
	toolExec := &ToolExecution{
		ID:       "tool-update",
		TaskID:   "task-tool-update",
		AgentID:  "agent-tool-update",
		ToolName: "test-tool",
		Status:   "running",
	}
	toolRepo.Create(ctx, toolExec)
	
	// Update tool execution
	toolExec.Status = "completed"
	toolExec.Output = `{"result":"done"}`
	toolExec.Duration = 200
	
	err := toolRepo.Update(ctx, toolExec)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	
	// Verify update
	updated, err := toolRepo.Get(ctx, "tool-update")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	
	if updated.Status != "completed" {
		t.Errorf("Status not updated")
	}
	if updated.Duration != 200 {
		t.Errorf("Duration not updated")
	}
}

func TestToolExecutionRepository_MarkFailed(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	toolRepo := NewToolExecutionRepository(db)
	agentRepo := NewAgentRepository(db)
	taskRepo := NewTaskRepository(db)
	
	// Create dependencies
	agent := &Agent{
		ID:           "agent-tool-fail",
		Name:         "Tool Fail Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	task := &Task{
		ID:        "task-tool-fail",
		AgentID:   "agent-tool-fail",
		Input:     "test",
		Status:    "running",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	taskRepo.Create(ctx, task)
	
	// Create tool execution
	toolExec := &ToolExecution{
		ID:       "tool-fail",
		TaskID:   "task-tool-fail",
		AgentID:  "agent-tool-fail",
		ToolName: "failing-tool",
		Status:   "running",
	}
	toolRepo.Create(ctx, toolExec)
	
	// Mark as failed
	err := toolRepo.MarkFailed(ctx, "tool-fail", "Something went wrong", 100)
	if err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}
	
	// Verify failure status
	failed, err := toolRepo.Get(ctx, "tool-fail")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	
	if failed.Status != "failed" {
		t.Errorf("Status should be failed, got %s", failed.Status)
	}
	if failed.Error != "Something went wrong" {
		t.Errorf("Error message not set correctly")
	}
}

func TestToolExecutionRepository_ListByAgent(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	toolRepo := NewToolExecutionRepository(db)
	agentRepo := NewAgentRepository(db)
	taskRepo := NewTaskRepository(db)
	
	// Create agent
	agent := &Agent{
		ID:           "agent-tool-list",
		Name:         "Tool List Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	task := &Task{
		ID:        "task-tool-list",
		AgentID:   "agent-tool-list",
		Input:     "test",
		Status:    "running",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	taskRepo.Create(ctx, task)
	
	// Create multiple tool executions
	for i := 0; i < 3; i++ {
		toolExec := &ToolExecution{
			ID:       fmt.Sprintf("tool-list-%d", i),
			TaskID:   "task-tool-list",
			AgentID:  "agent-tool-list",
			ToolName: fmt.Sprintf("tool-%d", i),
			Status:   "completed",
		}
		toolRepo.Create(ctx, toolExec)
	}
	
	// List by agent
	executions, err := toolRepo.ListByAgent(ctx, "agent-tool-list", 100, 0)
	if err != nil {
		t.Fatalf("ListByAgent failed: %v", err)
	}
	
	if len(executions) < 3 {
		t.Errorf("Expected at least 3 executions, got %d", len(executions))
	}
}

func TestToolExecutionRepository_ListByTool(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	ctx := context.Background()
	toolRepo := NewToolExecutionRepository(db)
	agentRepo := NewAgentRepository(db)
	taskRepo := NewTaskRepository(db)
	
	// Create agent
	agent := &Agent{
		ID:           "agent-tool-name",
		Name:         "Tool Name Agent",
		Capabilities: []string{"test"},
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	agentRepo.Create(ctx, agent)
	
	task := &Task{
		ID:        "task-tool-name",
		AgentID:   "agent-tool-name",
		Input:     "test",
		Status:    "running",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	taskRepo.Create(ctx, task)
	
	// Create executions for specific tool
	for i := 0; i < 3; i++ {
		toolExec := &ToolExecution{
			ID:       fmt.Sprintf("tool-specific-%d", i),
			TaskID:   "task-tool-name",
			AgentID:  "agent-tool-name",
			ToolName: "specific-tool",
			Status:   "completed",
		}
		toolRepo.Create(ctx, toolExec)
	}
	
	// List by tool name
	executions, err := toolRepo.ListByTool(ctx, "specific-tool", 100, 0)
	if err != nil {
		t.Fatalf("ListByTool failed: %v", err)
	}
	
	if len(executions) < 3 {
		t.Errorf("Expected at least 3 executions for specific-tool, got %d", len(executions))
	}
	
	// Verify all are for the right tool
	for _, exec := range executions {
		if exec.ToolName != "specific-tool" {
			t.Errorf("Expected tool name 'specific-tool', got %s", exec.ToolName)
		}
	}
}

func TestNewAgentDB_Success(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	adb, err := NewAgentDB(dbPath)
	if err != nil {
		t.Fatalf("NewAgentDB failed: %v", err)
	}
	defer adb.Close()

	// Verify all tables exist
	tables := []string{"agents", "tasks", "messages", "teams", "team_members", "tool_executions", "schema_migrations", "agent_stats"}
	db := adb.GetDB()
	for _, table := range tables {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("Failed to query table %s: %v", table, err)
		}
	}
}

func TestTaskRepository_Delete_ContextCancel(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewTaskRepository(db)

	err := repo.Delete(ctx, "nonexistent")
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected context canceled error, got %v", err)
	}
}

func TestTaskRepository_UpdateStatus_ContextCancel(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewTaskRepository(db)

	err := repo.UpdateStatus(ctx, "nonexistent", "idle")
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected context canceled error, got %v", err)
	}
}

func TestTeamRepository_RemoveMember_ContextCancel(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewTeamRepository(db)

	err := repo.RemoveMember(ctx, "nonexistent-team", "nonexistent-agent")
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected context canceled error, got %v", err)
	}
}

func TestTeamRepository_UpdateMemberRole_ContextCancel(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewTeamRepository(db)

	err := repo.UpdateMemberRole(ctx, "nonexistent-team", "nonexistent-agent", "admin")
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected context canceled error, got %v", err)
	}
}

func TestToolExecutionRepository_MarkCompleted_ContextCancel(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewToolExecutionRepository(db)

	err := repo.MarkCompleted(ctx, "nonexistent", "output", "completed", 100)
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected context canceled error, got %v", err)
	}
}

func TestToolExecutionRepository_MarkFailed_ContextCancel(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewToolExecutionRepository(db)

	err := repo.MarkFailed(ctx, "nonexistent", "error msg", 100)
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected context canceled error, got %v", err)
	}
}

func TestStorage_InitializeZeroState_ContextCancel(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	adb, err := NewAgentDB(dbPath)
	if err != nil {
		t.Fatalf("NewAgentDB failed: %v", err)
	}
	defer adb.Close()

	s := NewStorage(adb.GetDB(), time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = s.InitializeZeroState(ctx)
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected context canceled error, got %v", err)
	}
}

// Phase 3: Additional coverage for low-coverage functions

func TestTeamRepository_GetAgentTeams_MultipleTeamsWithMetadata(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	s := NewStorage(db, time.Hour)
	ctx := context.Background()
	
	// Create agent
	agent := s.NewAgentWithDefaults("test-agent", "test", nil)
	if err := s.Agents.Create(ctx, agent); err != nil {
		t.Fatalf("Create agent: %v", err)
	}
	
	// Create teams with metadata
	team1 := s.NewTeamWithDefaults("team1", "Team 1")
	team1.Metadata = map[string]interface{}{"priority": "high", "type": "dev"}
	if err := s.Teams.Create(ctx, team1); err != nil {
		t.Fatalf("Create team1: %v", err)
	}
	
	team2 := s.NewTeamWithDefaults("team2", "Team 2")
	team2.Metadata = map[string]interface{}{"priority": "low", "type": "ops"}
	if err := s.Teams.Create(ctx, team2); err != nil {
		t.Fatalf("Create team2: %v", err)
	}
	
	// Add agent to teams with different roles
	if err := s.Teams.AddMember(ctx, team1.ID, agent.ID, "member"); err != nil {
		t.Fatalf("Add to team1: %v", err)
	}
	if err := s.Teams.AddMember(ctx, team2.ID, agent.ID, "leader"); err != nil {
		t.Fatalf("Add to team2: %v", err)
	}
	
	// Get agent's teams - this covers the query path and metadata deserialization
	teams, err := s.Teams.GetAgentTeams(ctx, agent.ID)
	if err != nil {
		t.Fatalf("GetAgentTeams: %v", err)
	}
	
	if len(teams) != 2 {
		t.Fatalf("Expected 2 teams, got %d", len(teams))
	}
	
	// Verify metadata deserialization
	for _, team := range teams {
		if team.Metadata == nil {
			t.Error("Expected metadata to be deserialized")
		}
		if team.ID == team1.ID {
			if team.Metadata["priority"] != "high" {
				t.Errorf("Expected priority=high for team1, got %v", team.Metadata["priority"])
			}
		}
	}
}

func TestTeamRepository_GetAgentTeams_NoMemberships(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	s := NewStorage(db, time.Hour)
	ctx := context.Background()
	
	// Create agent but don't add to any teams
	agent := s.NewAgentWithDefaults("solo-agent", "test", nil)
	if err := s.Agents.Create(ctx, agent); err != nil {
		t.Fatalf("Create agent: %v", err)
	}
	
	// Should return empty list, not error
	teams, err := s.Teams.GetAgentTeams(ctx, agent.ID)
	if err != nil {
		t.Fatalf("GetAgentTeams should not error on no teams: %v", err)
	}
	
	if len(teams) != 0 {
		t.Errorf("Expected 0 teams for agent with no memberships, got %d", len(teams))
	}
}

func TestStorage_CleanupOldData_ContextCancel(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	
	s := NewStorage(db, time.Hour)
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	err := s.CleanupOldData(ctx)
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected context canceled error, got %v", err)
	}
}

func TestStorage_Close_NilDB(t *testing.T) {
	s := &Storage{db: nil}
	err := s.Close()
	if err != nil {
		t.Errorf("Close with nil db should not error, got %v", err)
	}
}
