# ModelScan System Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           MODELSCAN SYSTEM                              │
└─────────────────────────────────────────────────────────────────────────┘

                        ┌──────────────────────┐
                        │   User Interface     │
                        ├──────────────────────┤
                        │  CLI (modelscan)     │
                        │  HTTP API (:8080)    │
                        └──────────┬───────────┘
                                   │
                 ┌─────────────────┴─────────────────┐
                 │                                   │
         ┌───────▼────────┐                 ┌────────▼───────┐
         │   Discovery    │                 │   Routing      │
         │   Pipeline     │                 │   Layer        │
         └───────┬────────┘                 └────────┬───────┘
                 │                                   │
    ┌────────────┼────────────┐                      │
    │            │            │                      │
┌───▼───┐   ┌───▼───┐   ┌───▼───┐         ┌────────┼────────┐
│models │   │Hugging│   │ Model │         │        │        │
│ .dev  │   │ Face  │   │ Scope │         │ Direct │ Plano  │
└───┬───┘   └───┬───┘   └───┬───┘         │ Mode   │ Mode   │
    │           │           │             └────┬───┴────┬───┘
    └───────────┼───────────┘                  │        │
                │                              │        │
         ┌──────▼───────┐                      │   ┌────▼─────┐
         │ LLM Synthesis│                      │   │ Plano    │
         │ (Claude 4.5) │                      │   │ Gateway  │
         └──────┬───────┘                      │   └────┬─────┘
                │                              │        │
         ┌──────▼───────┐                      └────────┼──────┐
         │  Validation  │                               │      │
         │  (HTTP/Auth) │                               │      │
         └──────┬───────┘                      ┌────────▼──────▼─────┐
                │                              │   21 Provider SDKs   │
         ┌──────▼───────┐                      ├──────────────────────┤
         │ SDK Generator│                      │ openai   anthropic   │
         │  (Templates) │                      │ google   mistral     │
         └──────┬───────┘                      │ groq     deepseek    │
                │                              │ together fireworks   │
         ┌──────▼───────┐                      │ replicate cohere     │
         │  sdk/*/      │                      │ perplexity deepinfra │
         │  Generated   │                      │ hyperbolic xai       │
         │  Code        │                      │ minimax  kimi        │
         └──────────────┘                      │ zai      openrouter  │
                                               │ synthetic vibe       │
                ┌──────────────┐               │ nanogpt              │
                │              │               └──────────┬───────────┘
         ┌──────▼──────┐  ┌────▼────┐                    │
         │  Database   │  │  Config │           ┌────────▼───────────┐
         │  (SQLite)   │  │  (.env) │           │  Provider APIs     │
         └─────────────┘  └─────────┘           │  (External)        │
                                                └────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│  DATA FLOW                                                              │
├─────────────────────────────────────────────────────────────────────────┤
│  Discovery:  identifier → scrape sources → LLM → validate → save       │
│  Generate:   result → templates → code → write sdk/provider/           │
│  Runtime:    request → route → SDK → HTTP → provider → response        │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│  KEY PRINCIPLES                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│  • Zero external dependencies (pure Go stdlib)                          │
│  • LLM-powered auto-discovery                                           │
│  • Template-based SDK generation                                        │
│  • Multi-mode routing (direct/plano)                                    │
│  • 21 production-ready SDKs                                             │
└─────────────────────────────────────────────────────────────────────────┘
```

## Components

### User Interface Layer
- **CLI**: Command-line interface for discovery and SDK generation
- **HTTP API**: REST API server on port 8080 for programmatic access

### Discovery Pipeline
1. **Source Scrapers**: Fetch provider info from models.dev, HuggingFace, ModelScope
2. **LLM Synthesis**: Claude 4.5 analyzes and structures provider data
3. **Validation**: HTTP connectivity and auth method verification
4. **SDK Generator**: Template-based code generation

### Routing Layer
- **Direct Mode**: Requests go straight to provider SDKs
- **Plano Mode**: Routes through Plano gateway for advanced features

### Provider SDKs
21 production-ready SDKs, each with:
- Zero external dependencies
- Consistent API interface
- Full OpenAI compatibility layer
- Built-in error handling

### Storage
- **Database**: SQLite for provider metadata and discovery results
- **Config**: Environment-based configuration (.env)
