package providers

import "strings"

// containsSubstring checks if a string contains a substring
func containsSubstring(s, substr string) bool {
	return strings.Contains(s, substr)
}

// containsAny checks if a string contains any of the given substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// hasPrefix checks if a string starts with any of the given prefixes
func hasPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}
