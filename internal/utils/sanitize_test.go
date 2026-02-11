package utils

import (
	"testing"
)

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal URL",
			input:    "http://example.com/file",
			expected: "http://example.com/file",
		},
		{
			name:     "URL with Query Params",
			input:    "http://example.com/file?token=secret&id=123",
			expected: "http://example.com/file?REDACTED",
		},
		{
			name:     "URL with User Info",
			input:    "http://user:pass@example.com/file",
			expected: "http://REDACTED@example.com/file",
		},
		{
			name:     "URL with User Info and Query Params",
			input:    "http://user:pass@example.com/file?token=secret",
			expected: "http://REDACTED@example.com/file?REDACTED",
		},
		{
			name:     "Invalid URL",
			input:    "://invalid-url",
			expected: "://invalid-url",
		},
		{
			name:     "Empty URL",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeURL(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
