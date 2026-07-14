package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SessionFilter struct {
	Since           time.Time
	Until           time.Time
	Project         string
	Limit           int
	CursorStartedAt time.Time
	CursorID        int64
}

type SessionStats struct {
	ID           int64     `json:"id"`
	StartedAt    time.Time `json:"started_at"`
	EndedAt      time.Time `json:"ended_at"`
	Project      string    `json:"project"`
	RequestCount int       `json:"request_count"`
	TokenCount   int       `json:"token_count"`
	Cost         float64   `json:"cost"`
}

type CurrentSession struct {
	ID            int64        `json:"id"`
	StartedAt     time.Time    `json:"started_at"`
	LastRequestAt time.Time    `json:"last_request_at"`
	Project       string       `json:"project"`
	RequestCount  int          `json:"request_count"`
	TokenCount    int          `json:"token_count"`
	Status        string       `json:"status"`
	Active        bool         `json:"active"`
	Models        []ModelStats `json:"-"`
}

type sessionRequest struct {
	ID         int64
	Timestamp  time.Time
	Project    string
	TokenCount int
}

const defaultSessionGap = 30 * time.Minute

// assignSession finds the latest session and either adds rec to it or creates a
// new one. It must be called from the same transaction that inserts the request.
func (s *Store) assignSession(ctx context.Context, tx *sql.Tx, rec RequestRecord) (int64, error) {
	var (
		id      int64
		endedAt string
		project string
	)
	err := tx.QueryRowContext(ctx, `
SELECT id, ended_at, COALESCE(project, '')
FROM sessions
WHERE ended_at = (SELECT MAX(ended_at) FROM sessions)
ORDER BY id DESC
LIMIT 1`).Scan(&id, &endedAt, &project)
	if errors.Is(err, sql.ErrNoRows) {
		result, err := tx.ExecContext(ctx, `
INSERT INTO sessions (started_at, ended_at, project, request_count, token_count)
VALUES (?, ?, ?, 1, ?)`,
			rec.Timestamp.UTC().Format(time.RFC3339Nano),
			rec.Timestamp.UTC().Format(time.RFC3339Nano),
			nullString(rec.Project), rec.TotalTokens)
		if err != nil {
			return 0, err
		}
		return result.LastInsertId()
	}
	if err != nil {
		return 0, err
	}

	ended, err := time.Parse(time.RFC3339Nano, endedAt)
	if err != nil {
		return 0, err
	}
	gap := rec.Timestamp.Sub(ended)
	if gap < 0 {
		gap = -gap
	}
	if gap >= defaultSessionGap {
		result, err := tx.ExecContext(ctx, `
INSERT INTO sessions (started_at, ended_at, project, request_count, token_count)
VALUES (?, ?, ?, 1, ?)`,
			rec.Timestamp.UTC().Format(time.RFC3339Nano),
			rec.Timestamp.UTC().Format(time.RFC3339Nano),
			nullString(rec.Project), rec.TotalTokens)
		if err != nil {
			return 0, err
		}
		return result.LastInsertId()
	}

	projectValue := project
	if projectValue != rec.Project {
		projectValue = "<mixed>"
	}
	ts := rec.Timestamp.UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `
UPDATE sessions
SET started_at = CASE WHEN started_at > ? THEN ? ELSE started_at END,
    ended_at = CASE WHEN ended_at < ? THEN ? ELSE ended_at END,
    project = ?,
    request_count = request_count + 1,
    token_count = token_count + ?
WHERE id = ?`, ts, ts, ts, ts, nullString(projectValue), rec.TotalTokens, id); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) RebuildSessions(ctx context.Context, gap time.Duration) error {
	if s == nil || s.db == nil {
		return errors.New("nil store")
	}
	if gap <= 0 {
		return errors.New("gap must be positive")
	}

	s.rebuildMu.Lock()
	defer s.rebuildMu.Unlock()

	requests, err := s.sessionRequests(ctx)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE requests SET session_id = NULL`); err != nil {
		return err
	}

	for len(requests) > 0 {
		current, rest := takeSession(requests, gap)
		requests = rest
		if err := insertSession(ctx, tx, current); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// Vacuum compacts the SQLite database. It is intentionally only exposed for
// explicit maintenance commands because it takes an exclusive database lock.
func (s *Store) Vacuum(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("nil store")
	}
	_, err := s.db.ExecContext(ctx, "VACUUM")
	return err
}

func (s *Store) sessionRequests(ctx context.Context) ([]sessionRequest, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, ts, COALESCE(project, ''), COALESCE(total_tokens, 0)
FROM requests
ORDER BY ts ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []sessionRequest
	for rows.Next() {
		var req sessionRequest
		var ts string
		if err := rows.Scan(&req.ID, &ts, &req.Project, &req.TokenCount); err != nil {
			return nil, err
		}
		parsed, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return nil, err
		}
		req.Timestamp = parsed
		out = append(out, req)
	}
	return out, rows.Err()
}

func takeSession(requests []sessionRequest, gap time.Duration) ([]sessionRequest, []sessionRequest) {
	if len(requests) == 0 {
		return nil, nil
	}
	end := 1
	for end < len(requests) {
		if requests[end].Timestamp.Sub(requests[end-1].Timestamp) >= gap {
			break
		}
		end++
	}
	return requests[:end], requests[end:]
}

func insertSession(ctx context.Context, tx *sql.Tx, requests []sessionRequest) error {
	if len(requests) == 0 {
		return nil
	}
	started := requests[0].Timestamp.UTC()
	ended := requests[len(requests)-1].Timestamp.UTC()
	project := sessionProject(requests)
	requestCount := len(requests)
	tokenCount := 0
	for _, req := range requests {
		tokenCount += req.TokenCount
	}

	result, err := tx.ExecContext(ctx, `
INSERT INTO sessions (started_at, ended_at, project, request_count, token_count)
VALUES (?, ?, ?, ?, ?)`,
		started.Format(time.RFC3339Nano),
		ended.Format(time.RFC3339Nano),
		nullString(project),
		requestCount,
		tokenCount,
	)
	if err != nil {
		return err
	}
	sessionID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	for _, req := range requests {
		if _, err := tx.ExecContext(ctx, `UPDATE requests SET session_id = ? WHERE id = ?`, sessionID, req.ID); err != nil {
			return err
		}
	}
	return nil
}

func sessionProject(requests []sessionRequest) string {
	if len(requests) == 0 {
		return ""
	}
	project := requests[0].Project
	for _, req := range requests[1:] {
		if req.Project != project {
			return "<mixed>"
		}
	}
	return project
}

func (s *Store) CurrentSession(ctx context.Context) (*CurrentSession, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("nil store")
	}
	sessions, err := s.Sessions(ctx, SessionFilter{Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, nil
	}
	latest := sessions[0]
	models, err := s.sessionModelStats(ctx, latest.ID)
	if err != nil {
		return nil, err
	}
	active := latest.EndedAt.UTC().After(time.Now().UTC().Add(-30 * time.Minute))
	status := "idle"
	if active {
		status = "active"
	}
	return &CurrentSession{
		ID:            latest.ID,
		StartedAt:     latest.StartedAt,
		LastRequestAt: latest.EndedAt,
		Project:       latest.Project,
		RequestCount:  latest.RequestCount,
		TokenCount:    latest.TokenCount,
		Status:        status,
		Active:        active,
		Models:        models,
	}, nil
}

func (s *Store) SessionModels(ctx context.Context, sessionID int64) ([]ModelStats, error) {
	return s.sessionModelStats(ctx, sessionID)
}

// SessionModelsBatch returns model statistics for all requested sessions in one
// query, avoiding one query per row in session lists.
func (s *Store) SessionModelsBatch(ctx context.Context, sessionIDs []int64) (map[int64][]ModelStats, error) {
	result := make(map[int64][]ModelStats, len(sessionIDs))
	if s == nil || s.db == nil {
		return nil, errors.New("nil store")
	}
	if len(sessionIDs) == 0 {
		return result, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(sessionIDs)), ",")
	args := make([]any, len(sessionIDs))
	for i, id := range sessionIDs {
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
SELECT
  session_id,
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
  COUNT(CASE WHEN compression_status = 'applied' THEN 1 END) AS compressed_requests,
  COALESCE(SUM(CASE WHEN compression_status = 'applied' THEN compression_original_tokens END), 0) AS compression_original_tokens,
  COALESCE(SUM(CASE WHEN compression_status = 'applied' THEN compression_final_tokens END), 0) AS compression_final_tokens,
  COALESCE(SUM(CASE WHEN compression_status = 'applied' THEN compression_original_tokens - compression_final_tokens END), 0) AS compression_removed_tokens,
  COALESCE(AVG(CASE WHEN compression_status = 'applied' THEN NULLIF(compression_final_tokens, 0) * 1.0 / NULLIF(compression_original_tokens, 0) END), 0) AS avg_compression_ratio
FROM requests
WHERE session_id IN (%s)
GROUP BY session_id, model, endpoint, upstream_host
ORDER BY session_id, total_tokens DESC, requests DESC, model ASC, endpoint ASC`, placeholders), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sessionID int64
		var row ModelStats
		var notBilled, usageMissing int
		if err := rows.Scan(&sessionID, &row.Model, &row.Endpoint, &row.UpstreamHost, &row.Requests, &row.PromptTokens, &row.CachedInputTokens, &row.CacheWriteTokens, &row.CompletionTokens, &row.TotalTokens, &row.AvgLatencyMS, &notBilled, &row.Provider, &usageMissing, &row.CompressedRequests, &row.CompressionOriginalTokens, &row.CompressionFinalTokens, &row.CompressionRemovedTokens, &row.AvgCompressionRatio); err != nil {
			return nil, err
		}
		row.NotBilled = notBilled != 0
		row.UsageMissing = usageMissing != 0
		result[sessionID] = append(result[sessionID], row)
	}
	return result, rows.Err()
}

func (s *Store) sessionModelStats(ctx context.Context, sessionID int64) ([]ModelStats, error) {
	return s.queryModelStats(ctx, `
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
  COUNT(CASE WHEN compression_status = 'applied' THEN 1 END) AS compressed_requests,
  COALESCE(SUM(CASE WHEN compression_status = 'applied' THEN compression_original_tokens END), 0) AS compression_original_tokens,
  COALESCE(SUM(CASE WHEN compression_status = 'applied' THEN compression_final_tokens END), 0) AS compression_final_tokens,
  COALESCE(SUM(CASE WHEN compression_status = 'applied' THEN compression_original_tokens - compression_final_tokens END), 0) AS compression_removed_tokens,
  COALESCE(AVG(CASE WHEN compression_status = 'applied' THEN NULLIF(compression_final_tokens, 0) * 1.0 / NULLIF(compression_original_tokens, 0) END), 0) AS avg_compression_ratio
FROM requests
WHERE session_id = ?
GROUP BY model, endpoint, upstream_host
ORDER BY total_tokens DESC, requests DESC, model ASC, endpoint ASC`, sessionID)
}

func (s *Store) Sessions(ctx context.Context, filter SessionFilter) ([]SessionStats, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("nil store")
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	since := ""
	if !filter.Since.IsZero() {
		since = filter.Since.UTC().Format(time.RFC3339Nano)
	}
	until := ""
	if !filter.Until.IsZero() {
		until = filter.Until.UTC().Format(time.RFC3339Nano)
	}
	cursorStarted := ""
	if !filter.CursorStartedAt.IsZero() {
		cursorStarted = filter.CursorStartedAt.UTC().Format(time.RFC3339Nano)
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, started_at, ended_at, COALESCE(project, ''), request_count, token_count
FROM sessions
WHERE (? = '' OR started_at >= ?)
  AND (? = '' OR started_at < ?)
  AND (? = '' OR project = ?)
  AND (? = '' OR started_at < ? OR (started_at = ? AND id < ?))
ORDER BY started_at DESC, id DESC
LIMIT ?`, since, since, until, until, filter.Project, filter.Project,
		cursorStarted, cursorStarted, cursorStarted, filter.CursorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SessionStats
	for rows.Next() {
		var row SessionStats
		var started, ended string
		if err := rows.Scan(&row.ID, &started, &ended, &row.Project, &row.RequestCount, &row.TokenCount); err != nil {
			return nil, err
		}
		parsedStarted, err := time.Parse(time.RFC3339Nano, started)
		if err != nil {
			return nil, err
		}
		parsedEnded, err := time.Parse(time.RFC3339Nano, ended)
		if err != nil {
			return nil, err
		}
		row.StartedAt = parsedStarted
		row.EndedAt = parsedEnded
		out = append(out, row)
	}
	return out, rows.Err()
}

// CountSessions returns the total number of sessions, optionally limited by a
// SessionFilter. The optional form preserves callers that need the global count.
func (s *Store) CountSessions(ctx context.Context, filters ...SessionFilter) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("nil store")
	}
	filter := SessionFilter{}
	if len(filters) > 0 {
		filter = filters[0]
	}
	since, until := "", ""
	if !filter.Since.IsZero() {
		since = filter.Since.UTC().Format(time.RFC3339Nano)
	}
	if !filter.Until.IsZero() {
		until = filter.Until.UTC().Format(time.RFC3339Nano)
	}
	var count int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sessions
WHERE (? = '' OR started_at >= ?)
  AND (? = '' OR started_at < ?)
  AND (? = '' OR project = ?)`,
		since, since, until, until, filter.Project, filter.Project).Scan(&count)
	return count, err
}

// DistinctProjects returns recorded non-empty session project names.
func (s *Store) DistinctProjects(ctx context.Context) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("nil store")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT project FROM sessions
WHERE project IS NOT NULL AND project != ''
ORDER BY project`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := make([]string, 0)
	for rows.Next() {
		var project string
		if err := rows.Scan(&project); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}
