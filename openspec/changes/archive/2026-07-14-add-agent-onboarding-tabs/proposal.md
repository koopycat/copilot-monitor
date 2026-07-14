## Why

The landing page explains the product but does not give developers a short,
obvious path from installation to their first captured request. VS Code setup is
buried in the build-from-source procedure, while pi setup is absent, so the two
primary onboarding paths should be visible without making the page denser.

## What Changes

- Add a compact getting-started section to the landing page that separates
  shared startup steps from agent-specific configuration.
- Provide accessible, switchable setup tabs for VS Code with GitHub Copilot and
  pi.
- Keep each path focused on the minimum commands and configuration required to
  send a first request through Copilot Monitor and open the dashboard.
- Preserve useful setup content when JavaScript is unavailable and maintain
  responsive, keyboard-accessible behavior.
- Review the finished section for information hierarchy, accessibility,
  responsive behavior, and visual consistency with the existing landing page.

## Capabilities

### New Capabilities

- `website-onboarding`: Minimal, agent-specific onboarding guidance on the
  public landing page.

### Modified Capabilities

None.

## Impact

- Updates the GitHub Pages landing page and its inline styles and interaction
  script.
- Adds no runtime dependency and does not affect the proxy, dashboard API, or
  persisted data.
- Requires visual and interaction verification across desktop and narrow
  viewports.
- Isolates the existing dashboard E2E servers from normal local development
  after validation exposed a port and API-target conflict.
