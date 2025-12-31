package router

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jeffersonwarrior/modelscan/sdk/ratelimit"
	"github.com/jeffersonwarrior/modelscan/storage"
)

// RoutingStrategy determines how to select a provider
type RoutingStrategy string

const (
	StrategyCheapest   RoutingStrategy = "cheapest"    // Minimize cost
	StrategyFastest    RoutingStrategy = "fastest"     // Minimize latency
	StrategyBalanced   RoutingStrategy = "balanced"    // Balance cost and latency
	StrategyRoundRobin RoutingStrategy = "round_robin" // Cycle through providers
	StrategyFallback   RoutingStrategy = "fallback"    // Try primary, fallback on failure
)

// ProviderHealth tracks provider availability and performance
type ProviderHealth struct {
	ProviderName     string
	AvgLatencyMs     int64
	ErrorRate        float64
	LastSuccess      time.Time
	LastFailure      time.Time
	ConsecutiveFails int
	IsHealthy        bool
	mu               sync.RWMutex
}

// ProviderOption represents a provider with its cost and availability
type ProviderOption struct {
	ProviderName  string
	ModelID       string
	PlanType      string
	InputCost     float64
	OutputCost    float64
	EstimatedCost float64
	AvgLatencyMs  int64
	IsAvailable   bool
	RateLimiter   *ratelimit.RateLimiter
	Health        *ProviderHealth
}

// Router selects the best provider based on strategy
type Router struct {
	strategy      RoutingStrategy
	healthTracker map[string]*ProviderHealth
	rrIndex       int // Round-robin index
	mu            sync.RWMutex
}

// RouteRequest contains the routing decision context
type RouteRequest struct {
	Capability       string   // "chat", "embedding", "image", "audio", "video"
	EstimatedTokens  int64    // Input + output token estimate
	MaxCost          float64  // Budget constraint
	MaxLatencyMs     int64    // Latency requirement
	RequiredModels   []string // Specific models to consider
	ExcludeProviders []string // Providers to avoid
}

// RouteResult contains the selected provider
type RouteResult struct {
	Provider      *ProviderOption
	Reason        string
	Alternatives  []*ProviderOption
	EstimatedCost float64
}

// NewRouter creates a new intelligent router
func NewRouter(strategy RoutingStrategy) *Router {
	return &Router{
		strategy:      strategy,
		healthTracker: make(map[string]*ProviderHealth),
	}
}

// Route selects the best provider for the request
func (r *Router) Route(ctx context.Context, req RouteRequest) (*RouteResult, error) {
	// Get all providers that support the capability
	providers, err := r.getAvailableProviders(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get providers: %w", err)
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available for capability: %s", req.Capability)
	}

	// Filter by budget and latency constraints
	filtered := r.filterProviders(providers, req)
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no providers meet constraints (budget: $%.4f, latency: %dms)", req.MaxCost, req.MaxLatencyMs)
	}

	// Select based on strategy
	var selected *ProviderOption
	var reason string

	switch r.strategy {
	case StrategyCheapest:
		selected, reason = r.selectCheapest(filtered)
	case StrategyFastest:
		selected, reason = r.selectFastest(filtered)
	case StrategyBalanced:
		selected, reason = r.selectBalanced(filtered)
	case StrategyRoundRobin:
		selected, reason = r.selectRoundRobin(filtered)
	case StrategyFallback:
		selected, reason = r.selectFallback(filtered)
	default:
		selected, reason = r.selectBalanced(filtered)
	}

	return &RouteResult{
		Provider:      selected,
		Reason:        reason,
		Alternatives:  filtered,
		EstimatedCost: selected.EstimatedCost,
	}, nil
}

// getAvailableProviders fetches providers from database and checks rate limits
func (r *Router) getAvailableProviders(ctx context.Context, req RouteRequest) ([]*ProviderOption, error) {
	// Query database for providers with pricing
	query := `
		SELECT DISTINCT p.provider_name, p.model_id, p.plan_type, 
		       p.input_cost, p.output_cost
		FROM provider_pricing p
		WHERE p.input_cost > 0 OR p.output_cost > 0
	`

	rows, err := storage.GetRateLimitDB().QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []*ProviderOption
	for rows.Next() {
		var opt ProviderOption
		err := rows.Scan(&opt.ProviderName, &opt.ModelID, &opt.PlanType,
			&opt.InputCost, &opt.OutputCost)
		if err != nil {
			continue
		}

		// Calculate estimated cost (assuming 50/50 input/output split)
		inputTokens := req.EstimatedTokens / 2
		outputTokens := req.EstimatedTokens / 2
		opt.EstimatedCost = (float64(inputTokens) * opt.InputCost / 1_000_000) +
			(float64(outputTokens) * opt.OutputCost / 1_000_000)

		// Check if provider is in exclude list
		if r.isExcluded(opt.ProviderName, req.ExcludeProviders) {
			continue
		}

		// Get or create rate limiter
		limiter, err := ratelimit.NewRateLimiter(opt.ProviderName, opt.PlanType)
		if err != nil {
			// No rate limits configured - still available
			opt.IsAvailable = true
		} else {
			opt.RateLimiter = limiter
			// Check if rate limit allows this request
			opt.IsAvailable = r.checkRateLimitAvailability(ctx, limiter, req.EstimatedTokens)
		}

		// Get health status
		opt.Health = r.getHealth(opt.ProviderName)
		if !opt.Health.IsHealthy {
			opt.IsAvailable = false
		}

		providers = append(providers, &opt)
	}

	return providers, nil
}

// filterProviders removes providers that don't meet constraints
func (r *Router) filterProviders(providers []*ProviderOption, req RouteRequest) []*ProviderOption {
	var filtered []*ProviderOption
	for _, p := range providers {
		// Must be available
		if !p.IsAvailable {
			continue
		}

		// Must be within budget
		if req.MaxCost > 0 && p.EstimatedCost > req.MaxCost {
			continue
		}

		// Must meet latency requirement
		if req.MaxLatencyMs > 0 && p.AvgLatencyMs > req.MaxLatencyMs {
			continue
		}

		// Must match required models if specified
		if len(req.RequiredModels) > 0 && !r.matchesModel(p.ModelID, req.RequiredModels) {
			continue
		}

		filtered = append(filtered, p)
	}
	return filtered
}

// selectCheapest picks the lowest cost provider
func (r *Router) selectCheapest(providers []*ProviderOption) (*ProviderOption, string) {
	if len(providers) == 0 {
		return nil, ""
	}

	cheapest := providers[0]
	for _, p := range providers[1:] {
		if p.EstimatedCost < cheapest.EstimatedCost {
			cheapest = p
		}
	}

	return cheapest, fmt.Sprintf("cheapest option at $%.6f", cheapest.EstimatedCost)
}

// selectFastest picks the lowest latency provider
func (r *Router) selectFastest(providers []*ProviderOption) (*ProviderOption, string) {
	if len(providers) == 0 {
		return nil, ""
	}

	fastest := providers[0]
	for _, p := range providers[1:] {
		if p.AvgLatencyMs < fastest.AvgLatencyMs {
			fastest = p
		}
	}

	return fastest, fmt.Sprintf("fastest option at %dms", fastest.AvgLatencyMs)
}

// selectBalanced scores providers based on cost and latency
func (r *Router) selectBalanced(providers []*ProviderOption) (*ProviderOption, string) {
	if len(providers) == 0 {
		return nil, ""
	}

	// Normalize and score (lower is better)
	type scored struct {
		provider *ProviderOption
		score    float64
	}

	var maxCost, maxLatency float64
	for _, p := range providers {
		if p.EstimatedCost > maxCost {
			maxCost = p.EstimatedCost
		}
		if float64(p.AvgLatencyMs) > maxLatency {
			maxLatency = float64(p.AvgLatencyMs)
		}
	}

	if maxCost == 0 {
		maxCost = 1
	}
	if maxLatency == 0 {
		maxLatency = 1
	}

	var scores []scored
	for _, p := range providers {
		// Weighted score: 60% cost, 40% latency
		costScore := p.EstimatedCost / maxCost
		latencyScore := float64(p.AvgLatencyMs) / maxLatency
		totalScore := (0.6 * costScore) + (0.4 * latencyScore)
		scores = append(scores, scored{p, totalScore})
	}

	// Find best score (lowest)
	best := scores[0]
	for _, s := range scores[1:] {
		if s.score < best.score {
			best = s
		}
	}

	return best.provider, fmt.Sprintf("balanced score %.3f (cost: $%.6f, latency: %dms)",
		best.score, best.provider.EstimatedCost, best.provider.AvgLatencyMs)
}

// selectRoundRobin cycles through healthy providers
func (r *Router) selectRoundRobin(providers []*ProviderOption) (*ProviderOption, string) {
	if len(providers) == 0 {
		return nil, ""
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	selected := providers[r.rrIndex%len(providers)]
	r.rrIndex++

	return selected, fmt.Sprintf("round-robin selection #%d", r.rrIndex)
}

// selectFallback tries primary, then fallbacks
func (r *Router) selectFallback(providers []*ProviderOption) (*ProviderOption, string) {
	// First healthy provider
	for i, p := range providers {
		if p.Health.IsHealthy {
			reason := "primary"
			if i > 0 {
				reason = fmt.Sprintf("fallback #%d", i)
			}
			return p, reason
		}
	}
	// All unhealthy - return first anyway
	return providers[0], "all unhealthy, using first"
}

// checkRateLimitAvailability checks if rate limiter would allow the request
func (r *Router) checkRateLimitAvailability(ctx context.Context, limiter *ratelimit.RateLimiter, tokens int64) bool {
	info := limiter.GetRateLimitInfo()

	// Check RPM
	if rpmInfo, ok := info["rpm"]; ok {
		if available, ok := rpmInfo["available"].(int64); ok && available < 1 {
			return false
		}
	}

	// Check TPM
	if tpmInfo, ok := info["tpm"]; ok {
		if available, ok := tpmInfo["available"].(int64); ok && available < tokens {
			return false
		}
	}

	return true
}

// getHealth retrieves or creates health tracker for provider
func (r *Router) getHealth(providerName string) *ProviderHealth {
	r.mu.RLock()
	health, exists := r.healthTracker[providerName]
	r.mu.RUnlock()

	if exists {
		return health
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	health, exists = r.healthTracker[providerName]
	if exists {
		return health
	}

	health = &ProviderHealth{
		ProviderName:     providerName,
		AvgLatencyMs:     100, // Default
		ErrorRate:        0.0,
		LastSuccess:      time.Now(),
		IsHealthy:        true,
		ConsecutiveFails: 0,
	}
	r.healthTracker[providerName] = health
	return health
}

// RecordSuccess updates health metrics after successful request
func (r *Router) RecordSuccess(providerName string, latencyMs int64) {
	health := r.getHealth(providerName)
	health.mu.Lock()
	defer health.mu.Unlock()

	// Exponential moving average for latency
	alpha := 0.3
	health.AvgLatencyMs = int64(alpha*float64(latencyMs) + (1-alpha)*float64(health.AvgLatencyMs))
	health.LastSuccess = time.Now()
	health.ConsecutiveFails = 0
	health.IsHealthy = true
	health.ErrorRate = health.ErrorRate * 0.95 // Decay error rate
}

// RecordFailure updates health metrics after failed request
func (r *Router) RecordFailure(providerName string, err error) {
	health := r.getHealth(providerName)
	health.mu.Lock()
	defer health.mu.Unlock()

	health.LastFailure = time.Now()
	health.ConsecutiveFails++
	health.ErrorRate = health.ErrorRate*0.95 + 0.05 // Increase by 5%

	// Mark unhealthy after 3 consecutive failures
	if health.ConsecutiveFails >= 3 {
		health.IsHealthy = false
	}
}

// GetHealthStatus returns current health of all tracked providers
func (r *Router) GetHealthStatus() map[string]*ProviderHealth {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := make(map[string]*ProviderHealth)
	for name, health := range r.healthTracker {
		status[name] = health
	}
	return status
}

// isExcluded checks if provider is in exclude list
func (r *Router) isExcluded(providerName string, excludeList []string) bool {
	for _, excluded := range excludeList {
		if providerName == excluded {
			return true
		}
	}
	return false
}

// matchesModel checks if modelID matches any required models
func (r *Router) matchesModel(modelID string, requiredModels []string) bool {
	for _, required := range requiredModels {
		if modelID == required {
			return true
		}
	}
	return false
}
