package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type SessionFilter struct {
	Since   time.Time
	Project string
	Limit   int
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

func (s *Store) RebuildSessions(ctx context.Context, gap time.Duration) error {
	if s == nil || s.db == nil {
		return errors.New("nil store")
	}
	if gap <= 0 {
		return errors.New("gap must be positive")
	}

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
	return tx.Commit()
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

func (s *Store) sessionModelStats(ctx context.Context, sessionID int64) ([]ModelStats, error) {
	return s.queryModelStats(ctx, `
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
WHERE session_id = ?
GROUP BY model, endpoint
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
	rows, err := s.db.QueryContext(ctx, `
SELECT id, started_at, ended_at, COALESCE(project, ''), request_count, token_count
FROM sessions
WHERE (? = '' OR started_at >= ?)
  AND (? = '' OR project = ?)
ORDER BY started_at DESC, id DESC
LIMIT ?`, since, since, filter.Project, filter.Project, limit)
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
