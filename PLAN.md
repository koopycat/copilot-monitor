# Copilot Monitor - Implementation Plan

## Planning Principles

Work in small chunks that leave the repository buildable after each step.
Prefer isolated packages with tests before wiring them into the proxy.
Do not read GitHub tokens from disk in v1.
Do not implement TLS or certificate management.
Do not buffer streaming responses before forwarding them to VSCode.

## Dependency Map

```
Project skeleton
  -> CLI scaffold
  -> Path router
  -> Forwarder
      -> Real-time SSE tee
      -> Request capture
      -> Store writer
          -> Stats queries
          -> Cost queries
          -> Sessionizer
  -> README quickstart
```

Some tasks can be developed in parallel after the project skeleton exists.
The router, catalog, store schema, and table rendering can be implemented independently.
The end-to-end proxy path must be implemented in strict order because each step depends on the previous one running correctly.

## Ranked Task List

| Rank | Task | Can be done in isolation? | Depends on | Output |
|---:|---|---|---|---|
| 1 | Initialize Go module and directory layout | No | None | Buildable empty CLI project |
| 2 | Add CLI scaffold | Mostly | 1 | `copilot-monitor version`, help output |
| 3 | Add `configure-vscode` command | Yes | 2 | Prints exact VSCode settings JSON |
| 4 | Add HTTP server skeleton | No | 2 | `run` listens on `127.0.0.1:7733` |
| 5 | Add path router package | Yes | 1 | Pure function maps inbound path to upstream base URL and endpoint type |
| 6 | Add transparent forwarder for non-streaming requests | No | 4, 5 | Requests forward to selected upstream with incoming auth headers preserved |
| 7 | Add chat forwarding E2E | No | 6 | `/chat/completions` works through VSCode or curl fixture |
| 8 | Add request metadata parser | Yes | 1 | Extracts model, stream flag, and request hash from JSON body |
| 9 | Add real-time SSE tee parser | Yes | 1 | Parses `data:` events and usage from fixtures without delaying writes |
| 10 | Wire SSE tee into chat forwarding | No | 7, 8, 9 | Chat streams to VSCode in real time and capture struct is produced |
| 11 | Add SQLite schema and migration/open logic | Yes | 1 | Creates `store.db` with `requests`, `sessions`, `bodies` tables |
| 12 | Add request writer | Mostly | 10, 11 | Captured chat requests are persisted |
| 13 | Add `stats` query and command | Mostly | 11, 12 | Per-model usage table |
| 14 | Add model catalog loader | Yes | 1 | Embedded `models.json` parses and validates |
| 15 | Add cost calculator | Yes | 14 | Pure function computes input, output, and total estimated list-price cost |
| 16 | Add `cost` command | Mostly | 13, 15 | Cost report labeled as estimated list-price cost |
| 17 | Add sessionizer | Yes | 11 | Assigns 30-minute gap sessions in tests |
| 18 | Add `sessions` and `today` commands | Mostly | 13, 17 | Session and daily reports |
| 19 | Add inline completion routes | No | 6, 8, 9, 12 | `/v1/engines/*` and `/v1/completions` are forwarded and captured when usage is present |
| 20 | Add agent routes | No | 6, 8, 9, 12 | `/agents/*` is forwarded and captured when usage is present |
| 21 | Add `--store-bodies` | Mostly | 11, 12 | Optional prompt and completion persistence |
| 22 | Add `--json` output | Mostly | 13, 16, 18 | Machine-readable reports |
| 23 | Add README quickstart | Yes | 3, 4, 7 | User can configure VSCode and run the proxy |
| 24 | Add integration tests with local upstream server | No | 6, 9, 12 | Forwarding, streaming, and persistence covered |
| 25 | Final polish | No | All prior | `go test ./...`, `go vet ./...`, clean README |

## Detailed Chunks

### 1. Initialize Go module and layout

Create `go.mod` with module path `copilot-monitoring` unless a repository path is known later.
Create the directory structure from `SPEC.md`.
Add an empty `cmd/copilot-monitor/main.go` that calls the CLI package.
Add a minimal `Makefile` with `build`, `test`, and `fmt` targets.

Validation:

```sh
go test ./...
go build ./cmd/copilot-monitor
```

Isolation: not isolated, because it creates the foundation.

### 2. CLI scaffold

Use the stdlib `flag` package with a small internal command dispatcher.
Add `version` command with a constant version string.
Add help text for planned commands without implementing all behavior.

Validation:

```sh
go run ./cmd/copilot-monitor version
go run ./cmd/copilot-monitor --help
```

Isolation: mostly isolated after task 1.

### 3. `configure-vscode` command

Print the exact JSON snippet from the spec.
Use `http://127.0.0.1:<port>` and respect a `--port` flag.
Do not edit the user's settings file automatically in v1.

Validation:

```sh
go run ./cmd/copilot-monitor configure-vscode
```

Isolation: yes.

### 4. HTTP server skeleton

Implement `copilot-monitor run`.
Bind only to `127.0.0.1:<port>`.
Return `502 Not Implemented` for all paths at first.
Log method, path, status, and latency to stderr.

Validation:

```sh
go run ./cmd/copilot-monitor run --port 7733
curl -i http://127.0.0.1:7733/chat/completions
```

Isolation: depends on CLI scaffold.

### 5. Path router package

Implement a pure router function.
It should map inbound paths to upstream base URL, endpoint label, and capture mode.
It must not use inbound `Host` for routing.

Expected mapping:

| Path | Upstream | Endpoint | Capture |
|---|---|---|---|
| `/chat/completions` | `api.githubcopilot.com` | `chat` | yes |
| `/agents/*` | `api.githubcopilot.com` | `agent` | yes |
| `/models` | `api.githubcopilot.com` | `models` | no |
| `/models/session` | `api.githubcopilot.com` | `models-session` | no |
| `/responses` | `api.githubcopilot.com` | `responses-websocket` | tunnel only |
| `/_ping` | local response | `ping` | no |
| `/embeddings` | `api.githubcopilot.com` | `embeddings` | partial |
| `/v1/engines/*` | `copilot-proxy.githubusercontent.com` | `completions` | yes |
| `/v1/completions` | `copilot-proxy.githubusercontent.com` | `completions` | yes |
| `/v1/messages`, `/v1/messages/*` | `api.githubcopilot.com` | `chat` | yes |
| any other path | `api.githubcopilot.com` | `passthrough` | no |

Validation:

```sh
go test ./internal/proxy -run TestRoute
```

Isolation: yes.

### 6. Transparent forwarder

Implement request cloning and outbound URL rewriting.
Preserve method, query string, body, and relevant headers.
Preserve incoming `Authorization` exactly as sent by VSCode.
Do not inject or read tokens.
Rewrite outbound URL scheme to `https`, host to the selected upstream, and path to the original inbound path.
Remove hop-by-hop headers such as `Connection`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, `TE`, `Trailer`, `Transfer-Encoding`, and `Upgrade`.
Copy upstream response status and headers back to VSCode.

Validation:

Use `httptest.Server` as a fake upstream.
Assert that auth headers arrive unchanged.
Assert that host and path are rewritten correctly.

Isolation: no, it depends on server and router.

### 7. Chat forwarding E2E

Wire `/chat/completions` through the forwarder.
Use a fake upstream first.
Then optionally verify with VSCode once the user configures the override settings.
No persistence is required at this step.

Validation:

```sh
go test ./internal/proxy -run TestChatForwarding
```

Manual validation:

1. Start the proxy.
2. Configure VSCode with the printed JSON.
3. Ask Copilot Chat a trivial question.
4. Confirm the response arrives.

Isolation: no.

### 8. Request metadata parser

Create a parser that can read a JSON request body and return:

- `model`
- `stream`
- `request_hash`
- optional first user message when `--store-bodies` is enabled later

The parser must restore the request body so forwarding still works.
The parser must tolerate invalid or unknown JSON and return partial metadata.

Validation:

Use fixtures for chat requests, completion requests, unknown requests, and invalid JSON.

Isolation: yes.

### 9. Real-time SSE tee parser

Build a parser that accepts streamed bytes and observes SSE `data:` lines.
It should extract usage from either a final chunk or a separate usage event.
It should ignore `[DONE]`.
It should keep forwarding independent of parsing success.

Validation:

Use fixture streams with:

- OpenAI-style chunks.
- Final chunk with `usage`.
- Separate final `usage` event.
- Malformed JSON in the middle.
- Split lines across multiple reads.

Isolation: yes.

### 10. Wire SSE tee into chat forwarding

For streaming responses, wrap the upstream response body in a tee loop.
Write bytes to the downstream `ResponseWriter` immediately.
Flush after each write if the writer supports `http.Flusher`.
Collect usage and completion text only from the tee side.
Persist nothing yet, but return a capture record to the caller.

Validation:

Use an `httptest` upstream that emits delayed chunks.
Assert that the client receives the first chunk before the upstream stream completes.

Isolation: no.

### 11. SQLite schema and store open logic

Embed `schema.sql` or execute schema creation from Go code.
Open the DB at the configured path.
Create parent directories.
Use WAL mode for resilience if supported.
Expose a `Store` interface that can insert requests and query aggregates.

Validation:

```sh
go test ./internal/store
```

Isolation: yes.

### 12. Request writer

Persist the capture record after a request completes or fails.
Store status, latency, endpoint, path, upstream host, model, token counts, project, and error.
If the stream fails mid-response, still store the partial request with an error string.

Validation:

Use an in-memory or temp-file SQLite DB.
Assert rows are inserted for success and failure cases.

Isolation: mostly.

### 13. `stats` query and command

Implement aggregate queries grouped by model and endpoint.
Support `--since`, `--project`, and `--endpoint` filters.
Render a table with request count, prompt tokens, completion tokens, and total tokens.

Validation:

Seed a temp DB and test query output.
Run command manually against the temp DB.

Isolation: mostly.

### 14. Model catalog loader

Add `internal/catalog/models.json`.
Load it with `embed.FS`.
Validate that all prices are non-negative and currency is `USD`.
Expose lookup by exact model name with fallback.

Validation:

```sh
go test ./internal/catalog
```

Isolation: yes.

### 15. Cost calculator

Implement pure cost math.
Formula:

```text
input_cost = prompt_tokens / 1_000_000 * input_per_m
output_cost = completion_tokens / 1_000_000 * output_per_m
```

Unknown models use `fallback_per_m` for both input and output unless the catalog specifies otherwise.
Return a flag indicating whether fallback pricing was used.

Validation:

Unit test exact values with known token counts.

Isolation: yes.

### 16. `cost` command

Use stats aggregates and the catalog to print estimated list-price cost.
Label output as `EST. LIST $`.
If fallback pricing is used, show a note below the table.

Validation:

Seed a temp DB with known rows.
Assert total estimated list-price cost matches expected math.

Isolation: mostly.

### 17. Sessionizer

Implement 30-minute gap sessionization.
Read captured requests ordered by timestamp.
Assign session IDs to requests that do not have one or that are newer than the last sessionizer run.
Compute session start, end, request count, and token count.

Validation:

Unit test boundary cases:

- Gap of 29 minutes stays same session.
- Gap of 30 minutes starts new session if the rule is `>= 30m`.
- Multiple projects in one session are preserved.

Isolation: yes after store exists.

### 18. `sessions` and `today` commands

`today` prints current-day usage grouped by model and endpoint.
`sessions` prints session start, end, duration, requests, tokens, and estimated cost.
Both commands trigger sessionization before reading.

Validation:

Seed temp DB with known timestamps.
Assert table and JSON outputs.

Isolation: mostly.

### 19. Inline completion routes

Add capture support for `/v1/engines/*` and `/v1/completions`.
Parse model from body, path, or fallback endpoint label when the body lacks a model.
Capture usage only when usage is present.
If no usage is present, store model and latency with null token fields.

Validation:

Use completion fixtures with and without usage.

Isolation: no.

### 20. Agent routes

Add capture support for `/agents/*`.
Add tunnel-only support for `/responses` websocket traffic observed in Phase 0.
Do not inspect WebSocket frame contents in v1 unless later evidence shows token usage is only available there.
Forward unknown agent paths without capture if parsing fails.

Validation:

Use fixtures modeled after chat streaming.
Use a websocket fixture or manual VSCode validation for `/responses`.

Isolation: no.

### 21. `--store-bodies`

Add optional body capture.
Default remains metadata-only.
When enabled, store selected prompt text and reconstructed completion text in `bodies`.
Never store bodies for passthrough routes.

Validation:

Test both default and enabled behavior.
Confirm default DB has no body row.

Isolation: mostly.

### 22. `--json` report output

Add JSON encoders for `stats`, `cost`, `today`, and `sessions`.
Keep field names stable and documented.
Do not change table output while adding JSON.

Validation:

Run commands with `--json` and validate with `jq`.

Isolation: mostly.

### 23. README quickstart

Document:

1. Build command.
2. Run command.
3. VSCode settings snippet.
4. Privacy defaults.
5. Meaning of estimated list-price cost.
6. Troubleshooting for missing auth headers or 401s.

Validation:

Follow the README from a clean checkout.

Isolation: yes after CLI names settle.

### 24. Integration tests

Build an end-to-end test using a local fake upstream server.
Run the proxy against it with an injectable upstream base URL for tests.
Verify forwarding, stream flushing, auth preservation, capture, and persistence.
This requires test-only configuration for upstream hosts.

Validation:

```sh
go test ./... -race
```

Isolation: no.

### 25. Final polish

Run formatting, vetting, and tests.
Review all user-visible wording for accurate cost language.
Check that no token-loader code exists.
Check that no local TLS or cert generation exists.
Check that the server binds only to `127.0.0.1` by default.

Validation:

```sh
go fmt ./...
go vet ./...
go test ./...
go build ./cmd/copilot-monitor
```

Isolation: no.

## Parallelization Plan

After task 1 is complete, these tasks can be worked on independently:

- Task 3 (`configure-vscode`).
- Task 5 (path router).
- Task 8 (request parser).
- Task 9 (SSE parser).
- Task 11 (SQLite schema and store open logic).
- Task 14 (model catalog loader).

After task 11 and task 14 are complete, these tasks can be worked on independently:

- Task 15 (cost calculator).
- Task 17 (sessionizer).
- Task 13 (stats query), once task 12 provides sample data or test fixtures.

These tasks should not be parallelized because they depend on real integration behavior:

- Task 6 (transparent forwarder).
- Task 7 (chat forwarding E2E).
- Task 10 (wire SSE tee into forwarding).
- Task 19 (inline completions).
- Task 20 (agent routes).
- Task 24 (integration tests).

## Suggested Work Batches

### Batch A: Runnable skeleton

Tasks: 1, 2, 3, 4, 5.
Result: User can run the binary and configure VSCode, but forwarding is not useful yet.

### Batch B: First useful proxy

Tasks: 6, 7, 8, 9, 10.
Result: Chat can flow through the proxy and usage can be extracted in memory.

### Batch C: Persistence and first reports

Tasks: 11, 12, 13.
Result: Captured chat usage appears in `stats`.

### Batch D: Cost and session analytics

Tasks: 14, 15, 16, 17, 18.
Result: Estimated list-price cost and sessions are usable.

### Batch E: Broader Copilot coverage

Tasks: 19, 20.
Result: Inline completions and agent calls are handled.

### Batch F: Privacy, machine output, docs, integration

Tasks: 21, 22, 23, 24, 25.
Result: v1 is ready for daily use.

## Risk Register

| Risk | Impact | Mitigation |
|---|---|---|
| VSCode does not forward auth headers to override URLs | Proxy receives 401s | Keep v1 token-free, document 401 troubleshooting, only add token loading if proven necessary |
| Copilot streaming response lacks `usage` | Cost cannot be computed for that request | Store null token counts, report unknown token totals, optionally estimate later |
| Copilot changes endpoint paths | Some calls route to default CAPI passthrough | Keep router simple and log unknown paths |
| SSE parser fails on malformed chunk | Missing usage for one request | Forwarding continues, parser failure is non-fatal |
| Proxy accidentally binds externally | Local auth headers could be exposed | Default and tests enforce `127.0.0.1` only |
| Cost numbers are mistaken for actual Copilot bill | Misleading report | Label every cost table as estimated equivalent list-price cost |

## Definition of Done for v1

- VSCode Copilot Chat works through `http://127.0.0.1:7733`.
- Inline completions continue to work through the proxy.
- Auth headers are forwarded but never loaded from disk or injected by the proxy.
- Streaming is real time and not buffered to completion.
- Captured records are persisted to SQLite.
- `stats`, `cost`, `today`, and `sessions` commands work.
- Cost output is labeled as estimated list-price cost.
- Default behavior stores no prompt or completion bodies.
- `go test ./...` passes.
- README quickstart is accurate enough to follow from a clean checkout.
