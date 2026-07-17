## ADDED Requirements

### Requirement: Error messages use consistent format

Every CLI command SHALL format fatal error messages written to stderr with the
pattern `"error: <operation>: <detail>"`.

The message SHALL start with the literal prefix `error:` followed by a space.
The operation SHALL name the action that failed (e.g., `"opening database"`,
`"parsing --since"`, `"querying stats"`). The detail SHALL include the relevant
value or error, separated by `": "`.

#### Scenario: Flag parsing error

- **WHEN** `copilot-monitor stats --since invalid` is executed
- **THEN** stderr receives `"error: parsing --since \"invalid\": <parse error>"`

#### Scenario: Database open failure

- **WHEN** `copilot-monitor stats --db /nonexistent/db` is executed
- **THEN** stderr receives
  `"error: opening database \"/nonexistent/db\": <os error>"`

#### Scenario: Query failure

- **WHEN** `copilot-monitor stats` encounters a store error
- **THEN** stderr receives `"error: querying stats: <store error>"`

#### Scenario: Script-friendly grep

- **WHEN** a shell script runs any command `2>&1 | grep '^error:'`
- **THEN** the grep SHALL match every fatal error message
