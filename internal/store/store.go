package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"llm-proxy/internal/policy"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

type Store struct {
	db   *sql.DB
	path string
}

type RequestRecord struct {
	Timestamp                 time.Time
	Endpoint                  string
	Method                    string
	Path                      string
	UpstreamHost              string
	Model                     string
	Stream                    bool
	Status                    int
	Error                     string
	LatencyMS                 int64
	PromptTokens              int
	CachedInputTokens         int
	CacheWriteTokens          int
	CompletionTokens          int
	TotalTokens               int
	Project                   string
	NotBilled                 bool
	Provider                  string
	UsageMissing              bool
	CompressionStatus         string
	CompressionOriginalTokens int
	CompressionFinalTokens    int
	CompressionLatencyMS      int64
}

type StatsFilter struct {
	Since        time.Time
	Until        time.Time
	Project      string
	Endpoint     string
	UpstreamHost string
}

type ModelStats struct {
	Model                     string  `json:"model"`
	Endpoint                  string  `json:"endpoint"`
	UpstreamHost              string  `json:"upstream_host,omitempty"`
	Requests                  int     `json:"requests"`
	PromptTokens              int     `json:"prompt_tokens"`
	CachedInputTokens         int     `json:"cached_input_tokens"`
	CacheWriteTokens          int     `json:"cache_write_tokens"`
	CompletionTokens          int     `json:"completion_tokens"`
	TotalTokens               int     `json:"total_tokens"`
	AvgLatencyMS              float64 `json:"avg_latency_ms"`
	NotBilled                 bool    `json:"not_billed"`
	Provider                  string  `json:"provider,omitempty"`
	UsageMissing              bool    `json:"usage_missing"`
	CompressedRequests        int     `json:"compressed_requests"`
	CompressionOriginalTokens int     `json:"compression_original_tokens"`
	CompressionFinalTokens    int     `json:"compression_final_tokens"`
	CompressionRemovedTokens  int     `json:"compression_removed_tokens"`
	AvgCompressionRatio       float64 `json:"avg_compression_ratio"`
}

type TimelineBucket struct {
	Date             string  `json:"date"`
	Hour             int     `json:"hour,omitempty"`
	Model            string  `json:"model"`
	UpstreamHost     string  `json:"upstream_host,omitempty"`
	Requests         int     `json:"requests"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost"`
}

type ExportRow struct {
	Timestamp                 string `json:"ts"`
	Endpoint                  string `json:"endpoint"`
	Model                     string `json:"model"`
	Status                    int    `json:"status"`
	LatencyMS                 int64  `json:"latency_ms"`
	PromptTokens              int    `json:"prompt_tokens"`
	CachedInputTokens         int    `json:"cached_input_tokens"`
	CacheWriteTokens          int    `json:"cache_write_tokens"`
	CompletionTokens          int    `json:"completion_tokens"`
	TotalTokens               int    `json:"total_tokens"`
	Project                   string `json:"project"`
	CompressionStatus         string `json:"compression_status,omitempty"`
	CompressionOriginalTokens int    `json:"compression_original_tokens,omitempty"`
	CompressionFinalTokens    int    `json:"compression_final_tokens,omitempty"`
	CompressionLatencyMS      int64  `json:"compression_latency_ms,omitempty"`
}

// DBPath returns the path the store was opened with.
func (s *Store) DBPath() string {
	if s == nil {
		return DefaultPath()
	}
	return s.path
}

func DefaultPath() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "llm-proxy", "store.db")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", "llm-proxy.db")
	}
	return filepath.Join(home, ".local", "share", "llm-proxy", "store.db")
}

func Open(path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db, path: path}
	if err := store.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) init(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("nil store")
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		return err
	}
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, string(schema))
	if err != nil {
		return err
	}
	// Migration: add columns that may not exist in older schemas
	for _, m := range []struct{ name, def string }{
		{"usage_missing", "INTEGER NOT NULL DEFAULT 0"},
		{"compression_status", "TEXT"},
		{"compression_original_tokens", "INTEGER"},
		{"compression_final_tokens", "INTEGER"},
		{"compression_latency_ms", "INTEGER"},
	} {
		var count int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('requests') WHERE name = ?", m.name).Scan(&count); err == nil && count == 0 {
			_, _ = s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE requests ADD COLUMN %s %s", m.name, m.def))
		}
	}
	return nil
}

func (s *Store) InsertRequest(ctx context.Context, rec RequestRecord) error {
	if s == nil || s.db == nil {
		return nil
	}
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO requests (
  ts, endpoint, method, path, upstream_host, model, stream, status, error,
  latency_ms, prompt_tokens, cached_input_tokens, cache_write_tokens,
  completion_tokens, total_tokens, project, not_billed, provider, usage_missing,
  compression_status, compression_original_tokens, compression_final_tokens,
  compression_latency_ms
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.Timestamp.UTC().Format(time.RFC3339Nano),
		rec.Endpoint,
		rec.Method,
		rec.Path,
		rec.UpstreamHost,
		nullString(rec.Model),
		boolInt(rec.Stream),
		rec.Status,
		nullString(rec.Error),
		rec.LatencyMS,
		nullInt(rec.PromptTokens),
		nullInt(rec.CachedInputTokens),
		nullInt(rec.CacheWriteTokens),
		nullInt(rec.CompletionTokens),
		nullInt(rec.TotalTokens),
		nullString(rec.Project),
		boolInt(rec.NotBilled),
		rec.Provider,
		boolInt(rec.UsageMissing),
		nullString(rec.CompressionStatus),
		nullInt(rec.CompressionOriginalTokens),
		nullInt(rec.CompressionFinalTokens),
		nullInt64(rec.CompressionLatencyMS),
	)
	return err
}

func (s *Store) Stats(ctx context.Context, filter StatsFilter) ([]ModelStats, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("nil store")
	}
	query := `
SELECT
  COALESCE(NULLIF(model, ''), '<unknown>') AS model,
  endpoint,
  upstream_host,
  COUNT(*) AS requests,
  COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
  COALESCE(SUM(cached_input_tokens), 0) AS cached_input_tokens,
  COALESCE(SUM(cache_write_tokens), 0) AS cache_write_tokens,
  COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
  COALESCE(SUM(total_tokens), 0) AS total_tokens,
  COALESCE(AVG(latency_ms), 0) AS avg_latency_ms,
  MAX(not_billed) AS not_billed,
  COALESCE(NULLIF(MAX(provider), ''), '') AS provider,
  MAX(usage_missing) AS usage_missing,
  COUNT(CASE WHEN compression_status IN ('applied', 'no_change') THEN 1 END) AS compressed_requests,
  COALESCE(SUM(compression_original_tokens), 0) AS compression_original_tokens,
  COALESCE(SUM(compression_final_tokens), 0) AS compression_final_tokens,
  COALESCE(SUM(compression_original_tokens - compression_final_tokens), 0) AS compression_removed_tokens,
  COALESCE(AVG(NULLIF(compression_final_tokens, 0) * 1.0 / compression_original_tokens), 0) AS avg_compression_ratio
FROM requests
WHERE (? = '' OR ts >= ?)
  AND (? = '' OR ts < ?)
  AND (? = '' OR project = ?)
  AND (? = '' OR endpoint = ?)
  AND (? = '' OR upstream_host = ?)
GROUP BY model, endpoint, upstream_host
ORDER BY total_tokens DESC, requests DESC, model ASC, endpoint ASC`
	since := ""
	if !filter.Since.IsZero() {
		since = filter.Since.UTC().Format(time.RFC3339Nano)
	}
	until := ""
	if !filter.Until.IsZero() {
		until = filter.Until.UTC().Format(time.RFC3339Nano)
	}
	return s.queryModelStats(ctx, query, since, since, until, until, filter.Project, filter.Project, filter.Endpoint, filter.Endpoint, filter.UpstreamHost, filter.UpstreamHost)
}

func (s *Store) queryModelStats(ctx context.Context, query string, args ...any) ([]ModelStats, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ModelStats
	for rows.Next() {
		var row ModelStats
		var notBilled int
		var usageMissing int
		if err := rows.Scan(&row.Model, &row.Endpoint, &row.UpstreamHost, &row.Requests, &row.PromptTokens, &row.CachedInputTokens, &row.CacheWriteTokens, &row.CompletionTokens, &row.TotalTokens, &row.AvgLatencyMS, &notBilled, &row.Provider, &usageMissing, &row.CompressedRequests, &row.CompressionOriginalTokens, &row.CompressionFinalTokens, &row.CompressionRemovedTokens, &row.AvgCompressionRatio); err != nil {
			return nil, err
		}
		row.NotBilled = notBilled != 0
		row.UsageMissing = usageMissing != 0
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) Timeline(ctx context.Context, filter StatsFilter, granularity string) ([]TimelineBucket, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("nil store")
	}
	since := ""
	if !filter.Since.IsZero() {
		since = filter.Since.UTC().Format(time.RFC3339Nano)
	}
	until := ""
	if !filter.Until.IsZero() {
		until = filter.Until.UTC().Format(time.RFC3339Nano)
	}

	var groupExpr, dateExpr string
	switch granularity {
	case "hour":
		groupExpr = "strftime('%Y-%m-%d', ts), strftime('%H', ts)"
		dateExpr = "strftime('%Y-%m-%d', ts) AS date, CAST(strftime('%H', ts) AS INTEGER) AS hour"
	default:
		groupExpr = "strftime('%Y-%m-%d', ts)"
		dateExpr = "strftime('%Y-%m-%d', ts) AS date, 0 AS hour"
	}

	query := fmt.Sprintf(`
SELECT
  %s,
  COALESCE(NULLIF(model, ''), '<unknown>') AS model,
  upstream_host,
  COUNT(*) AS requests,
  COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
  COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
  COALESCE(SUM(total_tokens), 0) AS total_tokens
FROM requests
WHERE (? = '' OR ts >= ?)
  AND (? = '' OR ts < ?)
  AND (? = '' OR project = ?)
  AND (? = '' OR endpoint = ?)
  AND (? = '' OR upstream_host = ?)
  AND model IS NOT NULL AND model != ''
GROUP BY %s, model, upstream_host
ORDER BY date ASC, hour ASC, model ASC, upstream_host ASC`, dateExpr, groupExpr)

	rows, err := s.db.QueryContext(ctx, query, since, since, until, until, filter.Project, filter.Project, filter.Endpoint, filter.Endpoint, filter.UpstreamHost, filter.UpstreamHost)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TimelineBucket
	for rows.Next() {
		var row TimelineBucket
		if err := rows.Scan(&row.Date, &row.Hour, &row.Model, &row.UpstreamHost, &row.Requests, &row.PromptTokens, &row.CompletionTokens, &row.TotalTokens); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ExportRequests(ctx context.Context, since, until time.Time, upstreamHost string) ([]ExportRow, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("nil store")
	}
	sinceStr := ""
	if !since.IsZero() {
		sinceStr = since.UTC().Format(time.RFC3339Nano)
	}
	untilStr := ""
	if !until.IsZero() {
		untilStr = until.UTC().Format(time.RFC3339Nano)
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT ts, endpoint, COALESCE(model,''), status, latency_ms,
  COALESCE(prompt_tokens,0), COALESCE(cached_input_tokens,0), COALESCE(cache_write_tokens,0),
  COALESCE(completion_tokens,0), COALESCE(total_tokens,0), COALESCE(project,''),
  COALESCE(compression_status,''), COALESCE(compression_original_tokens,0), COALESCE(compression_final_tokens,0), COALESCE(compression_latency_ms,0)
FROM requests
WHERE (? = '' OR ts >= ?)
  AND (? = '' OR ts < ?)
  AND (? = '' OR upstream_host = ?)
  AND model IS NOT NULL AND model != ''
  AND status = 200
ORDER BY ts DESC`, sinceStr, sinceStr, untilStr, untilStr, upstreamHost, upstreamHost)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ExportRow
	for rows.Next() {
		var row ExportRow
		if err := rows.Scan(&row.Timestamp, &row.Endpoint, &row.Model, &row.Status, &row.LatencyMS,
			&row.PromptTokens, &row.CachedInputTokens, &row.CacheWriteTokens,
			&row.CompletionTokens, &row.TotalTokens, &row.Project,
			&row.CompressionStatus, &row.CompressionOriginalTokens, &row.CompressionFinalTokens, &row.CompressionLatencyMS); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) DistinctUpstreamHosts(ctx context.Context) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("nil store")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT upstream_host FROM requests WHERE upstream_host != '' ORDER BY upstream_host`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts = make([]string, 0)
	for rows.Next() {
		var host string
		if err := rows.Scan(&host); err != nil {
			return nil, err
		}
		hosts = append(hosts, host)
	}
	return hosts, rows.Err()
}

// GetPolicy returns the current policy. Returns DefaultPolicy() if no row exists.
func (s *Store) GetPolicy(ctx context.Context) (*policy.Policy, error) {
	if s == nil || s.db == nil {
		return policy.DefaultPolicy(), nil
	}
	var mode string
	var modelsJSON string
	err := s.db.QueryRowContext(ctx,
		"SELECT mode, models_json FROM policies WHERE id = 1",
	).Scan(&mode, &modelsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return policy.DefaultPolicy(), nil
	}
	if err != nil {
		return nil, err
	}
	var models []string
	if err := json.Unmarshal([]byte(modelsJSON), &models); err != nil {
		return nil, fmt.Errorf("unmarshal policy models_json: %w", err)
	}
	return &policy.Policy{Mode: policy.Mode(mode), Models: models}, nil
}

// SetPolicy atomically replaces the current policy.
func (s *Store) SetPolicy(ctx context.Context, p *policy.Policy) error {
	if s == nil || s.db == nil {
		return errors.New("nil store")
	}
	modelsJSON, err := json.Marshal(p.Models)
	if err != nil {
		return fmt.Errorf("marshal policy models: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO policies (id, mode, models_json) VALUES (1, ?, ?)",
		string(p.Mode), string(modelsJSON),
	)
	return err
}

// DistinctModels returns all unique model names from the requests table.
func (s *Store) DistinctModels(ctx context.Context) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT DISTINCT model FROM requests WHERE model IS NOT NULL AND model != '' ORDER BY model",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func nullInt64(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

func (s *Store) CountUsageMissing(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM requests WHERE usage_missing = 1").Scan(&count)
	return count, err
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func FormatPath(path string) string {
	if path == "" {
		return DefaultPath()
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
