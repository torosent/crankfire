package auth

import (
	"testing"
	"time"
)

func TestCacheFor(t *testing.T) {
	tests := []struct {
		name      string
		expiresIn int
		leeway    time.Duration
		want      time.Duration
	}{
		{"normal lifetime minus leeway", 3600, 30 * time.Second, 3570 * time.Second},
		{"no expires_in from the server", 0, 30 * time.Second, defaultTokenLifetime - 30*time.Second},
		{"lifetime shorter than the leeway", 20, 30 * time.Second, 10 * time.Second},
	}
	for _, tt := range tests {
		if got := cacheFor(tt.expiresIn, tt.leeway); got != tt.want {
			t.Errorf("%s: cacheFor(%d, %s) = %s, want %s", tt.name, tt.expiresIn, tt.leeway, got, tt.want)
		}
	}
}
