package http

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRateLimitHeaders(t *testing.T) {
	tests := []struct {
		name     string
		headers  http.Header
		want     *RateLimitInfo
		wantNil  bool
	}{
		{
			name: "OpenAI headers - all fields present",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests":     []string{"10000"},
				"X-Ratelimit-Remaining-Requests": []string{"9999"},
				"X-Ratelimit-Reset-Requests":     []string{"1s"},
				"X-Ratelimit-Limit-Tokens":       []string{"2000000"},
				"X-Ratelimit-Remaining-Tokens":   []string{"1999900"},
				"X-Ratelimit-Reset-Tokens":       []string{"27ms"},
			},
			want: &RateLimitInfo{
				LimitRequests:     10000,
				RemainingRequests: 9999,
				ResetRequests:     1 * time.Second,
				LimitTokens:       2000000,
				RemainingTokens:   1999900,
				ResetTokens:       27 * time.Millisecond,
			},
		},
		{
			name: "OpenAI headers - partial fields",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests":     []string{"10000"},
				"X-Ratelimit-Remaining-Requests": []string{"5000"},
			},
			want: &RateLimitInfo{
				LimitRequests:     10000,
				RemainingRequests: 5000,
			},
		},
		{
			name: "Anthropic headers",
			headers: http.Header{
				"Anthropic-Ratelimit-Requests-Limit":     []string{"1000"},
				"Anthropic-Ratelimit-Requests-Remaining": []string{"999"},
				"Anthropic-Ratelimit-Requests-Reset":     []string{"2024-01-15T10:30:00Z"},
				"Anthropic-Ratelimit-Tokens-Limit":       []string{"100000"},
				"Anthropic-Ratelimit-Tokens-Remaining":   []string{"99500"},
				"Anthropic-Ratelimit-Tokens-Reset":       []string{"2024-01-15T10:30:00Z"},
			},
			want: &RateLimitInfo{
				LimitRequests:     1000,
				RemainingRequests: 999,
				LimitTokens:       100000,
				RemainingTokens:   99500,
			},
		},
		{
			name: "Google Gemini headers",
			headers: http.Header{
				"X-Goog-Ratelimit-Limit":     []string{"60"},
				"X-Goog-Ratelimit-Remaining": []string{"59"},
			},
			want: &RateLimitInfo{
				LimitRequests:     60,
				RemainingRequests: 59,
			},
		},
		{
			name:    "No rate limit headers",
			headers: http.Header{},
			wantNil: true,
		},
		{
			name: "Invalid numeric values - should skip invalid fields",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests":     []string{"not-a-number"},
				"X-Ratelimit-Remaining-Requests": []string{"9999"},
			},
			want: &RateLimitInfo{
				RemainingRequests: 9999,
			},
		},
		{
			name: "Invalid duration format - should skip invalid fields",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests": []string{"10000"},
				"X-Ratelimit-Reset-Requests": []string{"invalid-duration"},
			},
			want: &RateLimitInfo{
				LimitRequests: 10000,
			},
		},
		{
			name: "Mixed provider headers - OpenAI takes precedence",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests":             []string{"10000"},
				"Anthropic-Ratelimit-Requests-Limit":     []string{"1000"},
				"X-Goog-Ratelimit-Limit":                 []string{"60"},
			},
			want: &RateLimitInfo{
				LimitRequests: 10000,
			},
		},
		{
			name: "Complex duration parsing - hours, minutes, seconds",
			headers: http.Header{
				"X-Ratelimit-Reset-Requests": []string{"1h30m45s"},
				"X-Ratelimit-Reset-Tokens":   []string{"500ms"},
			},
			want: &RateLimitInfo{
				ResetRequests: 1*time.Hour + 30*time.Minute + 45*time.Second,
				ResetTokens:   500 * time.Millisecond,
			},
		},
		{
			name: "Case insensitive header names",
			headers: func() http.Header {
				h := http.Header{}
				h.Set("x-ratelimit-limit-requests", "10000")
				h.Set("X-RATELIMIT-REMAINING-REQUESTS", "9999")
				return h
			}(),
			want: &RateLimitInfo{
				LimitRequests:     10000,
				RemainingRequests: 9999,
			},
		},
		{
			name: "Retry-After header - seconds format",
			headers: http.Header{
				"Retry-After": []string{"120"},
			},
			want: &RateLimitInfo{
				RetryAfter: 120 * time.Second,
			},
		},
		{
			name: "Retry-After header - HTTP date format",
			headers: func() http.Header {
				h := http.Header{}
				futureTime := time.Now().Add(2 * time.Minute)
				h.Set("Retry-After", futureTime.Format(http.TimeFormat))
				return h
			}(),
			want: &RateLimitInfo{
				// Will be non-zero duration calculated from current time
				// We'll check this separately in the test
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRateLimitHeaders(tt.headers)

			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseRateLimitHeaders() = %+v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatalf("ParseRateLimitHeaders() = nil, want %+v", tt.want)
			}

			// Special handling for Retry-After HTTP date format test
			if tt.name == "Retry-After header - HTTP date format" {
				if got.RetryAfter <= 0 {
					t.Errorf("RetryAfter should be positive duration, got %v", got.RetryAfter)
				}
				return
			}

			if got.LimitRequests != tt.want.LimitRequests {
				t.Errorf("LimitRequests = %d, want %d", got.LimitRequests, tt.want.LimitRequests)
			}
			if got.RemainingRequests != tt.want.RemainingRequests {
				t.Errorf("RemainingRequests = %d, want %d", got.RemainingRequests, tt.want.RemainingRequests)
			}
			if got.ResetRequests != tt.want.ResetRequests {
				t.Errorf("ResetRequests = %v, want %v", got.ResetRequests, tt.want.ResetRequests)
			}
			if got.LimitTokens != tt.want.LimitTokens {
				t.Errorf("LimitTokens = %d, want %d", got.LimitTokens, tt.want.LimitTokens)
			}
			if got.RemainingTokens != tt.want.RemainingTokens {
				t.Errorf("RemainingTokens = %d, want %d", got.RemainingTokens, tt.want.RemainingTokens)
			}
			if got.ResetTokens != tt.want.ResetTokens {
				t.Errorf("ResetTokens = %v, want %v", got.ResetTokens, tt.want.ResetTokens)
			}
			if got.RetryAfter != tt.want.RetryAfter {
				t.Errorf("RetryAfter = %v, want %v", got.RetryAfter, tt.want.RetryAfter)
			}
		})
	}
}

func TestSanitizeAPIKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "OpenAI key format",
			key:  "sk-1234567890abcdef1234567890abcdef1234567890abcdef",
			want: "sk-***0abcdef",
		},
		{
			name: "Anthropic key format",
			key:  "sk-ant-api03-1234567890abcdef1234567890abcdef1234567890abcdef",
			want: "sk-***0abcdef",
		},
		{
			name: "Short key - less than 10 chars",
			key:  "sk-abc",
			want: "sk-***abc",
		},
		{
			name: "Very short key - 5 chars",
			key:  "12345",
			want: "***45",
		},
		{
			name: "Empty key",
			key:  "",
			want: "",
		},
		{
			name: "Google API key format",
			key:  "AIzaSyC1234567890abcdefghijklmnop",
			want: "AIz***jklmnop",
		},
		{
			name: "Generic bearer token",
			key:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0",
			want: "eyJ***ODkwIn0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeAPIKey(tt.key)
			if got != tt.want {
				t.Errorf("sanitizeAPIKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestRateLimitInfoString(t *testing.T) {
	info := &RateLimitInfo{
		LimitRequests:     10000,
		RemainingRequests: 9999,
		ResetRequests:     1 * time.Second,
		LimitTokens:       2000000,
		RemainingTokens:   1999900,
		ResetTokens:       27 * time.Millisecond,
	}

	str := info.String()

	// Check that string representation contains key information
	expectedSubstrings := []string{
		"requests=9999/10000",
		"tokens=1999900/2000000",
		"reset_req=1s",
		"reset_tok=27ms",
	}

	for _, expected := range expectedSubstrings {
		if !contains(str, expected) {
			t.Errorf("String() = %q, should contain %q", str, expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
