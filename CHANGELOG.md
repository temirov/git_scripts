# Changelog

## [v0.1.0]

### Features ✨
- Added a `commit message` command that summarizes staged or worktree changes with the shared LLM client and returns a Conventional Commit draft.
- Added a `changelog message` command that turns tagged or time-based git history into Markdown release notes using the shared LLM client.
- Added a `branch cd` command that fetches, switches, and rebases repositories onto the requested branch, creating it from the remote when missing.
- Added a `repo release` command that annotates tags with customizable messages and pushes them to the selected remote across repositories.

### Improvements ⚙️
- Introduced hierarchical command namespaces (`repo`, `branch`) with short aliases (`r`, `b`, `a`, `w`) and removed the legacy hyphenated commands.
- Updated CLI bootstrap to register alias-aware help so the new paths and shortcuts surface in command discovery.
- Nested `commit message` under the `branch` namespace and `changelog message` under `repo` to keep related commands grouped.

### Bug Fixes 🐛
- Updated `repo release` help to surface the required `<tag>` argument along with usage guidance and examples across the CLI.
- Updated `branch cd` help to surface the required `<branch>` argument along with usage guidance and examples.
- Ensured `repo release` falls back to the embedded `.` repository root when user configuration omits the operation defaults.
- Updated `workflow` help text to surface the required configuration path and example usage.
- Disabled default CLI info logging and set the default log level to `error` so commands run silently unless verbosity is explicitly requested.
- Downgraded the configuration initialization banner to DEBUG so standard operations continue logging at INFO severity only.
- Clarified the remote owner constraint skip message to spell out the required `--owner` value and detected repository owner.

### Testing 🧪
- Added application command hierarchy coverage to ensure aliases and nested commands resolve to the existing operations.
- Added task operation planner/executor unit tests and a workflow CLI integration test covering the new `apply-tasks` step.
- Added unit coverage for the LLM client wrapper, commit message generator, changelog generator, and CLI dry-run flows.
- Added branch cd service and command tests covering fetch/switch/create flows and CLI execution.
- Added release service and CLI tests verifying tag annotation, push behavior, and dry-run handling.
- Added CLI and command unit tests to enforce the `<branch>` usage template for `branch cd`.
- Added configuration and CLI tests confirming the `repo release` command retains default roots without explicit configuration.
- Added branch refresh coverage to exercise the command-level `--branch` flag after removing the global variant.

### Docs 📚
- Documented the new CLI syntax and shortcuts in `README.md`, including refreshed quick-start examples.
- Added `apply-tasks` workflow guidance to `README.md`, including templating details and sample YAML.
- Documented the `commit message` assistant, configuration knobs, and usage examples.
- Documented the `changelog message` assistant, baseline controls, and sample invocations in `README.md`.
- Documented the `branch cd` helper with usage notes and remote/dry-run options.
- Documented the `repo release` helper including remote overrides, custom messages, and dry-run support.
- Documented the branch command expectations now that the global `--branch` flag is removed.
- Refreshed the README command catalog with up-to-date command paths and shortcuts.

## [v0.0.8] - 2025-10-07

### Features ✨
- Added a `branch-refresh` command that fetches, checks out, and pulls branches with optional recovery strategies and clean-worktree enforcement.
- Introduced a root `--version` flag that prints the detected gix version and exits before executing subcommands.

### Improvements ⚙️
- Branch refresh now survives intermediate rebase checkpoints by attempting to recover from checkpoint commits.
- Version output messages use the `gix` prefix for consistent CLI presentation.

### Bug Fixes 🐛
- `repo-prs-purge` prompts before deleting branches, respecting `--yes`, apply-to-all decisions, and reuse of confirmations during batch cleanup.
- Nested Git repositories are renamed before their parents during directory normalization to avoid conflicting rename sequences.

## [v0.0.7] - 2025-10-06

### Improvements ⚙️
- Guarded destructive repo-prs-purge operations behind confirmation prompts, centralizing apply-all handling and `--yes` defaults.
- Updated AGENTS and configuration guidance to minimize unnecessary output streaming.

### Testing 🧪
- Expanded coverage for nested rename ordering, branch cleanup prompting, and integration workflows.

### Features ✨
- Added a `branch-refresh` command that fetches, checks out, and pulls branches with optional recovery strategies and clean-worktree enforcement.
- Introduced a root `--version` flag that prints the detected gix version and exits before executing subcommands.

### Improvements ⚙️
- Branch refresh now survives intermediate rebase checkpoints by attempting to recover from checkpoint commits.
- Version output messages use the `gix` prefix for consistent CLI presentation.

### Bug Fixes 🐛
- `repo-prs-purge` prompts before deleting branches, respecting `--yes`, apply-to-all decisions, and reuse of confirmations during batch cleanup.
- Nested Git repositories are renamed before their parents during directory normalization to avoid conflicting rename sequences.

## [v0.0.6] - 2025-10-06

### Highlights
- Audit reports now surface non-repository directories with `--all` while presenting folder names relative to their configured roots for quicker scanning.
- Root sanitization trims nested duplicates so CLI commands and workflows operate on a predictable set of repositories.

### Features ✨
- Added an `--all` toggle to `audit` to include top-level directories lacking Git metadata, filling Git-specific columns with `n/a` in CSV output and workflow reports.

### Improvements ⚙️
- Reordered audit CSV columns to lead with `folder_name` and emit paths relative to each root, preserving canonical-name checks through the basename.
- Centralized root sanitation to deduplicate nested entries, expand tildes, and enforce the new singular `--root` flag across commands and workflows.
- Surfaced usage guidance whenever `audit` is invoked without required roots to clarify flag expectations.

### Bug Fixes 🐛
- Corrected repository containment detection so nested Git repositories are not skipped when scanning with `--all`.

### Docs 📚
- Expanded installation guidance and refreshed flag examples in `README.md` to reflect the singular `--root` flag and new audit behaviors.

### CI & Maintenance
- Updated the release workflow to build and publish the `gix` binary from this repository.

## [v0.0.5] - 2025-10-06

### Highlights
- CLI configuration discovery now honors `XDG_CONFIG_HOME` while normalizing resolved paths for consistent behavior across platforms and tests.

### Improvements ⚙️
- Updated the application bootstrap to expand XDG-aware configuration search paths and align emitted logs with the resolved directories.

### Testing 🧪
- Normalized temporary path expectations in repository and application tests so macOS `/private` prefixes no longer cause false negatives.

## [v0.0.4] - 2025-10-03

### Highlights
- Owner-aware repository rename workflows create missing owner directories and keep remotes aligned with canonical metadata.
- Boolean CLI toggles now accept yes/no/on/off forms everywhere thanks to shared parsing utilities.
- Operations audit reliably writes reports into nested output directories without manual setup.

### Features ✨
- Added an `--owner` toggle to `repo-folders-rename`, planned via a new directory planner that joins owner and repository segments and ensures parent directories exist.
- Propagated owner preferences through workflow configuration and remote update execution, including owner-constraint enforcement when rewriting origin URLs.
- Introduced reusable toggle flag helpers that register boolean flags accepting multiple literal forms and normalize command-line arguments before parsing.
- Added an `--all` flag to `audit` so directories without Git metadata appear in reports with git fields marked as `n/a`.

### Improvements ⚙️
- Normalized toggle arguments across commands so `--flag value` and `--flag=value` behave consistently for all boolean options.
- Refined rename workflow execution to skip no-op renames and to honor include-owner preferences sourced from configuration files.
- Ensured audit operations create nested target directories before emitting CSV reports.

### Docs 📚
- Documented owner-aware rename and remote update options in `README.md` and `docs/cli_design.md` examples.

### CI & Maintenance
- Added extensive unit coverage for toggle parsing, rename planners and executors, remote owner constraints, and workflow inspection helpers.

## [v0.0.3] - 2025-10-03

### Highlights
- Added a configuration initialization workflow that writes embedded defaults to either local or user scopes.
- Expanded configuration search paths so embedded defaults and user overrides are discovered automatically.

### Features ✨
- Introduced `--init` and `--force` flags that materialize the embedded configuration content with safe directory handling and conflict detection.
- Added integration coverage that exercises initialization end-to-end and verifies configuration loader behavior with new scopes.

### Improvements ⚙️
- Refined configuration loading to merge embedded defaults while tracking duplicates and missing operation definitions precisely.
- Strengthened CLI wiring with richer validation, clearer error surfaces, and deterministic command registration ordering.

### Docs 📚
- _No updates._

### CI & Maintenance
- Expanded unit and integration tests around configuration initialization and loader path resolution.

## [v0.0.2] - 2025-10-03

### Highlights
- Standardized global CLI flags so `--roots`, `--dry-run`, `--yes`, and `--require-clean` behave consistently across commands.
- Embedded configuration defaults and extended search paths improve out-of-the-box repository discovery.
- Enhanced branch and audit workflows with cleaner logging defaults and additional safeguards.

### Features ✨
- Enabled a shared root-resolution context that exposes `--roots` on every command and centralizes flag handling.
- Added `--from` and `--to` options for branch migration, alongside enforceable clean-worktree checks for branch-level operations.
- Embedded default configuration content into the binary and merged it with user configuration files discovered on disk.

### Improvements ⚙️
- Introduced apply-all confirmation tracking and structured prompt results to streamline batch confirmations.
- Added minimal audit inspection depth, optional branch data skipping, and normalized repository discovery paths for more predictable workflows.
- Defaulted console logging formats and eliminated redundant GitHub CLI view logging to reduce noise.

### Docs 📚
- _No updates._

### CI & Maintenance
- Broadened unit coverage for configuration loaders, CLI application wiring, and integration helpers supporting workflow tests.

## [v0.0.1] - 2025-09-28

### What's New 🎉

1. Bash scripts replaced with Go implementation
2. The config.yaml file stores the defaults 
3. The config.yaml file defines a runnable workflow, chaining multiple commands
