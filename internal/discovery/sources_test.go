package discovery

import (
	"context"
	"testing"
	"time"
)

func TestSourcePriority(t *testing.T) {
	sources := []Source{
		NewModelsDevSource(),
		NewGPUStackSource(),
		NewModelScopeSource(),
		NewHuggingFaceSource(),
	}

	expectedPriorities := map[string]int{
		"models.dev":  1,
		"GPUStack":    2,
		"ModelScope":  3,
		"HuggingFace": 4,
	}

	for _, source := range sources {
		name := source.Name()
		expected, ok := expectedPriorities[name]
		if !ok {
			t.Errorf("Unexpected source: %s", name)
			continue
		}

		priority := source.Priority()
		if priority != expected {
			t.Errorf("%s: expected priority %d, got %d", name, expected, priority)
		}
	}
}

func TestSourceStatsTracking(t *testing.T) {
	stats := NewSourceStats()

	// Test initial state
	summary := stats.GetSummary()
	if summary["total_calls"] != 0 {
		t.Errorf("Expected 0 total calls initially, got %v", summary["total_calls"])
	}

	// Record some successes
	stats.RecordSuccess("models.dev", 100)
	stats.RecordSuccess("models.dev", 200)
	stats.RecordSuccess("GPUStack", 150)

	// Record some failures
	stats.RecordFailure("HuggingFace", nil)
	stats.RecordFailure("ModelScope", nil)

	// Check totals
	summary = stats.GetSummary()
	if summary["total_calls"] != 5 {
		t.Errorf("Expected 5 total calls, got %v", summary["total_calls"])
	}
	if summary["total_errors"] != 2 {
		t.Errorf("Expected 2 total errors, got %v", summary["total_errors"])
	}

	// Check success rate
	successRate := summary["success_rate"].(float64)
	if successRate < 59.9 || successRate > 60.1 {
		t.Errorf("Expected ~60%% success rate, got %.2f%%", successRate)
	}

	// Check per-source stats
	sourceStats := stats.GetStats()

	modelsDevStat, ok := sourceStats["models.dev"]
	if !ok {
		t.Fatal("Expected models.dev stats")
	}
	if modelsDevStat.TotalCalls != 2 {
		t.Errorf("models.dev: expected 2 total calls, got %d", modelsDevStat.TotalCalls)
	}
	if modelsDevStat.SuccessCalls != 2 {
		t.Errorf("models.dev: expected 2 success calls, got %d", modelsDevStat.SuccessCalls)
	}
	if modelsDevStat.AvgLatencyMS != 150 {
		t.Errorf("models.dev: expected 150ms avg latency, got %d", modelsDevStat.AvgLatencyMS)
	}

	hfStat, ok := sourceStats["HuggingFace"]
	if !ok {
		t.Fatal("Expected HuggingFace stats")
	}
	if hfStat.FailedCalls != 1 {
		t.Errorf("HuggingFace: expected 1 failed call, got %d", hfStat.FailedCalls)
	}
}

func TestSourceStatsThreadSafety(t *testing.T) {
	stats := NewSourceStats()

	// Simulate concurrent recording
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				if j%2 == 0 {
					stats.RecordSuccess("test-source", int64(j))
				} else {
					stats.RecordFailure("test-source", nil)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify counts
	summary := stats.GetSummary()
	totalCalls := summary["total_calls"].(int)
	if totalCalls != 1000 {
		t.Errorf("Expected 1000 total calls, got %d", totalCalls)
	}
}

func TestAgentSourceStats(t *testing.T) {
	cfg := Config{
		ParallelBatch: 4,
		CacheDays:     1,
		MaxRetries:    1,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer agent.Close()

	// Initial stats should be empty
	summary := agent.GetStatsSummary()
	if summary["total_calls"] != 0 {
		t.Errorf("Expected 0 initial calls, got %v", summary["total_calls"])
	}

	// Make a discovery request (will fail without API keys, but should track stats)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req := DiscoveryRequest{
		Identifier: "test/model",
	}

	_, _ = agent.Discover(ctx, req)

	// Check that stats were recorded
	summary = agent.GetStatsSummary()
	totalCalls := summary["total_calls"].(int)
	if totalCalls == 0 {
		t.Error("Expected some source calls to be tracked")
	}

	// Check individual source stats
	sourceStats := agent.GetSourceStats()
	if len(sourceStats) == 0 {
		t.Error("Expected individual source stats to be recorded")
	}

	// Verify at least one source was attempted
	foundAttempt := false
	for sourceName, stat := range sourceStats {
		if stat.TotalCalls > 0 {
			foundAttempt = true
			t.Logf("Source %s: %d calls, %d successes, %d failures",
				sourceName, stat.TotalCalls, stat.SuccessCalls, stat.FailedCalls)
		}
	}

	if !foundAttempt {
		t.Error("Expected at least one source to have been attempted")
	}
}

func TestSourceStatsConcurrentDiscovery(t *testing.T) {
	cfg := Config{
		ParallelBatch: 4,
		CacheDays:     1,
		MaxRetries:    1,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer agent.Close()

	// Launch multiple concurrent discoveries
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func(id int) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			req := DiscoveryRequest{
				Identifier: "concurrent-test",
			}

			_, _ = agent.Discover(ctx, req)
			done <- true
		}(i)
	}

	// Wait for all discoveries
	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify stats were tracked correctly
	summary := agent.GetStatsSummary()
	totalCalls := summary["total_calls"].(int)

	// Should have attempted 4 sources per discovery (20 total expected)
	if totalCalls < 15 { // Allow some margin for failures
		t.Errorf("Expected ~20 total source calls from concurrent discoveries, got %d", totalCalls)
	}

	t.Logf("Concurrent discovery stats: %d total calls, %d errors, %.1f%% success rate",
		summary["total_calls"], summary["total_errors"], summary["success_rate"])
}
