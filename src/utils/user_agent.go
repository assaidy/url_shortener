// Package utils holds helpers shared by services and handlers.
package utils

import "strings"

type userAgentPattern struct {
	pattern string
	name    string
}

var platformPatterns = []userAgentPattern{
	// Order is critical: Edge, Opera, and Brave user agents also contain
	// "chrome", and Chrome user agents are built on Safari's engine.
	{"edg", "Edge"},
	{"opr", "Opera"},
	{"opera", "Opera"},
	{"brave", "Brave"},
	{"firefox", "Firefox"},
	{"chrome", "Chrome"},
	{"safari", "Safari"},
}

var osPatterns = []userAgentPattern{
	// Order is critical: Android user agents contain "linux", iOS user agents
	// contain "mac os", and more specific Windows versions must match first.
	{"windows nt 10.0", "Windows 10"},
	{"windows nt 6.3", "Windows 8.1"},
	{"windows nt 6.2", "Windows 8"},
	{"windows nt 6.1", "Windows 7"},
	{"windows", "Windows"},
	{"android", "Android"},
	{"iphone", "iOS"},
	{"ipad", "iOS"},
	{"ios", "iOS"},
	{"macintosh", "macOS"},
	{"mac os", "macOS"},
	{"linux", "Linux"},
}

// ExtractPlatformFromUserAgent returns the browser name detected in userAgent,
// or "Unknown" when no known browser pattern matches.
func ExtractPlatformFromUserAgent(userAgent string) string {
	return matchUserAgent(userAgent, platformPatterns)
}

// ExtractOsFromUserAgent returns the operating system detected in userAgent, or
// "Unknown" when no known OS pattern matches.
func ExtractOsFromUserAgent(userAgent string) string {
	return matchUserAgent(userAgent, osPatterns)
}

func matchUserAgent(userAgent string, patterns []userAgentPattern) string {
	if userAgent == "" {
		return "Unknown"
	}

	userAgent = strings.ToLower(userAgent)
	for _, p := range patterns {
		if strings.Contains(userAgent, p.pattern) {
			return p.name
		}
	}

	return "Unknown"
}
