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
	ID           int64
	StartedAt    time.Time
	EndedAt      time.Time
	Project      string
	RequestCount int
	TokenCount   int
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
