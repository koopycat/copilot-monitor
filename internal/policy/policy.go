package policy

import "strings"

type Mode string

const (
	AllowAll  Mode = "allow_all"
	Allowlist Mode = "allowlist"
	Blocklist Mode = "blocklist"
)

type Policy struct {
	Mode   Mode     `json:"mode"`
	Models []string `json:"models"`
}

// DefaultPolicy returns a policy that allows all models.
func DefaultPolicy() *Policy {
	return &Policy{Mode: AllowAll}
}

// Allowed returns true if the model should pass the policy.
// Empty model always passes (fail-open).
// Unknown mode or nil policy always passes (fail-open).
func (p *Policy) Allowed(model string) bool {
	if p == nil || model == "" {
		return true
	}
	switch p.Mode {
	case Allowlist:
		return p.matches(model)
	case Blocklist:
		return !p.matches(model)
	default:
		return true
	}
}

// matches returns true if model matches any pattern in Models.
// Exact match by default. * suffix does prefix matching (e.g., "gpt-*" matches "gpt-4o").
func (p *Policy) matches(model string) bool {
	for _, pattern := range p.Models {
		if matchPattern(pattern, model) {
			return true
		}
	}
	return false
}

func matchPattern(pattern, model string) bool {
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(model, prefix)
	}
	return pattern == model
}
