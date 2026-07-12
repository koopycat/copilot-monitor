## Context

Currently the startup logic is binary: `--routes-config` provided → load from
file; not provided → built-in defaults. The `init` command creates a routes file
at `~/.config/copilot-monitor/routes.json`, but `run` ignores it unless
explicitly passed.

## Goals / Non-Goals

**Goals:**

- Auto-detect `~/.config/copilot-monitor/routes.json` when no `--routes-config`
- Fall back to built-in defaults if the file doesn't exist or is invalid
- Never fail at startup because of a missing or broken default config file

**Non-Goals:**

- Watching the config file for changes
- Merging default config with built-in defaults
- Supporting a different default path

## Decisions

### Three-tier fallback: explicit flag → XDG default → built-in

**Why**: Users who run `init` get a seamless experience — `run` picks up their
config automatically. Users who never ran `init` still get built-in Copilot
defaults. Users who want a custom path still use `--routes-config`.

**Alternative considered**: Always load `~/.config/copilot-monitor/routes.json`
and fail if it's invalid. Rejected because that would break the zero-config
promise for users who never ran `init`.

### Silent fallback on invalid default config

**Why**: A malformed default config file could be from a failed `init` or manual
edit. Failing at startup would block the user from even running the proxy. A
warning log entry is sufficient.

## Risks / Trade-offs

- **User doesn't know their config is being loaded** → Mitigation: startup
  banner shows source ("using routes from config file" or "using built-in
  default routes")
- **Stale config file silently overrides updated built-in defaults** → Users who
  ran `init` once and forgot will use their old config. Acceptable: `init`
  writes the Copilot defaults, which match built-in defaults.
