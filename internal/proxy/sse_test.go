package proxy

import "testing"

func TestSSEObserverDetectsUsageAndModelAcrossSplitLines(t *testing.T) {
	observer := NewSSEObserver()
	observer.Observe([]byte("data: {\"mod"))
	observer.Observe([]byte("el\":\"gpt-4o\",\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\n"))
	observer.Observe([]byte("data: [DONE]\n\n"))
	observer.Finish()

	if !observer.UsageSeen {
		t.Fatal("usage was not detected")
	}
	if observer.Model != "gpt-4o" {
		t.Fatalf("model = %q, want gpt-4o", observer.Model)
	}
	if observer.Usage.PromptTokens != 3 || observer.Usage.CompletionTokens != 2 || observer.Usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v", observer.Usage)
	}
	if len(observer.UsageObjects) != 1 {
		t.Fatalf("usage objects = %d, want 1", len(observer.UsageObjects))
	}
	if observer.Bytes == 0 {
		t.Fatal("bytes were not counted")
	}
}

func TestSSEObserverToleratesMalformedJSON(t *testing.T) {
	observer := NewSSEObserver()
	observer.Observe([]byte("data: {broken json}\n"))
	observer.Observe([]byte("data: {\"usage\":{\"total_tokens\":1}}\n"))

	if !observer.UsageSeen {
		t.Fatal("usage was not detected after malformed event")
	}
	if observer.ParseErrors == 0 {
		t.Fatal("expected parse error count")
	}
}
