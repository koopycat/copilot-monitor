## Context

Two independent bugs, same root cause pattern: unintended consumers of a shared
path.

**Dashboard 404:** `combinedDashProxy` uses `http.ServeMux` with pattern `/` to
catch all paths. Unmatched proxy paths fall through to the dashboard SPA, which
returns its own 404 for unknown routes. API clients see a misleading 404 instead
of 502.

**Compression stats:** SQL aggregates use `IN ('applied', 'no_change')`.
`no_change` rows have ratio 1.0, dragging averages toward 100% (displayed as
"-99%"), masking real savings.

## Decisions

### Accept header check for dashboard routing

**Why**: The dashboard SPA needs `Accept: text/html` for client-side routing.
API clients (curl, VSCode, pi-agent) don't send this header. A simple header
check cleanly separates browser traffic from API traffic.

### Filter to `applied` only

**Why**: `no_change` means zero impact. Including zero-impact rows in an average
is mathematically correct but practically misleading.
