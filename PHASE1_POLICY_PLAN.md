# Phase 1 — Model Allow/Block Security Layer: Implementation Plan

## Overview

Add a policy enforcement layer that sits between request body parsing and upstream
forwarding, allowing the proxy to allow or block models. Phase 1 is global-scope only
(one policy for all routes) with three modes: allow_all (default), allowlist, blocklist.

**Effort:** ~3 days
**New files:** 4 | **Modified files:** 10

---

## Phase 1a: Policy Package + Schema

### Step 1: Create `internal/policy/policy.go`

**New file.**

Define the `Policy` type, mode constants, evaluation engine, and pattern matching.

```go
package policy

// New: types, constants, DefaultPolicy(), Allowed()

type Mode string

const (
    AllowAll   Mode = "allow_all"
    Allowlist  Mode = "allowlist"
    Blocklist  Mode = "blocklist"
)

type Policy struct {
    Mode   Mode     `json:"mode"`
    Models []string `json:"models"`
}

func DefaultPolicy() *Policy {
    return &Policy{Mode: AllowAll, Models: nil}
}

// Allowed returns true if model should pass.
// Empty model → always true (fail-open).
// Unknown mode → always true (fail-open).
func (p *Policy) Allowed(model string) bool {
    if model == "" {
        return true
    }
    switch p.Mode {
    case Allowlist:
        return p.matches(model)
    case Blocklist:
        return !p.matches(model)
    default:
        return true
    }
}

// matches returns true if model matches any pattern in Models.
// Exact match by default. * suffix = prefix match.
func (p *Policy) matches(model string) bool {
    for _, pattern := range p.Models {
        if matchPattern(pattern, model) {
            return true
        }
    }
    return false
}

func matchPattern(pattern, model string) bool {
    if strings.HasSuffix(pattern, "*") {
        prefix := strings.TrimSuffix(pattern, "*")
        return strings.HasPrefix(model, prefix)
    }
    return pattern == model
}
```

### Step 2: Create `internal/policy/policy_test.go`

**New file.** Table-driven tests covering all evaluation paths.

Test cases:

| Test | What it covers |
|---|---|
| `TestAllowedAllowAll` | nil policy, empty policy, allow_all with populated models all pass |
| `TestAllowedBlocklist` | exact match blocked, prefix match blocked (`gpt-*`), unblocked model passes, empty model passes |
| `TestAllowedAllowlist` | listed model passes, unlisted model blocked, empty model passes |
| `TestAllowedUnknownMode` | unrecognized mode → fail-open (returns true) |
| `TestMatchPattern` | exact match, prefix match, no match, empty strings, pattern `*` matches everything |

Use standard `testing` package, table-driven test pattern with `t.Run()` subtests.

### Step 3: Modify `internal/store/schema.sql`

**Append** after the last existing index (`idx_requests_upstream_host`):

```sql
CREATE TABLE IF NOT EXISTS policies (
  id          INTEGER PRIMARY KEY CHECK (id = 1),
  mode        TEXT    NOT NULL DEFAULT 'allow_all'
                      CHECK (mode IN ('allow_all', 'allowlist', 'blocklist')),
  models_json TEXT    NOT NULL DEFAULT '[]'
);
```

The `CHECK (id = 1)` constraint enforces the single-row design. No separate index needed
since it's a primary key.

---

## Phase 1b: Store Extensions

**Depends on:** Phase 1a (policy types must exist).

### Step 4: Modify `internal/store/store.go`

Add `"database/sql"`, `"encoding/json"`, and `"copilot-monitoring/internal/policy"` to
imports.

Add three new methods after the existing `ExportRequests` method:

```go
// GetPolicy returns the current policy. Returns DefaultPolicy() if no row exists.
func (s *Store) GetPolicy(ctx context.Context) (*policy.Policy, error) {
    if s == nil || s.db == nil {
        return policy.DefaultPolicy(), nil
    }
    var mode string
    var modelsJSON string
    err := s.db.QueryRowContext(ctx,
        "SELECT mode, models_json FROM policies WHERE id = 1",
    ).Scan(&mode, &modelsJSON)
    if errors.Is(err, sql.ErrNoRows) {
        return policy.DefaultPolicy(), nil
    }
    if err != nil {
        return nil, err
    }
    var models []string
    if err := json.Unmarshal([]byte(modelsJSON), &models); err != nil {
        return nil, fmt.Errorf("unmarshal policy models_json: %w", err)
    }
    return &policy.Policy{Mode: policy.Mode(mode), Models: models}, nil
}

// SetPolicy atomically replaces the current policy.
func (s *Store) SetPolicy(ctx context.Context, p *policy.Policy) error {
    if s == nil || s.db == nil {
        return errors.New("nil store")
    }
    modelsJSON, err := json.Marshal(p.Models)
    if err != nil {
        return fmt.Errorf("marshal policy models: %w", err)
    }
    _, err = s.db.ExecContext(ctx,
        "INSERT OR REPLACE INTO policies (id, mode, models_json) VALUES (1, ?, ?)",
        string(p.Mode), string(modelsJSON),
    )
    return err
}

// DistinctModels returns all unique model names from the requests table.
func (s *Store) DistinctModels(ctx context.Context) ([]string, error) {
    if s == nil || s.db == nil {
        return nil, nil
    }
    rows, err := s.db.QueryContext(ctx,
        "SELECT DISTINCT COALESCE(NULLIF(model, ''), '<unknown>') FROM requests WHERE model IS NOT NULL AND model != '' ORDER BY model",
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var out []string
    for rows.Next() {
        var m string
        if err := rows.Scan(&m); err != nil {
            return nil, err
        }
        out = append(out, m)
    }
    return out, rows.Err()
}
```

### Step 5: Add tests to `internal/store/store_test.go`

Add three test functions:

- `TestGetPolicyDefault` — open fresh store, call `GetPolicy()`, assert mode=allow_all, models empty/nil.
- `TestSetPolicyAndGet` — call `SetPolicy()` with blocklist + `["gpt-4o", "claude-*"]`, call `GetPolicy()`, assert round-trip exact match.
- `TestDistinctModels` — insert request rows with `model=gpt-4o`, `model=claude-3.5-sonnet`, `model=gpt-4o` (duplicate), call `DistinctModels()`, assert sorted `["claude-3.5-sonnet", "gpt-4o"]`.

Follow existing test patterns: use `store.Open(filepath.Join(t.TempDir(), "store.db"))`,
use context.Background().

**Validation:** `cd /Users/vk/src/inkubator/copilot_monitoring && just test ./internal/policy/ ./internal/store/`

---

## Phase 1c: Proxy Enforcement

**Depends on:** Phase 1b (store methods must exist).

### Step 6: Modify `internal/proxy/server.go`

#### 6a — Add imports

Add `"encoding/json"`, `"sync"`, and `"copilot-monitoring/internal/policy"` to imports.
(`"sync/atomic"` is already imported for `atomic.Uint64`.)

#### 6b — Add fields to Handler struct

After the `nextID` field, add:

```go
policyMu    sync.RWMutex
policyCache *policy.Policy
policyUntil time.Time
```

(`time` is already imported.)

#### 6c — Add cache TTL constant

At package level (near `const` block or after imports):

```go
const policyCacheTTL = 5 * time.Second
```

#### 6d — Add policy evaluation in ServeHTTP

Insert after `meta := ParseRequestMetadata(body)` and before `if isWebSocketUpgrade(r)`:

```go
// --- policy enforcement ---
{
    allowed := true
    if h.store != nil {
        h.policyMu.RLock()
        fresh := time.Now().Before(h.policyUntil)
        cached := h.policyCache
        h.policyMu.RUnlock()

        if !fresh {
            p, err := h.store.GetPolicy(r.Context())
            if err != nil {
                // Fail-open: keep using stale cache if available.
                h.log.Warn("id=%d policy_load_error=%q\n", id, err.Error())
                if cached != nil {
                    allowed = cached.Allowed(meta.Model)
                }
            } else {
                h.policyMu.Lock()
                // Double-check: another goroutine may have refreshed while we waited.
                if time.Now().After(h.policyUntil) {
                    h.policyCache = p
                    h.policyUntil = time.Now().Add(policyCacheTTL)
                }
                h.policyMu.Unlock()
                allowed = p.Allowed(meta.Model)
            }
        } else if cached != nil {
            allowed = cached.Allowed(meta.Model)
        }
    }

    if !allowed {
        h.log.Warn("id=%d policy_blocked model=%q\n", id, meta.Model)
        h.persistBlockedRequest(r.Context(), started, route, r, meta)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusForbidden)
        json.NewEncoder(w).Encode(map[string]string{
            "error":   "model_blocked",
            "model":   meta.Model,
            "message": "Model is blocked by policy",
        })
        return
    }
}
```

#### 6e — Add persistBlockedRequest method

Add after the existing `persistRequest` method:

```go
func (h *Handler) persistBlockedRequest(ctx context.Context, ts time.Time, route Route, r *http.Request, meta RequestMetadata) {
    if h.store == nil || route.Capture == CaptureNone || route.Capture == CaptureLocal || route.Capture == CaptureTunnel {
        return
    }
    if err := h.store.InsertRequest(ctx, store.RequestRecord{
        Timestamp:    ts,
        Endpoint:     string(route.Endpoint),
        Method:       r.Method,
        Path:         r.URL.RequestURI(),
        UpstreamHost: route.Upstream,
        Model:        meta.Model,
        Stream:       meta.Stream,
        Status:       403,
        LatencyMS:    0,
        Project:      h.project,
    }); err != nil {
        h.log.Warn("store_error=%q\n", err.Error())
    }
}
```

### Step 7: Add tests to `internal/proxy/server_test.go`

Add three test functions:

#### TestPolicyBlocklistBlocksModel

1. Open store via `store.Open(filepath.Join(t.TempDir(), "store.db"))`
2. Set blocklist policy: `st.SetPolicy(ctx, &policy.Policy{Mode: policy.Blocklist, Models: []string{"gpt-4o"}})`
3. Create handler with `NewHandlerWithStore(logWriter, st, "")`, set `h.client` to mock transport that returns 200
4. Send request with body `{"model": "gpt-4o", "messages": [...]}` → assert status 403, JSON body with `"error": "model_blocked"`
5. Verify store has one request row with status=403, model="gpt-4o"
6. Send request with body `{"model": "gpt-4o-mini", "messages": [...]}` → assert status 200 (passes through to mock upstream)
7. Verify store has second row with status=200

#### TestPolicyNotAppliedWhenNoStore

1. Create handler with `NewHandler(logWriter)` (no store)
2. Set mock transport
3. Send request with body `{"model": "gpt-4o"}` → assert status 200 (passes through, no policy evaluation, no panic)

#### TestPolicyCacheRefreshOnExpiry

1. Open store, set blocklist: `["gpt-4o"]`
2. Create handler with `NewHandlerWithStore(logWriter, st, "")`, set mock transport
3. Send request → 403 (warms cache)
4. Manually advance `h.policyUntil` to `time.Now().Add(-1 * time.Second)` (expired)
5. Change policy in store to allow_all via `st.SetPolicy()`
6. Send request → 200 (cache refreshed, now allows model)

**Validation:** `cd /Users/vk/src/inkubator/copilot_monitoring && just test ./internal/proxy/`

---

## Phase 1d: API Endpoints

**Depends on:** Phase 1b (store methods must exist).

### Step 8: Create `internal/api/policy.go`

**New file.** Two handler methods.

```go
package api

// imports: "encoding/json", "net/http", "copilot-monitoring/internal/policy"

func (h *Handler) handlePolicy(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        p, err := h.db.GetPolicy(r.Context())
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        if p == nil {
            p = policy.DefaultPolicy()
        }
        jsonHeader(w)
        json.NewEncoder(w).Encode(p)

    case http.MethodPut:
        var p policy.Policy
        if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
            http.Error(w, "invalid JSON", http.StatusBadRequest)
            return
        }
        switch p.Mode {
        case policy.AllowAll, policy.Allowlist, policy.Blocklist:
            // valid
        default:
            http.Error(w, "invalid mode: must be allow_all, allowlist, or blocklist", http.StatusBadRequest)
            return
        }
        if p.Models == nil {
            p.Models = []string{}
        }
        if err := h.db.SetPolicy(r.Context(), &p); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        jsonHeader(w)
        json.NewEncoder(w).Encode(p)

    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

func (h *Handler) handlePolicyModels(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    models, err := h.db.DistinctModels(r.Context())
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    jsonHeader(w)
    json.NewEncoder(w).Encode(models)
}
```

### Step 9: Modify `internal/api/api.go`

Add two cases to the `ServeHTTP` switch, before the `default` case:

```go
case "/api/policy":
    h.handlePolicy(w, r)
case "/api/policy/models":
    h.handlePolicyModels(w, r)
```

The `encoding/json` import is already present.

### Step 10: Add tests to `internal/api/api_test.go`

Add three test functions:

#### TestGetPolicyDefault

1. Open store, create handler, make `GET /api/policy` request
2. Assert status 200
3. Decode JSON body, assert `Mode == "allow_all"`, `Models` is empty array or nil

#### TestPutAndGetPolicy

1. Open store, create handler
2. Make `PUT /api/policy` with body `{"mode": "blocklist", "models": ["gpt-4o"]}`
3. Assert status 200, decoded body matches sent data
4. Make `GET /api/policy`
5. Assert round-trip: mode=blocklist, models=["gpt-4o"]
6. Make `PUT /api/policy` with invalid mode `"invalid_mode"`
7. Assert status 400

#### TestGetPolicyModels

1. Open store, insert requests with models "gpt-4o", "claude-3.5-sonnet", "gpt-4o" (duplicate)
2. Create handler, make `GET /api/policy/models`
3. Assert status 200, decoded body = `["claude-3.5-sonnet", "gpt-4o"]` (sorted, deduplicated)

**Validation:** `cd /Users/vk/src/inkubator/copilot_monitoring && just test ./internal/api/`

---

## Phase 1e: Dashboard UI

**Depends on:** Phase 1d (API endpoints must exist).

### Step 11: Add types to `dashboard/src/lib/types.ts`

Append:

```typescript
export interface Policy {
  mode: 'allow_all' | 'allowlist' | 'blocklist';
  models: string[];
}
```

### Step 12: Add API functions to `dashboard/src/lib/api.ts`

Append before the closing of the file:

```typescript
export async function fetchPolicy(signal: AbortSignal): Promise<Policy | null> {
  return safeFetch<Policy>('/api/policy', signal);
}

export async function putPolicy(policy: Policy, signal: AbortSignal): Promise<Policy | null> {
  try {
    const r = await fetch('/api/policy', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(policy),
      signal,
    });
    if (!r.ok) return null;
    return (await r.json()) as Policy;
  } catch (e) {
    if (e instanceof Error && e.name === 'AbortError') throw e;
    return null;
  }
}

export async function fetchPolicyModels(signal: AbortSignal): Promise<string[] | null> {
  return safeFetch<string[]>('/api/policy/models', signal);
}
```

Add `Policy` to the import from `./types` at the top of api.ts.

### Step 13: Add state and methods to `dashboard/src/stores/dashboard.svelte.ts`

Add `Policy` to the import from `../lib/types`.
Add `fetchPolicy, putPolicy, fetchPolicyModels` to the import from `../lib/api`.

Add state fields in the class body (alongside existing `$state` declarations):

```typescript
policy: Policy = $state({ mode: 'allow_all', models: [] });
policyModels: string[] = $state([]);
```

Add methods after the existing `load()` and lifecycle methods:

```typescript
async refreshPolicy(): Promise<void> {
  const ctrl = new AbortController();
  const p = await fetchPolicy(ctrl.signal);
  if (p) this.policy = p;
}

async savePolicy(policy: Policy): Promise<boolean> {
  const ctrl = new AbortController();
  const result = await putPolicy(policy, ctrl.signal);
  if (result) { this.policy = result; return true; }
  return false;
}
```

In the `load()` method, add alongside the existing `fetchUpstreams()` / `fetchConfig()` fire-and-forget calls:

```typescript
// Inside load(), alongside other parallel loads:
this.refreshPolicy().catch(() => {});
if (this.policyModels.length === 0) {
  fetchPolicyModels(this._abort!.signal).then((models) => {
    if (models) this.policyModels = models;
  }).catch(() => {});
}
```

### Step 14: Create `dashboard/src/components/PolicyPanel.svelte`

**New file.** Svelte 5 component. Two visual states: collapsed summary + expanded editor.

```svelte
<script lang="ts">
  import { dashboard } from '../stores/dashboard.svelte';

  let editing = $state(false);
  let editMode = $state(dashboard.policy.mode);
  let editText = $state(dashboard.policy.models.join('\n'));
  let saving = $state(false);
  let saved = $state(false);
  let error = $state('');

  function startEdit() {
    editMode = dashboard.policy.mode;
    editText = dashboard.policy.models.join('\n');
    saved = false;
    error = '';
    editing = true;
  }

  function cancelEdit() { editing = false; }

  async function save() {
    saving = true;
    error = '';
    const models = editText.split('\n').map(s => s.trim()).filter(s => s);
    const ok = await dashboard.savePolicy({ mode: editMode, models });
    saving = false;
    if (ok) {
      saved = true;
      editing = false;
    } else {
      error = 'Failed to save policy.';
    }
  }

  function toggleChip(model: string) {
    const lines = editText.split('\n').map(s => s.trim()).filter(s => s);
    const idx = lines.indexOf(model);
    if (idx >= 0) lines.splice(idx, 1);
    else lines.push(model);
    editText = lines.join('\n');
  }
</script>

<section class="policy-panel">
  <h2>Security Policy</h2>

  {#if !editing}
    <div class="policy-summary">
      <span class="tag policy-mode">{dashboard.policy.mode.replace('_', ' ')}</span>
      {#if dashboard.policy.models.length > 0}
        <span class="policy-count">{dashboard.policy.models.length} model pattern{dashboard.policy.models.length !== 1 ? 's' : ''}</span>
      {/if}
      <button class="btn-sm" onclick={startEdit}>Edit</button>
    </div>
  {:else}
    <div class="toggle-group">
      <label><input type="radio" bind:group={editMode} value="allow_all" /> Allow All</label>
      <label><input type="radio" bind:group={editMode} value="blocklist" /> Block List</label>
      <label><input type="radio" bind:group={editMode} value="allowlist" /> Allow List</label>
    </div>

    {#if editMode !== 'allow_all'}
      <textarea class="policy-textarea" bind:value={editText}
        placeholder="One model pattern per line. Use * for prefixes, e.g. gpt-*"></textarea>

      {#if dashboard.policyModels.length > 0}
        <div class="model-chips">
          <span class="chip-help">Known models:</span>
          {#each dashboard.policyModels as model}
            <button class="chip"
              class:active={editText.split('\n').map(s => s.trim()).filter(s => s).includes(model)}
              onclick={() => toggleChip(model)}>{model}</button>
          {/each}
        </div>
      {/if}
    {/if}

    <div class="policy-actions">
      <button class="btn-save" onclick={save} disabled={saving}>Save</button>
      <button class="btn-cancel" onclick={cancelEdit}>Cancel</button>
      {#if error}<span class="policy-error">{error}</span>{/if}
    </div>
  {/if}

  {#if saved}
    <div class="policy-saved">✓ Policy updated</div>
  {/if}
</section>

<style>
  /* New styles. Also add to app.css to ensure they are picked up.
     Inline styles here serve as fallback during development before build. */

  .toggle-group {
    display: flex;
    gap: 1rem;
    margin-bottom: 0.5rem;
  }
  .toggle-group label {
    font-size: 0.78rem;
    color: var(--muted);
    cursor: pointer;
  }
  .toggle-group input { margin-right: 0.3rem; }
</style>
```

### Step 15: Modify `dashboard/src/App.svelte`

Add import:

```svelte
import PolicyPanel from './components/PolicyPanel.svelte';
```

Insert `<PolicyPanel />` after the existing `<RoutesPanel />` and before `<footer>`:

```svelte
{#if dashboard.routes.length > 0}
  <RoutesPanel />
{/if}
<PolicyPanel />
```

### Step 16: Add CSS to `dashboard/src/app.css`

Append all styles from the planner output (the `.policy-panel`, `.policy-summary`, `.policy-mode`, `.policy-count`,
`.policy-textarea`, `.model-chips`, `.chip-help`, `.chip`, `.chip.active`, `.chip:hover`,
`.policy-actions`, `.btn-sm`, `.btn-save`, `.btn-cancel`, `.policy-error`, `.policy-saved` rulesets).

**Validation:**

```bash
cd /Users/vk/src/inkubator/copilot_monitoring/dashboard
pnpm build
# Then: just build && ./copilot-monitor serve --db testdata.db
# Navigate to http://127.0.0.1:7734 and verify PolicyPanel renders
```

---

## Phase 1f: Integration & End-to-End

### Step 17: Rebuild and run full test suite

```bash
cd /Users/vk/src/inkubator/copilot_monitoring
just all
```

### Step 18: Manual smoke test

```bash
# Terminal 1: Start proxy
./copilot-monitor run

# Terminal 2: Start dashboard
./copilot-monitor serve --db ~/.local/share/copilot-monitor/store.db

# Terminal 3: Set blocklist via API
curl -X PUT http://127.0.0.1:7734/api/policy \
  -H "Content-Type: application/json" \
  -d '{"mode":"blocklist","models":["gpt-4o"]}'

# Verify policy persisted
curl http://127.0.0.1:7734/api/policy

# Try blocked model (should return 403)
curl -s -w "\n%{http_code}\n" http://127.0.0.1:7733/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}'

# Reset to allow_all
curl -X PUT http://127.0.0.1:7734/api/policy \
  -H "Content-Type: application/json" \
  -d '{"mode":"allow_all","models":[]}'

# Open dashboard at http://127.0.0.1:7734
# Use PolicyPanel to toggle models and verify 403/200 behavior
```

### Step 19: Update docs

- `docs/architecture.md`: add Policy Layer to the architecture diagram, mention the new
  policy evaluation step in the request lifecycle.
- `specs/product-requirements.md`: add POL-* requirements for model allow/block.
- `PRODUCT.md`: update product description to mention security/policy management.
- `README.md`: add policy section under Features with a quick example.

---

## File Summary

### New Files (4)

| File | Purpose |
|---|---|
| `internal/policy/policy.go` | Policy type, evaluation, pattern matching (~60 lines) |
| `internal/policy/policy_test.go` | Unit tests for evaluation (~80 lines) |
| `internal/api/policy.go` | GET/PUT /api/policy, GET /api/policy/models handlers (~70 lines) |
| `dashboard/src/components/PolicyPanel.svelte` | Policy editor UI (~90 lines) |

### Modified Files (10)

| File | Change |
|---|---|
| `internal/store/schema.sql` | Add `policies` table (~5 lines) |
| `internal/store/store.go` | Add `GetPolicy`, `SetPolicy`, `DistinctModels` (~60 lines) |
| `internal/store/store_test.go` | Add 3 test functions (~50 lines) |
| `internal/proxy/server.go` | Add policy cache fields, enforcement block, `persistBlockedRequest` (~55 lines) |
| `internal/proxy/server_test.go` | Add 3 policy interception tests (~100 lines) |
| `internal/api/api.go` | Add 2 route cases (~4 lines) |
| `internal/api/api_test.go` | Add 3 test functions (~70 lines) |
| `dashboard/src/lib/types.ts` | Add `Policy` interface (~4 lines) |
| `dashboard/src/lib/api.ts` | Add `fetchPolicy`, `putPolicy`, `fetchPolicyModels` (~25 lines) |
| `dashboard/src/stores/dashboard.svelte.ts` | Add policy state + actions (~25 lines) |
| `dashboard/src/App.svelte` | Import + render PolicyPanel (~3 lines) |
| `dashboard/src/app.css` | Add policy panel styles (~70 lines) |

### Documentation (3)

| File | Change |
|---|---|
| `docs/architecture.md` | Add policy layer to lifecycle + component map |
| `specs/product-requirements.md` | Add POL-* requirements |
| `PRODUCT.md` | Update product description |
| `README.md` | Add policy section |

---

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| **Cache double-check missed** — concurrent requests after TTL expiry all hit store | Explicit double-check after acquiring write lock in server.go step 6d |
| **Policy applied to WebSocket** — `/responses` WebSocket also goes through block check | Intentional. `persistBlockedRequest` skips Tunnel routes. 403 is correct response. |
| **Dashboard build stale** — Svelte/TS changes not reflected | Run `cd dashboard && pnpm build` after frontend changes |
| **`encoding/json` conflict in api.go** | Already imported in api.go; policy.go's import is in a new file |
| **Empty model passes through always** | This is by design (fail-open) and tested explicitly in policy_test.go |
| **No CORS OPTIONS handling** | Dashboard is same-origin (served from same server), no issue |

---

## Implementation Order

```
1a (policy.go + test) ──→ 1b (store methods + tests) ──→ 1c (proxy enforcement + tests)
                                                      ──→ 1d (API endpoints + tests)

1d ──→ 1e (dashboard UI)

1c + 1e ──→ 1f (integration, docs)
```

Phase 1a and 1b have no dependencies between them and can be done in parallel if desired.
Phase 1c and 1d both depend on 1b but are independent of each other and can be done in parallel.
Phase 1e depends on 1d (API endpoints must exist for the dashboard to call).
Phase 1f depends on everything.

Recommended execution order: 1a → 1b → 1c → 1d → 1e → 1f.
