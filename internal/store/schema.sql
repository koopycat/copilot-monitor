CREATE TABLE IF NOT EXISTS requests (
  id                  INTEGER PRIMARY KEY,
  ts                  TEXT    NOT NULL,
  endpoint            TEXT    NOT NULL,
  method              TEXT    NOT NULL,
  path                TEXT    NOT NULL,
  upstream_host       TEXT    NOT NULL,
  model               TEXT,
  stream              INTEGER NOT NULL,
  status              INTEGER NOT NULL,
  error               TEXT,
  latency_ms          INTEGER NOT NULL,
  prompt_tokens       INTEGER,
  cached_input_tokens INTEGER,
  cache_write_tokens  INTEGER,
  completion_tokens   INTEGER,
  total_tokens        INTEGER,
  project             TEXT,
  not_billed          INTEGER NOT NULL DEFAULT 0,
  provider            TEXT NOT NULL DEFAULT '',
  session_id          INTEGER,
  usage_missing       INTEGER NOT NULL DEFAULT 0,
  headroom_proxied    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sessions (
  id            INTEGER PRIMARY KEY,
  started_at    TEXT NOT NULL,
  ended_at      TEXT NOT NULL,
  project       TEXT,
  request_count INTEGER NOT NULL,
  token_count   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_requests_stats    ON requests(ts, project);
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

CREATE TABLE IF NOT EXISTS anomalies (
  id          INTEGER PRIMARY KEY,
  ts          TEXT    NOT NULL,
  category    TEXT    NOT NULL,
  severity    TEXT    NOT NULL CHECK (severity IN ('info', 'warn', 'error')),
  request_id  INTEGER,
  path        TEXT,
  method      TEXT,
  endpoint    TEXT,
  model       TEXT,
  upstream    TEXT,
  status      INTEGER,
  detail      TEXT,
  json_detail TEXT
);

CREATE INDEX IF NOT EXISTS idx_anomalies_ts       ON anomalies(ts);
CREATE INDEX IF NOT EXISTS idx_anomalies_category  ON anomalies(category);
CREATE INDEX IF NOT EXISTS idx_anomalies_severity  ON anomalies(severity);
