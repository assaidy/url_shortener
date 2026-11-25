package utils_test

import (
	"testing"

	"github.com/assaidy/url_shortener/internals/utils"
	"github.com/stretchr/testify/require"
)

func TestExtractPlatformFromUserAgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		userAgent string
		want      string
	}{
		{
			name:      "empty",
			userAgent: "",
			want:      "Unknown",
		},
		{
			name:      "firefox",
			userAgent: "Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0",
			want:      "Firefox",
		},
		{
			name:      "chrome",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
			want:      "Chrome",
		},
		{
			name:      "edge",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			want:      "Edge",
		},
		{
			name:      "opera",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36 OPR/106.0.0.0",
			want:      "Opera",
		},
		{
			name:      "unknown",
			userAgent: "curl/8.0",
			want:      "Unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, utils.ExtractPlatformFromUserAgent(tc.userAgent))
		})
	}
}

func TestExtractOsFromUserAgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		userAgent string
		want      string
	}{
		{
			name:      "empty",
			userAgent: "",
			want:      "Unknown",
		},
		{
			name:      "windows 10",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
			want:      "Windows 10",
		},
		{
			name:      "windows 7",
			userAgent: "Mozilla/5.0 (Windows NT 6.1; Win64; x64)",
			want:      "Windows 7",
		},
		{
			name:      "macos",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
			want:      "macOS",
		},
		{
			name:      "linux",
			userAgent: "Mozilla/5.0 (X11; Linux x86_64)",
			want:      "Linux",
		},
		{
			name:      "android before linux",
			userAgent: "Mozilla/5.0 (Linux; Android 14; Pixel 8)",
			want:      "Android",
		},
		{
			name:      "ios",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)",
			want:      "iOS",
		},
		{
			name:      "unknown",
			userAgent: "curl/8.0",
			want:      "Unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, utils.ExtractOsFromUserAgent(tc.userAgent))
		})
	}
}
