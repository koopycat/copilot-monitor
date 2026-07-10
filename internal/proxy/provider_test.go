package proxy

import (
	"testing"
)

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		want       string
	}{
		// Copilot / GitHub tokens
		{"ghu_ prefix", "ghu_abc123def456", "copilot"},
		{"ghp_ prefix", "ghp_xxxxxxxxxxxxxxxxxxxx", "copilot"},
		{"github_pat_ prefix", "github_pat_11AA2222_BB33cc44dd55EE66ff77GG88", "copilot"},
		{"Bearer ghu_", "Bearer ghu_abc123def456", "copilot"},
		{"Bearer ghp_", "Bearer ghp_xxxxxxxxxxxxxxxxxxxx", "copilot"},
		{"Bearer github_pat_", "Bearer github_pat_11AA2222_BB33cc44dd55EE66ff77GG88", "copilot"},

		// OpenAI tokens
		{"sk- prefix (legacy)", "sk-abc123def456", "openai"},
		{"sk-proj- prefix", "sk-proj-xxxxxxxxxxxxxx", "openai"},
		{"Bearer sk-", "Bearer sk-abc123def456", "openai"},
		{"Bearer sk-proj-", "Bearer sk-proj-xxxxxxxxxxxxxx", "openai"},

		// Kilo tokens
		{"kl- prefix", "kl-abc123def456", "kilo"},
		{"kilocode_ prefix", "kilocode_abc123def456", "kilo"},
		{"Bearer kl-", "Bearer kl-abc123def456", "kilo"},
		{"Bearer kilocode_", "Bearer kilocode_abc123def456", "kilo"},

		// Unknown tokens
		{"unknown prefix", "unknown_token_abc", ""},
		{"Bearer unknown", "Bearer unknown_token_abc", ""},
		{"empty string", "", ""},
		{"only Bearer", "Bearer ", ""},
		{"Bearer plus garbage", "Bearer xyz_abc", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectProvider(tt.authHeader)
			if got != tt.want {
				t.Errorf("DetectProvider(%q) = %q, want %q", tt.authHeader, got, tt.want)
			}
		})
	}
}
