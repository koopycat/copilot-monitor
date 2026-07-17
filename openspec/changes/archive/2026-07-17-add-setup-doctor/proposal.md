## Why

Copilot Monitor's value depends on a client being pointed at the local proxy,
but a failed first capture currently leaves a developer to infer whether the
proxy, dashboard, database, or network setup is at fault. That makes the product
feel harder to set up than its single-binary architecture should be.

## What Changes

- Add `copilot-monitor doctor`, a standalone diagnostic command for the local
  setup.
- Check the configured database path without creating or migrating a database.
- Check the proxy and dashboard health endpoints by default, with flags to skip
  a service that was intentionally not started.
- Optionally test TCP reachability of an explicitly supplied upstream host; this
  does not make an authenticated API request.
- Provide concise human output, machine-readable JSON, actionable fixes, and
  documented limits: editor settings cannot be inferred safely from the proxy.

## Capabilities

### New Capabilities

- `setup-diagnostics`: Side-effect-minimizing checks for a local monitor
  installation.

### Modified Capabilities

- `reporting`: Expose the diagnostic command through the CLI and shell
  completion.
- `website-onboarding`: Point first-capture guidance to the diagnostic command.

## Impact

- Adds a small CLI-only diagnostic path; it does not alter proxy forwarding,
  stored request data, or dashboard APIs.
- Adds deterministic CLI tests using local HTTP servers and no external
  connectivity.
- Updates user-facing setup documentation and CLI reference material.
