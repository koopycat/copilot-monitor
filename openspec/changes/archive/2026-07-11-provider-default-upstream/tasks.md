## 1. Config support for provider default routes

- [x] 1.1 Modify `RouteConfig` struct to make `Path` optional (pointer or
      omitempty semantics) and add `isProviderDefault()` helper
- [x] 1.2 Update `Validate()` to allow routes without `path`: a route with no
      path MUST have `provider` and `upstream_host` set; a route with `path`
      follows existing validation
- [x] 1.3 Add validation: warn on duplicate provider defaults (same provider, no
      path); first wins per insertion order
- [x] 1.4 Add config validation tests for provider defaults (valid default,
      missing upstream_host, path+default conflict cases)

## 2. Router default-route matching

- [x] 2.1 Add `defaultRoutes map[string]Route` field to `Router` struct, keyed
      by provider
- [x] 2.2 Populate `defaultRoutes` in `NewRouter` when a `RouteConfig` has no
      `path` field
- [x] 2.3 Add fallback logic in `MatchModel`: after both exact and prefix entry
      loops find nothing, check `defaultRoutes[provider]`; return it with the
      original path
- [x] 2.4 Ensure default route fields (capture, upstream_path_prefix,
      not_billed) are inherited by matched requests

## 3. Tests

- [x] 3.1 Add `MatchModel` unit tests: provider default catches unmatched path,
      specific path takes precedence, different provider default does not match
- [x] 3.2 Add integration test: proxy uses provider default route for unknown
      path
- [x] 3.3 Ensure existing routing tests still pass

## 4. Example configs

- [x] 4.1 Simplify `examples/routes/github-copilot.json` to use a provider
      default route for copilot, keeping only routes that need specific
      overrides (e.g., `not_billed: true` completions paths, local `_ping`)
