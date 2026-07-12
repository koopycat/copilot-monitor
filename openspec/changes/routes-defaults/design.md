## Context

Currently `copilot-monitor run` requires `--routes-config` and exits with an
error if omitted. Users must create a routes JSON file before their first run.
The built-in Copilot route config in `examples/routes/github-copilot.json`
already covers the primary use case. The startup error is unnecessary ceremony.

## Goals / Non-Goals

**Goals:**

- Make `copilot-monitor run` work with zero configuration
- Provide sensible built-in defaults (GitHub Copilot API endpoints)
- Let users inspect defaults with `--routes-config-defaults`
- When `--routes-config` is provided, use it exclusively (replacement, not
  merge)

**Non-Goals:**

- Merging user routes with built-in defaults
- Runtime reloading of route configuration
- Supporting non-Copilot providers in the defaults
- Auto-detecting the user's provider from environment

## Decisions

### Built-in defaults live in Go code, not an embedded JSON file

**Why**: The defaults are small (~15 routes) and static. Go struct literals are
type-checked at compile time, require no JSON parsing, and can't silently drift
from the spec. `examples/routes/github-copilot.json` remains as the
human-readable reference that mirrors the Go code.

**Alternative considered**: `//go:embed defaults.json`. Rejected because
embedding a JSON file adds a runtime parse step that can fail, and the defaults
never change without a code change anyway.

### Replacement, not merge

**Why**: When a user provides `--routes-config`, they have an explicit intent.
Merging with defaults creates ambiguity — did the user intend to keep Copilot
routes alongside their custom ones? Replacement is predictable: "you asked for
these routes, you get exactly these routes."

**Alternative considered**: Merge user routes on top of defaults, with user
routes taking precedence. Rejected because it violates the principle of least
surprise — adding a route for one provider shouldn't silently keep routes for
another.

### `--routes-config-defaults` flag for discovery

**Why**: Users need to see what the defaults are before they can customize them.
A flag that prints the defaults as JSON lets users pipe to a file, edit it, and
use it with `--routes-config`. Follows the `copilot-monitor configure-vscode`
pattern of printing a snippet users can adapt.

### Defaults match `examples/routes/github-copilot.json` exactly

**Why**: The example file is already the canonical Copilot config. The built-in
defaults are the programmatic equivalent. This avoids having two different
"default" configurations.

## Risks / Trade-offs

- **Copilot API changes upstream URLs** → Defaults become stale. Mitigation:
  users can always override with `--routes-config`. The examples file and
  built-in defaults are updated together in the same commit.
- **User expects defaults to merge with `--routes-config`** → Confusion when
  their config replaces defaults entirely. Mitigation: startup banner clearly
  states "using routes from config file" vs "using built-in default routes".
- **New users don't know defaults exist** → They create unnecessary config
  files. Mitigation: `--routes-config-defaults` is discoverable via `--help` and
  the startup banner mentions it.
