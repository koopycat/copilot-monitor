## Context

Currently two ways to serve the dashboard:

1. `copilot-monitor serve` — dashboard on port 7734 (separate process)
2. `copilot-monitor run --dashboard` — dashboard on same port as proxy

Mode 2 requires `combinedDashProxy` to multiplex proxy and dashboard traffic,
plus `acceptsHTML` to distinguish browsers from API clients. This complexity
exists solely to save one port.

## Decisions

### Remove combined mode entirely

**Why**: The Accept-header approach already caused a bug. A separate process
(`serve`) is the correct architecture — proxy and dashboard have different
lifecycles, different concerns, and work better decoupled. The port savings
(7733 vs 7734) is negligible for a local tool.
