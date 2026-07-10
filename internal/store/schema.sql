CREATE TABLE IF NOT EXISTS requests (
  id                          INTEGER PRIMARY KEY,
  ts                          TEXT    NOT NULL,
  endpoint                    TEXT    NOT NULL,
  method                      TEXT    NOT NULL,
  path                        TEXT    NOT NULL,
  upstream_host               TEXT    NOT NULL,
  model                       TEXT,
  stream                      INTEGER NOT NULL,
  status                      INTEGER NOT NULL,
  error                       TEXT,
  latency_ms                  INTEGER NOT NULL,
  prompt_tokens               INTEGER,
  cached_input_tokens         INTEGER,
  cache_write_tokens          INTEGER,
  completion_tokens           INTEGER,
  total_tokens                INTEGER,
  project                     TEXT,
  session_id                  INTEGER,
  compression_status          TEXT,
  compression_original_tokens INTEGER,
  compression_final_tokens    INTEGER,
  compression_latency_ms      INTEGER
);

CREATE TABLE IF NOT EXISTS sessions (
  id            INTEGER PRIMARY KEY,
  started_at    TEXT NOT NULL,
  ended_at      TEXT NOT NULL,
  project       TEXT,
  request_count INTEGER NOT NULL,
  token_count   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS bodies (
  request_id    INTEGER PRIMARY KEY REFERENCES requests(id),
  prompt        TEXT,
  completion    TEXT
);

CREATE INDEX IF NOT EXISTS idx_requests_ts       ON requests(ts);
CREATE INDEX IF NOT EXISTS idx_requests_model    ON requests(model);
CREATE INDEX IF NOT EXISTS idx_requests_project  ON requests(project);
CREATE INDEX IF NOT EXISTS idx_requests_session  ON requests(session_id);
CREATE INDEX IF NOT EXISTS idx_requests_endpoint ON requests(endpoint);
CREATE INDEX IF NOT EXISTS idx_requests_upstream_host ON requests(upstream_host);

CREATE TABLE IF NOT EXISTS policies (
  id          INTEGER PRIMARY KEY CHECK (id = 1),
  mode        TEXT    NOT NULL DEFAULT 'allow_all'
                      CHECK (mode IN ('allow_all', 'allowlist', 'blocklist')),
  models_json TEXT    NOT NULL DEFAULT '[]'
);
