# Specification Index

Requirements are managed through OpenSpec. Behavioral requirements live in
`openspec/specs/<capability>/spec.md`. Non-behavioral design constraints live in
`openspec/config.yaml`.

## Capabilities

| Capability  | File                                 | Scope                                                                                                       |
| ----------- | ------------------------------------ | ----------------------------------------------------------------------------------------------------------- |
| proxy       | `openspec/specs/proxy/spec.md`       | Core proxy behavior, WebSocket, health, shutdown, request IDs, buffer limits, startup validation, live tail |
| routing     | `openspec/specs/routing/spec.md`     | Single --upstream forwarding, local endpoint reservation, WebSocket upgrade                                 |
| capture     | `openspec/specs/capture/spec.md`     | Usage metadata, session grouping, headroom-proxied flag, debug logging, structured logging                  |
| policy      | `openspec/specs/policy/spec.md`      | Model allow/block, fail-open, API management, model discovery                                               |
| compression | `openspec/specs/compression/spec.md` | Headroom-proxied detection via RemoteAddr, no inline compression                                            |
| reporting   | `openspec/specs/reporting/spec.md`   | CLI commands, dashboard API, filtering, export, init, validate                                              |
| dashboard   | `openspec/specs/dashboard/spec.md`   | Dashboard UI, metrics, charts, policy management, routes display                                            |
| privacy     | `openspec/specs/privacy/spec.md`     | Data minimization, locality, loopback binding, export boundaries                                            |

## Workflow

```text
/opsx:propose "description"  →  create change with proposal, specs, design, tasks
/opsx:apply                  →  implement tasks
/opsx:archive                →  merge delta specs into openspec/specs/
```

## Documentation

| Document               | Scope                                                                         |
| ---------------------- | ----------------------------------------------------------------------------- |
| `README.md`            | User quickstart, local smoke test, commands, and privacy summary              |
| `docs/architecture.md` | Request lifecycle, package map, schema notes, and implementation traceability |
| `docs/api.md`          | Read-only HTTP API and embedded dashboard reference                           |
| `AGENTS.md`            | Contributor and agent workflow rules                                          |
| `PRODUCT.md`           | Product intent, audience, and design principles                               |
