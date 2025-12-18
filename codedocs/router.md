# Router Package Documentation

## Package Overview

**Package**: `sdk/router`  
**Purpose**: In-memory message routing for inter-agent communication  
**Stability**: Alpha  
**Test Coverage**: 0%

---

## Core Types

### Router

```go
type Router struct {
    mu      sync.RWMutex
    routes  map[string][]Handler
    default Handler
}
```

**Key Methods**:
- `Register(method string, handler Handler)`
- `Send(ctx context.Context, msg Message) error`
- `Default(handler Handler)`

**Handler**:
```go
type Handler func(ctx context.Context, msg Message) error
```

**Message**:
```go
type Message struct {
    Method  string
    Target  string
    Payload interface{}
    Reply   chan interface{}
}
```

---

## Usage

```go
router := router.New()

router.Register(\"ping\", func(ctx context.Context, msg Message) error {
    msg.Reply <- \"pong\"
    return nil
})

msg := router.Message{
    Method: \"ping\",
}
err := router.Send(ctx, msg)
reply := <-msg.Reply
```

---

## Limitations

- In-memory only (no persistence)
- No timeouts on replies
- No dead letter queue

---

**Last Updated**: December 18, 2025