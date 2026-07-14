package store

import (
	"context"
	"errors"
	"runtime"
	"time"
)

const pruneBatchSize = 1000

// PruneCounts describes rows eligible for retention pruning.
type PruneCounts struct {
	Requests  int `json:"requests"`
	Sessions  int `json:"sessions"`
	Anomalies int `json:"anomalies"`
}

func (c PruneCounts) Total() int { return c.Requests + c.Sessions + c.Anomalies }

// RetentionRowCount returns the number of rows managed by retention pruning.
func (s *Store) RetentionRowCount(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("nil store")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `
SELECT (SELECT COUNT(*) FROM requests) +
       (SELECT COUNT(*) FROM sessions) +
       (SELECT COUNT(*) FROM anomalies)`).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// RetentionStatus is process-local retention state exposed by the health API.
type RetentionStatus struct {
	RetentionDays int       `json:"retention_days"`
	LastPruneAt   time.Time `json:"last_prune_at"`
	PrunedCount   int       `json:"pruned_count"`
}

// SetRetentionDays records the configured request retention period.
func (s *Store) SetRetentionDays(days int) {
	if s == nil {
		return
	}
	s.retentionMu.Lock()
	defer s.retentionMu.Unlock()
	s.retention.RetentionDays = days
}

// RecordPrune records the result of a successful retention run.
func (s *Store) RecordPrune(at time.Time, count int) {
	if s == nil {
		return
	}
	s.retentionMu.Lock()
	defer s.retentionMu.Unlock()
	s.retention.LastPruneAt = at.UTC()
	s.retention.PrunedCount = count
}

// RetentionStatus returns a copy of the current process-local retention state.
func (s *Store) RetentionStatus() RetentionStatus {
	if s == nil {
		return RetentionStatus{}
	}
	s.retentionMu.RLock()
	defer s.retentionMu.RUnlock()
	return s.retention
}

// PrunableCounts returns the number of rows that would be removed by request
// and anomaly pruning. A zero cutoff disables the corresponding count.
func (s *Store) PrunableCounts(ctx context.Context, requestBefore, anomalyBefore time.Time) (PruneCounts, error) {
	if s == nil || s.db == nil {
		return PruneCounts{}, errors.New("nil store")
	}
	var counts PruneCounts
	if !requestBefore.IsZero() {
		cutoff := requestBefore.UTC().Format(time.RFC3339Nano)
		// A request in a session is eligible only after the entire session has
		// ended before the cutoff. This preserves sessions crossing the boundary.
		if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM requests AS r
LEFT JOIN sessions AS s ON s.id = r.session_id
WHERE (r.session_id IS NULL AND r.ts < ?)
   OR (s.ended_at < ?)`, cutoff, cutoff).Scan(&counts.Requests); err != nil {
			return PruneCounts{}, err
		}
		if err := s.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sessions WHERE ended_at < ?", cutoff,
		).Scan(&counts.Sessions); err != nil {
			return PruneCounts{}, err
		}
	}
	if !anomalyBefore.IsZero() {
		if err := s.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM anomalies WHERE ts < ?", anomalyBefore.UTC().Format(time.RFC3339Nano),
		).Scan(&counts.Anomalies); err != nil {
			return PruneCounts{}, err
		}
	}
	return counts, nil
}

// PruneRequests deletes requests older than before in batches of at most 1000,
// then removes their now-empty sessions. Sessions which cross before are kept
// intact, including their older requests.
func (s *Store) PruneRequests(ctx context.Context, before time.Time) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("nil store")
	}
	if before.IsZero() {
		return 0, nil
	}
	cutoff := before.UTC().Format(time.RFC3339Nano)
	deleted := 0
	for {
		result, err := s.db.ExecContext(ctx, `
DELETE FROM requests
WHERE id IN (
  SELECT r.id
  FROM requests AS r
  LEFT JOIN sessions AS s ON s.id = r.session_id
  WHERE (r.session_id IS NULL AND r.ts < ?)
     OR s.ended_at < ?
  LIMIT ?
)`, cutoff, cutoff, pruneBatchSize)
		if err != nil {
			return deleted, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return deleted, err
		}
		deleted += int(n)
		if n == 0 {
			break
		}
		runtime.Gosched()
	}

	for {
		result, err := s.db.ExecContext(ctx, `
DELETE FROM sessions
WHERE id IN (
  SELECT id FROM sessions
  WHERE ended_at < ?
  LIMIT ?
)`, cutoff, pruneBatchSize)
		if err != nil {
			return deleted, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return deleted, err
		}
		if n == 0 {
			return deleted, nil
		}
		runtime.Gosched()
	}
}

// PruneAnomalies deletes anomalies older than before in batches of at most
// 1000 rows. A zero cutoff disables anomaly pruning.
func (s *Store) PruneAnomalies(ctx context.Context, before time.Time) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("nil store")
	}
	if before.IsZero() {
		return 0, nil
	}
	cutoff := before.UTC().Format(time.RFC3339Nano)
	deleted := 0
	for {
		result, err := s.db.ExecContext(ctx, `
DELETE FROM anomalies
WHERE id IN (
  SELECT id FROM anomalies WHERE ts < ? LIMIT ?
)`, cutoff, pruneBatchSize)
		if err != nil {
			return deleted, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return deleted, err
		}
		deleted += int(n)
		if n == 0 {
			return deleted, nil
		}
		runtime.Gosched()
	}
}
