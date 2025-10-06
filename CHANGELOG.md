## [v0.0.4] - 2025-10-03

### Highlights
- Owner-aware repository rename workflows create missing owner directories and keep remotes aligned with canonical metadata.
- Boolean CLI toggles now accept yes/no/on/off forms everywhere thanks to shared parsing utilities.
- Operations audit reliably writes reports into nested output directories without manual setup.

### Features ✨
- Added an `--owner` toggle to `repo-folders-rename`, planned via a new directory planner that joins owner and repository segments and ensures parent directories exist.
- Propagated owner preferences through workflow configuration and remote update execution, including owner-constraint enforcement when rewriting origin URLs.
- Introduced reusable toggle flag helpers that register boolean flags accepting multiple literal forms and normalize command-line arguments before parsing.

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
- Standardized global CLI flags so `--root`, `--dry-run`, `--yes`, and `--require-clean` behave consistently across commands.
- Embedded configuration defaults and extended search paths improve out-of-the-box repository discovery.
- Enhanced branch and audit workflows with cleaner logging defaults and additional safeguards.

### Features ✨
- Enabled a shared root-resolution context that exposes `--root` on every command and centralizes flag handling.
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

