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
If a repository doesnt have a remote, there is nothing to fetch, but we can still change the default branch, methinks. Identify non-critical steps and ensure they produce warnings but are non-blocking. Encode this semntics into tasks and workflows.

## BugFixes (300–399)

- [x] [GX-300] `gix b default` aborts for repositories without remotes; it treats the `git fetch` failure as fatal instead of warning and skipping the fetch, so the branch switch never executes.
  - Resolution: The branch change service now enumerates remotes once, skips fetch/pull when none exist, and creates branches without tracking nonexistent remotes. Added regression coverage for the zero-remote case.

## Maintenance (400–499)

- [x] [GX-400] Update the documentation @README.md and focus on the usefullness to the user. Move the technical details to @ARCHITECTURE.md
- [x] [GX-401] Ensure architrecture matches the reality of code. Update @ARCHITECTURE.md when needed
  - Resolution: `ARCHITECTURE.md` now documents the current Cobra command flow, workflow step registry, and per-package responsibilities so the guide mirrors the Go CLI.
- [x] [GX-402] Review @POLICY.md and verify what code areas need improvements and refactoring. Prepare a detailed plan of refactoring. Check for bugs, missing tests, poor coding practices, uplication and slop. Ensure strong encapsulation and following the principles og @AGENTS.md and policies of @POLICY.md
  - Resolution: Authored `docs/policy_refactor_plan.md` detailing domain-model introductions, error strategy, shared helper cleanup, and new test coverage aligned with the confident-programming policy.
- [x] [GX-403] Introduce domain types for repository metadata and enforce edge validation
  - Resolution: Added smart constructors in `internal/repos/shared` for repository paths, owners, repositories, remotes, branches, and protocols, refactored repos/workflow options to require these types, updated CLI/workflow edges to construct them once, and expanded tests to cover the new constructors.
- [ ] [GX-404] Establish contextual error strategy for repository executors
  - Define typed sentinel errors (for example, `ErrUnknownProtocol`, `ErrCanonicalOwnerMissing`) with helpers that wrap them using operation + subject + stable code identifiers.
  - Refactor `internal/repos/remotes`, `internal/repos/protocol`, `internal/repos/rename`, and `internal/repos/history` executors to return contextual errors instead of printing failure strings, leaving user messaging to CLI/reporters.
  - Adjust CLI layers and tests to assert on wrapped errors and render human-readable output.
- [ ] [GX-405] Consolidate shared helpers and eliminate duplicated validation
  - Extract owner/repository parsing into a single reusable helper and share prompt/output formatting via a reporter interface.
  - Remove repeated `strings.TrimSpace` and similar defensive code paths, trusting normalized domain types introduced in GX-403.
  - Review boolean flags such as `AssumeYes` and `RequireCleanWorktree`; convert to clearer enums or document retained semantics when multiple behaviors are conflated.
- [ ] [GX-406] Expand regression coverage for policy compliance
  - Add table-driven tests for the new domain constructors and protocol conversion edge cases (current vs. target protocol mismatches, missing owner slugs, unknown protocols).
  - Test dependency resolvers in `internal/repos/dependencies` to ensure logger wiring and error propagation.
  - Extend workflow integration tests to confirm domain types propagate correctly through task execution.
- [ ] [GX-407] Update documentation and CI tooling for the refactor
  - Document newly introduced domain types, error codes, and edge-validation flow in `docs/cli_design.md` (or a dedicated `docs/refactor_status.md`) and cross-link from `POLICY.md`.
  - Update developer docs describing prompt/output handling after GX-405 cleanup.
  - Extend CI to run `staticcheck` and `ineffassign` alongside the existing `go test ./...` gate.

## Planning 
do not work on the issues below, not ready

    - [ ] [GX-22] Implement adding licenses to repos. The prototype is under tools/licenser
    - [ ] [GX-23] Implement git retag, which allows to alter git history and straigtens up the git tags based on the timeline. The prototype is under tools/git_retag
