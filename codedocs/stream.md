# Stream Package Documentation

## Package Overview

**Package**: `sdk/stream`  
**Purpose**: Streaming response handling for LLMs  
**Stability**: Alpha  
**Test Coverage**: 0%

---

## Core Types

### Stream

```go
type Stream interface {
    Next() bool
    Current() Chunk
    Err() error
    Close() error
}
```

**Chunk**:
```go
type Chunk struct {
    Content string
    Done    bool
    Error   error
}
```

---

## Usage

```go
stream, err := provider.GenerateStream(ctx, prompt)
for stream.Next() {
    chunk := stream.Current()
    fmt.Print(chunk.Content)
}
if err := stream.Err(); err != nil {
    log.Error(err)
}
```

---

**Last Updated**: December 18, 2025