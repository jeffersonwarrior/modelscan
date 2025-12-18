package providers

import "testing"

func TestContainsSubstring(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"found", "hello world", "world", true},
		{"not found", "hello world", "xyz", false},
		{"empty substr", "hello", "", true},
		{"empty string", "", "test", false},
		{"case sensitive", "Hello", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsSubstring(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("containsSubstring(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name       string
		s          string
		substrings []string
		expected   bool
	}{
		{"found first", "hello world", []string{"hello", "xyz"}, true},
		{"found second", "hello world", []string{"xyz", "world"}, true},
		{"not found", "hello world", []string{"xyz", "abc"}, false},
		{"empty slice", "hello", []string{}, false},
		{"empty string", "", []string{"test"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsAny(tt.s, tt.substrings)
			if result != tt.expected {
				t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.substrings, result, tt.expected)
			}
		})
	}
}

func TestHasPrefix(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		prefixes []string
		expected bool
	}{
		{"found first", "hello world", []string{"hello", "xyz"}, true},
		{"found second", "hello world", []string{"xyz", "hello"}, true},
		{"not found", "hello world", []string{"xyz", "abc"}, false},
		{"empty slice", "hello", []string{}, false},
		{"empty string", "", []string{"test"}, false},
		{"exact match", "test", []string{"test"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasPrefix(tt.s, tt.prefixes)
			if result != tt.expected {
				t.Errorf("hasPrefix(%q, %v) = %v, want %v", tt.s, tt.prefixes, result, tt.expected)
			}
		})
	}
}
