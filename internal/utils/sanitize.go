package utils

import (
	"net/url"
)

// SanitizeURL removes sensitive information (like query parameters and user info) from a URL string for logging.
func SanitizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		// If parsing fails, return the original string.
		// We assume that if it's not a valid URL, it might not be a sensitive URL,
		// or at least we can't easily identify parts to redact.
		return rawURL
	}

	// Redact User Info
	if u.User != nil {
		u.User = url.User("REDACTED")
	}

	// Redact Query Parameters
	if u.RawQuery != "" {
		u.RawQuery = "REDACTED"
	}

	return u.String()
}
