# Security Layer Analysis: From Monitoring Proxy to AI Interaction Platform

## Purpose

This document analyzes how the copilot-monitor proxy can be extended from a pure monitoring
tool into a **security layer** that controls which models clients are allowed to use, and more
broadly into a **central platform for managing AI provider interactions**.

The core new capability is the ability to toggle allowed models -- a policy enforcement point
that sits between the client and upstream LLM providers.

---

## 1. Current Architecture Recap

The proxy's request lifecycle:

```
Client → ServeHTTP → Router.Match → Parse body (model, stream)
       → Build upstream request → Forward → Stream response → Observe usage
       → Persist metadata + token counts → Respond to client
```

Key design properties that make this extension natural:

| Property | Why it helps |
|---|---|
| Config-driven router | Already supports arbitrary upstream hosts and paths |
| Blind JSON walk for `model` key | Works for any JSON API shape, extracts model before forwarding |
| Streaming observer pipeline | Format-agnostic SSE/JSON observer |
| Generic data model | `requests` table has no Copilot coupling |
| Separate `run` (proxy) and `serve` (API) commands | Policy enforcement and management naturally split |
| SQLite shared between commands | Policies written by `serve`, read by `run` |

---

## 2. What Changes: The Enforcement Point

### 2.1 Where to intercept

The interception happens **after** model extraction from the request body and
**before** the upstream request is built.

```
ServeHTTP:
  Match route → Parse body → **EVALUATE POLICY** → Allow/Block → Forward upstream
```

`ParseRequestMetadata(body)` already returns `meta.Model` at exactly this point in
the flow. No refactoring needed -- the model is already available.

### 2.2 What happens on block

```
HTTP 403 Forbidden
Content-Type: application/json

{
  "error": "model_blocked",
  "model": "gpt-4o",
  "message": "Model gpt-4o is blocked by policy"
}
```

The blocked attempt is:
- Logged to the terminal (log level: warn)
- Persisted to the `requests` table with `status=403`, zero token counts
- Never forwarded to the upstream

This means blocked requests are visible in stats/timeline queries with zero tokens, which
provides observability into what was attempted and blocked.

### 2.3 Edge cases

| Case | Behavior |
|---|---|
| Request body has no `"model"` key | Allow through (fail-open, model-less requests like `/models` pass) |
| Model found in response but not request | Can't evaluate at decision time. Allow through. |
| Policy store is nil (default, no config) | Must not evaluate. All requests allowed. Backward compatible. |
| Policy store is empty (no rules defined) | Default: allow all. |

---

## 3. Policy Model

### 3.1 Policy modes

```
AllowAll    — default, backward compatible, everything passes
Allowlist   — only explicitly listed models pass
Blocklist   — everything passes except explicitly listed models
```

### 3.2 Scope

**Global only for phase 1.** All routes share the same policy.

Per-route policies add complexity (which route's policy applies when multiple routes match?)
and don't match the primary use case: "don't let my team use expensive models." Global scope
covers 90% of needs.

### 3.3 Data model (SQLite)

```sql
CREATE TABLE IF NOT EXISTS policies (
  id         INTEGER PRIMARY KEY,
  mode       TEXT    NOT NULL DEFAULT 'allow_all',  -- 'allow_all' | 'allowlist' | 'blocklist'
  models     TEXT    NOT NULL DEFAULT '[]',          -- JSON array of model names
  created_at TEXT    NOT NULL,
  updated_at TEXT    NOT NULL
);

-- Only one row: the global policy. Simplifies querying and avoids race conditions.
CREATE UNIQUE INDEX IF NOT EXISTS idx_policies_single ON policies((1));
```

A single-row table is deliberate: there is one global policy, and updates replace it
atomically. No multi-row aggregation, no partial updates, no race conditions.

Model names are stored as a JSON array to keep the schema flat. At the scale of a
single-user tool (hundreds of models, not millions), JSON in SQLite is fine.

### 3.4 Policy evaluation logic

```go
func (p *Policy) Evaluate(model string) Decision {
    if model == "" {
        return Decision{Allowed: true} // model-less requests pass
    }
    switch p.Mode {
    case Allowlist:
        if p.contains(model) { return Decision{Allowed: true} }
        return Decision{Allowed: false, Reason: "model not in allowlist"}
    case Blocklist:
        if p.contains(model) { return Decision{Allowed: false, Reason: "model is blocked"} }
        return Decision{Allowed: true}
    default: // AllowAll
        return Decision{Allowed: true}
    }
}
```

Model matching should be **exact** by default, with optional prefix matching for
provider-level blocks (e.g., `gpt-*` blocks all GPT models).

### 3.5 Provider-level blocking

Instead of listing every model individually, a model name with a trailing `*` acts as a
prefix pattern:

- `gpt-*` blocks `gpt-4o`, `gpt-4o-mini`, `gpt-4.1`, etc.
- `claude-*` blocks all Claude models
- `*` blocks everything (in blocklist mode)

This is simpler than a separate "block provider" concept and uses the same mechanism.

---

## 4. Component Map

### 4.1 New package: `internal/policy`

```
internal/policy/
  policy.go        — Policy type, evaluation, serialization
  policy_test.go   — Evaluation tests, edge cases
  store.go         — CRUD against SQLite (load, save, update)
  store_test.go    — Persistence tests
```

**policy.go:**

```go
package policy

type Mode string
const (
    AllowAll   Mode = "allow_all"
    Allowlist  Mode = "allowlist"
    Blocklist  Mode = "blocklist"
)

type Policy struct {
    Mode      Mode     `json:"mode"`
    Models    []string `json:"models"`
    UpdatedAt string   `json:"updated_at"`
}

type Decision struct {
    Allowed bool   `json:"allowed"`
    Reason  string `json:"reason,omitempty"`
}

func (p *Policy) Evaluate(model string) Decision { ... }
func (p *Policy) Contains(model string) bool { ... }
```

**store.go:**

```go
func LoadPolicy(ctx context.Context, db *sql.DB) (*Policy, error)
func SavePolicy(ctx context.Context, db *sql.DB, p *Policy) error
```

### 4.2 Modified: `internal/proxy/server.go`

The `Handler` struct gains a `policyStore *policy.Store` field (or a `*sql.DB` for
loading policies, plus an in-memory cache with TTL for performance).

`ServeHTTP` gains a policy evaluation step between metadata parsing and upstream forwarding:

```go
// After meta := ParseRequestMetadata(body)

if h.policyStore != nil {
    policy, err := h.policyStore.Load(ctx)
    if err == nil {
        decision := policy.Evaluate(meta.Model)
        if !decision.Allowed {
            h.log.Warn("id=%d policy_blocked model=%q reason=%q\n", id, meta.Model, decision.Reason)
            h.persistBlockedRequest(ctx, started, route, r, meta)
            writeJSON(w, http.StatusForbidden, map[string]string{
                "error": "model_blocked",
                "model": meta.Model,
                "message": decision.Reason,
            })
            return
        }
    }
}
```

### 4.3 Modified: `internal/api/api.go`

New endpoints:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/policy` | Return current policy (mode + model list) |
| `PUT` | `/api/policy` | Update policy (mode + model list) |
| `GET` | `/api/policy/models` | Return all known models from request history + catalog for the toggle UI |

The `Handler` struct needs a `*sql.DB` or `*store.Store` reference, which it already has
via `NewHandler(st *store.Store)`.

### 4.4 Modified: `internal/store/schema.sql`

Add the `policies` table (see section 3.3).

Add a query function in `store.go`:

```go
func (s *Store) DistinctModels(ctx context.Context) ([]string, error)
    // SELECT DISTINCT model FROM requests WHERE model IS NOT NULL AND model != ''
```

### 4.5 Modified: `internal/cli/run.go` and `serve.go`

Both already open the store. The policy store wraps the same `*sql.DB`:

- `serve.go`: passes `st` to `api.NewHandler(st)` -- API handler gets policy CRUD
- `run.go`: passes `st` (or policy store) to proxy handler for enforcement

### 4.6 Dashboard

**New: `dashboard/src/components/PolicyPanel.svelte`**

A collapsible panel with:
- Mode selector: Allow All / Allowlist / Blocklist (radio buttons or segmented control)
- Model list with toggle switches per model
- Search/filter input for finding models
- Provider group headers (OpenAI, Anthropic, Google, etc.)
- "Add model" input for models not yet seen
- Save button

**Modified files:**
- `dashboard/src/App.svelte` — render PolicyPanel
- `dashboard/src/stores/dashboard.svelte.ts` — policy state
- `dashboard/src/lib/api.ts` — `fetchPolicy()`, `updatePolicy()`, `fetchModels()`
- `dashboard/src/lib/types.ts` — `Policy`, `PolicyMode` types

---

## 5. Design Decisions & Tradeoffs

### 5.1 Single-row policy table vs multi-row rules

**Chosen: single-row.** One global policy stored as a single row.

| Approach | Pros | Cons |
|---|---|---|
| Single row | Atomic updates, no race conditions, dead simple | Can't have per-route policies (deferred) |
| Multi-row rules | Per-route or per-provider rules possible | Ordering, priority, merge semantics add complexity |

The single-row approach is correct for phase 1. If per-route policies are needed later,
the table can be extended with a `route` column without breaking the API -- the existing
row becomes the default/fallback policy.

### 5.2 Policy cache in proxy handler

The proxy handler processes every request. Loading the policy from SQLite on every request
adds latency. A simple in-memory cache with a 5-second TTL eliminates this:

```go
type policyCache struct {
    mu       sync.RWMutex
    policy   *policy.Policy
    loadedAt time.Time
    ttl      time.Duration // 5s
}
```

This is fast enough that policy updates from the dashboard take effect within seconds
without adding meaningful latency to proxied requests.

### 5.3 Model matching: exact vs prefix

Chosen: **exact match by default, `*` suffix for prefix matching.**

- `gpt-4o` matches exactly `gpt-4o`
- `gpt-*` matches `gpt-4o`, `gpt-4o-mini`, `gpt-4.1`, etc.
- `*` matches everything

This gives users both precision and convenience without introducing a separate "provider"
concept that would need to stay in sync with the catalog.

### 5.4 Policy initialization flow

1. Fresh install → no policies table row → default behavior is `AllowAll`
2. User opens dashboard → sees all models from their history + catalog
3. User switches to `Blocklist` mode and blocks specific models
4. OR user switches to `Allowlist` mode and enables only specific models
5. Policy is saved to SQLite → proxy picks it up within 5 seconds (cache TTL)

No config file needed. No migration step. Zero-config for existing users.

### 5.5 Why not extend routes.json

The routes config (`routes.json`) defines WHAT to proxy (paths, upstreams). Adding model
policies there would couple routing and security config, making it impossible to change
policies without restarting the proxy. The dashboard needs runtime mutability.

---

## 6. Privacy & Security Review

### 6.1 No new data collected

The model name is already extracted from the request body for logging and persistence.
Policy evaluation uses the same `meta.Model` field. No additional body parsing, no new
data fields.

### 6.2 Blocked requests in the database

Persisting blocked requests adds rows with `status=403` and zero token counts. This
reveals that someone tried to use a blocked model, which is intentional observability.
The model name was already being logged before the block was introduced.

### 6.3 No content inspection

Policy evaluation only looks at `meta.Model` -- a single string extracted from the request
body's `"model"` key. It does not inspect prompts, completions, or any other content.

### 6.4 No auth bypass

The policy is enforced at the proxy level. If a client bypasses the proxy (connects
directly to the upstream), the policy is not enforced. This is acceptable for a
local developer tool. If enforcement is critical, the proxy must be made non-bypassable
via network configuration (iptables, firewall rules, etc.) -- out of scope for this tool.

---

## 7. The "Central Platform" Vision

Beyond model toggling, the proxy architecture supports these additional capabilities.
The policy layer designed here is the foundation for all of them.

### 7.1 Phase 2+: Rate limiting

Add a `rate_limits` table:

```sql
CREATE TABLE rate_limits (
  model      TEXT NOT NULL,
  window     TEXT NOT NULL,  -- 'hour', 'day', 'month'
  max_requests INTEGER NOT NULL,
  max_tokens INTEGER
);
```

The proxy handler checks rate limits before forwarding, same interception point as model
policies. Uses a sliding window counter stored in SQLite (or in-memory for performance).

### 7.2 Phase 2+: Cost budgets

Daily/monthly spending caps per model or globally. The proxy tracks cumulative cost
(using the existing catalog lookup) and blocks when the budget is exceeded.

### 7.3 Phase 2+: Request transformation

Rules to add, remove, or override HTTP headers or request body fields. Useful for:
- Injecting organization IDs
- Overriding temperature or max_tokens
- Adding custom metadata

### 7.4 Phase 2+: Provider failover

If upstream A returns 5xx, retry with upstream B. Uses the config-driven router's ability
to have multiple upstreams for the same path.

### 7.5 Phase 3: Team management

Multiple project labels, per-project policies, usage quotas per team member. This is the
step from "single-user tool" to "team tool" and requires auth, which is a significant
scope increase.

---

## 8. Implementation Phases

### Phase 1 — Model Allow/Block (S, ~3 days)

1. Add `policies` table to `schema.sql`
2. Implement `internal/policy/policy.go` with evaluation logic
3. Implement `internal/policy/store.go` with CRUD
4. Add policy enforcement in `server.go` ServeHTTP
5. Add `GET/PUT /api/policy` and `GET /api/policy/models` endpoints
6. Add `PolicyPanel.svelte` dashboard component
7. Tests: policy evaluation, blocked requests, API endpoints

### Phase 2 — Dashboard Polish (S, ~1 day)

1. Provider grouping in the toggle UI
2. Search/filter for models
3. Visual indicator for blocked model count
4. Empty state when no models are known

### Phase 3 — Provider-Level Blocking (XS, ~0.5 day)

1. Prefix matching with `*` suffix in policy evaluation
2. UI: "Block all OpenAI" / "Block all Anthropic" quick actions

### Phase 4 — Rate Limiting (M, ~2 days)

1. Rate limit data model
2. Sliding window counter
3. Enforcement in proxy handler
4. Dashboard configuration panel

---

## 9. What Does NOT Change

These invariants are preserved:

| Invariant | Why |
|---|---|
| Loopback-only default (`127.0.0.1`) | Security boundary unchanged |
| No prompt/completion storage | Policy only reads model name, never content |
| No auth header logging | Headers are forwarded (or blocked), never stored |
| Streaming response path | Policy evaluation happens before upstream, doesn't touch response streaming |
| Zero-config backward compatibility | No policies table → `AllowAll` → identical behavior to today |
| Existing routes and upstream config | `routes.json` is orthogonal to policy |
| All existing reports, stats, cost, timeline | Work unchanged; blocked requests appear as 403s with zero tokens |

---

## 10. File Change Summary

### New files

| File | Purpose |
|---|---|
| `internal/policy/policy.go` | Policy type, modes, evaluation, model matching |
| `internal/policy/policy_test.go` | Unit tests for evaluation edge cases |
| `internal/policy/store.go` | SQLite CRUD for policy persistence |
| `internal/policy/store_test.go` | Persistence tests |
| `internal/api/policy.go` | `GET/PUT /api/policy`, `GET /api/policy/models` |
| `dashboard/src/components/PolicyPanel.svelte` | Toggle UI for model policies |

### Modified files

| File | Change |
|---|---|
| `internal/store/schema.sql` | Add `policies` table |
| `internal/store/store.go` | Add `DistinctModels()` query |
| `internal/proxy/server.go` | Add policy evaluation + cache in ServeHTTP, blocked request handling |
| `internal/proxy/server_test.go` | Test blocked requests, test policy cache |
| `internal/api/api.go` | Route `/api/policy` and `/api/policy/models` |
| `internal/cli/run.go` | Wire policy store into proxy handler (already has `*store.Store`) |
| `internal/cli/serve.go` | Policy API already available via existing `*store.Store` |
| `dashboard/src/App.svelte` | Render PolicyPanel |
| `dashboard/src/stores/dashboard.svelte.ts` | Policy state, fetch/update actions |
| `dashboard/src/lib/api.ts` | `fetchPolicy()`, `updatePolicy()`, `fetchModels()` |
| `dashboard/src/lib/types.ts` | `Policy`, `PolicyMode`, `PolicyDecision` types |
| `PRODUCT.md` | Update product description to include security/policy management |
| `specs/product-requirements.md` | Add POL-* requirements for policy enforcement |
| `docs/architecture.md` | Add policy layer to architecture diagram and lifecycle |

---

## 11. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Policy store unavailable → proxy blocks everything | Low | High | `h.policyStore == nil` → skip evaluation (backward compat). Load error → log warning, allow request (fail-open). |
| Cache TTL too long → policy changes lag | Low | Low | 5s TTL is conservative. Dashboard shows "policy active" timestamp. Force-refresh on save. |
| Model name mismatch (catalog vs actual API) | Medium | Medium | Policy model names are user-entered. "gpt-4o" in dashboard must match what the API sends. The `GET /api/policy/models` endpoint returns models as they appear in request history, not catalog-normalized names, so the user sees exactly what to toggle. |
| Single-row table limits future per-route policies | Low | Low | Add `route TEXT` column later. Existing row becomes default policy. Backward compatible. |
| 403 responses break client applications | Medium | Low | Client sees a clear JSON error. VS Code / pi / etc. show the error to the user, who can adjust the policy or switch models. This is the intended behavior. |
