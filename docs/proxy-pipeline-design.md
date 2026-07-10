# Lightweight Proxy Pipelines: Architecture & Design

> Synthesized from three research streams: proxy pipeline architectures, LLM
> proxy landscape, and composable middleware patterns. July 2026.

## 1. The Problem

### 1.1 What we have

Copilot Monitor is a Go reverse proxy with a **monolithic `Handler.ServeHTTP`**
that does everything inline:

```
ServeHTTP (server.go)
  ├─ readAndRestoreBody → ParseRequestMetadata
  ├─ Router.MatchModel → route, upstream
  ├─ Policy check (inline, cached)
  ├─ WebSocket branch (hijack + frame inspector)
  ├─ MakeUpstreamRequest → HTTP call
  ├─ streamResponse + SSEObserver (inline observation)
  ├─ Logging (inline calls)
  └─ persistRequest (inline call)
```

This works. It's a single binary at ~15MB RSS. It handles HTTP, SSE, and
WebSocket. It's well-factored into helper files (`capture.go`, `sse.go`,
`websocket.go`, `forward.go`, `router.go`). The concerns are separated at the
file level, but the **control flow is locked in the handler body**.

### 1.2 What we want

The ability to compose proxy behaviors — observation, compression, policy,
routing — into a **pipeline** that can be assembled from config or code, without
running multiple full proxy processes. The pipeline should:

1. **Compose behaviors** — add/remove/reorder stages without touching the core
   handler
2. **Stay lightweight** — single binary, low memory, fast startup, loopback-only
3. **Handle streaming** — SSE responses, WebSocket frames
4. **Preserve privacy** — no prompts, completions, or auth material in logs/db
5. **Be testable** — each stage independently testable with mock upstreams

### 1.3 The immediate use case

The [Headroom integration analysis](./headroom-integration-analysis.md)
identified that running two full proxies in sequence (Headroom + Copilot
Monitor) works but adds operational complexity. A pipeline architecture would
let a single proxy run multiple stages:

```
Tool → [Route] → [Policy] → [Compress] → [Observe] → [Forward] → LLM API
```

## 2. The Landscape: What the Ecosystem Teaches Us

### 2.1 The consensus pattern: lifecycle hooks

Every mature LLM proxy uses some form of **hook/middleware chain**:

| Project             | Pattern                                      | Interface                                                    |
| ------------------- | -------------------------------------------- | ------------------------------------------------------------ |
| **LiteLLM**         | `CustomLogger` subclass with lifecycle hooks | `async_pre_call_hook`, `async_post_call_failure_hook`        |
| **mitmproxy**       | Addon system with event hooks                | `request(flow)`, `response(flow)`, `websocket_message(flow)` |
| **Helicone**        | Worker-based async logging pipeline          | Message queue pattern, async post-processing                 |
| **Portkey**         | Config-driven guardrails + transformations   | Declarative config attached to client                        |
| **GoModel**         | Hardcoded pipeline with provider adapters    | Config-driven routing, no hook system                        |
| **Copilot Monitor** | Hardcoded pipeline (current)                 | Inline stages in `ServeHTTP`                                 |

The **lifecycle hook model** is proven: define clear lifecycle points, let
plugins register for them, keep ordering explicit. Every successful proxy in the
space converges on this.

### 2.2 Go middleware patterns (and their proxy limitations)

The standard Go middleware pattern is `func(http.Handler) http.Handler`:

```go
// Standard Go middleware
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Println("→", r.URL.Path)
        next.ServeHTTP(w, r)
        log.Println("← done")
    })
}
```

**Why this doesn't work well for proxy pipelines:**

1. **`http.Handler` returns void** — no way to inspect the response after the
   inner handler writes it. You must wrap `http.ResponseWriter`, which is
   awkward for streaming.
2. **No explicit "response phase"** — the "post-upstream" work happens in a
   wrapped writer or a deferred function. This is fragile and hard to test.
3. **No shared context** — state between middleware must go through
   `context.Context` or closures. This works but is implicit.
4. **SSE streaming** — a wrapped `ResponseWriter` sees raw bytes, not parsed SSE
   events. You need to re-parse the stream, which is what `SSEObserver` already
   does.

**The better pattern for proxies: explicit pre/post phases**

```go
type Stage interface {
    // Before upstream: inspect request, possibly short-circuit.
    Before(ctx context.Context, req *ProxyRequest) (*http.Response, error)

    // After upstream: observe response, capture data, log.
    // Always called, even if Before or upstream returned an error.
    After(ctx context.Context, req *ProxyRequest, resp *http.Response, latency time.Duration, upstreamErr error)
}
```

This is what all the successful patterns converge to. It separates the "in" and
"out" phases and gives each stage access to both the request context and the
response.

### 2.3 The Envoy model: the gold standard (but complex)

Envoy's HTTP filter chain is the most expressive pattern, with:

- **Dual callbacks**: `decodeHeaders` → `decodeData` → `encodeHeaders` →
  `encodeData`
- **Return codes**: `Continue`, `Stop`, `StopAndBuffer`,
  `StopAndBufferWatermark`
- **Shared context**: `StreamInfo` carries per-request metadata
- **Buffer control**: Filters can pause the stream, buffer the body, then resume

```
Request  → [Filter1.decode] → [Filter2.decode] → [Filter3.decode] → Upstream
Response ← [Filter1.encode] ← [Filter2.encode] ← [Filter3.encode] ← Upstream
```

This is the right model for a proxy that _transforms_ bodies (compression,
format conversion, content filtering). But it **requires buffering** for
body-aware transformations, which adds latency and memory. For Copilot Monitor's
current observation-only needs, it's overkill.

**When to adopt the Envoy model:**

- You need to transform request/response bodies (compression, format conversion)
- You need streaming body modification (not just observation)
- You need fine-grained buffer control
- You have 6+ stages with complex ordering requirements

### 2.4 The Tower model: Rust's answer (inspirational)

Rust's Tower framework uses two traits that are worth understanding even for Go
design:

```rust
// Service: the actual request processor
pub trait Service<Request> {
    type Response;
    type Error;
    type Future: Future<Output = Result<Self::Response, Self::Error>>;
    fn call(&self, req: Request) -> Self::Future;
}

// Layer: factory that wraps a Service to produce a new Service
pub trait Layer<S> {
    type Service;
    fn layer(&self, inner: S) -> Self::Service;
}
```

**Key insight: separating factory (Layer) from instance (Service).**

- `Layer` is configured once (startup) and produces a `Service`
- `Service` handles per-request processing
- Each `Service` wraps the inner `Service` (onion model)
- Types are fully resolved at compile time (zero-cost abstraction)

A Layer can create a new Service instance per request if needed (e.g., each
request gets its own timing span). This is more elegant than inline setup in
each middleware.

**Go adaptation:**

```go
// Service: handles a single request
type Service interface {
    Serve(ctx context.Context, req *ProxyRequest) (*http.Response, error)
}

// Middleware: wraps a Service to produce a new Service
type Middleware interface {
    // Wrap is called once at startup to build the chain
    Wrap(next Service) Service
}
```

This gives a clean startup-time assembly phase where middleware can initialize
(open DB connections, load config, start background workers) and then produce a
per-request Service.

### 2.5 What "lightweight" means quantitatively

From the research, a lightweight proxy pipeline should meet these targets:

| Metric            | Lightweight target             | What we have now      |
| ----------------- | ------------------------------ | --------------------- |
| Resident memory   | <50 MB                         | ~15 MB                |
| Startup time      | <100 ms                        | <50 ms                |
| Per-stage latency | <100 μs (sync) / <1 ms (async) | ~10 μs (inline calls) |
| Dependencies      | stdlib + SQLite driver         | stdlib + go-sqlite3   |
| Binary size       | <20 MB                         | ~12 MB                |
| Config            | 1 file or env vars             | 1 routes.json + flags |
| Process count     | 1                              | 2 (proxy + dashboard) |

Copilot Monitor already meets or exceeds all lightweight targets. Adding a
pipeline architecture should maintain this profile.

## 3. Design: The Proxy Pipeline Architecture

### 3.1 Core concept

A **proxy pipeline** is an ordered sequence of stages. Each stage can:

1. **Continue**: pass the request to the next stage
2. **Short-circuit**: return a response immediately (e.g., 403 policy block)
3. **Fail**: return an error

After the upstream call (the final stage), each stage gets a second callback for
observation, logging, and persistence.

```
                    PRE-UPSTREAM                    POST-UPSTREAM
                    ────────────                    ─────────────
Request ──► [Route] ──► [Policy] ──► [Forward] ──► [Route] ──► [Policy] ──► Client
               │           │            │              │           │
               │           │            │              │           │
               ▼           ▼            ▼              ▼           ▼
            set route   check allow    call API      observe     observe
            upstream    / deny         return        response    response
```

### 3.2 The Stage interface

```go
// ProxyRequest carries everything a stage needs about the request.
type ProxyRequest struct {
    ID        uint64
    Method    string
    Path      string
    Upstream  string          // resolved by routing stage
    Endpoint  string          // e.g., "chat", "agent"
    Model     string          // extracted from request body
    Stream    bool
    Headers   http.Header
    Body      []byte          // the original request body
    RawReq    *http.Request   // the underlying http.Request
    StartedAt time.Time

    // Capture mode (set by routing stage):
    Capture   CaptureMode     // none, metadata, usage

    // Stage-specific state (set by stages, read by later stages):
    Extra     map[string]any  // typed bag for stage communication
}

// ProxyResponse carries everything about the upstream response.
type ProxyResponse struct {
    StatusCode int
    Headers    http.Header
    LatencyMS  int64
    BytesRead  int64

    // Usage extracted from SSE/JSON response body:
    Usage      *Usage          // nil if no usage detected
    Model      string          // response model (fallback if request model unknown)
}

// Stage processes a request through the proxy pipeline.
type Stage interface {
    Name() string

    // Before is called in order, request → upstream.
    // Return nil, nil to continue to the next stage.
    // Return non-nil response to short-circuit (bypass remaining stages and upstream).
    // Return non-nil error to fail the request.
    Before(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error)

    // After is called in reverse order, upstream → response.
    // Always called for completed requests (even if Before or upstream failed).
    // resp is nil if the upstream call failed or was short-circuited.
    After(ctx context.Context, req *ProxyRequest, resp *ProxyResponse, upstreamErr error)
}
```

### 3.3 The Pipeline

```go
// Pipeline executes stages in order.
type Pipeline struct {
    stages   []Stage
    upstream func(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error)
}

// NewPipeline creates a pipeline with the given stages and upstream function.
func NewPipeline(upstream func(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error), stages ...Stage) *Pipeline {
    return &Pipeline{
        stages:   stages,
        upstream: upstream,
    }
}

// Execute runs the full pipeline for a single request.
func (p *Pipeline) Execute(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error) {
    // Phase 1: Pre-upstream (forward: stages[0] → stages[n])
    var finalResp *ProxyResponse
    for _, s := range p.stages {
        resp, err := s.Before(ctx, req)
        if err != nil {
            // Run After on all stages that had Before called
            p.runAfter(ctx, req, s, resp, err)
            return nil, err
        }
        if resp != nil {
            // Short-circuit: this stage returned a response
            p.runAfter(ctx, req, s, resp, nil)
            return resp, nil
        }
    }

    // Phase 2: Upstream call
    upstreamResp, upstreamErr := p.upstream(ctx, req)

    // Phase 3: Post-upstream (reverse: stages[n] → stages[0])
    for i := len(p.stages) - 1; i >= 0; i-- {
        p.stages[i].After(ctx, req, upstreamResp, upstreamErr)
    }

    return upstreamResp, upstreamErr
}

// runAfter calls After on all stages up to and including the stopping point,
// in reverse order.
func (p *Pipeline) runAfter(ctx context.Context, req *ProxyRequest, stopAt Stage, resp *ProxyResponse, err error) {
    for i := len(p.stages) - 1; i >= 0; i-- {
        p.stages[i].After(ctx, req, resp, err)
        if p.stages[i] == stopAt {
            break
        }
    }
}
```

### 3.4 Built-in stages

#### RoutingStage

```go
type RoutingStage struct {
    router *Router
}

func (s *RoutingStage) Name() string { return "routing" }

func (s *RoutingStage) Before(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error) {
    route, ok := s.router.MatchModel(req.Path, req.Model)
    if !ok {
        return &ProxyResponse{StatusCode: 502}, fmt.Errorf("unknown path: %s", req.Path)
    }
    req.Upstream = route.Upstream
    req.Endpoint = string(route.Endpoint)
    req.Capture = route.Capture
    return nil, nil
}

func (s *RoutingStage) After(ctx context.Context, req *ProxyRequest, resp *ProxyResponse, upstreamErr error) {
    // No post-routing work needed.
}
```

#### PolicyStage

```go
type PolicyStage struct {
    store *store.Store
    cache *policyCache
}

func (s *PolicyStage) Name() string { return "policy" }

func (s *PolicyStage) Before(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error) {
    policy, err := s.store.GetPolicy(ctx)
    if err != nil {
        // Fail-open: allow the request, log the error
        return nil, nil
    }
    if !policy.Allowed(req.Model) {
        return &ProxyResponse{StatusCode: 403}, nil // short-circuit
    }
    return nil, nil // continue
}

func (s *PolicyStage) After(ctx context.Context, req *ProxyRequest, resp *ProxyResponse, upstreamErr error) {
    if resp != nil && resp.StatusCode == 403 {
        // Persist blocked request
        s.persistBlocked(ctx, req)
    }
}
```

#### ForwardStage

```go
type ForwardStage struct {
    client  *http.Client
    log     *log.Writer
}

func (s *ForwardStage) Name() string { return "forward" }

func (s *ForwardStage) Before(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error) {
    // This stage IS the upstream call. In the pipeline model, the ForwardStage
    // encapsulates the upstream HTTP request. It's the last stage in the Pre phase.
    // Actually no — this is the *upstream function*, not a stage.
    // See design discussion below.
}
```

**Design note:** The upstream call itself is not a Stage — it's the `upstream`
function passed to `NewPipeline`. This keeps the Stage interface clean (stages
observe, don't execute the core proxy behavior). The ForwardStage would be a
thin wrapper that makes the HTTP call.

#### CaptureStage

```go
type CaptureStage struct {
    store *store.Store
    log   *log.Writer
}

func (s *CaptureStage) Name() string { return "capture" }

func (s *CaptureStage) Before(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error) {
    return nil, nil // nothing before upstream
}

func (s *CaptureStage) After(ctx context.Context, req *ProxyRequest, resp *ProxyResponse, upstreamErr error) {
    if req.Capture == CaptureNone || req.Capture == CaptureLocal {
        return
    }
    if resp == nil {
        return // upstream failed, nothing to capture
    }
    if req.Capture == CaptureUsage && (resp.Usage == nil) {
        return // no usage to capture
    }
    s.persistRequest(ctx, req, resp)
}
```

### 3.5 Assembly

```go
func buildPipeline(log *log.Writer, st *store.Store, project string, cfg *ProxyConfig) (*Pipeline, error) {
    router := NewRouter(cfg)

    // Define the upstream function (the actual HTTP call to the LLM API).
    // For WebSocket, this would be the wsProxy function.
    upstreamFn := func(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error) {
        return makeUpstreamCall(ctx, req)
    }

    pipeline := NewPipeline(upstreamFn,
        &RoutingStage{router: router},
        &PolicyStage{store: st},
        &CaptureStage{store: st, log: log},
    )

    return pipeline, nil
}
```

### 3.6 Integration with http.Handler

The pipeline produces a `Pipeline`, which needs to be an `http.Handler`:

```go
func (p *Pipeline) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Parse request
    body, _ := readAndRestoreBody(r)
    meta := ParseRequestMetadata(body)

    req := &ProxyRequest{
        ID:        atomic.AddUint64(&nextID, 1),
        Method:    r.Method,
        Path:      r.URL.Path,
        Model:     meta.Model,
        Stream:    meta.Stream,
        Headers:   r.Header,
        Body:      body,
        RawReq:    r,
        StartedAt: time.Now(),
        Extra:     make(map[string]any),
    }

    // Execute pipeline
    resp, err := p.Execute(r.Context(), req)

    // Write response to client
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadGateway)
        return
    }
    copyHeaders(w.Header(), resp.Headers)
    w.WriteHeader(resp.StatusCode)
    // resp.Body would need special handling for streaming
}
```

**Streaming challenge:** The current pipeline model returns a complete
`*ProxyResponse`. For streaming responses (SSE, large bodies), we need the
response to arrive as chunks. This requires either:

1. **Buffering the full response** (simple but memory-hungry)
2. **Making ProxyResponse.Body a stream** and passing it through stages
3. **Keeping the current observer pattern** (inline observation during stream)

For Copilot Monitor's current observation-only needs, option 3 is the right
choice. The `After` callback should receive a stream, not a complete body.

### 3.7 Streaming-aware variant

```go
type ProxyResponse struct {
    StatusCode int
    Headers    http.Header
    LatencyMS  int64

    // For streaming: the body is a reader that stages can wrap/observe.
    Body       io.ReadCloser

    // Usage is populated after the body is fully consumed by an observer.
    Usage      *Usage
    Model      string
}

type Stage interface {
    Name() string
    Before(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error)
    // After receives the response with a stream body. Stages can wrap the body
    // to observe or transform chunks. The pipeline drains the body after all
    // stages have had a chance to wrap it.
    After(ctx context.Context, req *ProxyRequest, resp *ProxyResponse, upstreamErr error)
}
```

With this, a CaptureStage wraps `resp.Body` with a reading observer:

```go
func (s *CaptureStage) After(ctx context.Context, req *ProxyRequest, resp *ProxyResponse, upstreamErr error) {
    if resp == nil || req.Capture == CaptureNone {
        return
    }
    // Wrap the body with an SSE observer that extracts usage as chunks flow through.
    observer := NewSSEObserver()
    originalBody := resp.Body
    resp.Body = &observerReader{
        inner:    originalBody,
        observer: observer,
        onDone: func() {
            resp.Usage = observer.Usage
            resp.Model = observer.Model
            s.persistRequest(ctx, req, resp)
        },
    }
}
```

This preserves true streaming — chunks flow through the observer to the client
without buffering. The `onDone` callback fires when the body is fully consumed.

### 3.8 WebSocket support

WebSocket bypasses the pipeline because it's a protocol upgrade, not an HTTP
request/response. The current pattern (hijack → two goroutines → inspect frames)
works and should stay as a separate code path. But it should be called from a
WebSocket-aware pipeline variant:

```go
func (p *Pipeline) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // ... build ProxyRequest ...

    if isWebSocketUpgrade(r) {
        p.handleWebSocket(w, r, req)
        return
    }

    resp, err := p.Execute(r.Context(), req)
    // ...
}
```

### 3.9 Configuration-driven pipelines

The pipeline can be assembled from config, not hardcoded. This is the bridge to
the Headroom integration use case:

```json
{
  "pipeline": {
    "stages": [
      { "type": "routing", "routes_config": "routes.json" },
      { "type": "policy" },
      {
        "type": "capture",
        "db_path": "~/.local/share/copilot-monitor/store.db"
      }
    ]
  }
}
```

Or programmatically:

```go
builder := NewPipelineBuilder(upstreamFn).
    WithRouting(cfg).
    WithPolicy(store).
    WithCapture(store, log)

// In the Headroom integration, the pipeline could also include:
builder.WithCompression(headroomClient) // future stage

pipeline := builder.Build()
```

## 4. Integration with Headroom

### 4.1 The current problem

Today, to get both compression (Headroom) and observability (Copilot Monitor),
you run two full proxy processes:

```
Tool → copilot-monitor:7733 → headroom:8787 → LLM API
```

This works but has dual configs, dual ports, dual process management.

### 4.2 The pipeline solution

With a pipeline architecture, a single process can run both stages:

```
Tool → copilot-monitor:7733
         ├─ RoutingStage: match path, set upstream = localhost:8787
         ├─ PolicyStage: check allow/deny
         ├─ ForwardStage: call Headroom proxy at 127.0.0.1:8787
         │   (Headroom compresses, forwards to LLM API)
         ├─ CaptureStage: observe response, extract usage
         └─ Client ← response
```

But this still has Headroom as a separate process. The **ideal** integration
would be Headroom running as a **pipeline stage within the same process**:

```
Tool → copilot-monitor:7733
         ├─ RoutingStage
         ├─ PolicyStage
         ├─ CompressStage (Headroom logic, in-process)
         ├─ ForwardStage (to LLM API)
         ├─ CaptureStage (observe compressed response)
         └─ Client ← response
```

This requires Headroom's compression logic to be callable as a Go library, which
it currently is not (it's Python). Options:

| Option                                                                                | Latency | Complexity | Feasibility       |
| ------------------------------------------------------------------------------------- | ------- | ---------- | ----------------- |
| **Unix socket callout**: Go proxy calls Headroom via Unix socket                      | +100μs  | Medium     | Today             |
| **Headroom as sidecar**: Headroom runs as a small process, proxy calls it per-request | +500μs  | Low        | Today             |
| **WASM plugin**: Compile Headroom's compression to WASM, run in Go                    | +50μs   | High       | Research required |
| **Go port of compression**: Rewrite SmartCrusher/CodeCompressor in Go                 | +10μs   | Very high  | Not practical     |
| **CGo bridge**: Call Python from Go via CGo                                           | +200μs  | High       | Brittle           |

**Recommendation:** The Unix socket callout pattern is the pragmatic choice.
Headroom exposes a small HTTP service on a Unix socket. The CompressStage calls
it for compression. This keeps Headroom's Python ecosystem intact while adding
minimal latency (~100μs on loopback Unix socket).

### 4.3 The CompressStage

```go
type CompressStage struct {
    socketPath string  // /tmp/headroom-compress.sock
    client     *http.Client
}

func (s *CompressStage) Name() string { return "compress" }

func (s *CompressStage) Before(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error) {
    // Send request body to Headroom's compression endpoint
    compressReq, _ := http.NewRequestWithContext(ctx, "POST", "http://unix/compress", bytes.NewReader(req.Body))
    compressReq.Header = cloneHeaders(req.Headers)

    resp, err := s.client.Do(compressReq)
    if err != nil {
        // Fail-open: continue without compression if Headroom is down
        return nil, nil
    }
    defer resp.Body.Close()

    compressed, _ := io.ReadAll(resp.Body)

    // Replace request body with compressed version
    req.Body = compressed
    req.Extra["compressed"] = true
    req.Extra["original_size"] = len(req.Body)

    return nil, nil
}
```

## 5. Comparison of Pipeline Approaches

### 5.1 `http.Handler` wrapping (Alice/chi)

```
func(http.Handler) http.Handler
```

- **Pro**: Standard Go, easy to adopt, works with existing routers
- **Con**: No explicit response phase, must wrap ResponseWriter, awkward for
  streaming
- **Best for**: Simple middleware (logging, auth, CORS)
- **Not for**: Proxy pipelines with post-upstream observation

### 5.2 Explicit Stage interface (recommended)

```
Stage { Before(); After() }
```

- **Pro**: Clear pre/post phases, testable, composable, streaming-aware variant
  possible
- **Con**: Custom interface, not standard Go middleware
- **Best for**: Proxy pipelines with distinct pre-upstream and post-upstream
  work
- **This is the recommended approach for Copilot Monitor**

### 5.3 Envoy-style filter chain

```
Filter { decodeHeaders(); decodeData(); encodeHeaders(); encodeData() }
```

- **Pro**: Maximum control, streaming body transformation, buffer control
- **Con**: Complex, overkill for observation-only, requires buffering
  architecture
- **Best for**: Proxies that transform bodies (compression, format conversion)
- **Adopt when**: You need to modify streaming bodies inline

### 5.4 Tower-style Service + Layer

```
Service { call(Request) → Future<Response> }
Layer { layer(Service) → Service }
```

- **Pro**: Type-safe, zero-cost, factory/instance separation
- **Con**: Requires generics for Go approximation, unfamiliar to most Go
  developers
- **Best for**: When you need per-request service instantiation (tracing spans,
  etc.)

### 5.5 MaaS (Middleware as a Service)

```
Proxy → HTTP/Unix socket → External service → Proxy
```

- **Pro**: Language independence, failure isolation, independent scaling
- **Con**: Serialization overhead, operational complexity, network dependency
- **Best for**: Stages with heavy dependencies (ML, Python) or independent
  lifecycles
- **Use sparingly**: Only when a stage can't run in-process

## 6. Migration Path

### Phase 1: Extract stages from the monolith (no interface change)

Extract the existing inline logic into named functions/stages without changing
the handler's external behavior:

```go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    id := h.nextID.Add(1)
    started := time.Now().UTC()

    body, meta, err := h.parseRequest(r)
    if err != nil {
        h.log.Error(...)
        return
    }

    route, ok := h.router.MatchModel(r.URL.Path, meta.Model)
    if !ok {
        h.log.Error(...)
        return
    }

    if route.Local && route.Endpoint == EndpointPing {
        w.WriteHeader(http.StatusOK)
        return
    }

    allowed := h.checkPolicy(r.Context(), meta.Model)
    if !allowed {
        h.persistBlockedRequest(r.Context(), started, route, r, meta)
        w.WriteHeader(http.StatusForbidden)
        return
    }

    if isWebSocketUpgrade(r) {
        h.proxyWebSocket(id, w, r, route, body)
        return
    }

    outReq, err := MakeUpstreamRequest(r, route, body)
    resp, err := h.client.Do(outReq)
    latencyMS := time.Since(started).Milliseconds()

    observer := newResponseObserver(route, resp.Header.Get("Content-Type"))
    streamResponse(w, resp.Body, observer)
    h.persistRequest(r.Context(), started, route, r, meta, resp.StatusCode, latencyMS, observer)
}
```

This is already well-factored. The change is extracting `checkPolicy()` and
`persistRequest()` into named methods, which is done. The key insight from this
research: **don't add a pipeline interface until you have a second pipeline
configuration to support**. The current monolithic handler with well-named
helper methods is the right architecture for today.

### Phase 2: Add the Stage interface (when needed)

When a second pipeline configuration is needed (e.g., Headroom compression
before/after, or a user wants custom stages), introduce the `Stage` interface
and refactor the handler to use it. The existing behavior becomes the default
pipeline. Custom pipelines can add/remove/reorder stages.

### Phase 3: Config-driven pipelines (future)

When stage selection needs to be user-configurable (not just developer-coded),
add a pipeline configuration format (JSON, YAML) that defines which stages run
in which order. This enables:

```json
{
  "pipeline": {
    "stages": [
      { "type": "routing" },
      { "type": "policy" },
      { "type": "compress", "socket": "/tmp/headroom-compress.sock" },
      { "type": "capture" }
    ]
  }
}
```

### Phase 4: External stage services (distant future)

When stages need independent lifecycles (e.g., ML-based content filtering in
Python), support Unix socket callouts. This should be an escape hatch, not the
default — in-process stages are simpler, faster, and easier to debug.

## 7. Recommendations

### 7.1 For Copilot Monitor today

**Don't add a pipeline interface yet.** The current monolithic handler is:

1. **Correct**: It handles HTTP, SSE, WebSocket, policy, and capture correctly.
2. **Tested**: The existing test suite covers the current architecture.
3. **Lightweight**: 15MB RSS, sub-50ms startup, single binary.
4. **Well-factored**: Each concern is in its own file (`capture.go`, `sse.go`,
   `websocket.go`, `router.go`, `policy/`).

The next refactoring step should be **extracting the policy check** into a
standalone `PolicyChecker` struct (it's already mostly there with
`internal/policy/`). After that, the observer/persistence logic into a
`RequestRecorder`.

### 7.2 For the Headroom integration (what to build)

The most impactful near-term work is **documentation and tooling** for chaining
proxies externally (see the Headroom integration plan). This delivers value
immediately with zero code changes.

The second most impactful is adding a **`--upstream` flag** to
`copilot-monitor run` that sends all traffic to a single host. This makes the
"monitor before Headroom" or "monitor after Headroom" setup trivial — no
routes-config needed.

### 7.3 When to build the pipeline interface

Build it when you need **one of these**:

1. **A second pipeline configuration** — e.g., "compress then observe" vs
   "observe then compress" are both valid use cases.
2. **User-provided stages** — via config file or plugin.
3. **Stage reordering** — users want to run policy before or after compression.
4. **External stage services** — a stage needs its own process (Python, ML
   runtime).

Until then, the monolithic handler with well-factored helper functions is the
right architecture for a single-purpose, lightweight, local proxy.

### 7.4 The "pipeline spectrum" principle

```
Monolithic handler  ──►  Named stages/helpers  ──►  Stage interface  ──►  Config-driven  ──►  External services
      ↑                          ↑                        ↑                    ↑                  ↑
   We were here            We are here              Phase 2 milestone    Phase 3            Phase 4
  (1 week ago)            (already done)            (when needed)      (future)           (distant)
```

Move right on the spectrum only when the current position is causing concrete
pain. Each step adds flexibility but also complexity, test surface, and mental
overhead. The current position (named stages/helpers with a monolithic handler)
is the sweet spot for a single-purpose local developer tool.

## 8. Appendix: Anti-patterns to Avoid

### 8.1 The "generic plugin system" trap

Don't build a plugin registry, module system, or dynamic loader until you have
at least 3 distinct third-party plugins. Caddy and Traefik needed these because
they have ecosystems of 100+ plugins. Copilot Monitor has one codebase and one
team. A simple `[]Stage` slice is a more honest representation of the need.

### 8.2 The "YAML DSL" trap

Don't design a pipeline configuration language (YAML/JSON) until the pipeline
has stabilized across multiple releases. Configuration drift between the config
format and the code is a source of bugs and user confusion. Start with
code-based pipeline assembly and add config later.

### 8.3 The "framework" trap

Don't extract a reusable "proxy pipeline framework" from Copilot Monitor. The
pipeline is specific to this domain (LLM API observation) and this trust model
(local, loopback, single-user). Generalizing it for other use cases
(multi-tenant gateways, content filtering, caching) would add complexity without
benefiting the primary use case.

### 8.4 The "async everything" trap

Don't make every stage async. LiteLLM and Helicone use async logging because
they run at scale (thousands of RPS) with remote backends. Copilot Monitor runs
locally on a developer machine handling <10 RPS. Sync writes to SQLite are
perfectly fine and simpler to reason about. If SQLite writes become a bottleneck
(they won't), batching is simpler than async queues.

### 8.5 The "HTTP everywhere" trap

Don't use HTTP for inter-stage communication within the same process. A function
call with typed parameters is faster, safer, and easier to debug than an HTTP
request to `localhost:9xxx`. HTTP is only appropriate when stages run in
separate processes (Headroom compression in Python, for example).
