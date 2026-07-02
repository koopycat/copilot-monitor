# File Size Analysis & Refactor Plan

Sizes as of 2026-07-02. Tests excluded from split goals (they belong with their packages).

## Largest files

| Lines | File | Status |
|---|---|---|
| 664 | `internal/cli/cli.go` | **Needs split** |
| 478 | `internal/proxy/server.go` | **Needs split** |
| 393 | `internal/store/store.go` | Mostly fine - one cohesive store |
| 385 | `internal/api/api.go` | Mostly fine - one file per resource |
| 317 | `internal/cli/cli_test.go` | Test, not production code |
| 272 | `internal/proxy/server_test.go` | Test |

## cli.go (664 lines) - Split into per-command files

Each subcommand has a `runX` function plus helpers. The single file mixes 8 concerns. Split by command:

```
internal/cli/
├── root.go           # Run(), printUsage()
├── configure.go      # runConfigure, printVSCodeSettings, settingsAddr
├── run.go            # runServer (proxy)
├── stats.go          # runStats + printStatsRows
├── cost.go           # runCost + cost formatting helpers
├── compare.go        # runCompare + printCompareRows + compare helpers
├── today.go          # runToday
├── sessions.go       # runSessions
├── serve.go          # runServe (read-only API)
├── export.go         # runExport + csvField
└── shared.go         # parseSince, emptyDash, formatDollars, formatTokens, formatDelta
```

After split:
- `run.go` (~70 lines) - HTTP proxy startup
- `compare.go` (~120 lines) - the most complex command, justified
- `cost.go` (~100 lines) - cost calculation output
- Others stay small (20-60 lines each)

The `Run` dispatcher in root.go stays tiny (just `switch args[0]`). `cli_test.go` can be split too, but tests are usually kept together for the CLI package.

## proxy/server.go (478 lines) - Split by responsibility

Currently mixes:

| Function | Lines | Concern |
|---|---|---|
| `ServeHTTP` routing | 55 | Entry dispatch |
| `writeUsageDebug` | 25 | Debug logger output |
| `persistRequest` | 40 | DB persistence |
| `MakeUpstreamRequest` | 25 | Upstream URL rewriting |
| `StripHopByHopHeaders` | 35 | Header manipulation |
| `streamResponse` | 35 | Stream + observer integration |
| `proxyWebSocket` + helpers | 80 | WebSocket tunnel |
| `readAndRestoreBody`, `cloneHeaders`, `copyHeaders` | 25 | Helper utilities |

Proposed split:

```
internal/proxy/
├── server.go         # Handler struct + NewHandler + ServeHTTP routing (entry point only)
├── forward.go        # readAndRestoreBody, MakeUpstreamRequest, StripHopByHopHeaders, copyHeaders, cloneHeaders, streamResponse
├── persist.go        # Handler.persistRequest
├── websocket.go      # proxyWebSocket + helpers
├── debug.go          # writeUsageDebug (moved from usage_debug.go perhaps)
├── capture.go        # RequestMetadata parsing (existing)
├── sse.go            # SSE observer (existing)
└── usage_debug.go    # usage debug logger (existing)
```

After split:
- `server.go` ~70 lines - just the handler interface and routing
- `forward.go` ~180 lines - all upstream HTTP forwarding concerns
- `persist.go` ~50 lines - DB write logic
- `websocket.go` ~100 lines - WebSocket tunnel isolation

## store/store.go (393 lines) - fine, leave as-is

The store is one cohesive resource with: open/close/init/insert/stats/compare/timeline/export. Splitting by table would create artificial boundaries. Adding the RequestRecord method is the largest open issue. Leave alone unless the file grows past 500 lines.

## api/api.go (385 lines) - fine, leave as-is

Already organized by concern (json/csv, compare, session). Each handler function is ~30-60 lines. Could split into `api/compare.go`, `api/sessions.go`, `api/stats.go` but adds files without clear gain.

## Recommended order

1. **Split `cli.go` first** - highest line count, cleanest split (each command is independent), most testable changes
2. **Split `server.go` second** - bigger refactor, risk of breaking tests
3. **Keep `store.go` and `api.go`** - they're cohesive enough
4. **Tests stay put** - cli_test.go can stay as one file; it tests the same Run() entry point anyway

## Effort & risk

| Split | Effort | Risk | Test impact |
|---|---|---|---|
| `cli.go` | 1-2h | Low - just moving functions, dispatcher unchanged | Run all CLI tests after |
| `server.go` | 2-3h | Medium - helpers move, must keep CaptureUsageDebug type | Run all proxy tests; visual smoke test |
| `store.go` | skip | n/a | n/a |
| `api.go` | skip | n/a | n/a |

## What I would NOT do

- Split by feature folder (e.g. `feature/copilot/handlers.go`) - benefits unclear, increases friction
- Split tests away from packages - they're integration tests for the package, valuable as-is
- Add a `pkg/` directory or move files to top level - existing layout is consistent
