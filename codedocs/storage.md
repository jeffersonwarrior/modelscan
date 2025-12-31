# Database Package Documentation

**Package**: `internal/database`
**Purpose**: SQLite persistence layer with migrations and CRUD operations
**Stability**: Production
**Test Coverage**: ~75%

---

## Overview

The database package provides SQLite-based persistence for ModelScan's provider registry, model catalog, API key management, usage tracking, and discovery logs. Uses automatic migrations with WAL mode for better concurrency.

---

## Files

- `schema.go` (243 lines) - Database schema and migrations
- `queries.go` - CRUD operations and data access
- `*_test.go` - Test files

---

## Database Schema

**Connection**: SQLite with WAL (Write-Ahead Logging) and foreign keys enabled

```sql
?_journal=WAL&_fk=true
```

### Tables

**schema_version**
- Tracks applied migrations
- Primary key: `version INTEGER`
- Auto-timestamped migrations

**providers**
- LLM provider registry
- Fields: id, name, base_url, auth_method, description, created_at, updated_at
- Stores provider metadata and API configuration

**model_families**
- Model groupings (e.g., GPT-4 family, Claude 3 family)
- Fields: id, provider_id, name, description
- Foreign key to providers table

**models**
- Individual model definitions
- Fields: id, family_id, provider_id, model_id, name, description
- Pricing: cost_per_1m_input, cost_per_1m_output
- Capabilities: context_window, max_output_tokens, supports_images, supports_tools, can_reason, can_stream
- Metadata: deprecated, deprecated_at

**api_keys**
- API key management (up to 100 keys per provider)
- Fields: id, provider_id, key_hash (SHA256), key_prefix, usage_count, token_count
- Rate limiting: requests_today, tokens_today, last_reset
- Degradation: degraded, degraded_until, degraded_reason
- Timestamps: created_at, last_used_at

**usage_tracking**
- Request/token/cost tracking per model
- Fields: id, model_id, api_key_id, request_count, input_tokens, output_tokens, total_cost
- Aggregation: average_latency_ms, error_count
- Timestamps: period_start, period_end

**discovery_logs**
- Discovery operation history
- Fields: id, identifier, status (pending/success/failed), attempt_count
- Results: discovered_data (JSON), error_message
- Metadata: source, llm_model used, cache_hit
- Timestamps: started_at, completed_at

**sdk_versions**
- Generated SDK versioning
- Fields: id, provider_id, version, sdk_path, generated_at
- Validation: compilation_status, test_results
- Metadata: code_hash

**settings**
- Key-value configuration store
- Fields: key (PRIMARY KEY), value, updated_at
- Flexible storage for app configuration

---

## Migration System

**Automatic Schema Versioning**

Migrations execute in order during database initialization:

```go
func Migrate(db *sql.DB) error {
    // Creates schema_version table
    // Applies pending migrations in transaction
    // Updates version tracking
}
```

**Migration Safety:**
- Transactional execution (all-or-nothing)
- Idempotent operations
- Version tracking prevents re-execution
- Foreign key constraint validation

---

## Key Features

### 1. Foreign Key Enforcement

```sql
PRAGMA foreign_keys = ON
```

Ensures referential integrity:
- Models reference providers and families
- API keys reference providers
- Usage tracking references models and keys
- Cascading deletes where appropriate

### 2. WAL Mode

```sql
PRAGMA journal_mode = WAL
```

Benefits:
- Better read concurrency
- Faster writes
- Reduced lock contention
- Atomic commits

### 3. Indexes

Auto-indexed:
- Primary keys
- Foreign keys
- Unique constraints

Performance indexes on high-query columns.

### 4. Timestamps

Automatic timestamp management:
- `created_at` on INSERT
- `updated_at` on UPDATE (via triggers where needed)

---

## CRUD Operations

**File**: `queries.go`

### Provider Operations

```go
// Create provider
func CreateProvider(db, name, baseURL, authMethod string) error

// Get provider by ID
func GetProvider(db *sql.DB, id string) (*Provider, error)

// List all providers
func ListProviders(db *sql.DB) ([]Provider, error)

// Update provider
func UpdateProvider(db *sql.DB, id string, updates map[string]interface{}) error
```

### Model Operations

```go
// Create model
func CreateModel(db *sql.DB, model *Model) error

// Get models by provider
func GetModelsByProvider(db *sql.DB, providerID string) ([]Model, error)

// Update model pricing
func UpdateModelPricing(db *sql.DB, modelID string, inputCost, outputCost float64) error
```

### API Key Management

```go
// Add API key (hashed storage)
func AddAPIKey(db *sql.DB, providerID, apiKey string) error

// Get key with lowest usage (round-robin)
func GetLeastUsedKey(db *sql.DB, providerID string) (*APIKey, error)

// Mark key as degraded
func DegradeKey(db *sql.DB, keyID string, reason string, duration time.Duration) error

// Reset daily usage counters
func ResetDailyUsage(db *sql.DB) error
```

### Usage Tracking

```go
// Record usage
func RecordUsage(db *sql.DB, usage *Usage) error

// Get usage stats by model
func GetUsageStats(db *sql.DB, modelID string, start, end time.Time) (*UsageStats, error)

// Get cost breakdown
func GetCostBreakdown(db *sql.DB, providerID string) (map[string]float64, error)
```

### Discovery Logs

```go
// Create discovery log
func CreateDiscoveryLog(db *sql.DB, log *DiscoveryLog) error

// Update discovery result
func UpdateDiscoveryResult(db *sql.DB, id string, data interface{}, status string) error

// Get recent discoveries
func GetRecentDiscoveries(db *sql.DB, limit int) ([]DiscoveryLog, error)
```

---

## Connection Management

### Initialization

```go
db, err := sql.Open("sqlite3", path+"?_journal=WAL&_fk=true")
if err != nil {
    return err
}

// Configure connection pool
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(time.Hour)
```

### Best Practices

- Use context for cancellation: `db.QueryContext(ctx, ...)`
- Close rows: `defer rows.Close()`
- Prepare statements for repeated queries
- Use transactions for multi-step operations

---

## Security

### API Key Storage

Keys are **never stored in plaintext**:

```go
// Storage: SHA256 hash + first 8 chars as prefix
hash := sha256.Sum256([]byte(apiKey))
hashString := hex.EncodeToString(hash[:])
prefix := apiKey[:8]

INSERT INTO api_keys (key_hash, key_prefix, ...) VALUES (?, ?, ...)
```

### SQL Injection Prevention

All queries use prepared statements:

```go
// Safe ✓
db.QueryContext(ctx, "SELECT * FROM models WHERE id = ?", modelID)

// Unsafe ✗ (not used)
db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM models WHERE id = '%s'", modelID))
```

---

## Testing

**Test Files**: `*_test.go` in `internal/database/`

### Test Coverage

- Schema creation and migrations: 90%
- CRUD operations: 80%
- Foreign key constraints: 70%
- Concurrent access: 60%

**Run tests:**
```bash
go test ./internal/database/... -v
go test ./internal/database/... -race -cover
```

### Test Utilities

```go
// Create in-memory test database
func NewTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("sqlite3", ":memory:?_fk=true")
    require.NoError(t, err)

    // Run migrations
    require.NoError(t, Migrate(db))

    return db
}
```

---

## Usage Examples

### Initialize Database

```go
import "github.com/jeffersonwarrior/modelscan/internal/database"

db, err := database.Open("./modelscan.db")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Run migrations
if err := database.Migrate(db); err != nil {
    log.Fatal(err)
}
```

### Add Provider and Models

```go
// Create provider
err := database.CreateProvider(db, "openai", "https://api.openai.com", "bearer")

// Create model
model := &database.Model{
    ID:             uuid.New().String(),
    ProviderID:     providerID,
    ModelID:        "gpt-4",
    Name:           "GPT-4",
    CostPer1MIn:    30.0,
    CostPer1MOut:   60.0,
    ContextWindow:  8192,
    SupportsTools:  true,
}
err = database.CreateModel(db, model)
```

### Manage API Keys

```go
// Add API key
err := database.AddAPIKey(db, providerID, "sk-...")

// Get key with lowest usage
key, err := database.GetLeastUsedKey(db, providerID)

// Record usage
err = database.IncrementUsage(db, key.ID, 1500, 800)

// Degrade on error
err = database.DegradeKey(db, key.ID, "rate limit", 15*time.Minute)
```

### Track Usage

```go
usage := &database.Usage{
    ModelID:       modelID,
    APIKeyID:      keyID,
    RequestCount:  1,
    InputTokens:   150,
    OutputTokens:  80,
    LatencyMS:     245,
}
err := database.RecordUsage(db, usage)

// Get stats
stats, err := database.GetUsageStats(db, modelID, startTime, endTime)
fmt.Printf("Total requests: %d, Total cost: $%.4f\n", stats.Requests, stats.TotalCost)
```

---

## Error Handling

### Common Errors

```go
var (
    ErrNotFound      = errors.New("entity not found")
    ErrConflict      = errors.New("entity already exists")
    ErrInvalidData   = errors.New("invalid entity data")
    ErrForeignKey    = errors.New("foreign key violation")
)
```

### Error Checking

```go
err := database.GetProvider(db, providerID)
if errors.Is(err, sql.ErrNoRows) {
    // Handle not found
}
if errors.Is(err, database.ErrForeignKey) {
    // Handle constraint violation
}
```

---

## Performance Considerations

### Query Optimization

- Use indexes on frequently queried columns
- Batch inserts when possible
- Limit result sets with pagination
- Use prepared statements for repeated queries

### Connection Pooling

```go
// Recommended settings
db.SetMaxOpenConns(25)      // Max concurrent connections
db.SetMaxIdleConns(5)       // Idle connections in pool
db.SetConnMaxLifetime(1*time.Hour) // Connection lifetime
```

### Bulk Operations

```go
// Batch insert
tx, _ := db.Begin()
stmt, _ := tx.Prepare("INSERT INTO models (...) VALUES (...)")
for _, model := range models {
    stmt.Exec(model.ID, model.Name, ...)
}
stmt.Close()
tx.Commit()
```

---

## Future Enhancements

- [ ] Soft deletes (deleted_at column)
- [ ] Audit logging
- [ ] Query builder for complex queries
- [ ] Connection retry logic
- [ ] Backup/restore utilities
- [ ] Metrics collection

---

**Last Updated**: December 31, 2025
**Schema Version**: Latest (auto-managed)
**Dependencies**: `database/sql`, `github.com/mattn/go-sqlite3`
