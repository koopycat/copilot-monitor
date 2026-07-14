---
description: Run an OpenSpec change end to end - propose, implement, validate, sync, and archive
argument-hint: "<change description or active change name>"
---

Run the complete OpenSpec lifecycle for:

$@

## Operating mode

Run every phase in this session and continue automatically between phases.
This invocation authorizes creating or resuming the exact change, implementing it, updating its main specs, and archiving it after every completion gate passes.
Treat phase summaries and suggested next commands as progress, not stop points.
Keep progress updates brief and reserve questions for decisions that cannot be inferred and would materially affect user-visible behavior, security, data compatibility, or destructive scope.
For ordinary technical choices, prefer the most robust and maintainable option consistent with the repository instructions and OpenSpec artifacts.

## Workflow

1. Resolve the change.
   - If the provided argument exactly matches an active change, resume that change from its first incomplete phase.
   - Otherwise, follow the `openspec-propose` skill to derive a name, create the change, and generate every artifact required for implementation.
   - If no argument was provided, ask for the change description.
   - Restrict all later phases to the resolved change and preserve any resolved `--store <id>` option.

2. Implement the change.
   - Follow the `openspec-apply-change` skill and complete every remaining task.
   - Read all context files returned by `openspec instructions apply` before editing code.
   - Keep planning artifacts coherent when implementation reveals a necessary correction.
   - Mark each task complete immediately after its implementation and verification have succeeded.
   - Diagnose and repair implementation, test, lint, and formatting failures before treating them as blockers.

3. Pass every completion gate.
   - Confirm that every required artifact is `done` using `openspec status --change "<name>" --json`.
   - Confirm that `openspec instructions apply --change "<name>" --json` reports `state: "all_done"`.
   - Run the repository-prescribed final checks plus any integration or end-to-end tests required by the affected behavior.
   - Run `openspec validate "<name>" --type change --strict --no-interactive`.
   - Re-run failed checks after repairs until they pass or a genuine blocker remains.

4. Sync and archive.
   - After every gate passes, run `openspec archive "<name>" -y --json` with the resolved store option when applicable.
   - Use the command's default validation and main-spec update behavior.
   - Verify that the command succeeded and report the resulting archive location.

## Stop and recovery behavior

Stop when the user interrupts, a material product decision needs input, or a failure cannot be resolved safely.
When stopped, keep the change active and preserve the visible working tree so the work can resume directly.
Report the completed phases and tasks, the exact blocker or failing command, and the next command needed to continue.
Archive only after every artifact, task, repository check, and OpenSpec validation has completed successfully.

## Final response

Report:

- Change name
- Implemented behavior
- Validation commands and results
- Archive location and spec-sync result
- Any noteworthy decisions
