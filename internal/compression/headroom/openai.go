package headroom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type MessageCompressor interface {
	Compress(context.Context, CompressionRequest) (CompressionResult, error)
}

var ErrUnsupportedEnvelope = errors.New("unsupported OpenAI chat envelope")

type OpenAIChatResult struct {
	Body        []byte
	Compression CompressionResult
}

// CompressOpenAIChat replaces only the messages field in an OpenAI-compatible
// chat request. Callers retain the original body for fail-open behavior.
func CompressOpenAIChat(ctx context.Context, compressor MessageCompressor, body []byte) (OpenAIChatResult, error) {
	if compressor == nil {
		return OpenAIChatResult{}, errors.New("compressor is required")
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil || envelope == nil {
		return OpenAIChatResult{}, fmt.Errorf("%w: request body must be a JSON object", ErrUnsupportedEnvelope)
	}

	var model string
	if err := json.Unmarshal(envelope["model"], &model); err != nil || strings.TrimSpace(model) == "" {
		return OpenAIChatResult{}, fmt.Errorf("%w: request model is required", ErrUnsupportedEnvelope)
	}
	messages, ok := envelope["messages"]
	if !ok {
		return OpenAIChatResult{}, fmt.Errorf("%w: request messages are required", ErrUnsupportedEnvelope)
	}
	if err := validateMessages(messages); err != nil {
		return OpenAIChatResult{}, fmt.Errorf("%w: validate request messages: %v", ErrUnsupportedEnvelope, err)
	}

	result, err := compressor.Compress(ctx, CompressionRequest{
		Model:    model,
		Messages: messages,
	})
	if err != nil {
		return OpenAIChatResult{}, err
	}
	if err := validateMessages(result.Messages); err != nil {
		return OpenAIChatResult{}, fmt.Errorf("validate compressor result: %w", err)
	}

	envelope["messages"] = result.Messages
	compressedBody, err := json.Marshal(envelope)
	if err != nil {
		return OpenAIChatResult{}, fmt.Errorf("encode compressed request: %w", err)
	}

	return OpenAIChatResult{
		Body:        compressedBody,
		Compression: result,
	}, nil
}
