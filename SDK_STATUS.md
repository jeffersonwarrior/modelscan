# ModelScan SDK Status & Recommendations

## Current Implementation Status

### Providers with Official SDKs ✅ (2/4)

#### 1. **Anthropic** ✅
- **Current**: Using official SDK
- **Package**: `github.com/anthropics/anthropic-sdk-go` v1.19.0
- **Status**: ✅ Fully integrated, working perfectly
- **Features**: Models list, message creation, proper header management

#### 2. **OpenAI** ✅
- **Current**: Using community SDK
- **Package**: `github.com/sashabaranov/go-openai` v1.41.2
- **Status**: ✅ Fully integrated, working perfectly
- **Features**: ListModels(), CreateChatCompletion(), CreateEmbeddings()
- **Note**: Most popular Go OpenAI client (18k+ stars)

### Providers Using Direct HTTP ⚠️ (2/4)

#### 3. **Google Gemini** ⚠️ → ✅ **SDK Available!**
- **Current**: Direct REST API calls to `/v1beta/models`
- **Problem**: Not using official SDK
- **Solution Available**: Official Google Go SDK exists!
  - **Package**: `google.golang.org/genai` (NEW unified SDK)
  - **Repository**: https://github.com/googleapis/go-genai
  - **Status**: Official Google SDK, 925+ stars, actively maintained
  - **Supports**: Both Gemini Developer API AND Vertex AI
  - **Installation**: `go get google.golang.org/genai`

**Migration Path for Google**:
```go
// OLD (current implementation)
url := "https://generativelanguage.googleapis.com/v1beta/models"
req, _ := http.NewRequest("GET", url, nil)
req.Header.Set("x-goog-api-key", apiKey)

// NEW (using official SDK)
import "google.golang.org/genai"

client, err := genai.NewClient(ctx, &genai.ClientConfig{
    APIKey:   apiKey,
    Backend:  genai.BackendGeminiAPI,
})
defer client.Close()

modelsPage, err := client.Models.List(ctx, &genai.ListModelsConfig{})
```

#### 4. **Mistral** ⚠️ → ✅ **Custom SDK Ready!**
- **Current**: Direct HTTP calls in `providers/mistral.go`
- **Problem**: Not using any SDK
- **Solution Available**: Custom SDK built at `/home/nexora/sdk/mistral/`
  - **Status**: ✅ Implementation complete
  - **Files**: client.go, chat.go, fim.go, agents.go, embeddings.go
  - **Features**: Full Mistral API support
  - **Needs**: Integration into modelscan

**Migration Path for Mistral**:
```go
// OLD (current implementation)
type MistralProvider struct {
    apiKey  string
    baseURL string
    client  *http.Client
}

// NEW (using custom SDK)
import "github.com/nexora/sdk/mistral"

type MistralProvider struct {
    client *mistral.Client
}

func NewMistralProvider(apiKey string) Provider {
    return &MistralProvider{
        client: mistral.NewClient(apiKey),
    }
}
```

## Summary: SDK Status Matrix

| Provider   | Current Implementation | SDK Available | Status | Action Needed |
|-----------|------------------------|---------------|--------|---------------|
| Anthropic | Official SDK ✅        | ✅            | ✅     | None - Perfect |
| OpenAI    | Community SDK ✅       | ✅            | ✅     | None - Perfect |
| Google    | Direct HTTP ⚠️         | ✅ Official   | ⚠️     | **MIGRATE** to official SDK |
| Mistral   | Direct HTTP ⚠️         | ✅ Custom     | ⚠️     | **INTEGRATE** custom SDK |

## Detailed Recommendations

### Priority 1: Migrate Google to Official SDK 🔥

**Why**:
- Official Google SDK now available (`google.golang.org/genai`)
- Replaces deprecated `github.com/google/generative-ai-go` (EOL: Aug 31, 2025)
- Better type safety, error handling, and feature support
- Supports both Gemini Developer API and Vertex AI

**Benefits**:
- ✅ Type-safe API interactions
- ✅ Automatic pagination for model lists
- ✅ Better error messages
- ✅ Streaming support
- ✅ Multimodal capabilities (images, audio, video)
- ✅ Function calling support
- ✅ Official Google maintenance

**Implementation Steps**:
1. Add dependency: `go get google.golang.org/genai`
2. Update `providers/google.go` to use SDK
3. Replace direct HTTP calls with SDK methods
4. Update tests
5. Verify with `./modelscan --provider=google --verbose`

**Code Changes Required**:
- Replace `http.NewRequest` calls with `client.Models.List()`
- Replace manual JSON parsing with SDK types
- Use `genai.BackendGeminiAPI` for Developer API
- Update model listing to handle pagination properly

### Priority 2: Integrate Mistral Custom SDK

**Why**:
- Custom SDK already built and ready at `/home/nexora/sdk/mistral/`
- Provides better abstraction and error handling
- Can be published as open-source package
- Enables future features (Agents API, Fine-tuning)

**Benefits**:
- ✅ Cleaner provider code
- ✅ Reusable across projects
- ✅ Better testing capabilities
- ✅ Support for native Mistral features (FIM, Agents)
- ✅ Can be shared with community

**Implementation Steps**:
1. Review custom SDK at `/home/nexora/sdk/mistral/`
2. Add as Go module dependency or vendor
3. Update `providers/mistral.go` to use SDK
4. Test all endpoints
5. Verify with `./modelscan --provider=mistral --verbose`

**Code Changes Required**:
- Replace manual HTTP client with `mistral.Client`
- Use SDK methods for ListModels, CreateChatCompletion
- Remove duplicate request/response handling code
- Add support for FIM endpoint (codestral models)

## Implementation Timeline

### Week 1: Google SDK Migration
- Day 1-2: Add dependency, update google.go
- Day 3: Update tests, verify functionality
- Day 4: Test all Google models
- Day 5: Documentation update

### Week 2: Mistral SDK Integration
- Day 1-2: Review and prepare custom SDK
- Day 3: Update mistral.go to use SDK
- Day 4: Test all Mistral endpoints
- Day 5: Add FIM support for Codestral

### Week 3: Testing & Documentation
- Full integration testing
- Update AGENTS.md with new SDK info
- Update README examples
- Performance benchmarking

## Expected Outcomes

After completing both migrations:

1. **All 4 providers using SDKs** ✅
   - Anthropic: Official SDK ✅
   - OpenAI: Community SDK ✅
   - Google: Official SDK ✅ (NEW)
   - Mistral: Custom SDK ✅ (NEW)

2. **Better Code Quality**
   - Reduced code duplication
   - Better error handling
   - Type-safe API calls
   - Easier maintenance

3. **New Capabilities**
   - Google: Streaming, multimodal, function calling
   - Mistral: FIM completions, Agents API support
   - Better model metadata
   - Improved pricing accuracy

4. **Future-Proofing**
   - Official Google SDK maintained by Google
   - Custom Mistral SDK can track API changes
   - Easier to add new features
   - Better deprecation handling

## References

### Google Generative AI Go SDK
- **Repository**: https://github.com/googleapis/go-genai
- **Package**: `google.golang.org/genai`
- **Documentation**: https://pkg.go.dev/google.golang.org/genai
- **Python Version**: https://github.com/googleapis/python-genai
- **Installation**: `go get google.golang.org/genai`

### Mistral Custom SDK
- **Location**: `/home/nexora/sdk/mistral/`
- **Status**: Built and ready for integration
- **Documentation**: See `/home/nexora/MISTRAL_IMPLEMENTATION_REVIEW.md`
- **Features**: Chat, FIM, Agents, Embeddings, Models, Files, Fine-tuning

### Current SDK Documentation
- **Anthropic**: https://github.com/anthropics/anthropic-sdk-go
- **OpenAI**: https://github.com/sashabaranov/go-openai

## Next Steps

**Immediate Actions**:
1. ✅ Document current SDK status (this file)
2. 🔲 Review Google genai SDK documentation
3. 🔲 Create migration plan for google.go
4. 🔲 Review custom Mistral SDK implementation
5. 🔲 Decide on Mistral SDK integration approach

**Questions to Answer**:
- Should we vendor the Mistral SDK or publish it as separate module?
- What's the testing strategy for SDK migrations?
- Do we need backward compatibility during migration?
- Should we update both providers simultaneously or sequentially?

---

**Last Updated**: December 17, 2025  
**Author**: Development Team  
**Status**: Planning Phase - Ready for Implementation
