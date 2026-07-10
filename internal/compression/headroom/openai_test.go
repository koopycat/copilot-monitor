package headroom

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type compressorFunc func(context.Context, CompressionRequest) (CompressionResult, error)

func (f compressorFunc) Compress(ctx context.Context, req CompressionRequest) (CompressionResult, error) {
	return f(ctx, req)
}

func TestCompressOpenAIChatPreservesEnvelope(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model":"gpt-4o",
		"messages":[{"role":"user","content":"long synthetic content"}],
		"stream":true,
		"temperature":0.2,
		"tools":[{"type":"function","function":{"name":"lookup"}}],
		"custom":{"preserve":true}
	}`)

	compressor := compressorFunc(func(ctx context.Context, req CompressionRequest) (CompressionResult, error) {
		if req.Model != "gpt-4o" {
			t.Fatalf("model = %q, want gpt-4o", req.Model)
		}
		if string(req.Messages) != `[{"role":"user","content":"long synthetic content"}]` {
			t.Fatalf("messages = %s", req.Messages)
		}
		return CompressionResult{
			Messages:         json.RawMessage(`[{"role":"user","content":"short"}]`),
			OriginalTokens:   20,
			CompressedTokens: 5,
			TokensSaved:      15,
			CompressionRatio: 0.25,
		}, nil
	})

	result, err := CompressOpenAIChat(context.Background(), compressor, body)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(result.Body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 6 {
		t.Fatalf("fields = %v, want all original fields", got)
	}
	if string(got["messages"]) != `[{"role":"user","content":"short"}]` {
		t.Fatalf("messages = %s", got["messages"])
	}
	for _, field := range []string{"model", "stream", "temperature", "tools", "custom"} {
		var before any
		var after any
		var original map[string]json.RawMessage
		if err := json.Unmarshal(body, &original); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(original[field], &before); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(got[field], &after); err != nil {
			t.Fatal(err)
		}
		if !equalJSON(before, after) {
			t.Errorf("field %q changed: before=%v after=%v", field, before, after)
		}
	}
}

func TestCompressOpenAIChatReturnsErrorWithoutMutatingInput(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"original"}]}`)
	original := append([]byte(nil), body...)
	wantErr := errors.New("Headroom unavailable")
	compressor := compressorFunc(func(context.Context, CompressionRequest) (CompressionResult, error) {
		return CompressionResult{}, wantErr
	})

	result, err := CompressOpenAIChat(context.Background(), compressor, body)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if result.Body != nil {
		t.Fatalf("result body = %q, want nil", result.Body)
	}
	if string(body) != string(original) {
		t.Fatalf("input mutated: got %q want %q", body, original)
	}
}

func TestCompressOpenAIChatRejectsUnsupportedEnvelope(t *testing.T) {
	t.Parallel()

	tests := []string{
		`not json`,
		`[]`,
		`{"messages":[]}`,
		`{"model":"gpt-4o"}`,
		`{"model":"gpt-4o","messages":{}}`,
	}
	compressor := compressorFunc(func(context.Context, CompressionRequest) (CompressionResult, error) {
		t.Fatal("compressor called for invalid envelope")
		return CompressionResult{}, nil
	})
	for _, body := range tests {
		body := body
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			if _, err := CompressOpenAIChat(context.Background(), compressor, []byte(body)); err == nil {
				t.Fatal("CompressOpenAIChat succeeded, want error")
			}
		})
	}
}

func equalJSON(a, b any) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}
