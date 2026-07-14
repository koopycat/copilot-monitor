## Context

The landing page already presents release downloads, product value, and a
source-build procedure in one static HTML document with inline CSS and
JavaScript. The current source-build procedure also contains the only VS Code
configuration example, and no equivalent landing-page path exists for pi. The
page uses a dark forensic visual system, progressive enhancement, and no
frontend runtime dependency.

## Goals / Non-Goals

**Goals:**

- Put the shortest path to a first captured request directly after the hero and
  release downloads.
- Let visitors switch between complete VS Code and pi procedures without
  displaying both procedures at once.
- Preserve the existing visual language, responsive behavior, and
  dependency-free static delivery.
- Keep both procedures readable when JavaScript is unavailable.

**Non-Goals:**

- Document every supported provider, route format, installation method, or pi
  authentication flow.
- Replace the detailed README or route examples.
- Add a reusable component framework or external tab library.
- Change proxy, routing, or dashboard behavior.

## Decisions

### Use one progressive-enhancement tab shell

The section will use a semantic tab list with two buttons and corresponding tab
panels. A small inline script will handle click, Arrow Left, Arrow Right, Home,
and End interactions while maintaining `aria-selected` and roving `tabindex`
state. Without JavaScript, the tab controls will be absent and both labeled
procedures will remain visible in document order.

A disclosure element was considered, but tabs better express mutually exclusive
client choices while using less vertical space. Separate cards were rejected
because they would display duplicate startup information and increase visual
clutter.

### Keep each client procedure complete

Each panel will contain the full minimum path for that client rather than
combining a shared step with conditional exceptions. VS Code can use the
built-in Copilot routes, while pi through Kilo requires a route configuration,
so a nominally shared startup step would be misleading. Each path ends with one
real request and the local dashboard URL, which is the onboarding success
moment.

### Reuse the existing page system

The onboarding shell will reuse the page's colors, radii, typography, terminal
treatment, and spacing scale. The implementation will add only styles required
for the tab bar, ordered steps, and compact command blocks. The existing
build-from-source section will be reduced to build instructions so client setup
is not duplicated.

### Use the tested pi-through-Kilo path

The pi panel will use the documented and previously verified Kilo route example
and `KILO_GATEWAY_BASE_URL` override. It will download the maintained route
example into the default configuration location, validate it, start Copilot
Monitor, and launch pi through the `/kilo` prefix. This avoids presenting an
unverified provider override as the primary path.

### Isolate browser-test servers from development servers

Validation revealed that the E2E suite shared Vite's normal port and started its
fixture API on a different port from Vite's configured proxy target. The Vite
development configuration will accept optional environment overrides for its
port and API target while retaining the existing defaults. The E2E configuration
will use dedicated ports and pass the fixture API target explicitly, preventing
normal dashboard development from blocking or contaminating the suite.

## Risks / Trade-offs

- [The pi path requires an authenticated Kilo-capable pi installation] -> State
  that prerequisite in one short sentence and link detailed documentation rather
  than expanding the panel.
- [Raw GitHub content must remain reachable for first-time setup] -> Use the
  maintained repository route example and keep the full configuration available
  in the README for manual fallback.
- [Tabs can hide information from keyboard or assistive-technology users] ->
  Implement standard ARIA tab semantics, roving focus, and a no-JavaScript
  fallback that displays both procedures.
- [Long commands can overflow narrow screens] -> Keep command containers
  horizontally scrollable and verify narrow viewport behavior.
