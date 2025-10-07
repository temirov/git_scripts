# gix, a Git/GitHub helper CLI

A Go-based command-line interface that automates routine Git and GitHub maintenance.

### Execution modes at a glance

You can run the CLI in two complementary ways depending on how much orchestration you need:

- **Direct commands with persisted defaults** – invoke commands such as `repo-folders-rename`, `repo-protocol-convert`,
  `repo-packages-purge`, and `audit` from the shell,
  optionally loading shared flags (for example, log level or default package names) via [
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

```shell
go install github.com/temirov/gix@latest
```

The CLI targets Go **1.24** or newer.

```shell
go run . --help
```

Build a reusable binary with either `go build` or the provided make target:

```shell
go build
./gix --help

make build
./bin/gix --help
```

`make release` cross-compiles platform-specific artifacts into `./dist` for distribution.

## Configuration and logging

Global flags configure logging and optional configuration files:

- `--config path/to/config.yaml` – load persisted defaults for any command.
- `--log-level <debug|info|warn|error>` – override the configured log level.
- `--log-format <structured|console>` – switch between JSON and human-readable logs.

Configuration files are discovered automatically. gix checks the following locations in order and stops at the first match:

1. A `config.yaml` file in the current working directory.
2. `$XDG_CONFIG_HOME/gix/config.yaml` (or the OS-specific user configuration directory reported by `os.UserConfigDir`).
3. `$HOME/.gix/config.yaml`.

Provide `--config path/to/override.yaml` when you need to load a file outside of the search paths.

#### Initializing configuration defaults

Run `gix --init` to write the embedded defaults into `./config.yaml`. Pass a scope to control the destination:

- `gix --init local` (default) writes to the current working directory.
- `gix --init user` writes to the persistent user location (`$XDG_CONFIG_HOME/gix/config.yaml` or `$HOME/.gix/config.yaml`).

Append `--force` when you want to overwrite an existing configuration file in the selected scope.

Configuration keys mirror the flags (`common.log_level`, `common.log_format`) and can also be provided via environment
variables prefixed with
`GIX_` (for example, `GIX_COMMON_LOG_LEVEL=error`).

### Global execution flags

Every command accepts the shared execution flags below in addition to its command-specific options:

- `--roots <path>` – add one or more repository roots (repeat the flag; nested paths are ignored automatically).
- `--dry-run` – plan work without performing any side effects.
- `--yes`/`-y` – automatically confirm interactive prompts.
- `--remote <name>` – override the Git remote to inspect or mutate (defaults to `origin`).

## Command catalog

The commands below share repository discovery, prompting, and logging helpers. Use the quick-start examples to align
with the registered command names and flags.

| Command                 | Summary                                                       | Key flags / example                                                                                                                |
|-------------------------|---------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------|
| `audit`                 | Audit and reconcile local GitHub repositories                 | Flags: `--roots`, `--all`, `--log-level`. Example: `go run . audit --log-level=debug --roots ~/Development --all`                                    |
| `repo-folders-rename`   | Rename repository directories to match canonical GitHub names | Flags: `--dry-run`, `--yes`, `--require-clean`, `--owner`, `--roots`. Example: `go run . repo-folders-rename --yes --require-clean --owner --roots ~/Development`        |
| `repo-remote-update`    | Update origin URLs to match canonical GitHub repositories     | Flags: `--dry-run`, `--yes`, `--owner`, `--roots`. Example: `go run . repo-remote-update --dry-run --owner canonical --roots ~/Development`          |
| `repo-protocol-convert` | Convert repository origin remotes between protocols           | Flags: `--from`, `--to`, `--dry-run`, `--yes`, `--roots`. Example: `go run . repo-protocol-convert --from https --to ssh --yes --roots ~/Development` |
| `repo-prs-purge`        | Remove remote and local branches for closed pull requests     | Flags: `--remote`, `--limit`, `--dry-run`, `--roots`. Example: `go run . repo-prs-purge --remote origin --limit 100 --roots ~/Development`            |
| `branch-migrate`        | Migrate repository defaults from main to master               | Flags: `--from`, `--to`, `--roots`. Example: `go run . branch-migrate --from main --to master --roots ~/Development/project-repo`                     |
| `repo-packages-purge`   | Delete untagged GHCR versions                                 | Flags: `--package` (override), `--dry-run`, `--roots`. Example: `go run . repo-packages-purge --dry-run --roots ~/Development`       |
| `workflow`              | Run a workflow configuration file                             | Flags: `--roots`, `--dry-run`, `--yes`. Example: `go run . workflow config.yaml --roots ~/Development --dry-run`                     |

Persist defaults and workflow plans in a single configuration file to avoid long flag lists and keep the runner in sync:

The audit command exposes `--all` to enumerate top-level folders lacking Git repositories for each root alongside canonical results, marking git-related fields as `n/a` when metadata is unavailable.

The purge command derives the GHCR owner, owner type, and default package name from each repository's `origin` remote
and the canonical metadata returned by the GitHub CLI. Ensure the remotes point at the desired GitHub repositories
before running the command. Provide one or more roots with `--roots` or in configuration to run the purge across
multiple repositories; the command discovers Git repositories beneath every root and executes the purge workflow for
each repository, continuing after non-fatal errors. Specify `--package` only when the container name in GHCR differs
from the repository name.

```yaml
# config.yaml
common:
  log_level: info
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

  - operation: branch-migrate
    with: &branch_migrate_defaults
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
      operation: migrate-branch
      with:
        <<: *branch_migrate_defaults
        targets:
          - remote_name: origin
            source_branch: main
            target_branch: master
            push_to_remote: true
            delete_source_branch: false

  - step:
      order: 5
      operation: audit-report
      with:
        output: ./reports/audit-latest.csv
```

The purge command automatically targets the public GitHub API. Set the
`GIX_REPO_PACKAGES_PURGE_BASE_URL` environment variable when you need to
point at a GitHub Enterprise instance during local testing.

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

Each entry in the `workflow` array is a full step definition. Reuse the option maps defined for CLI commands (for
example,
`*repo_protocol_defaults`) to keep workflow steps and direct invocations in sync. Inline overrides remain possible:
apply
another merge inside the `with` map or specify the final values directly alongside the alias.

## Development and testing

```
make check-format   # Verify gofmt formatting
make lint           # Run go vet across the module
make test-unit      # Execute unit tests
make test-integration  # Run integration tests under ./tests
make test           # Run unit and integration suites
make build          # Compile ./bin/gix
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
`write:packages`, and `delete:packages` scopes. Export the token before invoking the command; if the token is missing
any of these scopes the GHCR API responds with HTTP 403 and the command surfaces an error similar to `unable to purge
package versions: unexpected status code 403 for DELETE ... (requires delete:packages)`.

```shell
export GITHUB_PACKAGES_TOKEN=ghp_XXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

## Repository roots and bulk execution tips

- Provide explicit repository roots (or configure defaults) to operate on multiple directories; commands return an error when no roots are supplied.
- Use `--dry-run` to preview changes. Combine with `--yes` once you are comfortable executing the plan without prompts.
- Workflow configurations let you mix and match operations (for example, convert protocols, migrate branches, and audit)
  while
  sharing discovery costs.

## License

This project is licensed under the MIT License. See `LICENSE` for details.
