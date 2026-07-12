## ADDED Requirements

### Requirement: Browser-only dashboard routing in combined mode

When `--dashboard` is active, the combined mux SHALL only serve the dashboard
SPA for requests that accept `text/html`. Non-browser requests that match no
proxy route SHALL be forwarded to the proxy handler for a 502 response.

#### Scenario: API request gets 502, not dashboard 404

- **WHEN** `--dashboard` is active and an API client sends a request to an
  unmatched path without `Accept: text/html`
- **THEN** the proxy handler returns 502 Bad Gateway

#### Scenario: Browser request gets dashboard SPA

- **WHEN** `--dashboard` is active and a browser navigates to any path with
  `Accept: text/html`
- **THEN** the dashboard SPA is served
