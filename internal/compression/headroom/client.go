package headroom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const maxResponseBytes = 16 << 20

// CompressionRequest is the content sent to Headroom's compression-only API.
// It deliberately has no HTTP headers or raw provider request attached.
type CompressionRequest struct {
	Model    string
	Messages json.RawMessage
}

type CompressionConfig struct {
	CompressUserMessages   bool     `json:"compress_user_messages,omitempty"`
	TargetRatio            *float64 `json:"target_ratio,omitempty"`
	ProtectRecent          *int     `json:"protect_recent,omitempty"`
	ProtectAnalysisContext *bool    `json:"protect_analysis_context,omitempty"`
}

type CompressionResult struct {
	Messages         json.RawMessage
	OriginalTokens   int
	CompressedTokens int
	TokensSaved      int
	CompressionRatio float64
	Transforms       []string
	CCRHashes        []string
}

type Client struct {
	endpoint   *url.URL
	httpClient *http.Client
	config     CompressionConfig
}

type ClientOptions struct {
	HTTPClient  *http.Client
	Compression CompressionConfig
}

type StatusError struct {
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("headroom compression failed with HTTP %d", e.StatusCode)
}

func NewClient(endpoint string, options ClientOptions) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse headroom endpoint: %w", err)
	}
	if u.Scheme != "http" {
		return nil, errors.New("headroom endpoint must use http")
	}
	if u.User != nil {
		return nil, errors.New("headroom endpoint must not contain user information")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("headroom endpoint must not contain a query or fragment")
	}
	if u.Path != "/v1/compress" {
		return nil, errors.New("headroom endpoint path must be /v1/compress")
	}
	if !isLoopbackHost(u.Hostname()) {
		return nil, errors.New("headroom endpoint must use a loopback address")
	}
	if u.Port() == "" {
		return nil, errors.New("headroom endpoint must include a port")
	}
	if options.HTTPClient == nil {
		options.HTTPClient = http.DefaultClient
	}
	clientCopy := *options.HTTPClient
	clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	if err := validateConfig(&options.Compression); err != nil {
		return nil, fmt.Errorf("validate headroom configuration: %w", err)
	}
	return &Client{endpoint: u, httpClient: &clientCopy, config: options.Compression}, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (c *Client) Compress(ctx context.Context, input CompressionRequest) (CompressionResult, error) {
	if strings.TrimSpace(input.Model) == "" {
		return CompressionResult{}, errors.New("compression model is required")
	}
	if err := validateMessages(input.Messages); err != nil {
		return CompressionResult{}, fmt.Errorf("validate compression messages: %w", err)
	}
	if err := validateConfig(&c.config); err != nil {
		return CompressionResult{}, fmt.Errorf("validate compression config: %w", err)
	}

	var config *CompressionConfig
	if c.config != (CompressionConfig{}) {
		config = &c.config
	}
	payload, err := json.Marshal(struct {
		Model    string             `json:"model"`
		Messages json.RawMessage    `json:"messages"`
		Config   *CompressionConfig `json:"config,omitempty"`
	}{
		Model:    input.Model,
		Messages: input.Messages,
		Config:   config,
	})
	if err != nil {
		return CompressionResult{}, fmt.Errorf("encode headroom request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return CompressionResult{}, fmt.Errorf("build Headroom request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return CompressionResult{}, fmt.Errorf("call headroom: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return CompressionResult{}, &StatusError{StatusCode: resp.StatusCode}
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return CompressionResult{}, errors.New("headroom response must have application/json content type")
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return CompressionResult{}, fmt.Errorf("read headroom response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return CompressionResult{}, errors.New("headroom response exceeds size limit")
	}

	var output struct {
		Messages         json.RawMessage `json:"messages"`
		TokensBefore     *int            `json:"tokens_before"`
		TokensAfter      *int            `json:"tokens_after"`
		TokensSaved      *int            `json:"tokens_saved"`
		CompressionRatio *float64        `json:"compression_ratio"`
		Transforms       []string        `json:"transforms_applied"`
		CCRHashes        []string        `json:"ccr_hashes"`
	}
	if err := json.Unmarshal(body, &output); err != nil {
		return CompressionResult{}, errors.New("decode headroom response")
	}
	if err := validateMessages(output.Messages); err != nil {
		return CompressionResult{}, fmt.Errorf("validate compressed messages: %w", err)
	}
	if output.TokensBefore == nil || output.TokensAfter == nil || output.TokensSaved == nil || output.CompressionRatio == nil {
		return CompressionResult{}, errors.New("headroom response is missing token metrics")
	}
	if *output.TokensBefore < 0 || *output.TokensAfter < 0 || *output.TokensSaved < 0 {
		return CompressionResult{}, errors.New("headroom response contains negative token metrics")
	}
	if *output.TokensAfter > *output.TokensBefore {
		return CompressionResult{}, errors.New("headroom response reports token expansion")
	}
	if *output.TokensSaved != *output.TokensBefore-*output.TokensAfter {
		return CompressionResult{}, errors.New("headroom response token metrics are inconsistent")
	}
	if math.IsNaN(*output.CompressionRatio) || math.IsInf(*output.CompressionRatio, 0) || *output.CompressionRatio < 0 || *output.CompressionRatio > 1 {
		return CompressionResult{}, errors.New("headroom response has an invalid compression ratio")
	}

	return CompressionResult{
		Messages:         append(json.RawMessage(nil), output.Messages...),
		OriginalTokens:   *output.TokensBefore,
		CompressedTokens: *output.TokensAfter,
		TokensSaved:      *output.TokensSaved,
		CompressionRatio: *output.CompressionRatio,
		Transforms:       append([]string(nil), output.Transforms...),
		CCRHashes:        append([]string(nil), output.CCRHashes...),
	}, nil
}

func validateConfig(config *CompressionConfig) error {
	if config == nil {
		return nil
	}
	if config.TargetRatio != nil {
		ratio := *config.TargetRatio
		if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio <= 0 || ratio > 1 {
			return errors.New("target ratio must be greater than zero and at most one")
		}
	}
	if config.ProtectRecent != nil && *config.ProtectRecent < 0 {
		return errors.New("protect recent must not be negative")
	}
	return nil
}

func validateMessages(raw json.RawMessage) error {
	if len(raw) == 0 {
		return errors.New("messages are required")
	}
	var messages []json.RawMessage
	if err := json.Unmarshal(raw, &messages); err != nil {
		return errors.New("messages must be a JSON array")
	}
	if messages == nil {
		return errors.New("messages must be a JSON array")
	}
	for i, message := range messages {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(message, &object); err != nil || object == nil {
			return fmt.Errorf("message %d must be a JSON object", i)
		}
	}
	return nil
}
