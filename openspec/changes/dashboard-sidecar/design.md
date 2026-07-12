## Context

Previously `run --dashboard` used `combinedDashProxy` to serve both proxy and
dashboard on the same port via `http.ServeMux` + Accept-header sniffing. This
was removed in favor of a simpler approach.

## Goals / Non-Goals

**Goals:**

- `run --dashboard` starts dashboard API+UI on port 7734 in a separate goroutine
- Both proxy and dashboard share the same SQLite store
- Graceful shutdown of both listeners
- `serve` command unchanged (still works standalone)

**Non-Goals:**

- Configurable dashboard port via `run` (use `serve --addr` for that)

## Decisions

### Separate listener, not combined mux

**Why**: Eliminates the need to distinguish browser traffic from API traffic.
The dashboard gets its own `http.Server` on its own port. No header sniffing, no
path-based routing ambiguity.

### Dashboard server in a goroutine

**Why**: Single process, single `store.Store`, single lifecycle. SIGINT stops
both. The goroutine approach is simpler than `serve` requiring a separate
terminal.

## Risks / Trade-offs

- **Port 7734 might be in use** → Fail with clear error at startup (same as
  `serve`)
- **Dashboard server crashes** → Proxy unaffected (separate goroutine, separate
  listener)
