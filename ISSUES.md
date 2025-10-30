# ISSUES (Append-only Log)

Entries record newly discovered requests or changes, with their outcomes. No instructive content lives here. Read @NOTES.md for the process to follow when fixing issues.

## Features (120–199)


## Improvements (200–299)

- [x] [GX-200] Remove `--to` flag from default command and accept the new branch as an argument, e.g. `gix b default master`
  - Resolution: `branch default` now takes the target branch as a positional argument while still honoring configured defaults; docs and tests updated accordingly.
- [x] [GX-201] Identify non-critical operations and turn them into warnings, which do not stop the flow:
```
14:58:29 tyemirov@computercat:~/Development/Poodle/product_page_analysis.py [main] $ gix b default --to master
default branch update failed: GitHub Pages update failed: GetPagesConfig operation failed: gh command exited with code 1
```
The Pages may be not configured and that's ok
- Resolution: GitHub Pages lookup/update failures during `branch default` now emit warnings and the migration continues, leaving branch promotion untouched.
- [x] [GX-202] have descriptive and actionable error messages, explaining where was the failure and why the command failed:
```
14:56:43 tyemirov@computercat:~/Development/Poodle $ gix --roots . b cd master
failed to fetch updates: git command exited with code 128
```
If a repository doesnt have a remote, there is nothing to fetch, but we can still change the default branch, methinks. Identify non-critical steps and ensure they produce warnings but are non-blocking. Encode this semntics into tasks and workflows.
- Resolution: `branch cd` now logs `FETCH-SKIP`/`PULL-SKIP` warnings when network operations fail and continues switching branches, so repositories without remotes (or offline) still migrate.

## BugFixes (300–399)

- [x] [GX-300] `gix b default` aborts for repositories without remotes; it treats the `git fetch` failure as fatal instead of warning and skipping the fetch, so the branch switch never executes.
  - Resolution: The branch change service now enumerates remotes once, skips fetch/pull when none exist, and creates branches without tracking nonexistent remotes. Added regression coverage for the zero-remote case.
- [x] [GX-301] The message is repetitive, it's enough to say -- unable to update default branch. But it's absolutely unclear why or where it has happened. The error message shall be actionable, not informative. We must specify what folder/branch/etc we were at, and what command has failed, and why. That is an error/warning criteria for all commands.
  - Resolution: Default branch update failures now raise `DefaultBranchUpdateError`, which includes repository path, identifier, source/target branches, and the GitHub CLI failure summary; the workflow operation forwards this error without extra wrapping, and new tests assert the actionable messaging.
```
01:06:39 tyemirov@computercat:~/Development/Poodle $ gix --roots . b default master
WORKFLOW-DEFAULT-SKIP: /home/tyemirov/Development/Poodle/ProductScanner already defaults to master
workflow operation apply-tasks failed: default branch update failed: unable to update default branch: UpdateDefaultBranch operation failed: gh command exited with code 1
```
- [x] [GX-302] Produces non-sensical messages about failures. It's not clear what exactly has failed and what shall be the user's action item. Why would it need to create a master branch here, if it already exists ?
```
14:17:45 tyemirov@computercat:~/Development/loopaware [improvement/LA-201-theme-switch-footer] $ gix b cd master
SWITCHED: /home/tyemirov/Development/loopaware -> master
workflow operation apply-tasks failed: failed to create branch "master" from origin: git command exited with code 128
```
  - Resolution: Branch change service now distinguishes missing-branch failures from dirty working tree errors, surfaces the Git diagnostics in returned messages, and adds regression coverage for both scenarios so the CLI reports actionable guidance instead of redundant branch creation attempts.
  - Update: Fetch and pull skip warnings now include repository paths so operators can see which repository triggered the Git error.
  - Update: Missing or inaccessible remotes now raise `WARNING: no remote counterpart for <repo>` so branch-cd skips fetches without dumping Git internals while still pointing to the affected repository.

## Maintenance (400–499)

- [x] [GX-400] Update the documentation @README.md and focus on the usefullness to the user. Move the technical details to @ARCHITECTURE.md
- [x] [GX-401] Ensure architrecture matches the reality of code. Update @ARCHITECTURE.md when needed
  - Resolution: `ARCHITECTURE.md` now documents the current Cobra command flow, workflow step registry, and per-package responsibilities so the guide mirrors the Go CLI.
- [x] [GX-402] Review @POLICY.md and verify what code areas need improvements and refactoring. Prepare a detailed plan of refactoring. Check for bugs, missing tests, poor coding practices, uplication and slop. Ensure strong encapsulation and following the principles og @AGENTS.md and policies of @POLICY.md
  - Resolution: Authored `docs/policy_refactor_plan.md` detailing domain-model introductions, error strategy, shared helper cleanup, and new test coverage aligned with the confident-programming policy.
- [x] [GX-403] Introduce domain types for repository metadata and enforce edge validation
  - Resolution: Added smart constructors in `internal/repos/shared` for repository paths, owners, repositories, remotes, branches, and protocols, refactored repos/workflow options to require these types, updated CLI/workflow edges to construct them once, and expanded tests to cover the new constructors.
- [x] [GX-404] Establish contextual error strategy for repository executors
  - Resolution: Added `internal/repos/errors` sentinel catalog, refactored remotes/protocol/rename/history executors to wrap failures with operation-specific codes, taught workflow operations to log the contextual errors, and extended unit/integration tests to assert on the new propagation semantics.
- [x] [GX-405] Consolidate shared helpers and eliminate duplicated validation
  - Resolution: Added shared reporter/policy helpers for repository executors, refactored protocol/remotes/rename workflows to reuse optional owner parsing and structured confirmation policies, and updated tests/CLI bridges to exercise the new abstractions without redundant trimming or boolean flags.
- [x] [GX-406] Expand regression coverage for policy compliance
  - Add table-driven tests for the new domain constructors and protocol conversion edge cases (current vs. target protocol mismatches, missing owner slugs, unknown protocols).
  - Test dependency resolvers in `internal/repos/dependencies` to ensure logger wiring and error propagation.
  - Extend workflow integration tests to confirm domain types propagate correctly through task execution.
- Resolution: Added shared constructor/optional parser tables, expanded protocol executor edge cases, introduced resolver unit tests, and enforced canonical messaging in workflow integration output; suites now cover policy boundaries.
- [x] [GX-407] Update documentation and CI tooling for the refactor
  - Document newly introduced domain types, error codes, and edge-validation flow in `docs/cli_design.md` (or a dedicated `docs/refactor_status.md`) and cross-link from `POLICY.md`.
  - Update developer docs describing prompt/output handling after GX-405 cleanup.
  - Extend CI to run `staticcheck` and `ineffassign` alongside the existing `go test ./...` gate.
  - Resolution: Added domain model section and prompt guidance to `docs/cli_design.md`, cross-linked from `POLICY.md`, refreshed README developer notes, wired `staticcheck`/`ineffassign` into `make ci`, and resolved all new lint findings.

## Planning 
do not work on the issues below, not ready

    - [ ] [GX-22] Implement adding licenses to repos. The prototype is under tools/licenser
    - [ ] [GX-23] Implement git retag, which allows to alter git history and straigtens up the git tags based on the timeline. The prototype is under tools/git_retag
