package proxy

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"strings"
	"time"

	"copilot-monitoring/internal/compression/headroom"
)

var errCompressionRequired = errors.New("request compression failed")

type requestCompressor interface {
	Compress(context.Context, headroom.CompressionRequest) (headroom.CompressionResult, error)
}

// compressionMeta records the outcome of an optional compression step.
// Zeros mean compression was not attempted or did not apply.
type compressionMeta struct {
	Status         string
	OriginalTokens int
	FinalTokens    int
	LatencyMS      int64
}

// ConfigureCompression installs the startup-time request compressor. It must
// be called before the handler is serving requests.
func (h *Handler) ConfigureCompression(compressor headroom.MessageCompressor, required bool) {
	h.compressor = compressor
	h.compressionRequired = required
}

func (h *Handler) maybeCompress(ctx context.Context, id uint64, r *http.Request, route Route, body []byte, meta *compressionMeta) ([]byte, error) {
	if !compressionEligible(r, route, h.compressor) {
		return body, nil
	}

	start := time.Now()
	result, err := headroom.CompressOpenAIChat(ctx, h.compressor, body)
	meta.LatencyMS = time.Since(start).Milliseconds()

	if err == nil {
		meta.Status = "applied"
		meta.OriginalTokens = result.Compression.OriginalTokens
		meta.FinalTokens = result.Compression.CompressedTokens
		if result.Compression.TokensSaved == 0 {
			meta.Status = "no_change"
		}
		h.log.Info("id=%d compression=%s before=%d after=%d saved=%d ratio=%.3f latency_ms=%d\n",
			id,
			meta.Status,
			meta.OriginalTokens,
			meta.FinalTokens,
			result.Compression.TokensSaved,
			result.Compression.CompressionRatio,
			meta.LatencyMS,
		)
		return result.Body, nil
	}

	if errors.Is(err, headroom.ErrUnsupportedEnvelope) {
		meta.Status = "bypassed"
		h.log.Info("id=%d compression=bypassed reason=unsupported_envelope\n", id)
		return body, nil
	}

	category := compressionErrorCategory(ctx, err)
	mode := "fail_open"
	if h.compressionRequired {
		mode = "required"
	}
	meta.Status = "failed_" + mode
	h.log.Warn("id=%d compression=failed mode=%s category=%s\n", id, mode, category)
	if h.compressionRequired {
		return nil, errCompressionRequired
	}
	return body, nil
}

func compressionEligible(r *http.Request, route Route, compressor requestCompressor) bool {
	if compressor == nil || r.Method != http.MethodPost || route.Local {
		return false
	}
	if r.URL.Path != "/chat/completions" && r.URL.Path != "/v1/chat/completions" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

func compressionErrorCategory(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var statusErr *headroom.StatusError
	if errors.As(err, &statusErr) {
		if statusErr.StatusCode >= 400 && statusErr.StatusCode < 500 {
			return "http_4xx"
		}
		return "http_5xx"
	}
	// Fallback: classify by error message substring for Headroom response errors.
	// The raw error text is not logged — only this stable category string is emitted.
	if strings.Contains(err.Error(), "response") || strings.Contains(err.Error(), "metrics") || strings.Contains(err.Error(), "messages") {
		return "invalid_response"
	}
	return "transport"
}
