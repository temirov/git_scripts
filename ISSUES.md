# ISSUES (Append-only Log)

Entries record newly discovered requests or changes, with their outcomes. No instructive content lives here. Read @NOTES.md for the process to follow when fixing issues.

## Features (120–199)


## Improvements (200–299)

- [ ] [GX-200] Remove `--to` flag from default command and accept the new branch as an argument, e.g. `gix b default master`
- [ ] [GX-201] Identify non-critical operations and turn them into warnings, which do not stop the flow:
```
14:58:29 tyemirov@computercat:~/Development/Poodle/product_page_analysis.py [main] $ gix b default --to master
default branch update failed: GitHub Pages update failed: GetPagesConfig operation failed: gh command exited with code 1
```
The Pages may be not configured and that's ok
- [ ] [GX-202] have descriptive and actionable error messages, explaining where was the failure and why the command failed:
```
14:56:43 tyemirov@computercat:~/Development/Poodle $ gix --roots . b cd master
failed to fetch updates: git command exited with code 128
```
If aa repository doesnt have a remote, there is nothing to fetch, but we can still change the default branch, methinks

## BugFixes (300–399)


## Maintenance (400–499)

- [x] [GX-400] Update the documentation @README.md and focus on the usefullness to the user. Move the technical details to @ARCHITECTURE.md
- [x] [GX-401] Ensure architrecture matches the reality of code. Update @ARCHITECTURE.md when needed
  - Resolution: `ARCHITECTURE.md` now documents the current Cobra command flow, workflow step registry, and per-package responsibilities so the guide mirrors the Go CLI.
- [x] [GX-402] Review @POLICY.md and verify what code areas need improvements and refactoring. Prepare a detailed plan of refactoring. Check for bugs, missing tests, poor coding practices, uplication and slop. Ensure strong encapsulation and following the principles og @AGENTS.md and policies of @POLICY.md
  - Resolution: Authored `docs/policy_refactor_plan.md` detailing domain-model introductions, error strategy, shared helper cleanup, and new test coverage aligned with the confident-programming policy.

## Planning 
do not work on the issues below, not ready

    - [ ] [GX-22] Implement adding licenses to repos. The prototype is under tools/licenser
    - [ ] [GX-23] Implement git retag, which allows to alter git history and straigtens up the git tags based on the timeline. The prototype is under tools/git_retag
    - Roadmap:
        Phase A – Domain Modeling
            1. Introduce smart constructors for repository identifiers (path, owner, repo, remote URL, remote name, branch, protocol) under `internal/repos/domain` or extend `shared`.
            2. Replace raw string usage across `internal/repos` and `internal/workflow` with the new domain types.
            3. Update CLI builders (`cmd/cli/repos`, `cmd/cli/workflow`) to validate inputs once and emit the domain types.
        Phase B – Error Strategy
            1. Define sentinel errors (e.g., `ErrUnknownProtocol`, `ErrCanonicalOwnerMissing`) and helpers that wrap them with operation+subject codes.
            2. Refactor executors (`remotes`, `protocol`, `rename`, `history`) to return contextual errors; move user messaging to CLI reporters.
            3. Adapt tests to assert on wrapped errors rather than stdout strings.
        Phase C – Service Cleanup
            1. Centralize owner/repo parsing and output/prompt helpers for reuse.
            2. Remove duplicated `strings.TrimSpace` validation by trusting normalized domain types.
            3. Review boolean flags (`AssumeYes`, `RequireCleanWorktree`) and replace with sum-types where behaviors diverge.
        Phase D – Testing Expansion
            1. Add table-driven unit tests for the new constructors and protocol conversion edge cases.
            2. Cover dependency resolver helpers in `internal/repos/dependencies`.
            3. Extend workflow integration tests to ensure domain types propagate correctly.
        Phase E – Documentation & Tooling
            1. Document new domain types and error codes (`docs/refactor_status.md` or `POLICY.md` addendum).
            2. Update `docs/cli_design.md` with edge-validation guidance.
            3. Extend CI to include `staticcheck` and `ineffassign` alongside `go test ./...`.
    - Note: The roadmap listed above applies to [GX-402]; although appended after the planning guard it remains the authoritative execution plan for the maintenance refactor sequence.
