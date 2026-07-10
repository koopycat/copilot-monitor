# Plan: From Copilot Proxy to Universal AI Proxy

## Goal

Transform `copilot-monitor` from a GitHub Copilot-first proxy with hardcoded
routes into a configuration-first universal AI proxy. GitHub Copilot becomes one
supported provider among many, not the default or privileged backend. This
aligns with the current `--routes-config` capability and the observed usage for
kilocode.

Backwards compatibility is **not required**. The new design can break existing
CLI flags, default behavior, the database schema, and the binary name. A clear
migration guide replaces implicit compatibility.

## Principles

1. **Configuration is the source of truth for routing.** No path-to-upstream
   mappings are hardcoded. All routes come from explicit configuration,
   including any Copilot routes.
2. **Provider-agnostic naming.** Replace Copilot-specific error messages,
   endpoint labels, and UI copy with generic terms (`provider`, `upstream`,
   `endpoint`).
3. **Fail closed stays the default.** Keep ROUTE-001: unknown paths are rejected
   unless the user configures a catch-all route.
4. **Explicit over implicit.** There is no hidden default behavior. If a user
   does not configure a route, the proxy returns an error.
5. **No privileged provider.** Copilot, OpenAI, Anthropic, and local backends
   are treated identically by the core code.
6. **Clean break.** Remove rather than deprecate obsolete concepts, flags, and
   constants.

## Current State

### Hardcoded routing

- `internal/proxy/router.go` defines two hardcoded upstream hosts:
  - `api.githubcopilot.com`
  - `copilot-proxy.githubusercontent.com`
- `copilotRoutePath()` hardcodes 10 path patterns and maps them to upstream
  hosts and `CaptureMode`.
- `combinedDashProxy()` in `internal/cli/run.go` uses `router.Match(path)` to
  decide whether a request is proxy traffic or dashboard traffic.

### Provider-specific assumptions

- `Endpoint` constants (`EndpointChat`, `EndpointAgent`, etc.) are
  Copilot-oriented names.
- `isNotBilledEndpoint()` in `internal/cost/cost.go` treats `completions` as not
  billed.
- `inferProvider()` in `internal/catalog/catalog.go` guesses providers from
  model names.
- Error message "unknown Copilot path" in `internal/proxy/server.go`.
- Product/docs language is Copilot-centric.

### What is already generic

- Request metadata parsing recursively looks for `model` and `stream` keys.
- Usage extraction already normalizes OpenAI-style and Anthropic-style token
  fields.
- The route config format supports arbitrary paths, upstream hosts, path
  prefixes, model filters, and capture modes.

## Phased Plan

### Phase 0: Audit and Baseline

Create an inventory of all Copilot-specific assumptions before changing
behavior.

- [ ] List every literal occurrence of `githubcopilot.com`, `copilot-proxy`, and
      `Copilot` in Go, SQL, JavaScript, HTML, Markdown, and specs.
- [ ] Identify which assumptions are user-facing (names, docs) versus
      implementation (routing, cost).
- [ ] Add an integration test that starts the proxy with the default config and
      asserts all current Copilot paths still route correctly.

Deliverable: `docs/audit-copilot-assumptions.md` or comments in this plan.

### Phase 1: Rename and Config-First Foundation

Make a clean break from the Copilot-centric identity and require explicit
routing configuration.

- [ ] Rename the module path, binary, and repository to a provider-neutral name
      (e.g., `ai-proxy-monitor` / `ai-monitor`).
- [ ] Rename the executable entry point and build target accordingly.
- [ ] Remove `copilotRoutePath()` from `router.go`.
- [ ] Remove the `GitHubCopilotAPIHost` and `GitHubCopilotProxyHost` constants.
- [ ] Change `NewRouter(nil)` to return a router with no routes.
- [ ] Make `--routes-config` required for `run`.
  - If missing, print a helpful error that includes an example Copilot route
    config.
- [ ] Ship example configs under `examples/routes/`:
  - `github-copilot.json` reproduces current behavior.
  - `openai.json`, `anthropic.json`, and `local.json` show common setups.
- [ ] Update `Router.Match` and `Router.MatchModel` so the only fallback is "no
      route".
- [ ] Update `combinedDashProxy()` to keep using `router.Match(path)`.
- [ ] Update `server.go` to return "unknown route" instead of "unknown Copilot
      path".
- [ ] Rename or remove `configure-vscode` if it is Copilot-specific.

Acceptance criteria:

- `just test` passes.
- Running the proxy without `--routes-config` exits immediately with a clear
  error.
- Running with `examples/routes/github-copilot.json` reproduces the old routing
  behavior.

### Phase 2: Generic Capture and Cost

Make usage capture and cost calculation provider-agnostic.

- [ ] Delete the `Endpoint` enum from `router.go`.
  - Endpoints become free-form labels supplied by route config.
- [ ] Replace `isNotBilledEndpoint("completions")` with a per-route
      `not_billed: true` flag.
- [ ] Add `not_billed` and `provider` to `RouteConfig`.
- [ ] Persist `provider` in the request record and expose it in reports.
- [ ] Remove `inferProvider()` from `catalog.go`.
  - Provider must come from the route config or the pricing catalog.
- [ ] Make the price catalog reloadable from a user-supplied file
      (`--pricing-config`).
  - The embedded catalog becomes an optional fallback, not a Copilot-priced
    default.
- [ ] Remove Copilot-specific pricing assumptions from the embedded catalog, or
      split it into a Copilot example catalog.

Acceptance criteria:

- A route for OpenAI `/v1/chat/completions` reports provider `openai` and
  correct cost without model-name heuristics.
- A route can be marked `not_billed: true` and cost reports show zero cost.
- The cost package no longer references any endpoint name as special.

### Phase 3: Advanced Routing

Add routing capabilities needed by other providers and clients.

- [ ] Support routing by `Host` header in addition to path.
- [ ] Support a catch-all route such as `path: "/*"` or `prefix_match: true` on
      `/`.
- [ ] Support per-route header manipulation (add/remove/replace headers before
      forwarding).
- [ ] Support upstream scheme selection (`https` vs `http`; default `https`).
- [ ] Support path stripping/rewriting beyond simple prefix prepend (e.g., regex
      or template).
- [ ] Allow route priority to be explicit; default is still insertion order.
- [ ] Support WebSocket upgrades generically, not only for the hardcoded Copilot
      `/responses` path.

Acceptance criteria:

- kilocode-style OpenAI-compatible traffic can be routed to a local or remote
  upstream with only a config change.
- A catch-all route can transparently forward any path while still capturing
  usage.
- WebSocket routes are declared in config, not hardcoded.

### Phase 4: Dashboard and API Unbranding

Remove all Copilot-centric language from the dashboard, CLI, and API.

- [ ] Update dashboard HTML/JS to say "AI usage" or "LLM usage" everywhere.
- [ ] Remove "GitHub Copilot" as a privileged term; mention it only in
      provider-specific example docs.
- [ ] Update CLI help text and command descriptions.
- [ ] Update API field documentation and endpoint descriptions.
- [ ] Add a provider filter to stats/cost/sessions endpoints and dashboard.
- [ ] Rename CLI commands or flags that contain "copilot".
- [ ] Update default database path and log prefixes to match the new project
      name.

Acceptance criteria:

- A user running only OpenAI routes sees no Copilot branding in the dashboard,
  CLI, or logs.
- The dashboard can filter by provider.

### Phase 5: Documentation and Spec Updates

Update durable documentation and requirements to describe the universal proxy
behavior.

- [ ] Rewrite `specs/product-requirements.md`:
  - Change scope from "GitHub Copilot model usage" to "LLM API usage".
  - Add ROUTE requirements for config-driven routing, required route config,
    catch-all routes, and host-based routing.
  - Add a requirement that GitHub Copilot is one example provider, not a
    default.
- [ ] Rewrite `docs/architecture.md` with the new routing data flow.
- [ ] Rewrite `README.md` to lead with "universal AI proxy" and show Copilot as
      an example.
- [ ] Add `docs/custom-routes.md` with examples for OpenAI, Anthropic, GitHub
      Copilot, and local backends.
- [ ] Add `docs/migration-from-copilot-monitor.md` for existing users.
- [ ] Update `AGENTS.md` project description.

## Breaking Changes Summary

The following will change without backwards compatibility:

- The binary and repository name.
- The module import path.
- Default behavior: `--routes-config` becomes required for `run`.
- Hardcoded Copilot routes are removed; users must supply a route config.
- CLI flags and commands containing "copilot" are renamed.
- The default database path and local data directory change.
- The `Endpoint` enum is removed; endpoints are free-form route labels.
- `inferProvider()` is removed; provider comes from config.
- The embedded price catalog is no longer Copilot-specific.

## Migration Path for Existing Users

1. Install the renamed binary.
2. Copy `examples/routes/github-copilot.json` to a local path.
3. Start the proxy with `--routes-config /path/to/github-copilot.json`.
4. Optionally merge in routes for other providers.
5. Update any scripts, systemd units, or VSCode settings that reference the old
   binary name or default port.

## Open Questions

1. Should route configs support environment-variable substitution for API keys
   in headers?
2. Should the proxy support multiple upstream hosts per route for load balancing
   or fallback?
3. Should the dashboard group endpoints by label, provider, or both?
4. Should the project provide a one-command `init` that writes a starter route
   config?
5. Should the database schema include a `provider` column, or derive provider
   from route config at query time?
