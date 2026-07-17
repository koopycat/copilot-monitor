## Context

The proxy has health endpoints, and the dashboard has an API health endpoint,
but neither is assembled into an onboarding check. Opening the store through the
normal store constructor would create directories, apply migrations, and mutate
an otherwise unused installation, so it is unsuitable for a diagnostic command
that promises not to change a user's local capture state.

## Goals / Non-Goals

**Goals:**

- Tell a developer whether the local proxy and optional dashboard are reachable
  and healthy.
- Surface a missing or unreadable database path without creating it.
- Optionally test name resolution and TCP connectivity to a selected upstream.
- Give scripts stable, snake_case JSON output and give humans a next action.

**Non-Goals:**

- Read or edit VS Code, pi, shell, or credential configuration.
- Authenticate with, send a request body to, or spend credits at an upstream.
- Create, migrate, repair, vacuum, or otherwise mutate the SQLite database.
- Prove that a client will generate traffic after its editor-specific override
  is configured.

## Command Contract

`copilot-monitor doctor` uses the following flags:

- `--db <path>`: database to inspect, defaulting to the normal store path.
- `--proxy-url <url>`: local proxy base URL, default `http://127.0.0.1:7733`.
- `--dashboard-url <url>`: dashboard base URL, default `http://127.0.0.1:7734`.
- `--skip-proxy` / `--skip-dashboard`: omit a service intentionally not running.
- `--upstream <host[:port]>`: opt in to a TCP connection check; absence means
  the check is skipped.
- `--timeout <duration>`: shared HTTP/TCP timeout, default two seconds.
- `--json`: emit the report as JSON.

Each report item has `name`, `status`, `detail`, and optional `fix` fields.
Statuses are `pass`, `warn`, `fail`, `skip`, or `info`. The overall `ok` field
is false when any check fails. Warnings such as a database that has not been
created yet do not fail the command; a failed required local endpoint exits 1.
Invalid flags, malformed URLs, malformed upstream targets, and non-positive
timeouts exit 2.

## Decisions

### Use existing health endpoints

The command calls `/_health` on the proxy and `/api/health` on the dashboard.
This verifies a listening service and its store access without forwarding an API
request upstream. Response reads are bounded and require a successful `status`
field.

### Inspect the database file, never open it through `store.Open`

The diagnostic checks whether the resolved path exists, is a regular readable
file, and starts with the SQLite file signature. A missing file is a warning: it
is expected before the first captured request. This preserves the intended
side-effect-free behavior.

### Make upstream checking explicit

Network availability is environment-dependent and must not make a default
diagnostic flaky or unexpectedly reach an external host. `--upstream` enables a
bounded TCP connect to port 443 unless the host includes a port. It sends no
HTTP request, authentication, or request content.

### Report the client-configuration boundary honestly

The proxy cannot reliably inspect an editor's private settings. The doctor
output calls this out as an informational check and directs a VS Code user to
the documented `overrideCapiUrl` setting rather than claiming a false positive.
