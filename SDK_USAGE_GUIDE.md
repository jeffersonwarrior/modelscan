# SDK Usage Guide: Including the ModelScan SDKs

## Overview

The ModelScan project contains 21 production-ready Go SDKs for LLM providers. You can use them in three ways:

1. **Direct Import** - Import from the monorepo (recommended for internal use)
2. **Go Workspace** - Use Go workspaces for local development
3. **Git Submodules** - Include as a submodule in other projects

---

## Option 1: Direct Import (Recommended)

### Setup

The SDKs are already set up as Go modules under `github.com/jeffersonwarrior/modelscan/sdk/*`.

### In Your Project

```go
import (
    "github.com/jeffersonwarrior/modelscan/sdk/openai"
    "github.com/jeffersonwarrior/modelscan/sdk/anthropic"
    "github.com/jeffersonwarrior/modelscan/sdk/together"
    // ... any other SDK
)
```

### Install Dependencies

```bash
# In your project directory
go get github.com/jeffersonwarrior/modelscan/sdk/openai
go get github.com/jeffersonwarrior/modelscan/sdk/anthropic
go get github.com/jeffersonwarrior/modelscan/sdk/together
```

### Usage Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/jeffersonwarrior/modelscan/sdk/openai"
    "github.com/jeffersonwarrior/modelscan/sdk/together"
)

func main() {
    ctx := context.Background()
    
    // Use OpenAI SDK
    openaiClient := openai.NewClient("your-openai-key")
    resp1, err := openaiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4",
        Messages: []openai.ChatMessage{
            {Role: "user", Content: "Hello!"},
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(resp1.Choices[0].Message.Content)
    
    // Use Together AI SDK
    togetherClient := together.NewClient("your-together-key")
    resp2, err := togetherClient.CreateChatCompletion(ctx, together.ChatCompletionRequest{
        Model: "meta-llama/Llama-3-70b-chat-hf",
        Messages: []together.ChatMessage{
            {Role: "user", Content: "Hello!"},
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(resp2.Choices[0].Message.Content)
}
```

---

## Option 2: Go Workspace (Local Development)

Best for developing against the SDKs locally without publishing.

### Setup Workspace

```bash
# In your projects directory
mkdir my-ai-project
cd my-ai-project

# Initialize workspace
go work init

# Add the modelscan repo
go work use /home/nexora/.local/tools/modelscan

# Create your project
mkdir myapp
cd myapp
go mod init github.com/yourname/myapp
go work use .
```

### Your Project Structure

```
my-ai-project/
├── go.work                          # Workspace file
├── myapp/
│   ├── go.mod
│   ├── main.go
└── (modelscan is at /home/nexora/.local/tools/modelscan)
```

### go.work File

```go
go 1.23

use (
    ./myapp
    /home/nexora/.local/tools/modelscan
)
```

### Usage

```go
// myapp/main.go
package main

import (
    "github.com/jeffersonwarrior/modelscan/sdk/openai"
    "github.com/jeffersonwarrior/modelscan/sdk/groq"
)

func main() {
    client := openai.NewClient("key")
    // ... use client
}
```

**Advantages**:
- Changes to SDKs are immediately available
- No need to commit/push for local testing
- Great for development

---

## Option 3: Git Submodule

Include the entire modelscan repo as a submodule.

### Setup

```bash
# In your project root
git submodule add https://github.com/jeffersonwarrior/modelscan.git vendor/modelscan

# Initialize and update
git submodule init
git submodule update
```

### Using Submodule SDKs

```bash
# In your go.mod, use replace directive
go mod edit -replace github.com/jeffersonwarrior/modelscan=./vendor/modelscan
```

### Your go.mod

```go
module github.com/yourname/yourproject

go 1.23

require (
    github.com/jeffersonwarrior/modelscan/sdk/openai v0.0.0
    github.com/jeffersonwarrior/modelscan/sdk/anthropic v0.0.0
)

replace github.com/jeffersonwarrior/modelscan => ./vendor/modelscan
```

### Clone Project with Submodules

```bash
# Clone your project
git clone https://github.com/yourname/yourproject.git
cd yourproject

# Initialize submodules
git submodule update --init --recursive
```

---

## Option 4: Unified SDK Package

Create a single import point for all SDKs.

### Create sdk.go

I can create a unified package that exports all SDKs:

```go
// sdk/sdk.go
package sdk

import (
    "github.com/jeffersonwarrior/modelscan/sdk/openai"
    "github.com/jeffersonwarrior/modelscan/sdk/anthropic"
    "github.com/jeffersonwarrior/modelscan/sdk/google"
    // ... all 21 SDKs
)

// Re-export all clients
type (
    OpenAIClient = openai.Client
    AnthropicClient = anthropic.Client
    GoogleClient = google.Client
    // ... all 21 clients
)

// Constructor functions
var (
    NewOpenAI = openai.NewClient
    NewAnthropic = anthropic.NewClient
    NewGoogle = google.NewClient
    // ... all 21 constructors
)
```

### Usage

```go
import "github.com/jeffersonwarrior/modelscan/sdk"

func main() {
    openaiClient := sdk.NewOpenAI("key")
    anthropicClient := sdk.NewAnthropic("key")
}
```

---

## Recommended Approach by Use Case

### Internal/Private Use
**Use**: Direct Import (Option 1)
- Simple, clean imports
- Full type safety
- Standard Go practices

### Active SDK Development
**Use**: Go Workspace (Option 2)
- Live changes
- No publish cycle
- Easy testing

### Third-Party Distribution
**Use**: Git Submodule (Option 3) or publish to GitHub
- Self-contained
- Version control
- Easy updates

### Convenience/Unified API
**Use**: Unified Package (Option 4)
- Single import
- Simplified interface
- Less verbose

---

## Publishing to GitHub (If Public)

If you want others to use your SDKs:

### 1. Push to GitHub

```bash
cd /home/nexora/.local/tools/modelscan
git push origin main
```

### 2. Tag Releases

```bash
# Tag the repo
git tag v1.0.0
git push origin v1.0.0

# Tag individual SDKs (optional)
git tag sdk/openai/v1.0.0
git push origin sdk/openai/v1.0.0
```

### 3. Users Install

```bash
go get github.com/jeffersonwarrior/modelscan/sdk/openai@v1.0.0
```

---

## Environment Variable Setup

### For ModelScan CLI

The main app uses NEXORA environment variables:

```bash
export NEXORA_API_KEY_OPENAI="sk-..."
export NEXORA_API_KEY_ANTHROPIC="sk-ant-..."
export NEXORA_API_KEY_GOOGLE="..."
# ... etc for all 21 providers
```

### For SDK Direct Use

SDKs accept API keys directly:

```go
client := openai.NewClient("sk-...") // Direct key
```

Or load from environment in your app:

```go
import "os"

apiKey := os.Getenv("OPENAI_API_KEY")
client := openai.NewClient(apiKey)
```

---

## Complete Example Project

### Project Structure

```
myproject/
├── go.mod
├── go.sum
├── main.go
└── README.md
```

### go.mod

```go
module github.com/yourname/myproject

go 1.23

require (
    github.com/jeffersonwarrior/modelscan/sdk/openai v0.0.0
    github.com/jeffersonwarrior/modelscan/sdk/together v0.0.0
    github.com/jeffersonwarrior/modelscan/sdk/groq v0.0.0
)
```

### main.go

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"
    
    "github.com/jeffersonwarrior/modelscan/sdk/openai"
    "github.com/jeffersonwarrior/modelscan/sdk/together"
    "github.com/jeffersonwarrior/modelscan/sdk/groq"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    // Load API keys from environment
    openaiKey := os.Getenv("OPENAI_API_KEY")
    togetherKey := os.Getenv("TOGETHER_API_KEY")
    groqKey := os.Getenv("GROQ_API_KEY")
    
    // Create clients
    openaiClient := openai.NewClient(openaiKey)
    togetherClient := together.NewClient(togetherKey)
    groqClient := groq.NewClient(groqKey)
    
    // Query all three providers
    prompt := "What is the capital of France?"
    
    // OpenAI
    resp1, err := openaiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4",
        Messages: []openai.ChatMessage{{Role: "user", Content: prompt}},
    })
    if err != nil {
        log.Printf("OpenAI error: %v", err)
    } else {
        fmt.Printf("OpenAI: %s\n", resp1.Choices[0].Message.Content)
    }
    
    // Together AI
    resp2, err := togetherClient.CreateChatCompletion(ctx, together.ChatCompletionRequest{
        Model: "meta-llama/Llama-3-70b-chat-hf",
        Messages: []together.ChatMessage{{Role: "user", Content: prompt}},
    })
    if err != nil {
        log.Printf("Together error: %v", err)
    } else {
        fmt.Printf("Together: %s\n", resp2.Choices[0].Message.Content)
    }
    
    // Groq (fastest)
    resp3, err := groqClient.CreateChatCompletion(ctx, groq.ChatCompletionRequest{
        Model: "llama-3.3-70b-versatile",
        Messages: []groq.ChatMessage{{Role: "user", Content: prompt}},
    })
    if err != nil {
        log.Printf("Groq error: %v", err)
    } else {
        fmt.Printf("Groq: %s (%.2fs)\n", 
            resp3.Choices[0].Message.Content,
            resp3.Usage.TotalTime)
    }
}
```

### Run

```bash
export OPENAI_API_KEY="sk-..."
export TOGETHER_API_KEY="..."
export GROQ_API_KEY="..."

go run main.go
```

---

## Testing Your Integration

```bash
# Verify SDKs are accessible
go list -m github.com/jeffersonwarrior/modelscan/sdk/openai
go list -m github.com/jeffersonwarrior/modelscan/sdk/together

# Build your project
go build

# Run tests
go test ./...
```

---

## Troubleshooting

### "module not found"

```bash
# Ensure the repo is accessible
cd /home/nexora/.local/tools/modelscan
git remote -v

# Or use replace directive in go.mod
replace github.com/jeffersonwarrior/modelscan => /home/nexora/.local/tools/modelscan
```

### "version not found"

```bash
# Use @latest or specific commit
go get github.com/jeffersonwarrior/modelscan/sdk/openai@latest
go get github.com/jeffersonwarrior/modelscan/sdk/openai@commit-hash
```

### Import cycle errors

Each SDK is independent - import only what you need:

```go
// Good
import "github.com/jeffersonwarrior/modelscan/sdk/openai"

// Avoid importing the whole sdk/ directory
```

---

## Next Steps

1. Choose your integration method (I recommend Option 1)
2. Create example project using the SDKs
3. Set up your API keys
4. Start building!

All 21 SDKs are ready to use with zero external dependencies. 🚀
