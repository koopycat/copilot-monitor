# DX Requirements Review — `dx-requirements.md`

> Reviewer: glm52 (worker agent). Date: 2026-07-10. Scope: critical review of
> the 36 NFRs in `specs/dx-requirements.md`. Posture: harsh. The goal is
> adoption, not completeness-for-its-own-sake.

## Overall Quality Assessment of the Document Itself

**Grade: B−.** This is a solid, well-organized wish-list, but it is not yet a
_requirements_ document. Three structural problems run through the whole thing:

1. **It is a feature backlog wearing a requirements costume.** NFRs like "must
   support `//` comments" (NFR-011), "must exist a `validate` command"
   (NFR-010), "must expose a `/health` endpoint" (NFR-036) are _functional_
   requirements mislabeled as non-functional. That matters because NFRs are
   supposed to be qualities you can measure _continuously_ (latency, RSS,
   exit-code consistency). Conflating them with one-shot features means there is
   no acceptance gate distinguishing "done" from "vaguely addressed."

2. **Almost nothing has a measurement protocol.** A requirement that says "under
   500ms" (NFR-005) is meaningless without: on what hardware, what DB shape,
   what query, p50 or p99, cold or warm cache, single-run or CI-benchmarked.
   Only NFR-015 and NFR-029 even gesture at a number, and neither specifies
   _how_ it's measured. A reviewer cannot pass or fail these.

3. **Naming inconsistency is a real bug in the spec.** The binary the user
   downloads is `copilot-monitor` (per README and release table), but every NFR
   here uses `llm-proxy`. Anyone copying a command from this doc into a terminal
   will get `command not found`. The internal `binaryName` is still `llm-proxy`
   too (`internal/cli/root.go`). This is the single most onboarding-breaking
   inconsistency in the project and the DX doc does not even mention it. Fix
   this before anything else.

The doc's strengths: it's persona-aware, the "Why it matters" column forces
justification, and the Summary table at the end is a genuine attempt at
prioritization. Good instincts. But the Summary lists 9 "key gaps" while there
are 36 NFRs — it under-indexes on what actually matters.

---

## Section-by-Section Critique

### §1 Onboarding — Time to First Dopamine

- **NFR-001 (`init` command).** Strong. But "interactive or auto-detected" is
  doing a lot of work. Auto-detection of _what_? Env vars? Existing config? This
  needs a concrete decision: if no env vars present, fall back to a single
  OpenAI route stub and print "edit ~/.config/llm-proxy/routes.json". Also:
  where does the file go? XDG? `.`? The NFR doesn't say. **Testable?** Mostly —
  "did a file get created and `run` succeed on it?" Yes. But "without reading
  the readme" is untestable theater; cut that clause.
- **NFR-002 (first-line message).** Good and measurable. Add: the message must
  be on the _first_ stderr line, before any logging, so users see it even if
  logs are redirected.
- **NFR-003 (≤3 shell commands).** Vague. "Not counting tool configuration" is a
  hole big enough to drive a truck through — config IS the friction. Reframe:
  "from `curl -L -o` to a 200 on `/_ping` in ≤3 commands, with a real API key
  already in the environment." Now it's testable.
- **NFR-004 (examples work verbatim).** Excellent and trivially testable in CI.
  Should be a hard gate. Worth more than NFR-001.

### §2 Day-to-Day Ergonomics

- **NFR-005 (<500ms for <1M rows).** The threshold is arbitrary and the load
  shape is undefined. Is that p50? p99? A `stats` with no `--since` filter over
  1M rows is an O(n) scan; 500ms is plausible, but "under 500ms" with no
  percentile is unenforceable. Specify p99 ≤ 500ms, cold-cache, on a
  representative DB fixture checked into `testdata/`. Otherwise this is an
  aspiration.
- **NFR-006 (`live` behavior).** Good and precise. The "2s refresh without
  flickering" sub-clause is the most actionable thing in this section.
- **NFR-007 (`--dashboard`).** This is a _functional_ requirement disguised as
  DX. Also: forcing one port is arguable — many users _want_ the proxy port
  clean of dashboard assets. "Must be the documented default" is opinion
  masquerading as requirement. Cut "must"; say "documented as the recommended
  single-process mode."
- **NFR-008 (path expansion).** Trivially correct and testable. Cheap. Keep. But
  `~` expansion in Go is a 5-line helper — this is barely worth an NFR; it's a
  bug fix.

### §3 Configuration UX

- **NFR-009 (labeled errors).** Cheap, high-value, obviously right. The example
  message is gold — keep the example, it's the spec.
- **NFR-010 (`validate` command).** Good. Functional, but correct to call out.
  Add: `validate` must also resolve env-var refs and report unresolved ones,
  since that's the #2 misconfig after typos.
- **NFR-011 (comments / HJSON / YAML).** **Over-engineered.** YAML has its own
  footguns (the Norway problem, indent-sensitivity), HJSON is a niche dep, and
  JSON5 is yet another parser. Pick _one_: JSONC (JSON with `//` comments) is a
  single regex-strip before `json.Unmarshal` and covers 95% of the pain. Do not
  introduce a second config grammar. This NFR as written invites scope creep.
  **Cut to: "route config may contain `//` line comments, stripped before
  parsing."**
- **NFR-012 (capture-mode descriptions in `--help`).** Cheap, correct, testable
  by asserting the help text contains each sentence. Keep.

### §4 Reliability & Trust

- **NFR-013 (no silent drop).** **The single most important NFR in the doc.**
  Silent data loss in a _monitoring_ tool is existential — the product's entire
  value proposition is "you see everything." This is correctly identified. But
  "persisted with zero token counts and a `usage_missing` flag" is a schema
  change — call that out as a prerequisite, because it implies a migration and
  downstream dashboard changes. Also: must backfill or only apply going forward?
  State it.
- **NFR-014 (metrics endpoint).** Reasonable, but "Prometheus-style" is vague.
  Pick: expose `/metrics` in Prometheus exposition text format (prom-aware, no
  client lib needed for ~5 counters). Don't promise a full metrics framework.
  Worth doing but **defer** past first adoption — a `/health` counter (NFR-036)
  covers 80% of the need.
- **NFR-015 (latency overhead).** **Untestable as written.** "Median ... under
  5ms ... p99 under 20ms" — median and p99 of _what distribution_, measured
  _how_, against _which upstream_? And 1ms for streaming is below typical
  network jitter for the measurement itself. Reframe: "p99 of
  (proxy_response_start − client_request_received − upstream_response_start) ≤
  10ms on a localhost mock upstream in a CI benchmark." Without a benchmark
  harness this NFR is a wish.
- **NFR-016 (startup connectivity check).** **Wrong default.** Failing to start
  because a _third-party_ host is unreachable at bind time is operationally
  hostile — upstreams flap, DNS blips, and now your proxy won't start at boot
  before the network is up. Make the check opt-in (`--check-connectivity`), not
  opt-out. As written it inverts the right default. Also "unreachable" is
  undefined for a host that accepts TCP but 503s.

### §5 Observability

- **NFR-017 (one structured line per request).** Strong, testable (assert each
  proxied request yields exactly one JSON line with the listed fields). Keep.
  This is the backbone of trust.
- **NFR-018 (`X-LLM-Proxy-Id` header).** Good for correlation but **security
  smell**: injecting a custom header into _upstream_ requests to third-party
  APIs can trip strict providers' WAFs (some reject unknown headers). Make it
  opt-in and document the risk. Also: header name collision — `X-` prefix is
  legacy per RFC 6648; `LLM-Proxy-Id` is fine.
- **NFR-019 (`--log-format json`).** Cheap, correct. Fold into NFR-017 — don't
  have two NFRs for the same feature. Having both implies the author wasn't sure
  JSON logging was one thing.
- **NFR-020 (TTY + file dual output).** This is a **bug report**, not an NFR. It
  describes current broken behavior and the fix. Move to a GitHub issue; keep
  one line in the NFR: "log routing must not drop the request log when stderr is
  redirected."

### §6 Integration Surface

- **NFR-021 (exit codes).** Table stakes, testable in CI, cheap. Keep. Add: exit
  130 for SIGINT-interrupted long-running commands (ripgrep/jj do this), so
  wrappers can distinguish "user bailed" from "crashed."
- **NFR-022 (env vars listed in help).** Cheap and right. But "or in `--help`"
  is a cop-out — pick `llm-proxy help environment` as the canonical surface and
  assert it in tests.
- **NFR-023 (SIGHUP reload).** **Over-engineered for v1.** SIGHUP reload with
  per-route success/failure logging, in-flight request handling, and atomic
  config swap is a real subsystem. For a personal monitoring proxy, "stop and
  restart, in-flight requests fail loudly" is acceptable and is what every dev
  already does. **Defer.** If kept, scope it to "reload re-reads the file;
  in-flight requests complete on the old config; new requests use the new
  config" — no fancy per-route diff logging.
- **NFR-024 (empty `--json` exits 0).** Trivially correct, cheap, testable. Keep
  — this is the kind of detail that separates pro tools from toys.

### §7 Documentation & Examples

- **NFR-025 (three recipes).** Good but "ending with a dashboard screenshot" is
  a maintenance burden that rots. Reframe: "ending with a `curl` that returns
  real JSON, plus a link to the live dashboard." Screenshots lie within two
  releases.
- **NFR-026 (troubleshooting.md, 5 errors).** Reasonable. But "the 5 most common
  errors" is a guess — instrument the proxy to _count_ error emissions and
  derive the top 5 from telemetry/issues. Otherwise it's folklore. Also:
  troubleshooting docs that aren't linked from the actual error message are
  useless; require each error string to mention
  `docs/troubleshooting.md#<slug>`.
- **NFR-027 (local.json example).** Good, cheap. Keep.
- **NFR-028 (per-example README).** Overlaps with NFR-004/NFR-010. One
  `examples/routes/README.md` is fine; don't mandate per-file headers. Minor.

### §8 Performance

- **NFR-029 (RSS ≤30MB idle, ≤80MB load).** Numbers are plausible for Go but
  unmeasured. State the measurement: `ru_maxrss` on Linux, after 60s idle,
  single binary, no dashboard. Also 30MB idle is generous for a Go proxy —
  ripgrep is ~10MB; aim lower. The "80MB under 100 concurrent" claim needs a
  benchmark harness (same gap as NFR-015). **Pair these two under one
  "performance budget" NFR with a shared benchmark.**
- **NFR-030 (startup <200ms).** Cheap, testable, correct. Keep. But "modern
  machine" is undefined — pin to the CI runner spec.
- **NFR-031 (body limit, default 1MB).** **Wrong default.** LLM requests
  routinely exceed 1MB (large code context, image attachments for vision
  models). A 1MB default will break real Copilot traffic and _cause_ the trust
  failure NFR-013 is trying to prevent. Default to 10MB, make it configurable
  per-route. The real fix is streaming the body to upstream without full
  buffering, not a tighter cap.
- **NFR-032 (WAL + batched writes).** Correct diagnosis (per-request
  `ExecContext` serializes). But "batched inserts or a write queue" is an
  implementation hint, not a requirement. State the _observable_ property: "p99
  write stall ≤ 5ms under 100 rps" — then let the implementer pick batching vs.
  a queue. As written it dictates the solution.

### §9 Error Handling

- **NFR-033 (404 vs 502 for unknown path).** Obviously correct, cheap, the kind
  of thing that should already be done. Keep.
- **NFR-034 (categorized 502 body).** Good. But the category list (`dns`,
  `connection`, `tls`, `timeout`) omits `http_status` (upstream returned 5xx)
  and `rate_limited` (429) — the two most common real failures. Extend the enum.
- **NFR-035 (store failure non-blocking).** Correct and important — but tensions
  with NFR-013's "never lose data." Pick a stance: a store failure is _logged
  and counted_ (a `capture_dropped_total{reason="store_error"}` counter, tying
  back to NFR-014), never fatal. State the tie-in explicitly.
- **NFR-036 (`/health`).** Cheap, correct, high leverage (Docker HEALTHCHECK,
  systemd, monitoring). Keep. Slightly redundant with NFR-014's metrics — fold
  the counters into `/health` for v1 and only add `/metrics` if asked.

---

## Top 5 Must-Haves (do these first)

Ranked by adoption impact per unit effort. These are the changes a user will
_feel_ within their first session.

1. **NFR-013 — Stop silently dropping usage-less requests.** This is the
   product's existential bug. A monitoring tool that hides rows is dead on
   arrival. Schema change + `usage_missing` flag + backfill decision.
2. **NFR-001 + NFR-002 — `init` + a one-line "you're running, here's the next
   command" message.** Together these convert "I downloaded it" into "I have
   data on screen" in under a minute. This is the adoption funnel.
3. **NFR-017 — One structured JSON log line per request.** The trust +
   debuggability backbone; unblocks NFR-018/019/020 and every operator workflow.
   Cheap, high leverage.
4. **NFR-009 + NFR-010 — Labeled config errors + a `validate` command.** Route
   config is the #1 user surface; making it fail loudly and legibly before `run`
   eliminates the most common "it doesn't work" report. Combined because they're
   the same UX win.
5. **NFR-036 + NFR-021 — Rich `/health` + consistent exit codes.** The
   integration-enabling pair: unblocks Docker HEALTHCHECK, systemd, and CI in
   one cheap sweep. Operators adopt tools that fit their existing automation.

_(Optional 6th–8th, if budget allows: NFR-004 examples-as-CI-gate, NFR-033
404-vs-502, NFR-024 empty-`--json`-exits-0 — all cheap, all "pro tool" tells.)_

---

## Should-Cut or Defer

| NFR                                               | Verdict                              | Reason                                                                                           |
| ------------------------------------------------- | ------------------------------------ | ------------------------------------------------------------------------------------------------ |
| **NFR-011** (HJSON/YAML/comments)                 | **Cut to JSONC only**                | A second grammar is scope creep; `//` comments cover the real pain.                              |
| **NFR-014** (Prometheus `/metrics`)               | **Defer**                            | `/health` counters (NFR-036) cover 80%. Add exposition format only when an operator asks.        |
| **NFR-019** (`--log-format json`)                 | **Merge into NFR-017**               | Duplicate; one NFR for structured logging.                                                       |
| **NFR-020** (TTY + file dual output)              | **Demote to bug**                    | Describes broken current behavior; one line in NFR-017 suffices.                                 |
| **NFR-023** (SIGHUP reload)                       | **Defer to v2**                      | A real subsystem (atomic swap + in-flight handling + per-route logging). Restart is fine for v1. |
| **NFR-007** (`--dashboard` as default)            | **Soften**                           | "Documented recommended mode," not "must." Some users want a clean proxy port.                   |
| **NFR-016** (startup connectivity check, opt-out) | **Invert**                           | Make opt-in; failing to start on a flaky upstream is operationally hostile.                      |
| **NFR-031** (1MB body cap)                        | **Raise default to 10MB, per-route** | 1MB breaks real Copilot/vision traffic — causes the very data loss NFR-013 fears.                |
| **NFR-025** (recipes "ending with a screenshot")  | **Soften**                           | Screenshots rot; end with a real `curl` + dashboard link.                                        |
| **NFR-028** (per-example headers)                 | **Collapse into NFR-004**            | One directory README is enough; don't mandate per-file.                                          |

---

## Missing — what great dev tools do that this doc overlooks

Modeled on ripgrep, jj, fzf, nushell, lazygit, fd, bat, zoxide:

1. **`--help` that fits on one screen, with examples.** No NFR addresses help
   quality. ripgrep's `--help` is legendary because it has _examples per flag_.
   Require: every flag's help text includes a one-line example, and
   `llm-proxy --help` ≤ 24 lines with a "run `llm-proxy help <subcommand>`"
   nudge. Currently absent.
2. **Subcommand `--help` exists and is distinct.** `llm-proxy run --help`,
   `llm-proxy stats --help`, etc. — each with flags, defaults, and an example
   invocation. The hand-rolled CLI in `internal/cli` has no evidence of
   per-subcommand help. Add an NFR.
3. **Zero config should be a real mode.** "Just works with Copilot" is the
   README pitch but no NFR says: with no `routes.json` and Copilot env vars
   present, `llm-proxy run` proxies Copilot traffic out of the box. That's the
   fzf/zoxide bar ("no setup"). NFR-001 gets close but still requires an
   explicit `init`.
4. **`llm-proxy completion <shell>` for bash/zsh/fish/PowerShell.** Every modern
   Go CLI does this for free (cobra/bubbles). The hand-rolled CLI skips it.
   Composability NFRs (§6) miss tab-completion entirely — yet it's the single
   biggest "feels native" win.
5. **Color + `NO_COLOR` + auto-detection.** NFR-022 lists `NO_COLOR` as an env
   var but no NFR requires _respecting_ it, auto-disabling color when piped, or
   providing `--color=always|auto|never` (fd/rg convention). Output-formatting
   UX is unaddressed.
6. **Machine-readable `--help` / self-describing CLI.**
   `llm-proxy --help --json` (or a `help --json`) emitting command/flag metadata
   is how fzf- style wrappers and editors integrate. Not mentioned.
7. **Idempotent, safe `init` — never clobber.** No NFR says `init` must refuse
   to overwrite an existing `routes.json` without `--force`. zoxide/ gh behave
   this way. Silent overwrite is a trust-killer.
8. **A `--dry-run` / `--explain` for `run`.** "Here are the routes I would load,
   the upstreams I'd call, and the capture mode — confirm?" Huge for first-run
   confidence. Absent.
9. **Deterministic output for testing.** No NFR requires reporting commands to
   support `--no-color`, `--no-unicode` (box-drawing chars break some
   terminals), and stable sort order, so output can be diffed/snapshotted.
   nushell/lazygit take this seriously.
10. **Graceful SIGINT for `run`/`live`/`serve`.** NFR-021 covers exit codes but
    not the _experience_ of Ctrl-C: flush pending DB writes, print "shutting
    down, N requests in flight", exit 130 within 1s. Lazygit/jj nail this.
    Currently unspecified.
11. **Versioning + `llm-proxy version` with build info.** `version` exists but
    no NFR requires commit hash, build date, Go version, and a
    `--check-for-update` (or at least a doc link). Operators need to know what
    they're running when filing bugs. gh/rg do this.
12. **Single static binary, no runtime deps.** Implicit but never stated. For a
    tool users drop into `$PATH`, "statically linked, no libc surprises, works
    in a scratch container" is a DX NFR. Add it.
13. **Telemetry of _friction_, not just requests.** The doc treats observability
    as "request observability." Missing: the proxy should _count its own error
    emissions_ (per NFR-034 categories) so the top-5 troubleshooting list
    (NFR-026) is data-driven, not folklore.
14. **`llm-proxy doctor`.** A single command that checks: DB writable, routes
    parse, each upstream reachable, disk space, WAL healthy, version current.
    This is the lazygit/gh pattern for "why is it weird?" and subsumes NFR-010 +
    NFR-016 + NFR-036 into one user-facing verb. Strongly recommended as the
    synthesis of several existing NFRs.
15. **Streaming-first body handling.** NFR-031 assumes buffered bodies. The
    genuinely good answer (and what every serious proxy does) is to stream
    request/response bodies to upstream and to capture usage via a
    streaming-aware parser — eliminating the OOM class entirely and removing the
    1MB/10MB knob. Missing as an architectural principle.

---

## TL;DR

- The doc's heart is in the right place, but ~40% of its "NFRs" are features or
  bug reports, and almost none are _measurably_ testable as written. Fix the
  naming (`copilot-monitor` vs `llm-proxy`) first.
- **Do these 5 first:** stop silent data loss (013), `init`+first-message
  (001/002), structured one-line logs (017), labeled config errors + `validate`
  (009/010), rich `/health` + exit codes (036/021).
- **Cut/defer:** the second config grammar (011), Prometheus (014), SIGHUP
  reload (023), and the 1MB body cap (031 — it's actively harmful).
- **Add:** a `doctor` command, shell completion, real `--help` UX, color
  discipline, `--dry-run`, zero-config Copilot mode, streaming bodies, and
  deterministic/testable output. These are what separate "a proxy" from "a tool
  devs reach for daily."
