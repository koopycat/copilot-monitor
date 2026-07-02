package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

type Store struct {
	db *sql.DB
}

type RequestRecord struct {
	Timestamp         time.Time
	Endpoint          string
	Method            string
	Path              string
	UpstreamHost      string
	Model             string
	Stream            bool
	Status            int
	Error             string
	LatencyMS         int64
	PromptTokens      int
	CachedInputTokens int
	CacheWriteTokens  int
	CompletionTokens  int
	TotalTokens       int
	Project           string
	RequestHash       string
}

type StatsFilter struct {
	Since    time.Time
	Until    time.Time
	Project  string
	Endpoint string
}

type ModelStats struct {
	Model             string  `json:"model"`
	Endpoint          string  `json:"endpoint"`
	Requests          int     `json:"requests"`
	PromptTokens      int     `json:"prompt_tokens"`
	CachedInputTokens int     `json:"cached_input_tokens"`
	CacheWriteTokens  int     `json:"cache_write_tokens"`
	CompletionTokens  int     `json:"completion_tokens"`
	TotalTokens       int     `json:"total_tokens"`
	AvgLatencyMS      float64 `json:"avg_latency_ms"`
}

type TimelineBucket struct {
	Date             string `json:"date"`
	Hour             int    `json:"hour,omitempty"`
	Model            string `json:"model"`
	Requests         int    `json:"requests"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
}

type CompareResult struct {
	Periods []ComparePeriod `json:"periods"`
}

type ComparePeriod struct {
	Label       string       `json:"label"`
	Start       time.Time    `json:"start"`
	End         time.Time    `json:"end"`
	Models      []ModelStats `json:"models"`
	Requests    int          `json:"requests"`
	TotalTokens int          `json:"total_tokens"`
}

type ExportRow struct {
	Timestamp         string `json:"ts"`
	Endpoint          string `json:"endpoint"`
	Model             string `json:"model"`
	Status            int    `json:"status"`
	LatencyMS         int64  `json:"latency_ms"`
	PromptTokens      int    `json:"prompt_tokens"`
	CachedInputTokens int    `json:"cached_input_tokens"`
	CacheWriteTokens  int    `json:"cache_write_tokens"`
	CompletionTokens  int    `json:"completion_tokens"`
	TotalTokens       int    `json:"total_tokens"`
	Project           string `json:"project"`
}

func DefaultPath() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "copilot-monitor", "store.db")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", "copilot-monitor.db")
	}
	return filepath.Join(home, ".local", "share", "copilot-monitor", "store.db")
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
	store := &Store{db: db}
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
	return err
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
  completion_tokens, total_tokens, project, request_hash
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
		nullString(rec.RequestHash),
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
  COUNT(*) AS requests,
  COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
  COALESCE(SUM(cached_input_tokens), 0) AS cached_input_tokens,
  COALESCE(SUM(cache_write_tokens), 0) AS cache_write_tokens,
  COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
  COALESCE(SUM(total_tokens), 0) AS total_tokens,
  COALESCE(AVG(latency_ms), 0) AS avg_latency_ms
FROM requests
WHERE (? = '' OR ts >= ?)
  AND (? = '' OR ts < ?)
  AND (? = '' OR project = ?)
  AND (? = '' OR endpoint = ?)
GROUP BY model, endpoint
ORDER BY total_tokens DESC, requests DESC, model ASC, endpoint ASC`
	since := ""
	if !filter.Since.IsZero() {
		since = filter.Since.UTC().Format(time.RFC3339Nano)
	}
	until := ""
	if !filter.Until.IsZero() {
		until = filter.Until.UTC().Format(time.RFC3339Nano)
	}
	return s.queryModelStats(ctx, query, since, since, until, until, filter.Project, filter.Project, filter.Endpoint, filter.Endpoint)
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
		if err := rows.Scan(&row.Model, &row.Endpoint, &row.Requests, &row.PromptTokens, &row.CachedInputTokens, &row.CacheWriteTokens, &row.CompletionTokens, &row.TotalTokens, &row.AvgLatencyMS); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) CompareStats(ctx context.Context, aStart, aEnd, bStart, bEnd time.Time) (CompareResult, error) {
	if s == nil || s.db == nil {
		return CompareResult{}, errors.New("nil store")
	}
	a, err := s.statsPeriod(ctx, aStart, aEnd)
	if err != nil {
		return CompareResult{}, err
	}
	b, err := s.statsPeriod(ctx, bStart, bEnd)
	if err != nil {
		return CompareResult{}, err
	}
	return CompareResult{Periods: []ComparePeriod{a, b}}, nil
}

func (s *Store) statsPeriod(ctx context.Context, start, end time.Time) (ComparePeriod, error) {
	models, err := s.Stats(ctx, StatsFilter{Since: start, Until: end})
	if err != nil {
		return ComparePeriod{}, err
	}
	if models == nil {
		models = []ModelStats{}
	}
	period := ComparePeriod{
		Label:  start.UTC().Format("2006-01"),
		Start:  start.UTC(),
		End:    end.UTC(),
		Models: models,
	}
	for _, row := range models {
		period.Requests += row.Requests
		period.TotalTokens += row.TotalTokens
	}
	return period, nil
}

func (s *Store) Timeline(ctx context.Context, filter StatsFilter, granularity string) ([]TimelineBucket, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("nil store")
	}
	since := ""
	if !filter.Since.IsZero() {
		since = filter.Since.UTC().Format(time.RFC3339Nano)
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
  COUNT(*) AS requests,
  COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
  COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
  COALESCE(SUM(total_tokens), 0) AS total_tokens
FROM requests
WHERE (? = '' OR ts >= ?)
  AND (? = '' OR project = ?)
  AND (? = '' OR endpoint = ?)
  AND model IS NOT NULL AND model != ''
GROUP BY %s, model
ORDER BY date ASC, hour ASC, model ASC`, dateExpr, groupExpr)

	rows, err := s.db.QueryContext(ctx, query, since, since, filter.Project, filter.Project, filter.Endpoint, filter.Endpoint)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TimelineBucket
	for rows.Next() {
		var row TimelineBucket
		if err := rows.Scan(&row.Date, &row.Hour, &row.Model, &row.Requests, &row.PromptTokens, &row.CompletionTokens, &row.TotalTokens); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ExportRequests(ctx context.Context, since time.Time) ([]ExportRow, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("nil store")
	}
	sinceStr := ""
	if !since.IsZero() {
		sinceStr = since.UTC().Format(time.RFC3339Nano)
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT ts, endpoint, COALESCE(model,''), status, latency_ms,
  COALESCE(prompt_tokens,0), COALESCE(cached_input_tokens,0), COALESCE(cache_write_tokens,0),
  COALESCE(completion_tokens,0), COALESCE(total_tokens,0), COALESCE(project,'')
FROM requests
WHERE (? = '' OR ts >= ?)
  AND model IS NOT NULL AND model != ''
  AND status = 200
ORDER BY ts DESC`, sinceStr, sinceStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ExportRow
	for rows.Next() {
		var row ExportRow
		if err := rows.Scan(&row.Timestamp, &row.Endpoint, &row.Model, &row.Status, &row.LatencyMS,
			&row.PromptTokens, &row.CachedInputTokens, &row.CacheWriteTokens,
			&row.CompletionTokens, &row.TotalTokens, &row.Project); err != nil {
			return nil, err
		}
		out = append(out, row)
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
