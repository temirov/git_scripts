# Git/GitHub helper CLI

A Go-based command-line interface that automates routine Git and GitHub maintenance.

### Execution modes at a glance

You can run the CLI in two complementary ways depending on how much orchestration you need:

- **Direct commands with persisted defaults** – invoke commands such as `repo-folders-rename`, `repo-protocol-convert`,
  `repo-packages-purge`, and `audit` from the shell,
  optionally loading shared flags (for example, log level or default owners) via [
  `--config` files](#configuration-and-logging).
  This mode mirrors the quick-start rows in the [command catalog](#command-catalog) and is ideal when you want
  immediate,
  one-off execution.
- **Workflow runner with YAML/JSON steps** – describe ordered operations in declarative workflow files and let
  `workflow`
  drive them. The [`Workflow bundling`](#workflow-bundling) section shows the domain-specific language (DSL) and how the
  runner reuses discovery, prompting, and logging across steps.

| Choose this mode | When it shines                                                                                                          | Example                                                                                |
|------------------|-------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------|
| Direct commands  | You need a focused, ad-hoc action with minimal setup, such as renaming directories or auditing repositories             | [`repo-folders-rename`](#command-catalog) and [`audit`](#command-catalog) quick-starts |
| Workflow runner  | You want to bundle several operations together, share discovery across them, or hand off a repeatable plan to teammates | [`workflow` with a YAML plan](#workflow-bundling)                                      |

## Feature highlights

- **Repository auditing** – generate CSV summaries describing canonical GitHub metadata for every repository under one
  or many
  roots.
- **Directory reconciliation** – rename working directories to match the canonical repository name returned by GitHub.
- **Remote normalization** – update `origin` to the canonical GitHub remote or convert between HTTPS, SSH, and `git`
  protocols.
- **Branch maintenance** – delete remote/local branches once their pull requests are closed and migrate defaults from
  `main` to
  `master` with safety gates.
- **GitHub Packages upkeep** – remove untagged GHCR container versions via the official API.
- **Workflow bundling** – describe ordered operations in YAML or JSON and execute them in one pass with shared
  discovery,
  prompting, and logging.

All repository-facing commands accept multiple root directories, honor `--dry-run` previews, and support non-interactive
confirmation via `--yes`.

## Installing and running

The CLI targets Go **1.24** or newer.

```shell
go run . --help
```

Build a reusable binary with either `go build` or the provided make target:

```shell
go build
./git_scripts --help

make build
./bin/git-scripts --help
```

`make release` cross-compiles platform-specific artifacts into `./dist` for distribution.

## Configuration and logging

Global flags configure logging and optional configuration files:

- `--config path/to/config.yaml` – load persisted defaults for any command.
- `--log-level <debug|info|warn|error>` – override the configured log level.
- `--log-format <structured|console>` – switch between JSON and human-readable logs.

Configuration keys mirror the flags (`common.log_level`, `common.log_format`) and can also be provided via environment
variables prefixed with
`GITSCRIPTS_` (for example, `GITSCRIPTS_COMMON_LOG_LEVEL=error`).

## Command catalog

The commands below share repository discovery, prompting, and logging helpers. Use the quick-start examples to align
with the registered command names and flags.

| Command                 | Summary                                                       | Key flags / example                                                                                                                                                                                                         |
|-------------------------|---------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `audit`                 | Audit and reconcile local GitHub repositories                 | Flags: `--root`, `--debug`. Example: `go run . audit --root ~/Development`                                                                                                                                                  |
| `repo-folders-rename`   | Rename repository directories to match canonical GitHub names | Flags: `--dry-run`, `--yes`, `--require-clean`. Example: `go run . repo-folders-rename --yes --require-clean ~/Development`                                                                                                 |
| `repo-remote-update`    | Update origin URLs to match canonical GitHub repositories     | Flags: `--dry-run`, `--yes`. Example: `go run . repo-remote-update --dry-run ~/Development`                                                                                                                                 |
| `repo-protocol-convert` | Convert repository origin remotes between protocols           | Flags: `--from`, `--to`, `--dry-run`, `--yes`. Example: `go run . repo-protocol-convert --from https --to ssh --yes ~/Development`                                                                                          |
| `repo-prs-purge`        | Remove remote and local branches for closed pull requests     | Flags: `--remote`, `--limit`, `--dry-run`. Example: `go run . repo-prs-purge --remote origin --limit 100 ~/Development`                                                                                                     |
| `branch-migrate`        | Migrate repository defaults from main to master               | Flag: `--debug` for verbose diagnostics. Example: `go run . branch-migrate --debug ~/Development/project-repo`                                                                                                              |
| `repo-packages-purge`   | Delete untagged GHCR versions                                 | Flags: `--owner`, `--package`, `--owner-type`, `--token-source`, `--dry-run`. Example: `go run . repo-packages-purge --owner my-org --package my-image --owner-type org --token-source env:GITHUB_PACKAGES_TOKEN --dry-run` |
| `workflow`              | Run a workflow configuration file                             | Flags: `--roots`, `--dry-run`, `--yes`. Example: `go run . workflow config.yaml --roots ~/Development --dry-run`                                                                                                            |

Persist defaults and workflow plans in a single configuration file to avoid long flag lists and keep the runner in sync:

```yaml
# config.yaml
common:
  log_level: info
  log_format: structured
tools:
  packages:
    purge:
      operation: repo-packages-purge
      with:
        owner: my-org
        package: my-image
        owner_type: org
        token_source: env:GITHUB_PACKAGES_TOKEN
        page_size: 50
  reports:
    audit:
      operation: audit-report
      with:
        output: ./audit.csv
  conversion_default: &conversion_default
    operation: convert-protocol
    with: &conversion_default_options
      from: https
      to: git
  rename_clean: &rename_clean
    operation: rename-directories
    with:
      require_clean: true
  migration_legacy: &migration_legacy
    operation: migrate-branch
    with:
      targets:
        - remote_name: origin
          source_branch: main
          target_branch: master
          push_to_remote: true
          delete_source_branch: false
  audit_weekly: &audit_weekly
    operation: audit-report
    with: &audit_weekly_options
      output: ./reports/audit.csv
workflow:
  - <<: *conversion_default
  - operation: update-canonical-remote
  - <<: *rename_clean
  - <<: *migration_legacy
  - <<: *audit_weekly
    with:
      <<: *audit_weekly_options
      output: ./reports/audit-latest.csv
```

```shell
go run . repo-packages-purge --dry-run=false
```

Specify `--config path/to/override.yaml` when you need to load an alternate configuration.

### Workflow bundling

Define ordered steps in YAML or JSON and execute them with `workflow`:

```shell
go run . workflow --roots ~/Development --dry-run
# Execute with confirmations suppressed
go run . workflow --roots ~/Development --yes
```

`workflow` reads the `workflow` array from `config.yaml`, reusing the same repository discovery, prompting, and logging
infrastructure as the standalone commands. Pass additional roots on the command line to override the configuration file
and
combine `--dry-run`/`--yes` for non-interactive execution.

Each entry in the `workflow` array is a full step definition. Use YAML anchors under `tools` to capture reusable
defaults and merge them into individual steps with the merge key (`<<`). Inline overrides remain possible: apply another
merge inside the `with` map or specify the final values directly alongside the alias.

## Development and testing

```
make check-format   # Verify gofmt formatting
make lint           # Run go vet across the module
make test-unit      # Execute unit tests
make test-integration  # Run integration tests under ./tests
make test           # Run unit and integration suites
make build          # Compile ./bin/git-scripts
make release        # Cross-compile binaries into ./dist
```

## Prerequisites

- Go 1.24+
- git
- GitHub CLI (`gh`) with an authenticated session (`gh auth login`)
    - Install the CLI via your platform's package manager or the [official releases](https://cli.github.com/).
    - Run `gh auth login` (or verify with `gh auth status`) so API calls succeed during branch cleanup and migration
      commands.

The `repo-packages-purge` command additionally requires network access and a GitHub Personal Access Token with
`read:packages`,
`write:packages`, and `delete:packages` scopes. Export the token before invoking the command:

```shell
export GITHUB_PACKAGES_TOKEN=ghp_XXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

## Repository roots and bulk execution tips

- Provide explicit repository roots to operate on multiple directories in one invocation. When omitted, commands default
  to the
  current working directory.
- Use `--dry-run` to preview changes. Combine with `--yes` once you are comfortable executing the plan without prompts.
- Workflow configurations let you mix and match operations (for example, convert protocols, migrate branches, and audit)
  while
  sharing discovery costs.

## License

This project is licensed under the MIT License. See `LICENSE` for details.
