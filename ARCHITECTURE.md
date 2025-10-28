# gix Architecture

## Overview

gix is a Go 1.24 command-line application built with Cobra and Viper. The binary exposed by `main.go` delegates all setup to `cmd/cli`, which wires logging, configuration, and command registration before executing user-facing operations. Domain logic lives in `internal` packages, each focused on a cohesive maintenance capability. Shared libraries that may be reused by external programs are published under `pkg/`.

```
.
├── main.go          # binary entrypoint
├── cmd/cli          # Cobra application, command registration, configuration bootstrap
├── internal         # feature domains (audit, repos, branches, etc.)
├── pkg              # reusable libraries (currently LLM automation)
├── docs             # design notes and developer references
└── tests            # behavior-driven integration tests
```

## Command Surface

The Cobra application (`cmd/cli/application.go`) initialises the root command and nests feature namespaces below it (`audit`, `repo`, `branch`, `commit`, `workflow`, and others). Each namespace hosts subcommands that ultimately depend on injected services from `internal/...` packages. Commands share common flag parsing helpers (`internal/utils/flags`) and prompt utilities.

- `cmd/cli/repos` registers multi-command groups such as `repo folder rename`, `repo remote update-to-canonical`, `repo prs delete`, and `repo files replace`.
- `cmd/cli/changelog`, `cmd/cli/commit`, and `cmd/cli/workflow` expose focused entrypoints for changelog generation, AI-assisted commit messaging, and workflow execution.
- `cmd/cli/default_configuration.go` houses the embedded default YAML used by the `gix --init` flag.

All commands accept shared flags for log level, log format, dry-run previews, repository roots, and confirmation prompts. Validation occurs in Cobra `PreRunE` functions, aligning with the confident-programming rules in `POLICY.md`.

## Domain Packages

Each feature area resides in `internal/<domain>` and exposes structs with methods instead of package-level functions. The primary packages are:

- `internal/audit`: Repository discovery, metadata reconciliation, and CSV reporting.
- `internal/branches`: Branch cleanup utilities (`cd`, `refresh`, PR deletion) built on top of Git adapters.
- `internal/repos`: Namespace containing subpackages for discovery, rename plans, remote/protocol updates, history rewriting, prompts, safeguards, and dependencies.
- `internal/packages`: GitHub Packages deletion workflow using the GitHub API.
- `internal/releases`: Tagging helpers for releases.
- `internal/workflow`: YAML/JSON workflow runner, shared step registry, and execution environment.
- `internal/utils`: Shell execution adapters, logging setup, filesystem helpers, and flag utilities.
- `internal/version`: Version reporting and build metadata.
- `internal/migrate`: Support for repository migrations such as default-branch transitions.

External integrations (for example, GitHub CLI wrappers, GHCR API clients, and shell execution) are isolated behind interfaces, enabling injection of fakes or mocks in tests.

## Workflow Runner

The workflow command consumes declarative YAML or JSON plans describing ordered actions. `internal/workflow` resolves steps into concrete executors registered through `internal/repos/dependencies` and other domain services. Discovery of repositories, confirmation prompts, and logging contexts are reused across steps to minimise duplicate code. Each workflow step enforces dry-run previews and respect the global confirmation strategy.

## Configuration and Logging

Configuration is managed by Viper with an uppercase `GIX` environment prefix. The search order is:

1. Explicit `--config` path, if provided.
2. `config.yaml` in the working directory.
3. `$XDG_CONFIG_HOME/gix/config.yaml`.
4. `$HOME/.gix/config.yaml`.

`gix --init` bootstraps either a local `./config.yaml` or a user-level configuration directory when invoked with `--init LOCAL` (default) or `--init user`. Logging relies on Uber's Zap; structured JSON is the default, and console mode is available through a flag or configuration.

## Workflow configuration example

The example below matches the configuration used in the documentation tests. It demonstrates how CLI defaults and workflow steps can share anchored maps so one file drives both direct commands and declarative workflows.

```yaml
# config.yaml
common:
  log_level: error
  log_format: structured

operations:
  - operation: audit
    with: &audit_defaults
      roots:
        - ~/Development
      debug: false

  - operation: repo-packages-purge
    with: &packages_purge_defaults
      # package: my-image  # Optional override; defaults to the repository name
      roots:
        - ~/Development

  - operation: repo-prs-purge
    with: &branch_cleanup_defaults
      remote: origin
      limit: 100
      dry_run: false
      roots:
        - ~/Development

  - operation: repo-remote-update
    with: &repo_remotes_defaults
      dry_run: false
      assume_yes: true
      owner: canonical
      roots:
        - ~/Development

  - operation: repo-protocol-convert
    with: &repo_protocol_defaults
      dry_run: false
      assume_yes: true
      roots:
        - ~/Development
      from: https
      to: git

  - operation: repo-folders-rename
    with: &repo_rename_defaults
      dry_run: false
      assume_yes: true
      require_clean: true
      include_owner: false
      roots:
        - ~/Development

  - operation: workflow
    with: &workflow_command_defaults
      roots:
        - ~/Development
      dry_run: false
      assume_yes: false

  - operation: branch-default
    with: &branch_default_defaults
      debug: false
      roots:
        - ~/Development

workflow:
  - step:
      order: 1
      operation: convert-protocol
      with:
        <<: *repo_protocol_defaults

  - step:
      order: 2
      operation: update-canonical-remote
      with:
        <<: *repo_remotes_defaults

  - step:
      order: 3
      operation: rename-directories
      with:
        <<: *repo_rename_defaults

  - step:
      order: 4
      operation: default-branch
      with:
        <<: *branch_default_defaults
        targets:
          - remote_name: origin
            target_branch: master
            push_to_remote: true
            delete_source_branch: false

  - step:
      order: 5
      operation: audit-report
      with:
        output: ./reports/audit-latest.csv
```

## Reusable Packages

`pkg/llm` contains the reusable client abstractions for LLM-backed features such as commit message and changelog generators. The package exposes an interface-based design so that other programs can reuse the same client without duplicating API plumbing.

## Testing Strategy

Domain packages rely on table-driven unit tests using injected fakes for Git, GitHub, and filesystem interactions. Integration coverage lives under `tests/`, where high-level flows execute through the public CLI surfaces to ensure behavior matches the documented commands. All tests are designed to run in isolated temporary directories (`t.TempDir`) without polluting the developer filesystem.
