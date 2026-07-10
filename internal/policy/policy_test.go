package policy

import "testing"

func TestAllowedAllowAll(t *testing.T) {
	tests := []struct {
		name   string
		policy *Policy
		model  string
		want   bool
	}{
		{"nil policy", nil, "gpt-4o", true},
		{"empty policy allow_all no models", &Policy{Mode: AllowAll}, "gpt-4o", true},
		{"allow_all with populated models", &Policy{Mode: AllowAll, Models: []string{"claude-*"}}, "gpt-4o", true},
		{"empty model", &Policy{Mode: AllowAll}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.Allowed(tt.model)
			if got != tt.want {
				t.Errorf("Allowed(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestAllowedBlocklist(t *testing.T) {
	tests := []struct {
		name   string
		policy *Policy
		model  string
		want   bool
	}{
		{"exact match blocks gpt-4o", &Policy{Mode: Blocklist, Models: []string{"gpt-4o"}}, "gpt-4o", false},
		{"prefix match blocks gpt-4o", &Policy{Mode: Blocklist, Models: []string{"gpt-*"}}, "gpt-4o", false},
		{"unblocked model claude passes", &Policy{Mode: Blocklist, Models: []string{"gpt-*"}}, "claude", true},
		{"empty model passes", &Policy{Mode: Blocklist, Models: []string{"gpt-*"}}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.Allowed(tt.model)
			if got != tt.want {
				t.Errorf("Allowed(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestAllowedAllowlist(t *testing.T) {
	tests := []struct {
		name   string
		policy *Policy
		model  string
		want   bool
	}{
		{"listed model gpt-4o passes", &Policy{Mode: Allowlist, Models: []string{"gpt-4o"}}, "gpt-4o", true},
		{"unlisted model claude blocked", &Policy{Mode: Allowlist, Models: []string{"gpt-4o"}}, "claude", false},
		{"empty model passes", &Policy{Mode: Allowlist, Models: []string{"gpt-4o"}}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.Allowed(tt.model)
			if got != tt.want {
				t.Errorf("Allowed(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestAllowedUnknownMode(t *testing.T) {
	t.Run("unrecognized mode returns true", func(t *testing.T) {
		p := &Policy{Mode: "bogus", Models: []string{"gpt-4o"}}
		got := p.Allowed("gpt-4o")
		if !got {
			t.Errorf("Allowed(%q) = %v, want true (fail-open)", "gpt-4o", got)
		}
	})
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		model   string
		want    bool
	}{
		{"exact match", "gpt-4o", "gpt-4o", true},
		{"prefix match with *", "gpt-*", "gpt-4o", true},
		{"no match", "gpt-4o", "claude", false},
		{"empty strings", "", "", true},
		{"star pattern matches everything", "*", "anything", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPattern(tt.pattern, tt.model)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.model, got, tt.want)
			}
		})
	}
}
