package headroom

import (
	"testing"
)

func TestNewClientRejectsUnsafeEndpoint(t *testing.T) {
	t.Parallel()

	tests := []string{
		"https://127.0.0.1:8787/v1/compress",
		"http://192.0.2.1:8787/v1/compress",
		"http://user@127.0.0.1:8787/v1/compress",
		"http://127.0.0.1:8787/v1/compress?debug=1",
		"http://127.0.0.1:8787/compress",
		"http://127.0.0.1/v1/compress",
	}
	for _, endpoint := range tests {
		endpoint := endpoint
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()
			if _, err := NewClient(endpoint, nil); err == nil {
				t.Fatalf("NewClient(%q) succeeded, want error", endpoint)
			}
		})
	}
}
