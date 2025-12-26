package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RateLimitInfo contains parsed rate limit information from response headers.
// Supports OpenAI, Anthropic, and Google Gemini header formats.
type RateLimitInfo struct {
	LimitRequests     int           // Maximum requests allowed in the current window
	RemainingRequests int           // Remaining requests in the current window
	ResetRequests     time.Duration // Time until the request limit resets
	LimitTokens       int           // Maximum tokens allowed in the current window
	RemainingTokens   int           // Remaining tokens in the current window
	ResetTokens       time.Duration // Time until the token limit resets
	RetryAfter        time.Duration // Time to wait before retrying (from Retry-After header)
}

// String returns a human-readable representation of rate limit info.
func (r *RateLimitInfo) String() string {
	var parts []string

	if r.LimitRequests > 0 || r.RemainingRequests > 0 {
		parts = append(parts, "requests="+strconv.Itoa(r.RemainingRequests)+"/"+strconv.Itoa(r.LimitRequests))
	}

	if r.LimitTokens > 0 || r.RemainingTokens > 0 {
		parts = append(parts, "tokens="+strconv.Itoa(r.RemainingTokens)+"/"+strconv.Itoa(r.LimitTokens))
	}

	if r.ResetRequests > 0 {
		parts = append(parts, "reset_req="+r.ResetRequests.String())
	}

	if r.ResetTokens > 0 {
		parts = append(parts, "reset_tok="+r.ResetTokens.String())
	}

	if r.RetryAfter > 0 {
		parts = append(parts, "retry_after="+r.RetryAfter.String())
	}

	if len(parts) == 0 {
		return "RateLimit{}"
	}

	return "RateLimit{" + strings.Join(parts, ", ") + "}"
}

// ParseRateLimitHeaders extracts rate limit information from HTTP response headers.
// Returns nil if no rate limit headers are found.
//
// Supported header formats:
//   - OpenAI: X-Ratelimit-Limit-Requests, X-Ratelimit-Remaining-Requests, etc.
//   - Anthropic: Anthropic-Ratelimit-Requests-Limit, etc.
//   - Google: X-Goog-Ratelimit-Limit, X-Goog-Ratelimit-Remaining
//   - Standard: Retry-After (seconds or HTTP date)
//
// Invalid values are silently skipped (e.g., non-numeric strings, invalid durations).
func ParseRateLimitHeaders(headers http.Header) *RateLimitInfo {
	info := &RateLimitInfo{}
	foundAny := false

	// OpenAI headers (priority 1)
	if val := headers.Get("X-Ratelimit-Limit-Requests"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			info.LimitRequests = n
			foundAny = true
		}
	}

	if val := headers.Get("X-Ratelimit-Remaining-Requests"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			info.RemainingRequests = n
			foundAny = true
		}
	}

	if val := headers.Get("X-Ratelimit-Reset-Requests"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			info.ResetRequests = d
			foundAny = true
		}
	}

	if val := headers.Get("X-Ratelimit-Limit-Tokens"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			info.LimitTokens = n
			foundAny = true
		}
	}

	if val := headers.Get("X-Ratelimit-Remaining-Tokens"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			info.RemainingTokens = n
			foundAny = true
		}
	}

	if val := headers.Get("X-Ratelimit-Reset-Tokens"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			info.ResetTokens = d
			foundAny = true
		}
	}

	// Anthropic headers (priority 2)
	if info.LimitRequests == 0 {
		if val := headers.Get("Anthropic-Ratelimit-Requests-Limit"); val != "" {
			if n, err := strconv.Atoi(val); err == nil {
				info.LimitRequests = n
				foundAny = true
			}
		}
	}

	if info.RemainingRequests == 0 {
		if val := headers.Get("Anthropic-Ratelimit-Requests-Remaining"); val != "" {
			if n, err := strconv.Atoi(val); err == nil {
				info.RemainingRequests = n
				foundAny = true
			}
		}
	}

	if info.LimitTokens == 0 {
		if val := headers.Get("Anthropic-Ratelimit-Tokens-Limit"); val != "" {
			if n, err := strconv.Atoi(val); err == nil {
				info.LimitTokens = n
				foundAny = true
			}
		}
	}

	if info.RemainingTokens == 0 {
		if val := headers.Get("Anthropic-Ratelimit-Tokens-Remaining"); val != "" {
			if n, err := strconv.Atoi(val); err == nil {
				info.RemainingTokens = n
				foundAny = true
			}
		}
	}

	// Google Gemini headers (priority 3)
	if info.LimitRequests == 0 {
		if val := headers.Get("X-Goog-Ratelimit-Limit"); val != "" {
			if n, err := strconv.Atoi(val); err == nil {
				info.LimitRequests = n
				foundAny = true
			}
		}
	}

	if info.RemainingRequests == 0 {
		if val := headers.Get("X-Goog-Ratelimit-Remaining"); val != "" {
			if n, err := strconv.Atoi(val); err == nil {
				info.RemainingRequests = n
				foundAny = true
			}
		}
	}

	// Retry-After header (standard)
	if val := headers.Get("Retry-After"); val != "" {
		// Try parsing as seconds first
		if seconds, err := strconv.Atoi(val); err == nil {
			info.RetryAfter = time.Duration(seconds) * time.Second
			foundAny = true
		} else {
			// Try parsing as HTTP date format
			if t, err := http.ParseTime(val); err == nil {
				info.RetryAfter = time.Until(t)
				if info.RetryAfter < 0 {
					info.RetryAfter = 0
				}
				foundAny = true
			}
		}
	}

	if !foundAny {
		return nil
	}

	return info
}

// sanitizeAPIKey masks sensitive parts of an API key for logging.
// Shows first 3 characters and last 7 characters, masks the rest.
// Format: "sk-1234567890abcdef..." -> "sk-***abcdef"
func sanitizeAPIKey(key string) string {
	if key == "" {
		return ""
	}

	keyLen := len(key)

	// For very short keys, show only last few chars
	if keyLen <= 5 {
		return "***" + key[max(0, keyLen-2):]
	}

	// For short keys (6-10 chars), show first 3 and last 3
	if keyLen <= 10 {
		return key[:3] + "***" + key[keyLen-3:]
	}

	// For normal keys, show first 3 and last 7
	return key[:3] + "***" + key[keyLen-7:]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
