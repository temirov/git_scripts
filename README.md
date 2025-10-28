# gix, a Git/GitHub helper CLI

[![GitHub release](https://img.shields.io/github/release/temirov/gix.svg)](https://github.com/temirov/gix/releases)

gix keeps large fleets of Git repositories in a healthy state. It bundles the day-to-day tasks every maintainer repeats: normalising folder names, aligning remotes, pruning stale branches, scrubbing GHCR images, and shipping consistent release notes.

## Highlights

- Run trusted maintenance commands across many repositories from one terminal session.
- Preview every action with `--dry-run` before touching remotes or the filesystem.
- Reuse discovery, prompting, and logging whether you call a single command or an entire workflow file.
- Lean on AI-assisted helpers for commit messages and changelog summaries when you want them.

## Quick Start

1. Install the CLI: `go install github.com/temirov/gix@latest` (Go 1.24+).
2. Explore the available commands: `gix --help`.
3. Bootstrap defaults in your workspace: `gix --init LOCAL` (or `gix --init user` for a per-user config).
4. Run a dry-run audit to confirm your environment: `gix audit --roots ~/Development --dry-run`.

## Everyday workflows

### Keep local folders canonical

```shell
gix repo folder rename --roots ~/Development --yes
```

Automatically rename each repository directory so it matches the canonical GitHub name.

### Ensure remotes point to the canonical URL

```shell
gix repo remote update-to-canonical --roots ~/Development --dry-run
```

Preview and apply remote URL fixes across every repository under one or more roots.

### Convert remote protocols in bulk

```shell
gix repo remote update-protocol --from https --to ssh --roots ~/Development --yes
```

Switch entire directory trees over to the protocol that matches your credential strategy.

### Prune branches that already merged

```shell
gix repo prs delete --roots ~/Development --limit 100
```

Delete local and remote branches whose pull requests are already closed.

### Clear out stale GHCR images

```shell
gix repo packages delete --roots ~/Development/containers --yes
```

Remove untagged GitHub Container Registry versions in one sweep.

### Generate audit CSVs for reporting

```shell
gix audit --roots ~/Development --all > audit.csv
```

Capture metadata (default branches, owners, remotes, protocol mismatches) for every repository in scope.

### Draft commit messages and changelog entries

```shell
gix commit message --repository .
gix repo changelog message --since-tag v1.2.0 --version v1.3.0
```

Use the reusable LLM client to summarise staged changes or recent history.

## Automate sequences with workflows

When you need several operations in one pass, describe them in YAML or JSON and execute them with the workflow runner:

```shell
gix workflow maintenance.yml --roots ~/Development --yes
```

Workflows reuse repository discovery, confirmation prompts, and logging so you can hand teammates a repeatable playbook.

## Shared command options

- `--roots <path>` — target one or more directories; nested repositories are ignored automatically.
- `--dry-run` — print the proposed actions without mutating anything.
- `--yes` (`-y`) — accept confirmations when you are ready to apply the plan.
- `--config path/to/config.yaml` — load persisted defaults for flags such as roots, owners, or log level.
- `--log-level`, `--log-format` — control Zap logging output (structured JSON or console).

## Configuration essentials

- `gix --init LOCAL` writes an embeddable starter `config.yaml` to the current directory; `gix --init user` places it under `$XDG_CONFIG_HOME/gix` or `$HOME/.gix`.
- Configuration precedence is: CLI flags → environment variables prefixed with `GIX_` → local config → user config.
- Default settings include log level, log format, dry-run behaviour, confirmation prompts, and reusable workflow definitions.

## Need more depth?

- Detailed architecture, package layout, and command wiring: [ARCHITECTURE.md](ARCHITECTURE.md)
- Historical roadmap and design notes: [docs/cli_design.md](docs/cli_design.md)
- Recent changes: [CHANGELOG.md](CHANGELOG.md)
