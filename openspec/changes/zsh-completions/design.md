## Context

The CLI is a hand-rolled switch statement in `internal/cli/root.go`. There is no
command framework (cobra, urfave/cli, etc.). Adding completions means either
generating a zsh script dynamically at runtime or embedding a static one.

The CLI has 13 top-level subcommands. Some accept flags (`run`, `serve`,
`stats`, `cost`, `today`, `sessions`, `live`, `export`). Flags are defined per
command in their respective `run*` functions using a simple ad-hoc flag-parsing
approach with `flag.FlagSet`.

## Goals / Non-Goals

**Goals:**

- Provide zsh completions for all top-level subcommands
- Provide flag completions for commands that accept flags
- Keep the implementation simple and maintainable (hand-rolled, no dependency)
- Composable output (stdout, no banners)

**Non-Goals:**

- bash or fish completions (only zsh)
- Dynamic completion based on runtime state (e.g., listing available projects
  from the database)
- Auto-install of completions (user is responsible for sourcing the output)
- Flag value completions beyond fixed enums (e.g., `--log-format` values)

## Decisions

### Decision 1: Static embedded script vs. library-based generation

**Chosen: Static zsh script embedded as a Go string constant.**

Alternatives evaluated:

| Approach                                           | Deps                   | Runtime cost                                          | Maintenance                        | Suitability                                                                                                          |
| -------------------------------------------------- | ---------------------- | ----------------------------------------------------- | ---------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| **cobra** `GenZshCompletion()`                     | Heavy (full framework) | Generates at build or runtime                         | Auto-updates from command tree     | Overkill -- would require rewriting the entire CLI as cobra commands                                                 |
| **posener/complete** (github.com/posener/complete) | 1 dep (pure Go)        | Generates completions dynamically at shell invocation | Auto-updates from command tree     | Best library fit -- lightweight, designed for any Go CLI. Adds a dependency and small runtime overhead per TAB press |
| **carapace** (carapace-sh/carapace)                | Medium (spec engine)   | Reads spec at shell invocation                        | Declarative spec file              | Very powerful but complex; overkill for 13 static commands                                                           |
| **Static embedded string**                         | None                   | Zero                                                  | Manual update when commands change | Simplest; aligns with project's zero-dependency philosophy. 13 commands change infrequently                          |

Why static over posener/complete: posener/complete is the best library fit, but
(a) it is not widely adopted (~1k GitHub stars, niche in the Go ecosystem), and
(b) for 13 commands that rarely change, the dependency and runtime cost don't
justify themselves. The project's dependency bar requires battle-proven,
well-maintained, widely-used libraries. posener/complete doesn't clear that bar
for what amounts to a static string generator.

Why not cobra: Adopting cobra solely for completions would be a massive refactor
of the entire CLI (all command handlers, flag parsing, help text). The project
intentionally uses a hand-rolled switch to avoid framework lock-in and keep the
binary small.

### Decision 2: Script structure

**Chosen: Single `_copilot-monitor` function with `_arguments` calls.**

The script defines a top-level `_copilot-monitor` function that uses zsh's
`_arguments` builtin with subcommands. Each subcommand with flags uses a nested
`_arguments` call. Commands without flags (`version`, `help`, `init`) need only
name registration.

### Decision 3: Where the command lives

**Chosen: New file `internal/cli/completion.go`.**

A new `runCompletion` function handles the `completion` subcommand, dispatched
from the root switch. This keeps concerns separated without modifying existing
files beyond the root switch.

## Risks / Trade-offs

- **Staleness risk**: Adding a new subcommand or flag requires remembering to
  update the completion string. Mitigation: include a comment in `completion.go`
  and a note in AGENTS.md about this requirement.
- **No flag value completions for dynamic data**: e.g., `--project` values can't
  be auto-completed from the DB. Mitigation: out of scope for now; can revisit
  with dynamic generation if needed.
