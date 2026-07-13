## Context

The proxy currently applies its model policy only after it has received a
request containing a selected model ID. OpenAI-compatible clients commonly
discover models with `GET /models` before creating requests, so they can present
model IDs that the proxy will later reject. Pi's Kilo provider follows this
discovery pattern when its base URL targets the proxy.

The policy remains the central security control because it is evaluated inside
the proxy, independent of client `settings.json` files. The implementation must
preserve response streaming for all non-discovery traffic.

## Goals / Non-Goals

**Goals:**

- Make the proxy's policy the authoritative source for both advertised and
  request-permitted model IDs.
- Filter standard OpenAI-compatible successful `GET /models` JSON responses with
  the same exact and prefix-wildcard matching rules used at request time.
- Retain request-time enforcement so direct requests and stale client caches
  cannot bypass the policy.
- Fail open without changing the upstream discovery response if policy loading
  or response parsing fails.

**Non-Goals:**

- Rewrite local Pi settings or model catalogs on disk.
- Filter nonstandard discovery endpoints such as session metadata or
  provider-specific agent catalogs.
- Buffer streaming responses or alter upstream model metadata beyond omitting
  disallowed entries.
- Make a direct client route through the proxy.

## Decisions

### Filter at the proxy's `GET /models` response boundary

For a successful JSON response to the standard OpenAI-compatible `GET /models`
endpoint, the proxy will read the bounded discovery payload, remove `data`
entries whose string `id` is not permitted by the active policy, then return the
original envelope with the filtered `data` array. The policy check happens
before headers are committed.

This centralizes model visibility for every OpenAI-compatible client and lets
Pi's provider consume the already-authoritative list. Filtering the Pi extension
instead was rejected because it would leave all other clients inconsistent and
would duplicate security logic outside the enforcement boundary.

### Reuse one policy lookup and matching path

A handler helper will retrieve the cached policy using the existing five-second
cache semantics and expose an allow decision for both discovery and request
forwarding. A policy in `allowlist` includes only matching IDs, a `blocklist`
excludes matching IDs, and `allow_all` leaves all entries visible.

This avoids divergence between model discovery and request-time matching.

### Fail open for discovery transformation

If the policy cannot be loaded and no usable cache exists, if the upstream
response is not a successful JSON OpenAI model-list envelope, or if the
discovery payload cannot be decoded, the proxy will forward the upstream
response unchanged. The request-time policy remains the final control when a
valid policy is available.

This retains the existing fail-open policy contract and prevents a discovery
parsing problem from breaking provider startup.

## Risks / Trade-offs

- [A model client returns a large model-list document] → Buffer only successful
  `GET /models` JSON discovery responses with a strict size limit; stream all
  other responses unchanged.
- [An upstream uses a nonstandard discovery envelope] → Leave the response
  unchanged and rely on request-time policy enforcement.
- [A client caches an old model list] → Keep request-time enforcement and return
  the established `403 model_blocked` response.
- [Policy changes while a client has a cached list] → Apply the existing short
  policy-cache TTL to discovery and enforce the latest available policy at
  request time.

## Migration Plan

1. Deploy with the existing default `allow_all` policy, which preserves current
   discovery results.
2. On the next discovery request after an allowlist or blocklist is configured,
   clients receive the filtered list.
3. No data migration or configuration update is required.
4. Roll back by deploying the previous binary or changing the policy to
   `allow_all`; request-time enforcement remains unchanged.

## Open Questions

None.
