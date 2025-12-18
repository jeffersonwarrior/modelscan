# Storage Package Documentation

## Package Overview

**Package**: `sdk/storage`  
**Purpose**: SQLite persistence layer with repository pattern  
**Stability**: Beta  
**Test Coverage**: ~40% (needs improvement)

---

## Architecture

The storage package implements a layered repository pattern:

```
┌─────────────────────────────────────────────┐
│          Application Layer                  │
├─────────────────────────────────────────────┤
│         Repository Interfaces               │
├─────────────────────────────────────────────┤
│         SQLite Implementation               │
├─────────────────────────────────────────────┤
│         Database Connection Pool            │
└─────────────────────────────────────────────┘
```

**Supported Entities**:
- `Agent` - AI agent configurations
- `Task` - Work items and execution state
- `Message` - Inter-agent communication
- `Team` - Agent groupings
- `ToolExecution` - Tool invocation tracking

---

## Core Types

### 1. Database

**File**: `storage.go`

Central database manager handling connections, migrations, and repository instantiation.

```go
type Database struct {
    db      *sql.DB
    path    string
    mu      sync.RWMutex
    repos   map[string]interface{}
}
```

**Key Methods**:
- `NewDatabase(dataSourceName string) (*Database, error)` - Create and open database
- `Migrate() error` - Run pending migrations
- `Close() error` - Close database connection
- `Agent() *AgentRepo`
- `Task() *TaskRepo`
- `Message() *MessageRepo`
- `Team() *TeamRepo`
- `ToolExecution() *ToolExecRepo`

**Connection Settings**:
```go
// Current settings (need improvement)
db.SetMaxOpenConns(1)  // Should be higher for production
```

**Migration Process**:
1. Check `schema_migrations` table
2. Find migrations not yet applied
3. Execute in transaction
4. Update migration version

---

### 2. Repository Pattern

Each entity has a repository implementing CRUD operations:

```go
type Repository[T any] interface {
    Create(ctx context.Context, entity T) (string, error)
    Get(ctx context.Context, id string) (T, error)
    Update(ctx context.Context, id string, entity T) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, limit, offset int) ([]T, error)
}
```

**Concrete Implementations**:
- `AgentRepo` - Agent repository
- `TaskRepo` - Task repository
- `MessageRepo` - Message repository
- `TeamRepo` - Team repository
- `ToolExecRepo` - Tool execution repository

---

### 3. Agent Repository

**File**: `agent.go`

Manages agent lifecycle and configuration.

**Schema**:
```sql
CREATE TABLE agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    mode TEXT NOT NULL,
    capabilities TEXT,  -- JSON array
    metadata TEXT,      -- JSON object
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
)
```

**Key Methods**:
- `Create(ctx context.Context, agent *Agent) error`
- `GetByID(ctx context.Context, id string) (*Agent, error)`
- `GetByName(ctx context.Context, name string) (*Agent, error)`
- `List(ctx context.Context, limit, offset int) ([]*Agent, error)`
- `Update(ctx context.Context, agent *Agent) error`
- `Delete(ctx context.Context, id string) error`

**Notes**:
- Name is unique
- Capabilities stored as JSON array
- Metadata is flexible JSON object
- Timestamps auto-managed via triggers

---

### 4. Task Repository

**File**: `task.go`

Handles task creation, status tracking, and execution state.

**Schema**:
```sql
CREATE TABLE tasks (
    id TEXT PRIMARY KEY,
    agent_id TEXT,
    workflow_id TEXT,
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    priority INTEGER DEFAULT 0,
    input TEXT,         -- JSON
    output TEXT,        -- JSON
    error TEXT,
    metadata TEXT,      -- JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id),
    FOREIGN KEY (workflow_id) REFERENCES workflows(id)
)
```

**Statuses**:
- `pending` - Waiting to execute
- `running` - Currently executing
- `completed` - Successfully finished
- `failed` - Error occurred
- `cancelled` - User cancelled

**Key Methods**:
- `Create(ctx context.Context, task *Task) error`
- `Get(ctx context.Context, id string) (*Task, error)`
- `ListByAgent(ctx context.Context, agentID string) ([]*Task, error)`
- `ListByStatus(ctx context.Context, status string) ([]*Task, error)`
- `UpdateStatus(ctx context.Context, id, status string) error`
- `UpdateOutput(ctx context.Context, id string, output interface{}) error`

---

### 5. Message Repository

**File**: `message.go`

Stores inter-agent communications.

**Schema**:
```sql
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    sender_id TEXT NOT NULL,
    receiver_id TEXT,
    content_type TEXT NOT NULL,
    content TEXT NOT NULL,
    metadata TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_conversation (conversation_id),
    INDEX idx_sender (sender_id),
    INDEX idx_receiver (receiver_id)
)
```

**Content Types**:
- `text/plain` - Plain text
- `application/json` - Structured data
- `command/execute` - Tool execution request
- `result/success` - Successful result
- `result/error` - Error result

**Key Methods**:
- `Create(ctx context.Context, msg *Message) error`
- `GetConversation(ctx context.Context, convID string, limit int) ([]*Message, error)`
- `GetUnread(ctx context.Context, receiverID string) ([]*Message, error)`
- `MarkRead(ctx context.Context, msgID string) error`

---

### 6. Team Repository

**File**: `team.go`

Manages agent teams and groupings.

**Schema**:
```sql
CREATE TABLE teams (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    metadata TEXT,      -- JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
)

CREATE TABLE team_members (
    team_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (team_id, agent_id),
    FOREIGN KEY (team_id) REFERENCES teams(id),
    FOREIGN KEY (agent_id) REFERENCES agents(id)
)
```

**Key Methods**:
- `Create(ctx context.Context, team *Team) error`
- `Get(ctx context.Context, id string) (*Team, error)`
- `AddMember(ctx context.Context, teamID, agentID, role string) error`
- `RemoveMember(ctx context.Context, teamID, agentID string) error`
- `ListMembers(ctx context.Context, teamID string) ([]*Agent, error)`

---

### 7. Tool Execution Repository

**File**: `tool_exec.go`

Tracks external tool invocations.

**Schema**:
```sql
CREATE TABLE tool_executions (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    input TEXT,         -- JSON
    output TEXT,        -- JSON
    error TEXT,
    status TEXT NOT NULL,
    started_at DATETIME,
    completed_at DATETIME,
    metadata TEXT,
    FOREIGN KEY (agent_id) REFERENCES agents(id)
)
```

**Statuses**:
- `pending` - Waiting to start
- `running` - Tool is executing
- `success` - Completed successfully
- `failed` - Tool returned error
- `timeout` - Execution timed out

**Key Methods**:
- `Create(ctx context.Context, exec *ToolExecution) error`
- `UpdateStatus(ctx context.Context, id, status string) error`
- `GetByAgent(ctx context.Context, agentID string) ([]*ToolExecution, error)`

---

## Migration System

### Migration Structure

**Files**: `database.go` migrations v1-v3

Migrations are defined as SQL strings and executed in order:

```go
var migrations = []Migration{
    {Version: 1, SQL: migrationV1Schema},
    {Version: 2, SQL: migrationV2AddTeamDescription},
    {Version: 3, SQL: migrationV3ToolExecution},
}
```

**Migration Table**:
```sql
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
)
```

**Migration Process**:
1. Check current version from `schema_migrations`
2. For each migration with version > current
3. Execute in transaction
4. Insert version into `schema_migrations`
5. Commit transaction

**Best Practices**:
- Migrations are irreversible (no down migrations)
- Each migration must be idempotent
- Add columns, never remove them
- Use triggers for timestamp management

---

## Connection Management

### Current Configuration

```go
db, err := sql.Open("sqlite3", dsn+"?_journal=WAL&_fk=true")
```

**Settings Explained**:
- `_journal=WAL` - Write-Ahead Logging for better concurrency
- `_fk=true` - Enable foreign key constraints

**Recommended Improvements**:
```go
// Add these settings
 db.SetMaxOpenConns(25)
 db.SetMaxIdleConns(5)
 db.SetConnMaxLifetime(time.Hour)
```

---

## Error Handling

### Current Patterns

```go
// Generic error returns
return fmt.Errorf("failed to create agent: %v", err)
```

### Recommended Patterns

```go
// Sentinel errors
var (
    ErrNotFound    = errors.New("entity not found")
    ErrConflict    = errors.New("entity already exists")
    ErrInvalidData = errors.New("invalid entity data")
)

// Context-aware errors
return fmt.Errorf("agent repo: create agent %s: %w", agent.ID, err)

// Storage layer wrapping
if errors.Is(err, sql.ErrNoRows) {
    return nil, ErrNotFound
}
```

---

## Testing

### Current Coverage

- Basic CRUD operations: 60%
- Migration testing: 20%
- Concurrent access: 0%
- Error scenarios: 30%

**Overall**: ~40% (Target: 100%)

### Required Test Scenarios

1. **CRUD Operations**
   - Create with valid/invalid data
   - Read non-existent entities
   - Update existing/non-existing
   - Delete with cascade effects

2. **Concurrency**
   - Multiple goroutines reading/writing
   - Transaction isolation
   - Lock contention

3. **Error Cases**
   - Database connection failures
   - Constraint violations
   - Migration failures

4. **Performance**
   - Bulk insert performance
   - Query optimization
   - Index usage

### Test Utilities

```go
// Create in-memory test DB
func newTestDB(t *testing.T) *Database {
    t.Helper()
    db, err := NewDatabase(":memory:")
    require.NoError(t, err)
    return db
}
```

---

## Performance Considerations

### Indexes

**Current Indexes**:
- Primary keys (automatic)
- Foreign keys (automatic)
- Manual indexes on `conversation_id`, `sender_id`, `receiver_id` in messages

**Recommended Additional Indexes**:
```sql
CREATE INDEX idx_tasks_agent_status ON tasks(agent_id, status);
CREATE INDEX idx_tasks_workflow ON tasks(workflow_id);
CREATE INDEX idx_tool_exec_agent ON tool_executions(agent_id, started_at);
CREATE INDEX idx_teams_name ON teams(name);
```

### Query Optimization

**N+1 Problem in Teams**: Loading team members does N+1 queries
```go
// Current: N+1 queries
for _, agentID := range team.AgentIDs {
    agent, _ := agentRepo.Get(agentID) // N queries
}

// Better: JOIN query
SELECT a.* FROM agents a 
JOIN team_members tm ON a.id = tm.agent_id 
WHERE tm.team_id = ?
```

---

## Security Considerations

### SQL Injection Prevention

✓ **Safe**: All queries use prepared statements

```go
// Safe
_, err := db.ExecContext(ctx, "INSERT INTO agents (id, name) VALUES (?, ?)", id, name)

// Unsafe (NOT used in codebase)
_, err := db.ExecContext(ctx, fmt.Sprintf("INSERT INTO agents (id, name) VALUES ('%s', '%s')", id, name))
```

### Foreign Key Constraints

✓ **Enabled**: `_fk=true` in connection string ensures referential integrity

### Data Validation

**Current**: Minimal validation at storage layer
**Recommended**: Validate at repository level
```go
func (r *AgentRepo) Create(ctx context.Context, agent *Agent) error {
    if agent.Name == "" {
        return ErrInvalidData
    }
    if len(agent.Name) > 255 {
        return fmt.Errorf("name too long: %d chars (max 255)", len(agent.Name))
    }
    // ... rest of logic
}
```

---

## Usage Examples

### Basic CRUD

```go
// Initialize
db, err := storage.NewDatabase("~/.modelscan/modelscan.db")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Create agent
agent := &Agent{
    ID: uuid.New().String(),
    Name: "my-agent",
    Mode: "llm",
    Capabilities: []string{"text-generation"},
}
err = db.Agent().Create(context.Background(), agent)

// Read agent
found, err := db.Agent().GetByName(context.Background(), "my-agent")

// Update
agent.Metadata["version"] = "2.0"
err = db.Agent().Update(context.Background(), agent.ID, agent)

// Delete
err = db.Agent().Delete(context.Background(), agent.ID)
```

### Complex Queries

```go
// Get tasks by status
tasks, err := db.Task().ListByStatus(context.Background(), "running")

// Get team members
team, err := db.Team().Get(context.Background(), teamID)
members, err := db.Team().ListMembers(context.Background(), team.ID)

// Get conversation history
messages, err := db.Message().GetConversation(
    context.Background(), 
    conversationID, 
    100, // limit
)
```

### Transactions

```go
// (Not yet implemented - recommendation)
func (db *Database) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
    tx, err := db.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    
    if err := fn(tx); err != nil {
        tx.Rollback()
        return err
    }
    
    return tx.Commit()
}
```

---

## Migration Guide

### Creating a New Migration

1. Add migration constant in `database.go`:
```go
const migrationV4NewFeature = `
-- Add new table
CREATE TABLE new_table (
    id TEXT PRIMARY KEY,
    data TEXT NOT NULL
);
`
```

2. Add to migrations slice:
```go
var migrations = []Migration{
    // ... existing migrations ...
    {Version: 4, SQL: migrationV4NewFeature},
}
```

3. Test migration on existing DB:
```bash
$ go run cmd/modelscan/main.go status
# Should automatically apply migration
```

---

## Troubleshooting

### "Database is locked" errors

**Cause**: Too many concurrent writers
**Solution**: Increase connection pool, add retry logic

```go
// Add retry loop
for i := 0; i < 3; i++ {
    err = repo.Create(ctx, entity)
    if err == nil || !strings.Contains(err.Error(), "database is locked") {
        break
    }
    time.Sleep(10 * time.Millisecond * time.Duration(i+1))
}
```

### Migration failures

**Cause**: DB in inconsistent state
**Solution**: Check `schema_migrations` table, manually fix if needed

```bash
$ sqlite3 modelscan.db
sqlite> SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1;
```

### Foreign key violations

**Cause**: Deleting referenced entities
**Solution**: Check deletion order, use CASCADE carefully

---

## Future Enhancements

1. **Connection Pooling**: Better concurrency support
2. **Transaction Support**: Multi-operation atomicity
3. **Soft Deletes**: Add `deleted_at` column instead of hard delete
4. **Audit Log**: Track all changes to entities
5. **Backup/Restore**: Built-in utilities
6. **Metrics**: Query performance tracking
7. **Query Builder**: Type-safe query construction

---

**Last Updated**: December 18, 2025  
**Package Version**: v0.2.0  
**Maintainer**: ModelScan Team  
**Database Version**: 3 (ToolExecution support)