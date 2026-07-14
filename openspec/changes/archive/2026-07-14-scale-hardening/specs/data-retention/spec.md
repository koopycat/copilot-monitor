## ADDED Requirements

### Requirement: Configurable retention period

The system SHALL support a `--retention-days` flag on the `run` and `serve`
commands that limits how long captured data is kept. The default SHALL be 365
days.

#### Scenario: Retention flag set

- **WHEN** `copilot-monitor run --retention-days 90` is executed
- **THEN** on startup and every 24 hours thereafter, requests, sessions, and
  anomalies older than 90 days are pruned

#### Scenario: Retention flag default

- **WHEN** `copilot-monitor run` is executed without `--retention-days`
- **THEN** data older than 365 days is pruned

---

### Requirement: Safe batched pruning

Pruning SHALL delete old data in batches to avoid holding a long-running write
lock. Each batch SHALL delete no more than 1000 rows per table per iteration.

#### Scenario: Large dataset prune

- **WHEN** the retention check runs on a database with 500,000 rows to delete
- **THEN** deletes are executed in batches of 1000, with a brief yield between
  batches, so proxy write operations are not blocked for more than a single
  batch duration

---

### Requirement: Pruning boundary handling

When a session contains requests that span the retention boundary, the session
and its remaining requests SHALL be preserved intact.

#### Scenario: Session crossing retention boundary

- **WHEN** a session has some requests older than the retention cutoff and some
  newer
- **THEN** the session and all its requests are retained (no partial session
  deletion)

---

### Requirement: Manual VACUUM only

The system SHALL NOT automatically execute `VACUUM` after pruning. A manual
`copilot-monitor rebuild-sessions --vacuum` flag SHALL be available for users
who want to reclaim disk space.

#### Scenario: No automatic VACUUM

- **WHEN** the daily retention prune completes
- **THEN** the database file size is unchanged until the user manually runs a
  compaction command

#### Scenario: Manual VACUUM

- **WHEN** the user runs `copilot-monitor rebuild-sessions --vacuum`
- **THEN** the sessions table is rebuilt and a VACUUM is executed to compact the
  database file

---

### Requirement: Retention status in health

The `/api/health` endpoint SHALL include retention configuration and the
timestamp of the last successful prune.

#### Scenario: Health includes retention info

- **WHEN** a GET request is made to `/api/health`
- **THEN** the response includes `retention_days`, `last_prune_at`, and
  `pruned_count` fields
