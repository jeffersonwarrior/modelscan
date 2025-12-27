# Contributing to ModelScan

Thank you for your interest in contributing to ModelScan! This document provides guidelines and information for contributors.

## Code of Conduct

- Be respectful and constructive in all interactions
- Focus on technical merits and facts
- Welcome newcomers and help them learn
- Report issues professionally with clear reproduction steps

## Getting Started

### Prerequisites

- Go 1.21 or later
- Git
- Basic understanding of HTTP APIs and testing

### Development Setup

```bash
# Clone the repository
git clone https://github.com/jeffersonwarrior/modelscan.git
cd modelscan

# Build the project
go build ./...

# Run tests
go test ./...

# Run tests with race detector
go test -race ./...

# Check coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

## Project Structure

```
modelscan/
├── providers/          # AI provider implementations
├── internal/http/      # HTTP client foundation (zero dependencies)
├── sdk/               # Multi-agent framework SDKs
├── config/            # Configuration management
├── storage/           # Persistence layer
├── examples/          # Usage examples
├── scripts/           # Build and validation scripts
└── archive/           # Historical documentation
```

## Making Contributions

### 1. Find or Create an Issue

- Check existing issues first
- For bugs: provide reproduction steps, expected vs actual behavior
- For features: explain use case, proposed solution, alternatives considered

### 2. Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-description
```

Use descriptive branch names:
- `feature/` - New functionality
- `fix/` - Bug fixes
- `refactor/` - Code improvements without behavior changes
- `docs/` - Documentation updates

### 3. Write Code

**Follow Project Standards:**

- **Zero external dependencies** - Use only Go stdlib
- **90%+ test coverage** for new providers
- **Race-free code** - All tests must pass `-race`
- **gofmt compliance** - Run `gofmt -w .` before committing
- **go vet clean** - Fix all `go vet` warnings

**Provider Implementation Pattern:**

```go
// 1. Implement the Provider interface
type YourProvider struct {
    apiKey  string
    baseURL string
    client  *http.Client
}

// 2. Register in init()
func init() {
    RegisterProvider("yourprovider", NewYourProvider)
}

// 3. Implement required methods
func (p *YourProvider) ValidateEndpoints(ctx context.Context, verbose bool) error
func (p *YourProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error)
func (p *YourProvider) GetCapabilities() ProviderCapabilities
func (p *YourProvider) GetEndpoints() []Endpoint
func (p *YourProvider) TestModel(ctx context.Context, modelID string, verbose bool) error
```

**Testing Requirements:**

- Comprehensive unit tests using `httptest`
- Test success paths AND error paths
- Test concurrent operations where applicable
- Mock HTTP responses, never call real APIs in tests

Example test structure:
```go
func TestYourProvider_Method(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Mock response
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(mockData)
    }))
    defer server.Close()

    provider := &YourProvider{
        apiKey:  "test-key",
        baseURL: server.URL,
        client:  &http.Client{Timeout: 10 * time.Second},
    }

    // Test assertions
}
```

### 4. Run Validation

Before committing, ensure all validation passes:

```bash
# Build check
go build ./...

# All tests pass
go test ./...

# No race conditions
go test -race ./...

# Code is formatted
gofmt -l . | wc -l  # Should output 0

# No vet warnings
go vet ./...

# Coverage meets threshold (for providers)
go test -coverprofile=c.out ./providers
go tool cover -func=c.out
```

**For New Providers:**

Run the validation script:
```bash
cd providers
bash ../scripts/validate-provider.sh yourprovider 90
```

This enforces the 7-gate validation:
1. Build succeeds
2. All tests pass
3. Coverage ≥ 90.0%
4. Race detector clean
5. go vet clean
6. gofmt clean
7. Provider interface implemented

### 5. Commit Your Changes

Follow conventional commit format:

```
<type>(<scope>): <subject>

<body>

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Your Name <your.email@example.com>
```

**Types:**
- `feat` - New feature
- `fix` - Bug fix
- `refactor` - Code change without behavior change
- `test` - Adding/updating tests
- `docs` - Documentation only
- `chore` - Build/tooling changes

**Examples:**
```
feat(anthropic): Add extended thinking mode support

fix(mistral): Validate JSON responses in endpoint tests

refactor(http): Extract retry logic to separate method

test(openai): Add coverage for rate limiting

docs(readme): Update installation instructions

chore(deps): Update go.mod module path
```

### 6. Push and Create PR

```bash
git push origin your-branch-name
```

Create a pull request on GitHub with:
- Clear title following conventional commits
- Description of what changed and why
- Link to related issues
- Test results showing validation passed
- Screenshots/examples if applicable

## Provider Implementation Guide

### Tier 2 Providers (Core Integrations)

When implementing a new provider, follow this structure:

**File Structure:**
```
providers/
├── yourprovider.go       # Implementation
└── yourprovider_test.go  # Tests (aim for 95%+ coverage)
```

**Implementation Checklist:**

- [ ] Create `yourprovider.go` with Provider interface implementation
- [ ] Add `init()` registration: `RegisterProvider("yourprovider", NewYourProvider)`
- [ ] Implement `ValidateEndpoints` with parallel goroutines + mutex
- [ ] Implement `ListModels` with proper error handling
- [ ] Implement `GetCapabilities` with accurate capability reporting
- [ ] Implement `GetEndpoints` returning all API endpoints
- [ ] Implement `TestModel` for model-specific validation
- [ ] Create comprehensive tests in `yourprovider_test.go`
- [ ] Test error cases (401, 403, 404, 429, 500, invalid JSON, empty responses)
- [ ] Run validation script and achieve 90%+ coverage
- [ ] Update `providers/interface.go` if adding new capabilities
- [ ] Add examples to `examples/` if introducing new patterns

### Common Pitfalls

**❌ Don't:**
- Add external dependencies (breaks zero-dependency goal)
- Commit without running tests
- Ignore race detector warnings
- Use real API keys in tests
- Return nil errors when endpoints fail
- Skip edge case testing

**✅ Do:**
- Use `httptest.NewServer` for all HTTP tests
- Protect concurrent map/slice access with mutex
- Validate JSON responses before accepting them
- Return meaningful errors with context
- Test timeout scenarios
- Handle rate limiting gracefully

## Testing Philosophy

### Unit Tests

- Fast (< 1s per provider test suite)
- Isolated (no external dependencies)
- Deterministic (same input = same output)
- Comprehensive (success + all error paths)

### Integration Tests

- Use `httptest` for HTTP integration tests
- Mock external services completely
- Test realistic scenarios end-to-end
- Verify concurrent operation safety

### Coverage Goals

- **Providers**: 90%+ (enforced by validation script)
- **SDK packages**: 80%+ (good practice)
- **Internal packages**: 85%+ (critical paths)
- **Examples**: Not required (demonstration code)

## Documentation

### Code Comments

- Document exported types, functions, methods
- Explain non-obvious decisions in comments
- Keep comments up-to-date with code changes
- Use examples in godoc comments

### README Updates

- Update feature lists when adding capabilities
- Add new providers to the provider table
- Update installation if process changes
- Keep examples section current

## Release Process

(Maintainers only)

1. Update `CHANGELOG.md` with notable changes
2. Update version in relevant files
3. Run full test suite: `go test -race -coverprofile=coverage.out ./...`
4. Create git tag: `git tag -a v0.x.0 -m "Release v0.x.0"`
5. Push tag: `git push origin v0.x.0`
6. GitHub Actions will handle the release build

## Getting Help

- **Questions**: Open a GitHub Discussion
- **Bugs**: Open an Issue with reproduction steps
- **Security**: Email maintainers privately (see SECURITY.md)
- **Features**: Open an Issue for discussion first

## Recognition

Contributors are recognized in:
- Git commit history (Co-Authored-By tags)
- Release notes
- Project README (for significant contributions)

Thank you for contributing to ModelScan! 🚀
