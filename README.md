# gix, a Git/GitHub helper CLI

[![GitHub release](https://img.shields.io/github/release/temirov/gix.svg)](https://github.com/temirov/gix/releases)

A Go-based command-line interface that automates routine Git and GitHub maintenance.

### Execution modes at a glance

You can run the CLI in two complementary ways depending on how much orchestration you need:

- **Direct commands with persisted defaults** – invoke commands such as `repo folder rename` (`r folder rename`, formerly `repo-folders-rename`),
  `repo remote update-to-canonical` (`r remote update-to-canonical`, formerly `repo-remote-update`),
  `repo packages delete` (`r packages delete`, formerly `repo-packages-purge`), and `audit` (`a`)
  from the shell, optionally loading shared flags (for example, log level or default package names) via [
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
| Direct commands  | You need a focused, ad-hoc action with minimal setup, such as renaming directories or auditing repositories             | [`repo folder rename`](#command-catalog) and [`audit`](#command-catalog) quick-starts |
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

| Command path                        | Shortcut                    | Former command         | Summary                                                       | Example                                                                                                 |
|------------------------------------|-----------------------------|------------------------|---------------------------------------------------------------|---------------------------------------------------------------------------------------------------------|
| `audit`                            | `a`                         | `audit`                | Audit and reconcile local GitHub repositories                 | `go run . a --log-level=debug --roots ~/Development --all`                                              |
| `repo folder rename`               | `r folder rename`           | `repo-folders-rename`  | Rename repository directories to match canonical GitHub names | `go run . r folder rename --yes --require-clean --owner --roots ~/Development`                          |
| `repo remote update-to-canonical`  | `r remote update-to-canonical` | `repo-remote-update`   | Update origin URLs to match canonical GitHub repositories     | `go run . r remote update-to-canonical --dry-run --owner canonical --roots ~/Development`               |
| `repo remote update-protocol`      | `r remote update-protocol`  | `repo-protocol-convert`| Convert repository origin remotes between protocols           | `go run . r remote update-protocol --from https --to ssh --yes --roots ~/Development`                   |
| `repo prs delete`                  | `r prs delete`              | `repo-prs-purge`       | Remove remote and local branches for closed pull requests     | `go run . r prs delete --remote origin --limit 100 --roots ~/Development`                               |
| `repo packages delete`             | `r packages delete`         | `repo-packages-purge`  | Delete untagged GHCR versions                                 | `go run . r packages delete --dry-run --roots ~/Development`                                            |
| `repo release`                     | `r release`                 | `repo-release`         | Annotate and push a release tag across repositories           | `go run . r release v1.2.3 --roots ~/Development`                                                      |
| `branch migrate`                   | `b migrate`                 | `branch-migrate`       | Migrate repository defaults from main to master               | `go run . b migrate --from main --to master --roots ~/Development/project-repo`                         |
| `branch refresh`                   | `b refresh`                 | `branch-refresh`       | Fetch, checkout, and pull a branch with recovery options      | `go run . b refresh --branch main --roots ~/Development/project-repo --stash`                           |
| `branch cd`                        | `b cd`                      | `branch-cd`            | Switch to a branch across repositories, creating it if needed | `go run . b cd feature/rebrand --roots ~/Development/project-repo`                                      |
| `commit message`                   | `c message`                 | `commit-message`       | Draft a Conventional Commit message from staged or worktree changes | `go run . c message --roots . --dry-run`                                                             |
| `changelog message`                | `l message`                 | `changelog-message`    | Summarize recent history into a Markdown changelog section          | `go run . l message --roots . --version v1.0.0 --since-tag v0.9.0 --dry-run`                         |
| `workflow`                         | `w`                         | `workflow`             | Run a workflow configuration file                             | `go run . w config.yaml --roots ~/Development --dry-run`                                                |

Former command names are listed for reference only; the previous hyphenated invocations have been removed and now serve solely as `operations[].operation` identifiers in configuration files.

Persist defaults and workflow plans in a single configuration file to avoid long flag lists and keep the runner in sync:

The audit command exposes `--all` to enumerate top-level folders lacking Git repositories for each root alongside canonical results, marking git-related fields as `n/a` when metadata is unavailable.

The `repo packages delete` command (formerly `repo-packages-purge`) derives the GHCR owner, owner type, and default package name from each repository's `origin` remote
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

The `repo packages delete` command automatically targets the public GitHub API. Set the
`GIX_REPO_PACKAGES_PURGE_BASE_URL` environment variable when you need to
point at a GitHub Enterprise instance during local testing.

```shell
go run . r packages delete --dry-run=false
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

#### Declarative tasks (`apply-tasks`)

Use the `apply-tasks` operation when you need to fan out file mutations, commits, and pull requests across many
repositories without bespoke scripting. Each `task` entry accepts:

- `name` (required) plus optional `ensure_clean` (default `true`) to skip dirty worktrees.
- `branch` settings (`name`, `start_point`, `push_remote`) with templated defaults that fall back to the repository's
  detected default branch and `origin`.
- `files` describing `path`, `content`, `mode` (`overwrite` or `skip-if-exists`), and `permissions` (octal, default
  `0644`).
- `commit_message` (defaults to `Apply task {{ .Task.Name }}`) and an optional `pull_request` block (`title`, `body`,
  `base`, `draft`).

Branch names are sanitized automatically so unsafe characters (spaces, slashes, punctuation) are replaced with hyphens.

All string values are rendered with Go templates and can reference `.Task`, `.Repository` (`Path`, `Name`, `FullName`,
`DefaultBranch`, `Owner`), and `.Environment` values.

```yaml
workflow:
  - step:
      operation: apply-tasks
      with:
        tasks:
          - name: Seed notes
            branch:
              name: automation-{{ .Repository.Name }}-notes
              start_point: "{{ .Repository.DefaultBranch }}"
            files:
              - path: NOTES.md
                mode: skip-if-exists
                content: |
                  # Notes for {{ .Repository.Name }}
                  Maintained by gix automation.
            commit_message: "docs: seed NOTES.md for {{ .Repository.Name }}"
            pull_request:
              title: "Seed notes for {{ .Repository.Name }}"
              body: "Generated by gix apply-tasks."
              base: "{{ .Repository.DefaultBranch }}"
```

Dry runs surface `TASK-PLAN` entries summarizing the branch, base, and per-file actions, making it easy to preview
changes before letting the executor materialize commits and pull requests.

#### Branch switch assistant (`branch cd`)

`branch cd` standardizes the routine of jumping between branches across many repositories. For each configured root, gix
fetches with `--all --prune`, switches to the requested branch, creates it from the selected remote when missing, and
finishes with `git pull --rebase`.

- Provide the target branch as the first argument (`b cd feature/new-ui`).
- When chaining commands, use the branch family `--branch` flag (`gix --branch release/v1.2 b cd`) to avoid repeating the positional argument.
- Override the tracking remote with `--remote`; otherwise the command tracks `origin`.
- Use `--dry-run` to see the planned actions without touching the repositories.

Examples:

```shell
# Switch every repository under the configured roots to release/v1.2
go run . branch cd release/v1.2 --roots ~/Development

# Create and track a topic branch from upstream instead of origin
go run . b cd bugfix/api-timeout --remote upstream --roots ~/Development/services
```

#### Release assistant (`repo release`)

`repo release` creates an annotated tag (default message `Release <tag>`) and pushes it to the configured remote for each
repository root. Use it to stamp consistent version tags across many projects in one invocation.

- Provide the tag as the first argument (`r release v1.2.3`).
- Override the default remote (`origin`) with the global `--remote` flag.
- Supply custom release notes via `--message`; otherwise the command formats one automatically.
- Combine with `--dry-run` to inspect which repositories would be tagged without mutating them.

```shell
# Create and push v1.5.0 across all configured repositories
go run . repo release v1.5.0 --roots ~/Development

# Annotate with a custom message and push to upstream instead of origin
go run . r release v1.5.0 --remote upstream --message "Release v1.5.0 (hotfix rollup)" --roots ~/Development/services
```

#### Commit message assistant (`commit message`)

Use `commit message` to turn the current repository changes into a ready-to-paste Conventional Commit draft. The
command shells out to git, composes a structured prompt, and delegates to the configured LLM provider.

- Defaults expect `OPENAI_API_KEY` to hold your token, but you can override the lookup with `api_key_env` in the
  configuration or `--api-key-env` on the command line.
- `diff_source` controls whether the generator inspects staged changes (`staged`, default) or the full working tree
  (`worktree`).
- `max_completion_tokens`, `temperature`, `model`, and `timeout_seconds` mirror the options accepted by
  `pkg/llm`—override them globally in `config.yaml` or per invocation via flags.
- Provide `--dry-run` to print the system and user prompts without contacting the model, useful for verifying the
  gathered git context.

Examples:

```shell
# Preview the prompt before sending anything to the model
go run . commit message --roots . --dry-run

# Generate a message from staged changes using a local OpenAI-compatible endpoint
OPENAI_API_KEY=sk-xxxx go run . commit message \
  --roots ~/Development/project \
  --base-url http://localhost:11434/v1 \
  --model llama3.1 \
  --diff-source staged
```

#### Changelog assistant (`changelog message`)

`changelog message` converts the commits and diffs since a chosen boundary into a Markdown changelog section that matches the format used in this repository.

- Supply `--version` (and optionally `--release-date`) to control the heading.
- Pick the comparison baseline with `--since-tag` (tag or commit) or `--since-date` (RFC3339 or `YYYY-MM-DD`). If neither flag is set, gix falls back to the most recent tag or, when no tags exist, to the repository root.
- As with `commit message`, `--dry-run` prints the system/user prompts so you can audit the gathered git context.

Examples:

```shell
# Preview the prompt used to generate release notes for the next version
go run . changelog message --roots . --version v1.0.0 --since-tag v0.9.0 --dry-run

# Produce a Markdown section for changes since a specific date using a local model endpoint
OPENAI_API_KEY=sk-xxxx go run . changelog message \
  --roots ~/Development/project \
  --version v1.0.0 \
  --release-date 2025-10-08 \
  --since-date 2025-10-01T00:00:00Z \
  --base-url http://localhost:11434/v1 \
  --model llama3.1
```

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

The `repo packages delete` command additionally requires network access and a GitHub Personal Access Token with
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
