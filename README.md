# Git/GitHub helper CLI

A Go-based command-line interface that automates routine Git and GitHub maintenance. The CLI replaces the legacy Bash scripts
that previously lived in this repository and exposes explicit, composable commands for each workflow.

### Execution modes at a glance

You can run the CLI in two complementary ways depending on how much orchestration you need:

- **Direct commands with persisted defaults** – invoke subcommands such as `repos`, `packages`, and `audit` from the shell,
  optionally loading shared flags (for example, log level or default owners) via [`--config` files](#configuration-and-logging).
  This mode mirrors the examples throughout the [command catalog](#command-catalog) and is ideal when you want immediate,
  one-off execution.
- **Workflow runner with YAML/JSON steps** – describe ordered operations in declarative workflow files and let `workflow run`
  drive them. The [`Workflow bundling`](#workflow-bundling) section shows the domain-specific language (DSL) and how the
  runner reuses discovery, prompting, and logging across steps.

| Choose this mode | When it shines | Example |
| --- | --- | --- |
| Direct commands | You need a focused, ad-hoc action with minimal setup, such as renaming directories or auditing repositories | [`repos rename-folders`](#repository-maintenance-git-scripts-repos-) and [`audit`](#audit-reports) examples |
| Workflow runner | You want to bundle several operations together, share discovery across them, or hand off a repeatable plan to teammates | [`workflow run` with a YAML plan](#workflow-bundling) |

## Feature highlights

- **Repository auditing** – generate CSV summaries describing canonical GitHub metadata for every repository under one or many
  roots.
- **Directory reconciliation** – rename working directories to match the canonical repository name returned by GitHub.
- **Remote normalization** – update `origin` to the canonical GitHub remote or convert between HTTPS, SSH, and `git` protocols.
- **Branch maintenance** – delete remote/local branches once their pull requests are closed and migrate defaults from `main` to
  `master` with safety gates.
- **GitHub Packages upkeep** – remove untagged GHCR container versions via the official API.
- **Workflow bundling** – describe ordered operations in YAML or JSON and execute them in one pass with shared discovery,
  prompting, and logging.

All repository-facing commands accept multiple root directories, honor `--dry-run` previews, and support non-interactive
confirmation via `--yes`.

## Installing and running

The CLI targets Go **1.24** or newer.

```bash
go run . --help
```

Build a reusable binary with either `go build` or the provided make target:

```bash
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

Configuration keys mirror the flags (`common.log_level`, `common.log_format`) and can also be provided via environment variables prefixed with
`GITSCRIPTS_` (for example, `GITSCRIPTS_COMMON_LOG_LEVEL=error`).

## Command catalog

### Audit reports

```bash
go run . audit --audit ~/Development
```

`audit --audit` scans each repository beneath the provided roots (defaults to the current directory) and writes a CSV summary to
stdout. Add `--debug` to log discovery progress and inspection diagnostics.

### Repository maintenance (`git-scripts repos ...`)

*Rename directories to the canonical GitHub name*

```bash
go run . repos rename-folders --dry-run ~/Development
# Apply the plan with confirmations
go run . repos rename-folders --yes ~/Development
# Require clean worktrees before renaming
go run . repos rename-folders --yes --require-clean ~/Development
```

*Update `origin` to the canonical GitHub remote*

```bash
go run . repos update-canonical-remote --dry-run ~/Development
# Automatically confirm updates
go run . repos update-canonical-remote --yes ~/Development
```

*Convert `origin` between Git protocols*

```bash
go run . repos convert-remote-protocol --from https --to git --dry-run ~/Development
# Apply conversions without interactive prompts
go run . repos convert-remote-protocol --from https --to ssh --yes ~/Development
# Operate on a single repository path
go run . repos convert-remote-protocol --from ssh --to https --yes ~/Development/project-repo
```

### Pull-request branch cleanup

```bash
go run . pr-cleanup --remote origin --limit 100 ~/Development
# Preview deletions without mutating remotes or local branches
go run . pr-cleanup --dry-run ~/Development
```

The command deletes branches whose pull requests are already closed. Provide one or more repository roots or rely on the current
working directory.

### Default-branch migration

```bash
go run . branch migrate --debug ~/Development/project-repo
```

`branch migrate` rewrites workflows, retargets GitHub Pages, pushes branch updates, and flips the default branch from `main` to
`master`. Debug logging surfaces detailed progress for each safety gate and API call.

### GitHub Packages maintenance

```bash
go run . packages purge \
  --owner my-org \
  --package my-image \
  --owner-type org \
  --token-source env:GITHUB_PACKAGES_TOKEN \
  --dry-run
```

Persist defaults in a configuration file to avoid long flag lists:

```yaml
# packages.yaml
common:
  log_level: info
  log_format: structured
tools:
  packages:
    purge:
      owner: my-org
      package: my-image
      owner_type: org
      token_source: env:GITHUB_PACKAGES_TOKEN
      page_size: 50
```

```bash
go run . --config packages.yaml packages purge --dry-run=false
```

### Workflow bundling

Define ordered steps in YAML or JSON and execute them with `workflow run`:

```yaml
# workflow.yaml
steps:
  - operation: convert-protocol
    with:
      from: https
      to: git
  - operation: update-canonical-remote
  - operation: rename-directories
    with:
      require_clean: true
  - operation: migrate-branch
    with:
      targets:
        - remote_name: origin
          source_branch: main
          target_branch: master
          push_to_remote: true
          delete_source_branch: false
  - operation: audit-report
    with:
      output: ./audit.csv
```

Run the workflow:

```bash
go run . workflow run workflow.yaml --roots ~/Development --dry-run
# Execute with confirmations suppressed
go run . workflow run workflow.yaml --roots ~/Development --yes
```

`workflow run` reuses the same repository discovery, prompting, and logging infrastructure as the standalone commands. Pass
additional roots on the command line to override the configuration file and combine `--dry-run`/`--yes` for non-interactive
execution.

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
  - Run `gh auth login` (or verify with `gh auth status`) so API calls succeed during branch cleanup and migration commands.

The packages command additionally requires network access and a GitHub Personal Access Token with `read:packages`,
`write:packages`, and `delete:packages` scopes. Export the token before invoking the command:

```bash
export GITHUB_PACKAGES_TOKEN=ghp_XXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

## Repository roots and bulk execution tips

- Provide explicit repository roots to operate on multiple directories in one invocation. When omitted, commands default to the
  current working directory.
- Use `--dry-run` to preview changes. Combine with `--yes` once you are comfortable executing the plan without prompts.
- Workflow configurations let you mix and match operations (for example, convert protocols, migrate branches, and audit) while
  sharing discovery costs.

## License

This project is licensed under the MIT License. See `LICENSE` for details.
