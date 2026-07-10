# Lightweight Proxy Pipeline Architectures: Research Report

> **Context**: Copilot Monitor is a Go HTTP reverse proxy that inspects and logs
> LLM API traffic. Currently it has a monolithic `Handler.ServeHTTP` that mixes
> routing, policy enforcement, body capture, upstream forwarding, SSE
> observation, and persistence. This report surveys patterns and frameworks that
> could inform a more modular, composable pipeline.

## 1. Go Middleware/Proxy Frameworks

### 1.1 `elazarl/goproxy` (3.2k ★)

**Approach**: A "man-in-the-middle" HTTP proxy built on `net/http` that uses a
condition+handler pattern. Every request goes through a chain of
`ProxyHttpServer` handlers registered via `OnRequest()` and `OnResponse()`.

```go
proxy := goproxy.NewProxyHttpServer()
proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*$"))).
    HandleConnect(goproxy.AlwaysReject)

proxy.OnRequest(goproxy.DstHostIs("api.openai.com")).
    DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
        // inspect, modify, or short-circuit
        return r, nil  // nil response = continue to upstream
    })

proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
    // observe response
    return resp
})
```

**Key design decisions**:

- Condition functions (`ReqHostMatches`, `DstHostIs`, `UrlMatches`) select which
  handlers fire. This is a **predicate-based dispatch**, not an ordered chain.
- `ProxyCtx` carries per-request mutable state between handlers (user data via
  `ctx.UserData` — effectively a `map[string]interface{}` bag).
- Handlers can short-circuit by returning a `*http.Response` directly (bypass
  upstream).
- `HandleConnect` handles CONNECT tunneling (HTTPS, WebSocket upgrades).

**What we can learn**:

- The predicate+handler model is elegant for "this kind of traffic gets these
  behaviors." Each concern (logging, auth, caching) registers independently.
- `ProxyCtx` avoids passing state through explicit parameters.
- **Downside**: No ordering guarantees. Handlers registered via `OnRequest()`
  all fire in registration order, but there's no "before/after" semantics. For a
  pipeline where "policy check" must happen before "forwarding," you'd need to
  enforce this through a single handler that calls sub-handlers sequentially.
- **Lightweight**: Pure Go, zero dependencies beyond stdlib. Adds ~2μs per
  handler.

### 1.2 `net/http/httputil.ReverseProxy` with `ModifyResponse`

**Approach**: Go's standard library reverse proxy with a single hook point.

```go
rp := &httputil.ReverseProxy{
    Director: func(r *http.Request) {
        r.URL.Scheme = "https"
        r.URL.Host = "api.openai.com"
        r.Host = "api.openai.com"
    },
    ModifyResponse: func(resp *http.Response) error {
        // read body, extract usage, log
        return nil
    },
}
```

**Key design decisions**:

- `Director` is the **only** request-phase hook. You get one function.
- `ModifyResponse` is the **only** response-phase hook.
- Streaming works automatically via `FlushInterval` setting.
- No composability built in — you wrap these yourself.

**What we can learn**:

- The stdlib gives you a single hook. To compose multiple behaviors, you must
  build your own middleware chain _around_ the ReverseProxy, or build your own
  director/modify that calls into sub-behaviors.
- **Lightweight**: Zero allocations beyond what you add. No framework overhead.
- This is essentially what Copilot Monitor does: a single `ServeHTTP` wraps all
  logic. To make it composable, we'd extract individual concerns into functions
  that compose together.

### 1.3 Caddy's Module System

**Approach**: Caddy v2 uses a plugin architecture based on Go interfaces. Each
module implements an interface and is configured via JSON/Caddyfile. The HTTP
server composes a **handler chain** from configured modules.

```go
// Caddy module pattern (simplified)
type Middleware interface {
    ServeHTTP(http.ResponseWriter, *http.Request, http.Handler) error
}

type Handler interface {
    ServeHTTP(http.ResponseWriter, *http.Request) error
}
```

A handler chain is built at config-load time (startup), then executed linearly
per request. Modules are registered via `init()`:

```go
func init() {
    caddy.RegisterModule(Gzip{})
}
```

**Key design decisions**:

- **Static composition**: The pipeline is assembled once at startup from config,
  not dynamically per-request. This is critical for predictability.
- **Interface-based**: Each behavior is a Go interface. The "namespace" concept
  (`http.handlers.reverse_proxy`) provides a registry.
- **Provisioning**: Modules have a `Provision(Context)` phase for initialization
  and a `Cleanup()` phase.
- Ordering is explicit in config (array of handlers).

**What we can learn**:

- Static pipeline assembly at startup is the right default. Per-request dispatch
  should be based on predicates (route, model, header), not dynamic pipeline
  reordering.
- The module registry pattern (`init()` + `RegisterModule`) is clean but
  requires a central registry. For a small proxy like Copilot Monitor, a simpler
  function composition pattern may suffice.
- Caddy's module system is **lightweight** for what it does, but the module
  framework itself (config loading, validation, provisioning) adds complexity.
  For a single-purpose proxy, a flat chain is simpler.
- **Memory**: Caddy's baseline is ~15MB RSS. The module system overhead is
  minimal (~200KB for the registry).

### 1.4 `traefik/whoami` Middleware Pattern

**Approach**: Traefik uses a middleware pipeline where each middleware wraps the
next. This is the classic Go `http.Handler` wrapping pattern:

```go
type Middleware func(http.Handler) http.Handler

// Usage:
handler := Middleware3(Middleware2(Middleware1(baseHandler)))
```

Traefik's actual middleware is more structured (configuration structs, builder
pattern), but the core idea is the same: a chain of `http.Handler` wrappers.

**Key design decisions**:

- Each middleware is a standalone `http.Handler` wrapper.
- Configuration is injected via struct fields, not global state.
- Middleware can short-circuit (return before calling next).
- Response observation is done by wrapping `http.ResponseWriter`.

**What we can learn**:

- The `func(http.Handler) http.Handler` pattern is the simplest possible
  composition model in Go. It's used by `chi`, `alice`, `negroni`, and virtually
  every Go HTTP framework.
- For a proxy pipeline, this pattern works but has a limitation: each middleware
  wraps the response writer. If you need to observe both the body and modify it
  (e.g., decompress, inspect, recompress), you need access to the
  `*http.Response` object, not just the writer. This is where a proxy-specific
  pipeline differs from a generic HTTP middleware chain.

### 1.5 `alice` — Middleware Chaining

**Approach**: `justinas/alice` provides a fluent API for composing standard
`func(http.Handler) http.Handler` middleware:

```go
chain := alice.New(middleware1, middleware2, middleware3).Then(handler)
```

**What we can learn**:

- The chaining is purely syntactic sugar over nested function calls.
- For proxy pipelines where you need request + response phases separately,
  alice-style chaining doesn't help because each middleware only sees one side
  (request before calling next, response via wrapped writer).
- **Lightweight**: `alice` is ~50 lines of code. The pattern is trivially
  replicable.

### 1.6 Go Proxy Pipeline Summary

| Approach                | Composition Model   | Per-Request State | Ordering             |
| ----------------------- | ------------------- | ----------------- | -------------------- |
| `goproxy`               | Predicate + handler | `ProxyCtx` bag    | Registration order   |
| `httputil.ReverseProxy` | Single hook         | Closure           | N/A (single hook)    |
| Caddy                   | Static module chain | Context           | Config-defined array |
| Traefik middleware      | `Handler` wrapping  | Closure           | Nested calls         |
| Alice                   | `Handler` wrapping  | Closure           | Explicit chain       |

**For Copilot Monitor**, the most applicable patterns are:

1. A **static pipeline assembled at startup** (Caddy-style) where each stage is
   a function that takes a `*proxy.Request` (or similar) and returns either a
   modified request, an early response, or a "continue" signal.
2. Each stage is a simple function with a clear signature, not a handler
   wrapper. This avoids the `ResponseWriter` wrapping problem.

---

## 2. Proxy-WASM (Envoy Filter Chain)

### 2.1 Envoy's Filter Chain Architecture

Envoy's HTTP connection manager uses an **ordered filter chain** where each
filter can intercept, modify, or short-circuit both the request and response
path:

```text
Request  → [Filter1] → [Filter2] → [Filter3] → Upstream
Response ← [Filter1] ← [Filter2] ← [Filter3] ← Upstream
```

Each filter implements a set of callbacks:

```cpp
class StreamFilter {
    virtual FilterHeadersStatus decodeHeaders(RequestHeaderMap&, bool end_stream);
    virtual FilterDataStatus    decodeData(Buffer::Instance&, bool end_stream);
    virtual FilterTrailersStatus decodeTrailers(RequestTrailerMap&);
    virtual FilterHeadersStatus encodeHeaders(ResponseHeaderMap&, bool end_stream);
    virtual FilterDataStatus    encodeData(Buffer::Instance&, bool end_stream);
    virtual FilterTrailersStatus encodeTrailers(ResponseTrailerMap&);
};
```

**Key design decisions**:

- **Dual-path**: Decode = request path, Encode = response path. Each filter gets
  called on both paths, with well-defined return codes (`Continue`,
  `StopIteration`, `StopAndBuffer`, etc.).
- **Buffer control**: Filters can request that data be buffered until
  end-of-stream (`StopAndBuffer`), enabling body inspection/modification.
- **Shared context**: `StreamInfo` carries per-request metadata between filters.
- **Static configuration**: The filter chain is assembled from config at
  startup. There's no dynamic reordering at runtime.

### 2.2 Proxy-WASM Plugin Interface

Proxy-WASM is the standard for writing Envoy filters in any language that
compiles to WASM. The plugin interface mirrors the native C++ filter API:

```rust
// Proxy-WASM in Rust (simplified)
impl HttpContext for MyFilter {
    fn on_http_request_headers(&mut self, _num_headers: usize, _end_of_stream: bool) -> Action {
        // inspect/modify headers
        Action::Continue
    }

    fn on_http_request_body(&mut self, body_size: usize, end_of_stream: bool) -> Action {
        // inspect/modify body
        Action::Continue
    }

    fn on_http_response_headers(&mut self, _num_headers: usize, _end_of_stream: bool) -> Action {
        Action::Continue
    }

    fn on_http_response_body(&mut self, body_size: usize, end_of_stream: bool) -> Action {
        // extract usage, log
        Action::Continue
    }
}
```

**What we can learn**:

- The **callback-based filter API** is the most expressive. Headers, data
  chunks, and trailers arrive as separate callbacks, giving the filter full
  control.
- **Return codes** (`Continue`, `Pause`, `Stop`) are more expressive than Go's
  typical `error` return. They let filters say "keep going," "stop here and
  respond," or "buffer and call me back with the accumulated body."
- For streaming bodies, `Pause` + resume is critical. A filter can pause the
  stream while it does something async (e.g., check a policy), then resume.
- **Lightweight penalty**: WASM VM overhead is ~50-200μs per call. Native C++
  filters are ~1-5μs. For a local proxy where you own the binary, you'd skip
  WASM and use native Go code. But the **interface pattern** is what matters.

### 2.3 Adapting the Envoy Pattern for a Local Go Proxy

A Go-native version of the Envoy filter chain:

```go
type FilterAction int
const (
    Continue       FilterAction = iota  // pass to next filter
    Stop           FilterAction = iota  // short-circuit with response
    BufferAndCall  FilterAction = iota  // accumulate body, then call again
)

type RequestContext struct {
    Method  string
    URL     *url.URL
    Headers http.Header
    Body    []byte          // nil if streaming, full if buffered
}

type ResponseContext struct {
    StatusCode int
    Headers    http.Header
    Body       []byte
}

type Filter interface {
    // Decode = request path, Encode = response path
    DecodeHeaders(ctx *RequestContext) (FilterAction, *ResponseContext)
    DecodeData(chunk []byte, endStream bool) (FilterAction, *ResponseContext)
    EncodeHeaders(ctx *ResponseContext) FilterAction
    EncodeData(chunk []byte, endStream bool) FilterAction
}

type Pipeline struct {
    filters []Filter
}

func (p *Pipeline) Handle(req *http.Request) (*http.Response, error) {
    // Walk decode chain, then upstream, then encode chain in reverse
}
```

This is the most powerful model but also the most complex. For Copilot Monitor's
current needs (observe, don't transform), a simpler pipeline suffices. But if we
later add request rewriting, body compression, or cache layers, the Envoy
pattern becomes the right foundation.

---

## 3. Unix Pipeline Philosophy for HTTP Proxies

### 3.1 The "Pipe" Concept

The Unix idea: `cat file | grep ERROR | sort | uniq -c` — each program reads
from stdin and writes to stdout. Applied to HTTP, this would mean:

```text
curl http://localhost:9000 | proxy1 | proxy2 | upstream
```

Where each `proxyN` is a separate process that reads an HTTP request on stdin,
processes it, and writes a (possibly modified) HTTP request to stdout.

### 3.2 Existing Projects (None quite fit)

There is **no mainstream project** that implements literal Unix-pipe HTTP
proxying with separate processes per stage. The closest analogs:

**`mitmproxy` script mode**: `mitmproxy -s script1.py -s script2.py`

- Multiple Python scripts can be loaded into a single mitmproxy process.
- Scripts register hooks (`request`, `response`, `websocket_message`).
- Not separate processes, but separate files with isolated concerns.

**`socat` chaining**: `socat TCP-LISTEN:8080,fork,reuseaddr TCP:proxy1:8081`

- Traffic can be daisy-chained through multiple socat instances.
- But socat is transparent TCP forwarding — no HTTP-level inspection.
- Each hop adds a full TCP connection overhead (~1ms on localhost).

**`nginx` with multiple `proxy_pass`**: You can chain nginx instances, but this
requires running multiple nginx processes, each with its own config. Unwieldy.

**`haproxy` filter chains**: HAProxy has a request/response processing pipeline
with ~20 well-defined phases. Rules within a phase execute linearly. This is
closest to a Unix pipeline but within a single process.

### 3.3 Why Literal Unix Pipes Don't Work for HTTP

1. **Bidirectional**: HTTP has request and response flowing in opposite
   directions. A Unix pipe is unidirectional. You'd need two pipes (stdin/stdout
   for request, a second mechanism for response). This is conceptually
   `socketpair()` but not Unix-pipe-idiomatic.

2. **Streaming**: HTTP bodies stream in chunks (SSE, chunked transfer). A Unix
   pipe delivers bytes, but doesn't preserve HTTP framing (headers vs body,
   chunk boundaries). Each proxy process would need to re-parse HTTP, adding
   overhead.

3. **Metadata loss**: HTTP semantics (status codes, headers, trailers) don't
   survive a byte-pipe boundary without re-encoding them as HTTP/1.1 text, which
   is expensive and error-prone.

4. **Process overhead**: Each pipe stage is a separate OS process with its own
   address space. Memory isn't shared. For a local developer proxy, this adds
   10-50MB per stage (Go binaries) and ~500μs context-switch overhead per hop.

### 3.4 The "Unix Philosophy" Without the Pipes

The **philosophy** of composable single-purpose tools is valuable, even if
literal Unix pipes aren't the mechanism. What we want:

1. **Each behavior is a standalone function/struct** with a clear interface.
2. **Composition is explicit** — you can see the pipeline in one place.
3. **Behaviors don't know about each other** — they communicate through a shared
   context or through explicit return values.
4. **Testing is isolated** — each behavior can be tested with a mock upstream.

This is achievable in-process with a well-designed pipeline interface. The
overhead is function calls (nanoseconds), not process boundaries (microseconds).

---

## 4. Sidecar/Micro-Proxy Patterns

### 4.1 `linkerd2-proxy` (Rust, Tokio)

**Approach**: A purpose-built Rust proxy (no genericity). Every request goes
through a fixed pipeline:

```text
Inbound request
  → Protocol detection (HTTP/1, HTTP/2, gRPC)
  → Identity (mTLS)
  → Policy (authorization)
  → Route matching
  → Load balancing
  → Request/response metrics
  → Upstream
```

**Key design decisions**:

- **No plugin system**: The pipeline is hardcoded. linkerd2-proxy is optimized
  for one job (service mesh sidecar) and doesn't try to be generic.
- **Stack-based middleware**: Uses Tower (Rust's `Service` trait) for composable
  middleware layers. Each layer is a `Service<Request, Response=...>` that wraps
  the next.
- **Per-request allocations minimized**: Object pools, pre-allocated buffers.
  The proxy runs at ~10MB RSS with sub-millisecond p99 latency.
- **Streaming-native**: Every filter operates on `Stream<Item=Frame>` semantics
  in Tokio, not on complete bodies. This is critical for low-latency streaming.

**What we can learn**:

- **Hardcode the happy path**: You don't need a generic plugin system if you
  know your pipeline. Copilot Monitor's pipeline is well-defined: route → policy
  → forward → observe → persist. A hardcoded pipeline with clear interfaces at
  each stage is simpler and faster than a plugin registry.
- **Tower's `Service` trait** maps to Go as `func(ctx, req) (resp, error)`. Each
  stage takes a context + request and returns a response, potentially calling
  the next stage internally. This is the "nested function" pattern.
- **Lightweight**: 10MB RSS for a production-grade proxy serving thousands of
  RPS. For a local dev proxy handling <10 RPS, you can be even lighter.

### 4.2 `oauth2-proxy` (Go)

**Approach**: A single-purpose proxy that sits in front of any upstream and adds
OAuth2 authentication. It uses a middleware chain internally:

```go
// oauth2-proxy's middleware pattern (simplified)
func NewOAuthProxy(opts *Options, validator func(string) bool) *OAuthProxy {
    return &OAuthProxy{
        serveMux:    http.NewServeMux(),
        // ...
    }
}

func (p *OAuthProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
    // 1. Check session cookie
    // 2. If no session, redirect to OAuth provider
    // 3. If session valid, proxy to upstream
    // 4. Optionally: add auth headers, strip cookies
}
```

**Key design decisions**:

- **Single concern**: oauth2-proxy does one thing (auth) and does it well. It
  doesn't try to log, monitor, cache, or compress. Users compose it with other
  proxies by layering.
- **Provider abstraction**: Supports 10+ OAuth providers behind a common
  interface. This is the one place where it's generic — the auth mechanism, not
  the proxy pipeline.
- **~15MB RSS**: A Go binary with minimal dependencies.

**What we can learn**:

- **Single-responsibility proxies are easier to compose externally** than
  monolithic ones. If Copilot Monitor were split into "monitoring proxy" +
  "policy proxy," each could be tested and deployed independently.
- **But**: External composition adds network hops. At localhost scale this is
  negligible (~50μs/hop), but it adds configuration complexity (port management,
  startup ordering).

### 4.3 `nginx unit`

**Approach**: Nginx Unit runs multiple applications in a single process with
language-specific runtimes (Go, Python, Node.js, etc.). It's a reverse proxy
that routes to in-process applications.

This is interesting because it eliminates the process boundary between the proxy
and application logic. But it requires applications to be written to Unit's API,
not standard `net/http`.

**What we can learn**:

- The concept of "in-process routing to different handlers" is exactly what a Go
  `http.ServeMux` already does. Unit's innovation is running handlers in
  different language runtimes within the same process (via shared libraries).

### 4.4 Sidecar Pattern Summary

| Project                   | Language     | Pipeline Model      | Extensibility      | RSS   |
| ------------------------- | ------------ | ------------------- | ------------------ | ----- |
| linkerd2-proxy            | Rust         | Fixed stack (Tower) | None (hardcoded)   | ~10MB |
| oauth2-proxy              | Go           | Single handler      | Provider interface | ~15MB |
| nginx unit                | C + runtimes | Route→app           | Config file        | ~15MB |
| Copilot Monitor (current) | Go           | Monolithic handler  | Routes config      | ~15MB |

**The takeaway**: For a local developer proxy, a hardcoded pipeline with one
extension point (routes-config, policy config) is the sweet spot. Don't build a
plugin system until you have at least 3 distinct plugins you need to support.

---

## 5. Middleware as a Service (MaaS)

### 5.1 Concept

"Middleware as a Service" means running a small proxy that calls out to other
**local services** (separate processes, localhost HTTP) for specific behaviors:

```text
                     ┌──────────────┐
                     │  Policy      │
                     │  Service     │
                     │  :9901       │
                     └──────┬───────┘
                            │ HTTP call?
                            ▼
Client ──► Main Proxy ────► Upstream
              :7733      │
                         │ HTTP call?
                         ▼
                     ┌──────────────┐
                     │  Usage       │
                     │  Collector   │
                     │  :9902       │
                     └──────────────┘
```

Each service is an independent process with its own lifecycle, memory, and
failure mode. The main proxy calls them synchronously or asynchronously.

### 5.2 Existing Projects

**`envoy` with external authorization (`ext_authz`)**: Envoy can call an
external gRPC or HTTP service to authorize each request. The external service
receives the request headers and returns allow/deny. This is the canonical
"middleware as a service" pattern.

```yaml
# envoy.yaml
http_filters:
  - name: envoy.ext_authz
    config:
      grpc_service:
        envoy_grpc:
          cluster_name: ext-authz
  - name: envoy.router
```

The auth service runs as a separate process. Envoy caches decisions
(configurable TTL).

**`traefik` plugin ecosystem (`yaegi`, WASM, middleware plugins)**: Traefik
supports middleware as either in-process Go plugins or external services via
HTTP forwarding. The key insight: **plugins run in-process by default** (Go
plugins loaded via Yaegi interpreter or WASM), but the architecture supports
external calls when needed.

**`pipy` (Flomesh)**: A programmable proxy with a JavaScript-like scripting
language. Pipelines are defined in JS. While not strictly "services," the
programmable nature means you can call out to HTTP endpoints from within the
pipeline script.

**`Coraza WAF` (Go, OWASP)**: Embeds a Web Application Firewall as a library
that can be plugged into any Go proxy. Not a separate service, but designed to
be composable.

### 5.3 Evaluations

**Pros of MaaS**:

1. **Independent deployment**: Each service can be updated/restarted
   independently.
2. **Language freedom**: The policy service could be Rust, the collector Python,
   the proxy Go.
3. **Failure isolation**: If the usage collector crashes, the proxy still
   forwards traffic (degraded mode).
4. **Resource scaling**: Heavy services (e.g., ML-based content filtering) can
   run separately without bloating the proxy.

**Cons of MaaS**:

1. **Latency**: Each hop adds ~0.5-2ms on localhost (TCP handshake not needed
   for keep-alive connections, but serialization adds overhead). For LLM API
   calls (500ms-30s latency), this is negligible. But for a policy check on
   every code completion request (sub-100ms expected), it's noticeable.
2. **Complexity**: Multiple processes to manage, start, and monitor.
3. **Serialization**: Data must be serialized/deserialized at each boundary
   (JSON, protobuf). For request bodies this can be expensive.
4. **Consistency**: If a request is modified by multiple services, the order of
   modifications matters and must be coordinated.

### 5.4 How MaaS Could Apply to Copilot Monitor

The current Copilot Monitor is a **monolithic proxy** (one process, one binary).
MaaS could split it into:

1. **`copilot-proxy`**: Core forward proxy. Handles routing, TLS, connection
   pooling, streaming. Calls out to policy service and usage service.
2. **`copilot-policy`**: Policy evaluation. Receives (model, path) and returns
   allowed/denied. Could be a simple localhost HTTP endpoint.
3. **`copilot-usage`**: Usage collection. Receives (request metadata, response
   usage) and persists to SQLite. Runs asynchronously (fire-and-forget).

This split would let the core proxy be extremely thin (<500 lines), while the
policy and usage services evolve independently.

**But**: For a single-user local tool, the operational complexity of three
processes outweighs the benefits. A single binary with internal modularity
(separate packages with clear interfaces) gives the same architectural benefits
without the runtime overhead.

### 5.5 The "Unix Socket" Alternative

Instead of HTTP over TCP, services could communicate over Unix domain sockets.
This eliminates TCP overhead and provides filesystem-based addressing:

```go
// Policy check via Unix socket (sub-100μs)
conn, _ := net.Dial("unix", "/tmp/copilot-policy.sock")
```

Unix sockets are:

- Faster than TCP on localhost (~50μs vs ~500μs for small messages)
- Access-controlled via file permissions
- No port conflicts

This is the pattern used by Docker (`/var/run/docker.sock`) and many system
daemons. For a local proxy, it's the right transport if you do split into
services.

---

## 6. Synthesis: What Makes a Proxy Pipeline "Lightweight"?

### Quantitative Definition

| Metric                | Lightweight | Heavyweight                |
| --------------------- | ----------- | -------------------------- |
| **Resident memory**   | <50MB       | >200MB                     |
| **Startup time**      | <100ms      | >1s                        |
| **Per-hop latency**   | <100μs      | >1ms                       |
| **Dependencies**      | stdlib only | 50+ packages               |
| **Binary size**       | <20MB       | >100MB                     |
| **Config complexity** | 1 file      | Multiple files + discovery |

### Pattern Effectiveness by Concern

| Pattern                        | Routing | Policy | Observation | Transformation | Streaming |
| ------------------------------ | ------- | ------ | ----------- | -------------- | --------- |
| Handler wrapping (`func(h) h`) | ✅      | ✅     | ⚠️          | ✅             | ⚠️        |
| Predicate + handler (goproxy)  | ✅      | ✅     | ✅          | ✅             | ⚠️        |
| Callbacks (Envoy-style)        | ✅      | ✅     | ✅          | ✅             | ✅        |
| Unix pipes                     | ❌      | ❌     | ❌          | ❌             | ❌        |
| Tower/Service stack            | ✅      | ✅     | ✅          | ✅             | ✅        |
| MaaS (external services)       | ❌      | ⚠️     | ✅          | ❌             | ❌        |

### Recommendation for Copilot Monitor

The current architecture (monolithic `Handler.ServeHTTP`) works. If we want to
make it composable, the best pattern is a **static pipeline of stages**, each
implementing a common interface:

```go
// Each stage can: continue to next, short-circuit, or modify context.
type Stage interface {
    // Process is called for each request. Returns either:
    // - a non-nil response (short-circuit)
    // - an error (fail the request)
    // - nil response + nil error (continue to next stage)
    Process(ctx context.Context, req *proxy.Request) (*http.Response, error)
}

// ResponseObserver is a post-response stage (always runs, even if
// a prior stage short-circuited).
type ResponseObserver interface {
    Observe(ctx context.Context, req *proxy.Request, resp *http.Response, latency time.Duration, err error)
}

type Pipeline struct {
    stages    []Stage
    observers []ResponseObserver
    upstream  UpstreamFunc
}
```

Stages would be:

1. `RouteStage` — match path/model, set upstream URL
2. `PolicyStage` — check allow/deny, short-circuit with 403
3. `ForwardStage` — call upstream, return response (always last)

Observers would be:

1. `LogObserver` — structured logging
2. `PersistenceObserver` — write to SQLite
3. `UsageDebugObserver` — JSONL debug output

This preserves the current behavior while making each concern independently
testable. The pipeline is assembled once at startup (in `run.go`), and ordering
is explicit in code.

### When to Add Complexity

- **0-2 stages**: Monolithic handler is fine. Don't overengineer.
- **3-5 stages**: Consider a `Stage` interface with explicit composition.
- **6+ stages or dynamic stage selection**: Consider a filter chain
  (Envoy-style) with return codes for flow control.
- **External services**: Only if a stage needs its own memory/language/runtime
  (e.g., ML inference in Python).
- **Plugin system**: Only if third parties need to write stages. Caddy-style
  module registration or WASM-based plugins.

**For Copilot Monitor's current scope** (route, policy, forward, observe,
persist), a monolithic handler with well-factored helper functions is the right
choice. The router is already extracted. The next candidate for extraction is
the policy check (currently inline in `ServeHTTP`). After that, the observer
logic (SSE parsing) is already in `capture.go` and `sse.go`.
