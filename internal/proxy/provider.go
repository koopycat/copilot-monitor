package proxy

import "strings"

// DetectProvider returns the provider name based on the Authorization header token prefix.
// Known providers:
//   - "copilot": GitHub tokens (ghu_, ghp_, github_pat_)
//   - "openai":  OpenAI tokens (sk-, sk-proj-)
//   - "kilo":    Kilo tokens (kl-, kilocode_)
//
// Returns "" for unknown or missing auth headers.
func DetectProvider(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	// Strip "Bearer " prefix if present
	token := strings.TrimPrefix(authHeader, "Bearer ")
	switch {
	case strings.HasPrefix(token, "ghu_"),
		strings.HasPrefix(token, "ghp_"),
		strings.HasPrefix(token, "github_pat_"):
		return "copilot"
	case strings.HasPrefix(token, "sk-"),
		strings.HasPrefix(token, "sk-proj-"):
		return "openai"
	case strings.HasPrefix(token, "kl-"),
		strings.HasPrefix(token, "kilocode_"):
		return "kilo"
	default:
		return ""
	}
}
