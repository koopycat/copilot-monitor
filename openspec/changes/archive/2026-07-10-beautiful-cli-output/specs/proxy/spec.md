## ADDED Requirements

### Requirement: Log format flag

The `run` command SHALL support a `--log-format` flag accepting `human` and
`json`. The default SHALL be `human`. The `human` format SHALL produce rich
colored request output when stderr is a TTY, and plain aligned text when stderr
is not a TTY. The `json` format SHALL produce one JSON object per request,
identical to the previous default behavior.

#### Scenario: Default is human format

- **WHEN** `copilot-monitor run --routes-config routes.json` is executed without
  `--log-format`
- **THEN** the log format defaults to `human`

#### Scenario: Explicit JSON format

- **WHEN** `copilot-monitor run --routes-config routes.json --log-format json`
  is executed
- **THEN** each request is emitted as a single JSON object per line

#### Scenario: Invalid format value

- **WHEN** `copilot-monitor run --routes-config routes.json --log-format xml` is
  executed
- **THEN** the command exits with an error indicating valid values are `human`
  and `json`

## MODIFIED Requirements

### Requirement: Live session tail

The `run` command SHALL display a live session tail when stderr is a terminal,
updating every 2 seconds with the current session summary and replacing previous
output in-place. When the live tail is active, per-request log output (whether
human or JSON) SHALL be suppressed to avoid interleaving.

#### Scenario: Stderr is a TTY

- **WHEN** the proxy starts and stderr is a TTY and `--no-live` is not set
- **THEN** the live session tail activates, suppressing per-request log output
  to avoid interleaving

#### Scenario: Stderr is redirected or piped

- **WHEN** the proxy starts and stderr is not a TTY (redirected to file or pipe)
- **THEN** the live tail is disabled and per-request logs are emitted in the
  configured log format (human or json)

#### Scenario: Live tail is explicitly disabled

- **WHEN** the user passes `--no-live`
- **THEN** the live tail is disabled regardless of TTY status
- **AND** per-request logs are emitted in the configured log format (default:
  human)

#### Scenario: Live tail with JSON format

- **WHEN** the user passes `--log-format json` with live tail active
- **THEN** JSON per-request logs are still suppressed while the live tail is
  displayed
