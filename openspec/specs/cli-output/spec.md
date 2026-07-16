<!-- markdownlint-disable MD041 -->

## Purpose

Provide rich, color-formatted, human-readable request output for the `run`
command, with configurable log formats, sticky running totals, and graceful
non-TTY fallback.

## Requirements

### Requirement: Rich human-readable request output

The `run` command SHALL emit each proxied request as a single, color-formatted,
human-readable line when `--log-format human` is active (the default). The line
SHALL include: HTTP method, request path, endpoint label, upstream host, model
name, response status code (color-coded by class), latency, token counts
(input/cached/output), and estimated cost. Fields SHALL be aligned and padded so
that consecutive lines form readable columns.

#### Scenario: Successful streaming request

- **WHEN** a streaming chat completions request is successfully proxied with
  usage data captured
- **THEN** a single line is printed containing: method (dimmed), path, endpoint,
  model (bold), status (green for 2xx), latency, prompt/completion token counts,
  and cost estimate
- **AND** the line fits within terminal-typical widths (no more than 120
  characters for normal field widths)

#### Scenario: Request with no usage data

- **WHEN** a request completes but no token usage data was captured
- **THEN** the line shows `--` for token counts and cost fields instead of
  numeric values

#### Scenario: Error response from upstream

- **WHEN** the upstream returns a 4xx or 5xx status
- **THEN** the status code is displayed in red (for 5xx) or yellow (for 4xx)
- **AND** the line still includes method, path, endpoint, and model

#### Scenario: Health or ping request

- **WHEN** a request matches `/_health` or `/_ping`
- **THEN** the output is a minimal dimmed line indicating the health/ping
  response, without token or cost fields

---

### Requirement: Sticky running totals header

The `run` command SHALL maintain a sticky header line at the top of the terminal
output showing running totals: total request count, total tokens (input +
output), total estimated cost, and uptime. When `--no-live` is active and the
request log is visible (stderr is a TTY), this header SHALL be reprinted at
fixed intervals or after every N requests, using ANSI escape codes to stay in a
fixed position above the scrolling request lines.

#### Scenario: Header appears after first request

- **WHEN** the first proxied request completes with `--no-live` on a TTY
- **THEN** a sticky header appears showing `Requests: 1`, total tokens, total
  cost, and uptime

#### Scenario: Header updates periodically

- **WHEN** additional requests complete
- **THEN** the sticky header updates to reflect cumulative totals
- **AND** the header does not update more frequently than once per second to
  avoid flicker

#### Scenario: No header on non-TTY output

- **WHEN** stderr is redirected to a file or pipe
- **THEN** no sticky header is rendered; request lines scroll normally without
  ANSI cursor manipulation

---

### Requirement: Color and formatting

Output SHALL use ANSI escape codes for color and formatting when stderr is a
terminal. Colors SHALL follow a consistent semantic scheme:

- HTTP method and request path: dimmed/dark gray
- Endpoint label: blue
- Model name: bold white
- 2xx status: green
- 3xx status: cyan
- 4xx status: yellow
- 5xx status: red
- Latency: dimmed (normal speed), yellow (slow, >2s), red (very slow, >10s)
- Token counts: cyan for input, magenta for output
- Cost: green (under $0.01), yellow ($0.01-$0.10), white (over $0.10)
- Errors and warnings: red or yellow background highlight

#### Scenario: Color disabled when NO_COLOR is set

- **WHEN** `NO_COLOR` environment variable is set to any value
- **THEN** all output is plain text without ANSI escape codes
- **AND** column alignment is preserved

#### Scenario: Color disabled when TERM is dumb

- **WHEN** TERM environment variable is set to "dumb"
- **THEN** all output is plain text without ANSI escape codes

---

### Requirement: Log format selection

The `run` command SHALL accept a `--log-format` flag with values `human`
(default) and `json`. The `human` format SHALL produce the rich colored output
described above. The `json` format SHALL produce one JSON object per line
identical to the current structured log format.

#### Scenario: Default is human format

- **WHEN** `copilot-monitor run --upstream api.githubcopilot.com` is executed
  without `--log-format`
- **THEN** the default log format is `human` and requests are displayed with
  rich colored output

#### Scenario: Explicit JSON format

- **WHEN**
  `copilot-monitor run --upstream api.githubcopilot.com --log-format json` is
  executed
- **THEN** each request is emitted as a single JSON object per line

#### Scenario: Log format on serve command

- **WHEN** `copilot-monitor serve --log-format human` is executed
- **THEN** the flag is accepted but only affects request logging (serve has no
  proxy traffic to log)

---

### Requirement: Non-TTY output behavior

When stderr is not a TTY (redirected to a file or pipe), the `human` format
SHALL emit the same structured information as plain text without ANSI escape
codes. Column alignment SHALL still be attempted based on the actual field
widths in the output.

#### Scenario: Output piped to file

- **WHEN**
  `copilot-monitor run --upstream api.githubcopilot.com --no-live 2>log.txt` is
  executed
- **THEN** `log.txt` contains human-readable request lines without ANSI codes
- **AND** fields are space-separated in a grep-parseable format

#### Scenario: JSON format when piped

- **WHEN**
  `copilot-monitor run --upstream api.githubcopilot.com --no-live --log-format json 2>log.jsonl`
  is executed
- **THEN** `log.jsonl` contains one JSON object per line as before
