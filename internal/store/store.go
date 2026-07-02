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
	Project  string
	Endpoint string
}

type ModelStats struct {
	Model             string `json:"model"`
	Endpoint          string `json:"endpoint"`
	Requests          int    `json:"requests"`
	PromptTokens      int    `json:"prompt_tokens"`
	CachedInputTokens int    `json:"cached_input_tokens"`
	CacheWriteTokens  int    `json:"cache_write_tokens"`
	CompletionTokens  int    `json:"completion_tokens"`
	TotalTokens       int    `json:"total_tokens"`
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
  COALESCE(SUM(total_tokens), 0) AS total_tokens
FROM requests
WHERE (? = '' OR ts >= ?)
  AND (? = '' OR project = ?)
  AND (? = '' OR endpoint = ?)
GROUP BY model, endpoint
ORDER BY total_tokens DESC, requests DESC, model ASC, endpoint ASC`
	since := ""
	if !filter.Since.IsZero() {
		since = filter.Since.UTC().Format(time.RFC3339Nano)
	}
	rows, err := s.db.QueryContext(ctx, query, since, since, filter.Project, filter.Project, filter.Endpoint, filter.Endpoint)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ModelStats
	for rows.Next() {
		var row ModelStats
		if err := rows.Scan(&row.Model, &row.Endpoint, &row.Requests, &row.PromptTokens, &row.CachedInputTokens, &row.CacheWriteTokens, &row.CompletionTokens, &row.TotalTokens); err != nil {
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
		return fmt.Sprintf("%s", path)
	}
	return abs
}
