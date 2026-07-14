<!-- markdownlint-disable MD041 -->

## ADDED Requirements

### Requirement: Incremental session assignment

The system SHALL assign a `session_id` to each captured request at insertion
time, grouping consecutive requests from the same project where the gap between
adjacent timestamps does not exceed 30 minutes. Session summary fields
(`request_count`, `token_count`, `ended_at`, `project`) SHALL be updated
atomically within the same write transaction as the request insertion.

#### Scenario: New request within active session window

- **WHEN** a request is captured and its timestamp is within 30 minutes of the
  most recent session's `ended_at`
- **THEN** the request is assigned that session's id, and the session's
  `request_count`, `token_count`, and `ended_at` are updated

#### Scenario: New request after active session window

- **WHEN** a request is captured and its timestamp is more than 30 minutes after
  the most recent session's `ended_at`
- **THEN** a new session is created with the request, starting at the request's
  timestamp

#### Scenario: First request in a fresh database

- **WHEN** a request is captured and no sessions exist
- **THEN** a new session is created for that request

#### Scenario: Out-of-order request with late timestamp

- **WHEN** a request is captured with a timestamp older than the most recent
  session's `ended_at` but within 30 minutes of that session
- **THEN** the request is still assigned to that session, and `token_count` and
  `request_count` are updated (even if `ended_at` does not advance)

---

### Requirement: Session summary reads without full rebuild

API endpoints that return session data (`/api/sessions`, `/api/session/current`,
CLI `sessions`, CLI `live`) SHALL read directly from the pre-built `sessions`
table without triggering a full-table-scan rebuild of sessions.

#### Scenario: Dashboard sessions endpoint

- **WHEN** the dashboard calls `/api/sessions`
- **THEN** sessions are returned from the `sessions` table without re-reading
  the `requests` table

#### Scenario: Current session endpoint

- **WHEN** the dashboard calls `/api/session/current`
- **THEN** the current session is returned from the `sessions` table without
  re-reading the `requests` table

#### Scenario: CLI sessions command

- **WHEN** the user runs `copilot-monitor sessions`
- **THEN** sessions are read from the `sessions` table without triggering a
  rebuild

---

### Requirement: Offline session rebuild command

The system SHALL provide a standalone `rebuild-sessions` CLI subcommand that
performs a full reconstruction of the `sessions` table from the `requests`
table, for use after schema migrations or as a repair mechanism.

#### Scenario: Rebuild command

- **WHEN** the user runs `copilot-monitor rebuild-sessions`
- **THEN** the `sessions` table is cleared and rebuilt from all `requests`
  ordered by timestamp, and all `session_id` foreign keys in `requests` are
  updated

#### Scenario: Rebuild with custom gap

- **WHEN** the user runs `copilot-monitor rebuild-sessions --gap 1h`
- **THEN** sessions are split on a 1-hour gap instead of the default 30 minutes

---

### Requirement: Session write atomicity

Session summary updates SHALL be transactional: the request INSERT and the
session UPDATE SHALL be committed together or not at all.

#### Scenario: Transactional failure

- **WHEN** a request insert succeeds but the session update fails within the
  same transaction
- **THEN** the entire transaction is rolled back and neither the request nor the
  session update persists
